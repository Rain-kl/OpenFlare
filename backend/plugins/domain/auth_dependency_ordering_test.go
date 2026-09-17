// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"Wavelet/plugins/domain/admin"
	"Wavelet/plugins/domain/msg_gateway"
	"Wavelet/plugins/domain/user"
)

// The kernel only gates a plugin's Apply on the services it DECLARES in Inject,
// so a plugin that consumes contracts.AuthService without declaring it can be
// mounted before auth exists. Those plugins fall back to a pass-through
// middleware, which silently un-guards their routes.

func sentinelLogin(c *gin.Context)   { c.Next() }
func sentinelAdmin(c *gin.Context)   { c.Next() }
func sentinelNoToken(c *gin.Context) { c.Next() }

type stubAuthService struct{ contracts.AuthService }

func (stubAuthService) RequireAuthMiddleware() any       { return gin.HandlerFunc(sentinelLogin) }
func (stubAuthService) RequireAdminMiddleware() any      { return gin.HandlerFunc(sentinelAdmin) }
func (stubAuthService) DisallowTokenAuthMiddleware() any { return gin.HandlerFunc(sentinelNoToken) }

type stubDBService struct{ contracts.DBService }

// providerPlugin publishes a contract into the container at Apply time, the way
// the real infra and domain plugins do.
type providerPlugin struct {
	name    string
	provide func(*core.Context) error
}

func (p providerPlugin) Name() string { return p.name }

func (p providerPlugin) Apply(ctx *core.Context) error { return p.provide(ctx) }

func dbProvider() core.Plugin {
	return providerPlugin{name: "stub-database", provide: func(ctx *core.Context) error {
		core.Provide[contracts.DBService](ctx, stubDBService{})
		return nil
	}}
}

func authProvider() core.Plugin {
	return providerPlugin{name: "stub-auth", provide: func(ctx *core.Context) error {
		core.Provide[contracts.AuthService](ctx, stubAuthService{})
		return nil
	}}
}

func findRoute(routes []extpoints.RouteDefinition, method, path string) (extpoints.RouteDefinition, bool) {
	for _, rd := range routes {
		if rd.Method == method && rd.Path == path {
			return rd, true
		}
	}
	return extpoints.RouteDefinition{}, false
}

// codePointer resolves the function code pointer of a registered handler so
// identity can be compared without depending on gin internals.
func codePointer(handler any) uintptr {
	switch fn := handler.(type) {
	case gin.HandlerFunc:
		return reflect.ValueOf(fn).Pointer()
	case func(*gin.Context):
		return reflect.ValueOf(fn).Pointer()
	default:
		return 0
	}
}

func assertsMiddleware(t *testing.T, routes []extpoints.RouteDefinition, method, path string, want func(*gin.Context)) {
	t.Helper()

	rd, ok := findRoute(routes, method, path)
	if !ok {
		t.Fatalf("route %s %s was never registered", method, path)
	}
	wantPtr := codePointer(gin.HandlerFunc(want))
	for _, h := range rd.Handlers {
		if codePointer(h) == wantPtr {
			return
		}
	}
	for _, m := range rd.Middlewares {
		if codePointer(m) == wantPtr {
			return
		}
	}
	t.Errorf("route %s %s is not guarded by the auth middleware: handlers=%d middlewares=%d",
		method, path, len(rd.Handlers), len(rd.Middlewares))
}

// TestRoutesMountedBeforeAuthServiceAreGuarded 回归：cmd/app.go 把 user/message_gateway
// 注册在 auth 之前，若插件未在 Inject 中声明 contracts.AuthService，reconcile 会先
// Apply 它们，导致鉴权中间件退化为透传闭包，路由完全不受保护。
func TestRoutesMountedBeforeAuthServiceAreGuarded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := core.NewContext(context.Background())

	// Registration order mirrors cmd/app.go: the auth provider comes last.
	app := core.NewApp(
		core.WithContext(ctx),
		core.WithPlugins(
			dbProvider(),
			user.New(),
			msg_gateway.New(),
			authProvider(),
		),
	)
	require.NoError(t, app.ApplyPlugins())

	routes := ctx.Router().Routes()

	assertsMiddleware(t, routes, "POST", "/api/v1/user/change-password", sentinelLogin)
	assertsMiddleware(t, routes, "PUT", "/api/v1/user/profile", sentinelLogin)
	assertsMiddleware(t, routes, "GET", "/api/v1/user/access-tokens", sentinelLogin)
	assertsMiddleware(t, routes, "GET", "/api/v1/user/access-tokens", sentinelNoToken)
	assertsMiddleware(t, routes, "GET", "/api/v1/message-gateway/channels", sentinelLogin)
	assertsMiddleware(t, routes, "GET", "/api/v1/admin/message-gateway/channels", sentinelAdmin)
}

// TestAuthConsumersDeclareAuthDependency 是同一缺陷的架构面：声明依赖是内核排序的唯一
// 依据，漏声明会让正确性取决于注册表顺序。
func TestAuthConsumersDeclareAuthDependency(t *testing.T) {
	want := reflect.TypeFor[contracts.AuthService]()
	for _, tc := range []struct {
		name string
		deps []reflect.Type
	}{
		{"user", user.New().Inject()},
		{"msg_gateway", msg_gateway.New().Inject()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Contains(t, tc.deps, want,
				"%s resolves contracts.AuthService in Apply, so it must declare it in Inject", tc.name)
		})
	}
}

// firstGuard returns the outermost middleware registered for the first route
// matching method and path prefix — the auth guard the plugin resolved at Apply.
func firstGuard(t *testing.T, routes []extpoints.RouteDefinition, method, prefix string) gin.HandlerFunc {
	t.Helper()

	for _, rd := range routes {
		if rd.Method != method || !strings.HasPrefix(rd.Path, prefix) {
			continue
		}
		for _, candidate := range rd.Middlewares {
			if mw, ok := candidate.(gin.HandlerFunc); ok {
				return mw
			}
		}
		for _, candidate := range rd.Handlers {
			if mw, ok := candidate.(gin.HandlerFunc); ok {
				return mw
			}
		}
		t.Fatalf("route %s %s has no inspectable guard", method, rd.Path)
	}
	t.Fatalf("no route registered for %s %s*", method, prefix)
	return nil
}

// TestAuthGuardFailsClosed 回归：鉴权服务无法解析时兜底必须是拒绝。旧实现兜底为
// c.Next()，所以任何装配缺失——例如 admin 的 OnDispose 调用 service.ResetServices
// 把全局 authService 置 nil——都会让路由以“已登录”的姿态直达业务处理函数。
func TestAuthGuardFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name   string
		apply  func(*core.Context) error
		method string
		prefix string
	}{
		{"user", user.New().Apply, http.MethodPost, "/api/v1/user/change-password"},
		{"msg_gateway", msg_gateway.New().Apply, http.MethodGet, "/api/v1/message-gateway"},
		{"admin", admin.New().Apply, http.MethodGet, "/api/v1/admin"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := core.NewContext(context.Background())
			require.NoError(t, tc.apply(ctx))

			guard := firstGuard(t, ctx.Router().Routes(), tc.method, tc.prefix)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(tc.method, tc.prefix, nil)
			guard(c)

			assert.True(t, c.IsAborted(),
				"%s guard must reject the request when contracts.AuthService is unavailable", tc.name)
			assert.NotEmpty(t, c.Errors,
				"%s guard must record why the request was rejected", tc.name)
		})
	}
}
