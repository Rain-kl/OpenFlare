// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package user provides the user profile, credential management, role management, and access token domain plugin for Cordis.
package user

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"Wavelet/pkg/ginutil"
	"context"
	"embed"
	"reflect"

	"github.com/gin-gonic/gin"
)

//go:embed migrations/*/*.sql
var userMigrations embed.FS

// Option configures the user plugin.
type Option func(*Plugin)

// WithUserService sets a custom UserService implementation.
func WithUserService(svc contracts.UserService) Option {
	return func(p *Plugin) {
		p.userSvc = svc
	}
}

// Plugin implements core.Plugin to provide user account and credential domain services.
type Plugin struct {
	userSvc contracts.UserService
}

// New creates a new user domain plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// PluginName 用户插件唯一名称标识
const PluginName = "user"

// Name returns the unique identifier for the user domain plugin.
func (p *Plugin) Name() string {
	return PluginName
}

// Inject declares required dependencies for the user domain plugin.
func (p *Plugin) Inject() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[contracts.DBService](),
		// Apply resolves AuthService to build the route auth middleware. The
		// kernel only gates Apply on declared deps, so leaving this out lets
		// user mount before auth and fall back to a pass-through middleware.
		reflect.TypeFor[contracts.AuthService](),
	}
}

// Manifest returns the plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        PluginName,
		Version:     "1.0.0",
		Description: "User profiles, credentials, role management, and access token domain plugin",
		Author:      "Wavelet Team",
	}
}

// Apply registers user migrations, services, routes, tasks, schedules, and settings into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	core.Bind[contracts.DBService](ctx, SetDBService)
	core.Bind[contracts.CacheService](ctx, SetCacheService)
	core.Bind[contracts.TaskService](ctx, SetTaskService)
	core.Bind[contracts.LimiterService](ctx, SetLimiterService)
	ctx.OnDispose(func() error {
		SetDBService(nil)
		SetCacheService(nil)
		SetTaskService(nil)
		SetLimiterService(nil)
		return nil
	})

	// 0.1 Resolve auth service for middleware and cache invalidation (via IoC, not direct import)
	denyAuth := ginutil.AuthUnavailable()
	loginMW := denyAuth
	noTokenMW := denyAuth
	if authSvc, err := core.Inject[contracts.AuthService](ctx); err == nil && authSvc != nil {
		SetAuthService(authSvc)
		if mw, ok := authSvc.RequireAuthMiddleware().(gin.HandlerFunc); ok {
			loginMW = mw
		}
		if mw, ok := authSvc.DisallowTokenAuthMiddleware().(gin.HandlerFunc); ok {
			noTokenMW = mw
		}
	}
	core.Bind[contracts.AuthService](ctx, SetAuthService)
	ctx.OnDispose(func() error {
		SetAuthService(nil)
		return nil
	})

	// 1. Register migrations
	ctx.Migrations().Register("user", userMigrations)

	// 2. Initialize and provide UserService
	if p.userSvc == nil {
		p.userSvc = newUserService(ctx.Events())
	}
	core.Provide[contracts.UserService](ctx, p.userSvc)

	// CAP middleware is resolved per request. user Apply runs before cap in
	// the default plugin list; snapshotting CaptchaService here would leave
	// login/register as a permanent pass-through.
	loginCap := captchaGuard(ctx, "login")
	registerCap := captchaGuard(ctx, "register")
	emailCap := captchaGuard(ctx, "send_email_code")

	// 3. Register HTTP Routes
	userGroup := ctx.Router().Group("/api/v1/user")
	{
		userGroup.POST("/login", loginCap, Login)
		userGroup.POST("/register", registerCap, Register)
		userGroup.GET("/logout", Logout)
		userGroup.POST("/send-email-code", emailCap, SendEmailCode)
		userGroup.POST("/change-password", loginMW, ChangePassword)
		userGroup.GET("/self", loginMW, Self)
		userGroup.PUT("/profile", loginMW, UpdateProfile)

		// Access Tokens
		tokensGroup := userGroup.Group("/access-tokens", loginMW, noTokenMW)
		{
			tokensGroup.GET("", ListAccessTokens)
			tokensGroup.POST("", CreateAccessToken)
			tokensGroup.DELETE("/:id", DeleteAccessToken)
			tokensGroup.POST("/:id/rotate", RotateAccessToken)
		}
	}

	ctx.Task().Register(TaskSendEmailCode, &SendEmailCodeHandler{},
		extpoints.WithTaskMeta(SendEmailCodeMeta), extpoints.WithTaskRetry(defaultUserTaskRetry))
	ctx.Task().Register(TaskSendMail, &SendMailHandler{},
		extpoints.WithTaskMeta(SendMailMeta), extpoints.WithTaskRetry(defaultUserTaskRetry))
	ctx.Task().Register(TaskCleanupInactive, &CleanupInactiveHandler{},
		extpoints.WithTaskMeta(CleanupInactiveMeta))

	// 4.1 Register Event Listeners for domain events
	ctx.Events().On(contracts.EventTopicSystemCleanup, func(c context.Context, _ contracts.SystemCleanupEvent) error {
		handler := &CleanupInactiveHandler{}
		_, err := handler.Execute(c, nil)
		return err
	})

	// 5. Register Settings Schemas
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "user.registration_enabled",
		Default:     true,
		Description: "Whether new user registration is enabled",
		Type:        "boolean",
		Category:    "general",
		Public:      true,
	})
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "user.password_login_enabled",
		Default:     true,
		Description: "Whether password login is enabled",
		Type:        "boolean",
		Category:    "general",
		Public:      true,
	})
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "user.min_password_length",
		Default:     8,
		Description: "Minimum password length required for user accounts",
		Type:        "integer",
		Category:    "security",
	})

	return nil
}

func captchaGuard(appCtx *core.Context, scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := resolveCaptchaService(c.Request.Context(), appCtx)
		if svc == nil {
			c.Next()
			return
		}
		mw, ok := svc.VerifyMiddleware(scope).(gin.HandlerFunc)
		if !ok || mw == nil {
			c.Next()
			return
		}
		mw(c)
	}
}

func resolveCaptchaService(reqCtx context.Context, appCtx *core.Context) contracts.CaptchaService {
	if s, err := core.InjectFrom[contracts.CaptchaService](reqCtx); err == nil && s != nil {
		return s
	}
	if appCtx != nil {
		if s, err := core.Inject[contracts.CaptchaService](appCtx); err == nil && s != nil {
			return s
		}
	}
	return nil
}
