// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/apps/admin"
	"github.com/Rain-kl/Wavelet/internal/apps/cap"
	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/option"
	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/db/idgen"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type statusPayload struct {
	SystemName string `json:"system_name"`
}

func setupAuthOptionIntegration(t *testing.T) (*gorm.DB, *gin.Engine) {
	t.Helper()

	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	t.Cleanup(cleanup)

	require.NoError(t, dbConn.AutoMigrate(&model.OpenFlareOption{}))
	option.ResetInitializationForTest()
	t.Cleanup(option.ResetInitializationForTest)

	require.NoError(t, dbConn.Model(&model.SystemConfig{}).
		Where("key = ?", model.ConfigKeyCapLoginEnabled).
		Update("value", "false").Error)
	require.NoError(t, repository.InvalidateSystemConfigCache(context.Background(), model.ConfigKeyCapLoginEnabled))
	cap.InvalidateRuntimeSettings()

	oldCookieName := config.Config.App.SessionCookieName
	oldSecret := config.Config.App.SessionSecret
	oldDomain := config.Config.App.SessionDomain
	oldSecure := config.Config.App.SessionSecure
	oldHTTPOnly := config.Config.App.SessionHTTPOnly
	t.Cleanup(func() {
		config.Config.App.SessionCookieName = oldCookieName
		config.Config.App.SessionSecret = oldSecret
		config.Config.App.SessionDomain = oldDomain
		config.Config.App.SessionSecure = oldSecure
		config.Config.App.SessionHTTPOnly = oldHTTPOnly
	})

	config.Config.App.SessionCookieName = "test_openflare_session"
	config.Config.App.SessionSecret = "test_openflare_session_secret"
	config.Config.App.SessionDomain = ""
	config.Config.App.SessionSecure = false
	config.Config.App.SessionHTTPOnly = true

	store := cookie.NewStore([]byte(config.Config.App.SessionSecret))
	store.Options(oauth.GetSessionOptions(3600))
	r := testhelper.NewTestGinEngine(sessions.Sessions(config.Config.App.SessionCookieName, store))
	mountOpenFlareTestRoutes(r)

	return dbConn, r
}

func seedUser(t *testing.T, dbConn *gorm.DB, username, password string, isAdmin bool) *model.User {
	t.Helper()

	user := &model.User{
		ID:       idgen.NextUint64ID(),
		Username: username,
		Nickname: username,
		Email:    username + "@openflare.test",
		IsActive: true,
		IsAdmin:  isAdmin,
	}
	require.NoError(t, user.SetEncryptedPassword(password))
	require.NoError(t, dbConn.Create(user).Error)
	return user
}

func seedUserWithAccessToken(t *testing.T, dbConn *gorm.DB, username, password string, isAdmin bool) string {
	t.Helper()

	user := seedUser(t, dbConn, username, password, isAdmin)

	token, err := model.GenerateTokenString()
	require.NoError(t, err)

	tokenRecord := model.AccessToken{
		UserID:      user.ID,
		Name:        username + "-integration-token",
		TokenHash:   model.HashToken(token),
		MaskedToken: model.MaskTokenString(token),
		IsAdmin:     isAdmin,
	}
	require.NoError(t, dbConn.Create(&tokenRecord).Error)
	return token
}

func TestGETStatusReturnsSuccessEnvelope(t *testing.T) {
	_, r := setupAuthOptionIntegration(t)

	w := performJSONRequest(t, r, http.MethodGet, apiPath("/status"), nil, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := requireAPIOK(t, w)

	var status statusPayload
	unmarshalAPIData(t, resp.Data, &status)
	assert.NotEmpty(t, status.SystemName)
}

func TestGETOptionRequiresAdminAuth(t *testing.T) {
	dbConn, r := setupAuthOptionIntegration(t)
	commonToken := seedUserWithAccessToken(t, dbConn, "commonuser", "password123", false)
	adminToken := seedUserWithAccessToken(t, dbConn, "adminuser", "password123", true)

	t.Run("unauthenticated", func(t *testing.T) {
		w := performJSONRequest(t, r, http.MethodGet, apiPath("/option/"), nil, nil)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		resp := decodeAPIResponse(t, w)
		assert.NotEmpty(t, resp.ErrorMsg)
	})

	t.Run("non-admin user forbidden", func(t *testing.T) {
		w := performJSONRequest(t, r, http.MethodGet, apiPath("/option/"), nil, adminAuthHeaders(commonToken))
		assert.Equal(t, http.StatusNotFound, w.Code)
		resp := decodeAPIResponse(t, w)
		assert.Equal(t, admin.TokenAdminRequired, resp.ErrorMsg)
	})

	t.Run("admin user allowed", func(t *testing.T) {
		w := performJSONRequest(t, r, http.MethodGet, apiPath("/option/"), nil, adminAuthHeaders(adminToken))
		assert.Equal(t, http.StatusOK, w.Code)
		requireAPIOK(t, w)
	})
}

func TestPOSTOptionUpdateRejectsInvalidParams(t *testing.T) {
	dbConn, r := setupAuthOptionIntegration(t)
	adminToken := seedUserWithAccessToken(t, dbConn, "adminuser", "password123", true)

	req := httptest.NewRequest(http.MethodPost, apiPath("/option/update"), bytes.NewReader([]byte("{invalid")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Access-Token", adminToken)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	resp := decodeAPIResponse(t, w)
	assert.NotEmpty(t, resp.ErrorMsg)
}

func TestGETNodesWithAccessToken(t *testing.T) {
	dbConn, r := setupAuthOptionIntegration(t)
	require.NoError(t, dbConn.AutoMigrate(&model.OpenFlareNode{}))
	adminToken := seedUserWithAccessToken(t, dbConn, "admin", "password123", true)

	w := performJSONRequest(t, r, http.MethodGet, apiPath("/nodes/"), nil, adminAuthHeaders(adminToken))

	assert.Equal(t, http.StatusOK, w.Code)
	requireAPIOK(t, w)
}

func TestOptionHotReloadAfterUpdate(t *testing.T) {
	dbConn, r := setupAuthOptionIntegration(t)
	adminToken := seedUserWithAccessToken(t, dbConn, "admin", "password123", true)

	statusBefore := getStatusSystemName(t, r, nil)
	assert.NotEmpty(t, statusBefore)

	updateResp := performJSONRequest(t, r, http.MethodPost, apiPath("/option/update"), map[string]string{
		"key":   "SystemName",
		"value": "HotReloadIntegration",
	}, adminAuthHeaders(adminToken))
	assert.Equal(t, http.StatusOK, updateResp.Code)
	requireAPIOK(t, updateResp)

	statusAfter := getStatusSystemName(t, r, nil)
	assert.Equal(t, "HotReloadIntegration", statusAfter)
	assert.Equal(t, "HotReloadIntegration", model.SystemName)

	ctx := context.Background()
	require.NoError(t, option.EnsureInitialized(ctx))
	assert.Equal(t, "HotReloadIntegration", model.OptionValue("SystemName"))
}

func getStatusSystemName(t *testing.T, r http.Handler, headers map[string]string) string {
	t.Helper()

	w := performJSONRequest(t, r, http.MethodGet, apiPath("/status"), nil, headers)
	require.Equal(t, http.StatusOK, w.Code)
	resp := requireAPIOK(t, w)

	var status statusPayload
	unmarshalAPIData(t, resp.Data, &status)
	return status.SystemName
}