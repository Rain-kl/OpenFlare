// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user_test

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/pkg/response"
	"Wavelet/plugins/domain/auth"
	"Wavelet/plugins/domain/user"
	"Wavelet/plugins/infra/cache_memory"
	database "Wavelet/plugins/infra/database"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserLoginRateLimiting(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx := core.NewContext(context.Background())
	ctx.Config().SetSource(core.NewMapSource(nil))
	require.NoError(t, ctx.Config().Resolve())

	testDB := setupTestDB(t)
	require.NoError(t, database.New(database.WithDB(testDB)).Apply(ctx))
	require.NoError(t, cache_memory.New().Apply(ctx))
	require.NoError(t, auth.New().Apply(ctx))
	require.NoError(t, user.New().Apply(ctx))

	// Create a test user
	userSvc, err := core.Inject[contracts.UserService](ctx)
	require.NoError(t, err)
	createdUser, err := userSvc.CreateUser(context.Background(), contracts.CreateUserRequest{
		Username: "ratelimit_user",
		Password: "CorrectPassword123!",
	})
	require.NoError(t, err)
	require.NotNil(t, createdUser)

	router := gin.New()
	router.Use(response.ErrorHandlerMiddleware())
	store := cookie.NewStore([]byte("test-secret-key-session"))
	router.Use(sessions.Sessions("wavelet_session", store))
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(core.WithAppContext(c.Request.Context(), ctx.Root()))
		c.Next()
	})

	for _, rd := range ctx.Router().Routes() {
		handlers := make([]gin.HandlerFunc, 0, len(rd.Middlewares)+len(rd.Handlers))
		for _, m := range rd.Middlewares {
			if h, ok := m.(gin.HandlerFunc); ok {
				handlers = append(handlers, h)
			} else if fn, ok := m.(func(*gin.Context)); ok {
				handlers = append(handlers, fn)
			}
		}
		for _, raw := range rd.Handlers {
			if h, ok := raw.(gin.HandlerFunc); ok {
				handlers = append(handlers, h)
			} else if fn, ok := raw.(func(*gin.Context)); ok {
				handlers = append(handlers, fn)
			}
		}
		router.Handle(rd.Method, rd.Path, handlers...)
	}

	loginBody, _ := json.Marshal(map[string]string{
		"username": "ratelimit_user",
		"password": "WrongPassword!",
	})

	// Make 5 failed login attempts (Limit is 5)
	for i := 1; i <= 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/user/login", bytes.NewReader(loginBody))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code, "attempt %d should be 401 Unauthorized", i)
	}

	// 6th attempt from the same IP should be blocked with 429 Too Many Requests
	{
		req := httptest.NewRequest(http.MethodPost, "/api/v1/user/login", bytes.NewReader(loginBody))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code, "6th attempt should be 429 Too Many Requests")

		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "登录尝试过于频繁，请稍后重试", resp["error_msg"])
	}

	// Another IP is not blocked
	{
		req := httptest.NewRequest(http.MethodPost, "/api/v1/user/login", bytes.NewReader(loginBody))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.101:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code, "different IP should receive 401, not 429")
	}
}
