// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth_test

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/pkg/ginutil"
	"Wavelet/pkg/response"
	"Wavelet/plugins/domain/auth"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	testSessionCookieName = "auth-test-session"
	errUserNotInContext   = "auth: user not found in context"
)

// newTestAuthService 装配仅注册 auth 插件的 core.Context，并返回其对外契约实现。
func newTestAuthService(t *testing.T, db *gorm.DB) contracts.AuthService {
	t.Helper()

	ctx := core.NewContext(context.Background())
	if db != nil {
		core.Provide[contracts.DBService](ctx, &mockDBService{db: db})
		core.Provide[contracts.CacheService](ctx, newMockCacheService())
	}
	require.NoError(t, auth.New().Apply(ctx))

	svc, err := core.Inject[contracts.AuthService](ctx)
	require.NoError(t, err)
	auth.ResetAuthRAMCacheForTest()

	return svc
}

// newSessionEngine 构造一个带 Session 中间件的 gin 引擎，用于走通真实登录态链路。
//
// response.Abort* 只把错误挂载到 gin 错误链，状态码由全局错误中间件渲染，
// 因此这里必须同时装配 response.ErrorHandlerMiddleware()。
func newSessionEngine() *gin.Engine {
	engine := gin.New()
	engine.Use(response.ErrorHandlerMiddleware())
	engine.Use(sessions.Sessions(testSessionCookieName, cookie.NewStore([]byte("test-secret"))))
	return engine
}

func TestGetCurrentUserFromGinContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTestAuthService(t, nil)
	user := &contracts.UserDTO{ID: 4242, Username: "ctx_user", IsActive: true}

	t.Run("gin 上下文已由中间件写入用户时返回该用户", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user-info", nil)
		c.Set(contracts.AuthUserObjKey, user)

		got, err := svc.GetCurrentUser(c)
		require.NoError(t, err)
		assert.Same(t, user, got)
	})

	t.Run("开启 ContextWithFallback 时可从请求 context 回落读取", func(t *testing.T) {
		// 说明：本项目引擎默认不开启 ContextWithFallback，此时 (*gin.Context).Value
		// 等价于 c.Get，与改造前 ginutil.GetFromContext 的读取路径完全一致；
		// 开启回落后还能额外读到写入 Request.Context() 的登录态。
		reqCtx := context.WithValue(context.Background(), contracts.AuthUserObjKey, user) //nolint:staticcheck // 模拟写入请求 context 的登录态
		engine := gin.New()
		engine.ContextWithFallback = true
		var (
			gotUser *contracts.UserDTO
			gotErr  error
		)
		engine.GET("/probe", func(c *gin.Context) {
			gotUser, gotErr = svc.GetCurrentUser(c)
			c.Status(http.StatusNoContent)
		})

		engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/probe", nil).WithContext(reqCtx))
		require.NoError(t, gotErr)
		assert.Same(t, user, gotUser)
	})

	t.Run("未登录时报错且文案不变", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user-info", nil)

		got, err := svc.GetCurrentUser(c)
		require.Error(t, err)
		assert.Nil(t, got)
		assert.Equal(t, errUserNotInContext, err.Error())
	})

	t.Run("非 gin 的普通 context 仍按 Value 取值", func(t *testing.T) {
		got, err := svc.GetCurrentUser(context.WithValue(context.Background(), contracts.AuthUserObjKey, user)) //nolint:staticcheck // 与中间件写入的 key 语义一致
		require.NoError(t, err)
		assert.Same(t, user, got)

		_, err = svc.GetCurrentUser(context.Background())
		require.Error(t, err)
		assert.Equal(t, errUserNotInContext, err.Error())
	})
}

func TestGetCurrentUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTestAuthService(t, nil)

	t.Run("gin Session 中的用户 ID 可正常读取", func(t *testing.T) {
		engine := newSessionEngine()
		var (
			gotUID uint64
			gotErr error
		)
		engine.GET("/probe", func(c *gin.Context) {
			session := sessions.Default(c)
			session.Set(auth.UserIDKey, uint64(777))
			require.NoError(t, session.Save())

			gotUID, gotErr = svc.GetCurrentUserID(c)
			c.Status(http.StatusNoContent)
		})

		engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/probe", nil))
		require.NoError(t, gotErr)
		assert.Equal(t, uint64(777), gotUID)
	})

	t.Run("gin 上下文存在但 Session 无用户时返回 0 且不报错", func(t *testing.T) {
		engine := newSessionEngine()
		var (
			gotUID uint64
			gotErr error
		)
		engine.GET("/probe", func(c *gin.Context) {
			gotUID, gotErr = svc.GetCurrentUserID(c)
			c.Status(http.StatusNoContent)
		})

		engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/probe", nil))
		require.NoError(t, gotErr)
		assert.Equal(t, uint64(0), gotUID)
	})

	t.Run("非 gin context 报错且文案不变", func(t *testing.T) {
		// 即使普通 context 中已写入用户对象，该方法的 Session 语义也保持不变。
		uid, err := svc.GetCurrentUserID(
			context.WithValue(context.Background(), contracts.AuthUserObjKey, &contracts.UserDTO{ID: 1}), //nolint:staticcheck // 同上
		)
		require.Error(t, err)
		assert.Equal(t, uint64(0), uid)
		assert.Equal(t, errUserNotInContext, err.Error())
	})
}

func TestLoginRequiredMiddlewarePopulatesServiceContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	require.NoError(t, db.Create(&testUser{ID: 9001, Username: "session_user", IsActive: true}).Error)
	require.NoError(t, db.Create(&testUser{ID: 9002, Username: "token_user", IsActive: true}).Error)

	tokenStr := "integration-secret-token"
	require.NoError(t, db.Create(&testAccessToken{
		ID:        9101,
		UserID:    9002,
		TokenHash: hashToken(tokenStr),
		Name:      "integration",
		IsAdmin:   false,
	}).Error)

	svc := newTestAuthService(t, db)

	t.Run("Session 鉴权链路上 GetCurrentUser 与 GetCurrentUserID 一致", func(t *testing.T) {
		engine := newSessionEngine()
		engine.Use(func(c *gin.Context) {
			session := sessions.Default(c)
			session.Set(auth.UserIDKey, uint64(9001))
			require.NoError(t, session.Save())
			c.Next()
		})

		var (
			gotUser *contracts.UserDTO
			userErr error
			gotUID  uint64
			uidErr  error
		)
		engine.GET("/protected", auth.LoginRequired(), func(c *gin.Context) {
			gotUser, userErr = svc.GetCurrentUser(c)
			gotUID, uidErr = svc.GetCurrentUserID(c)
			c.Status(http.StatusNoContent)
		})

		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/protected", nil))
		require.Equal(t, http.StatusNoContent, recorder.Code)

		require.NoError(t, userErr)
		require.NotNil(t, gotUser)
		assert.Equal(t, uint64(9001), gotUser.ID)
		assert.Equal(t, "session_user", gotUser.Username)

		require.NoError(t, uidErr)
		assert.Equal(t, uint64(9001), gotUID)
	})

	t.Run("Access Token 鉴权链路上 GetCurrentUser 可用", func(t *testing.T) {
		engine := newSessionEngine()
		var (
			gotUser *contracts.UserDTO
			userErr error
		)
		engine.GET("/protected", auth.LoginRequired(), func(c *gin.Context) {
			gotUser, userErr = svc.GetCurrentUser(c)
			c.Status(http.StatusNoContent)
		})

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusNoContent, recorder.Code)

		require.NoError(t, userErr)
		require.NotNil(t, gotUser)
		assert.Equal(t, uint64(9002), gotUser.ID)
		assert.Equal(t, "token_user", gotUser.Username)
	})

	t.Run("未登录请求被中间件拒绝", func(t *testing.T) {
		engine := newSessionEngine()
		engine.GET("/protected", auth.LoginRequired(), func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		})

		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/protected", nil))
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	})
}

// legacyGetCurrentUser 逐字复刻改造前 Service 层的取值实现
// （*gin.Context 类型断言 + ginutil.GetFromContext + ctx.Value 回落），
// 用于与新实现做 differential 等价性校验。
func legacyGetCurrentUser(ctx context.Context) (*contracts.UserDTO, error) {
	if ginCtx, ok := ctx.(*gin.Context); ok {
		if u, ok := ginutil.GetFromContext[*contracts.UserDTO](ginCtx, contracts.AuthUserObjKey); ok && u != nil {
			return u, nil
		}
	}

	if v := ctx.Value(contracts.AuthUserObjKey); v != nil {
		if u, ok := v.(*contracts.UserDTO); ok && u != nil {
			return u, nil
		}
	}

	return nil, errors.New(errUserNotInContext)
}

// legacyGetCurrentUserID 逐字复刻改造前 Service 层基于 gin Session 的实现。
func legacyGetCurrentUserID(ctx context.Context) (uint64, error) {
	if ginCtx, ok := ctx.(*gin.Context); ok {
		return auth.GetUserIDFromContext(ginCtx), nil
	}

	return 0, errors.New(errUserNotInContext)
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// assertLoginStateParity 断言新实现与改造前实现在同一 ctx 上返回完全一致的结果与错误文案。
func assertLoginStateParity(t *testing.T, svc contracts.AuthService, ctx context.Context) {
	t.Helper()

	wantUser, wantUserErr := legacyGetCurrentUser(ctx)
	gotUser, gotUserErr := svc.GetCurrentUser(ctx)
	if (wantUser == nil) != (gotUser == nil) {
		t.Fatalf("GetCurrentUser nil-ness mismatch: want %v, got %v", wantUser, gotUser)
	}
	if wantUser != nil {
		assert.Same(t, wantUser, gotUser)
	}
	assert.Equal(t, errText(wantUserErr), errText(gotUserErr))

	wantUID, wantUIDErr := legacyGetCurrentUserID(ctx)
	gotUID, gotUIDErr := svc.GetCurrentUserID(ctx)
	assert.Equal(t, wantUID, gotUID)
	assert.Equal(t, errText(wantUIDErr), errText(gotUIDErr))
}

func TestLoginStateContextParityWithLegacyImplementation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTestAuthService(t, nil)
	user := &contracts.UserDTO{ID: 5150, Username: "parity_user", IsActive: true}

	t.Run("gin 上下文各分支", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user-info", nil)
		assertLoginStateParity(t, svc, c)

		c.Set(contracts.AuthUserObjKey, user)
		assertLoginStateParity(t, svc, c)

		c.Set(contracts.AuthUserObjKey, "not-a-user-dto")
		assertLoginStateParity(t, svc, c)

		var typedNil *contracts.UserDTO
		c.Set(contracts.AuthUserObjKey, typedNil)
		assertLoginStateParity(t, svc, c)
	})

	t.Run("普通 context 各分支", func(t *testing.T) {
		assertLoginStateParity(t, svc, context.Background())
		assertLoginStateParity(t, svc, context.WithValue(context.Background(), contracts.AuthUserObjKey, user))
		assertLoginStateParity(t, svc, context.WithValue(context.Background(), contracts.AuthUserObjKey, "nope"))
	})

	t.Run("Session 登录态各分支", func(t *testing.T) {
		cases := []struct {
			name   string
			userID any
		}{
			{name: "无用户", userID: nil},
			{name: "uint64 用户 ID", userID: uint64(3301)},
			{name: "float64 用户 ID", userID: float64(3302)},
			{name: "string 用户 ID", userID: "3303"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				engine := newSessionEngine()
				engine.GET("/probe", func(c *gin.Context) {
					if tc.userID != nil {
						session := sessions.Default(c)
						session.Set(auth.UserIDKey, tc.userID)
						require.NoError(t, session.Save())
					}
					assertLoginStateParity(t, svc, c)
				})

				engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/probe", nil))
			})
		}
	})
}

func TestAuthWhitelistMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := core.NewContext(context.Background())
	p := auth.New()
	require.NoError(t, p.Apply(ctx))

	svc, err := core.Inject[contracts.AuthService](ctx)
	require.NoError(t, err)

	mw, ok := svc.RequireAuthMiddleware().(gin.HandlerFunc)
	require.True(t, ok)

	engine := newSessionEngine()
	engine.Use(mw)
	engine.POST("/api/v1/user/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, response.OK("login-ok"))
	})
	engine.GET("/api/v1/secret-profile", func(c *gin.Context) {
		c.JSON(http.StatusOK, response.OK("profile-ok"))
	})

	// 1. Whitelisted route /api/v1/user/login passes through without auth
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodPost, "/api/v1/user/login", nil)
	engine.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// 2. Non-whitelisted route /api/v1/secret-profile gets 401 Unauthorized
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/v1/secret-profile", nil)
	engine.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)
}

func TestAdminRequiredReturnsForbiddenForLoggedInNonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.Create(&testUser{ID: 42, Username: "member", IsActive: true, IsAdmin: false}).Error)

	svc := newTestAuthService(t, db)
	mw, ok := svc.RequireAdminMiddleware().(gin.HandlerFunc)
	require.True(t, ok)

	engine := newSessionEngine()
	engine.Use(func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set(auth.UserIDKey, uint64(42))
		require.NoError(t, session.Save())
		c.Next()
	})
	engine.Use(mw)
	engine.GET("/admin-only", func(c *gin.Context) {
		c.JSON(http.StatusOK, response.OK("ok"))
	})

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin-only", nil))
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "权限不足")
}
