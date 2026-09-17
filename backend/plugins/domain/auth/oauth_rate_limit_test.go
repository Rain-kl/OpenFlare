// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth_test

import (
	"Wavelet/core"
	"Wavelet/pkg/response"
	"Wavelet/plugins/domain/auth"
	"Wavelet/plugins/infra/cache_memory"
	database "Wavelet/plugins/infra/database"
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

func TestOAuthRateLimiting(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx := core.NewContext(context.Background())
	ctx.Config().SetSource(core.NewMapSource(nil))
	require.NoError(t, ctx.Config().Resolve())

	testDB := setupTestDB(t)
	require.NoError(t, database.New(database.WithDB(testDB)).Apply(ctx))
	require.NoError(t, cache_memory.New().Apply(ctx))
	require.NoError(t, auth.New().Apply(ctx))

	// Create an active OIDC source
	authSrc := auth.AuthSource{
		ID:                 1,
		Name:               "google",
		Type:               "oidc",
		DisplayName:        "Google",
		ClientID:           "client-id-123",
		ClientSecret:       "client-secret-456",
		OpenIDDiscoveryURL: "https://accounts.google.com",
		IsActive:           true,
	}
	require.NoError(t, testDB.Create(&authSrc).Error)

	router := gin.New()
	router.Use(response.ErrorHandlerMiddleware())
	store := cookie.NewStore([]byte("test-session-secret-123"))
	router.Use(sessions.Sessions("wavelet_session_id", store))
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

	// 10 state slots are allowed per session (oauthStateLimitMax = 10)
	// We'll simulate 10 requests with the same cookie
	var cookies []*http.Cookie
	for i := 1; i <= 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/oauth/login?source=google", nil)
		for _, ck := range cookies {
			req.AddCookie(ck)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if len(w.Result().Cookies()) > 0 {
			cookies = w.Result().Cookies()
		}
	}

	// 11th request for the same session should be rate limited
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/oauth/login?source=google", nil)
		for _, ck := range cookies {
			req.AddCookie(ck)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "请求授权过于频繁，请稍后重试", resp["error_msg"])
	}
}
