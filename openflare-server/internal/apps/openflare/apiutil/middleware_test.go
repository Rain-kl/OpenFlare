// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package apiutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/apps/admin"
	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/db/idgen"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAdminMiddlewareTest(t *testing.T) (*gin.Engine, *gorm.DB, func()) {
	t.Helper()

	dbConn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, dbConn.AutoMigrate(&model.User{}, &model.AccessToken{}))
	db.SetDB(dbConn)

	sessionCookieName := "test_admin_middleware_session"
	if config.Config.App.SessionCookieName != "" {
		sessionCookieName = config.Config.App.SessionCookieName
	}
	store := cookie.NewStore([]byte("test_admin_middleware_session_secret"))
	store.Options(oauth.GetSessionOptions(3600))
	engine := testhelper.NewTestGinEngine(sessions.Sessions(sessionCookieName, store))
	protected := engine.Group("/protected", AdminMiddlewares()...)
	protected.GET("", func(c *gin.Context) {
		c.JSON(http.StatusOK, response.OK(gin.H{"ok": true}))
	})

	cleanup := func() {
		db.SetDB(nil)
	}

	return engine, dbConn, cleanup
}

func seedUser(t *testing.T, dbConn *gorm.DB, username string, isAdmin bool) *model.User {
	t.Helper()

	user := &model.User{
		ID:       idgen.NextUint64ID(),
		Username: username,
		Nickname: username,
		Email:    username + "@openflare.test",
		IsActive: true,
		IsAdmin:  isAdmin,
	}
	require.NoError(t, dbConn.Create(user).Error)
	return user
}

func seedAccessToken(t *testing.T, dbConn *gorm.DB, user *model.User, isAdmin bool) string {
	t.Helper()

	token, err := model.GenerateTokenString()
	require.NoError(t, err)
	require.NoError(t, dbConn.Create(&model.AccessToken{
		UserID:      user.ID,
		Name:        user.Username + "-token",
		TokenHash:   model.HashToken(token),
		MaskedToken: model.MaskTokenString(token),
		IsAdmin:     isAdmin,
	}).Error)
	return token
}

func decodeResponse(t *testing.T, rec *httptest.ResponseRecorder) response.Any {
	t.Helper()

	var resp response.Any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp
}

func TestAdminRequiredUnauthenticated(t *testing.T) {
	engine, _, cleanup := setupAdminMiddlewareTest(t)
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	resp := decodeResponse(t, rec)
	assert.NotEmpty(t, resp.ErrorMsg)
}

func TestAdminRequiredNonAdminToken(t *testing.T) {
	engine, dbConn, cleanup := setupAdminMiddlewareTest(t)
	defer cleanup()

	user := seedUser(t, dbConn, "regular", false)
	token := seedAccessToken(t, dbConn, user, false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-Access-Token", token)
	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	resp := decodeResponse(t, rec)
	assert.Equal(t, admin.TokenAdminRequired, resp.ErrorMsg)
}

func TestAdminRequiredAdminWithoutTokenAdmin(t *testing.T) {
	engine, dbConn, cleanup := setupAdminMiddlewareTest(t)
	defer cleanup()

	user := seedUser(t, dbConn, "admin-no-token-admin", true)
	token := seedAccessToken(t, dbConn, user, false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-Access-Token", token)
	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	resp := decodeResponse(t, rec)
	assert.Equal(t, admin.TokenAdminRequired, resp.ErrorMsg)
}

func TestAdminRequiredAdminWithTokenAdmin(t *testing.T) {
	engine, dbConn, cleanup := setupAdminMiddlewareTest(t)
	defer cleanup()

	user := seedUser(t, dbConn, "admin", true)
	token := seedAccessToken(t, dbConn, user, true)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-Access-Token", token)
	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResponse(t, rec)
	assert.Empty(t, resp.ErrorMsg)
}