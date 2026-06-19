// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth_source

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-gonic/gin"

	"github.com/Rain-kl/Wavelet/internal/common/response"
)

func setupTestRouter(authUser *model.User) *gin.Engine {
	r := testhelper.NewTestGinEngine()
	adminGroup := r.Group("/api/v1/admin")

	// Mock authentication middleware
	adminGroup.Use(func(c *gin.Context) {
		if authUser != nil {
			oauth.SetToContext(c, oauth.UserObjKey, authUser)
		}
		c.Next()
	})

	adminGroup.GET("/auth-sources", ListAuthSources)
	adminGroup.POST("/auth-sources", CreateAuthSource)
	adminGroup.PUT("/auth-sources/:id", UpdateAuthSource)
	adminGroup.PUT("/auth-sources/:id/toggle", ToggleAuthSource)
	adminGroup.DELETE("/auth-sources/:id", DeleteAuthSource)
	return r
}

func TestListAuthSources(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	// Seed source
	source := model.AuthSource{
		ID:                 1,
		Name:               "google",
		Type:               "oidc",
		DisplayName:        "Google Auth",
		IsActive:           true,
		ClientID:           "client_id_123",
		ClientSecret:       "client_secret_456",
		OpenIDDiscoveryURL: "https://accounts.google.com",
	}
	dbConn.Create(&source)

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	router := setupTestRouter(adminUser)

	req, _ := http.NewRequest("GET", "/api/v1/admin/auth-sources", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}

	var resp response.Any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	dataBytes, _ := json.Marshal(resp.Data)
	var sources []model.AuthSource
	_ = json.Unmarshal(dataBytes, &sources)

	if len(sources) != 1 {
		t.Errorf("expected 1 auth source, got %d", len(sources))
	}
	if sources[0].Name != "google" {
		t.Errorf("expected name 'google', got '%s'", sources[0].Name)
	}
	// Verify sanitize removed the secret
	if sources[0].ClientSecret != "" {
		t.Error("client secret should be sanitized")
	}
	if !sources[0].ClientSecretConfigured {
		t.Error("client secret configured flag should be true")
	}
}

func TestCreateAuthSource(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	router := setupTestRouter(adminUser)

	t.Run("create successfully", func(t *testing.T) {
		reqPayload := AuthSourceRequest{
			Name:               "github",
			Type:               "oidc",
			DisplayName:        "GitHub OIDC",
			IsActive:           true,
			ClientID:           "client_id_gh",
			ClientSecret:       "client_secret_gh",
			OpenIDDiscoveryURL: "https://github.com",
		}
		body, _ := json.Marshal(reqPayload)
		req, _ := http.NewRequest("POST", "/api/v1/admin/auth-sources", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}

		// Verify database
		var src model.AuthSource
		dbConn.Where("name = ?", "github").First(&src)
		if src.ClientID != "client_id_gh" {
			t.Errorf("expected client_id_gh, got '%s'", src.ClientID)
		}
	})

	t.Run("create invalid validation failure", func(t *testing.T) {
		reqPayload := AuthSourceRequest{
			Name:               "invalid name!",
			Type:               "oidc",
			DisplayName:        "Invalid",
			IsActive:           true,
			ClientID:           "client_id_val",
			ClientSecret:       "client_secret_val",
			OpenIDDiscoveryURL: "https://discovery.url",
		}
		body, _ := json.Marshal(reqPayload)
		req, _ := http.NewRequest("POST", "/api/v1/admin/auth-sources", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}

func TestUpdateAuthSource(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	// Seed source
	source := model.AuthSource{
		ID:                 1,
		Name:               "microsoft",
		Type:               "oidc",
		DisplayName:        "Microsoft",
		IsActive:           true,
		ClientID:           "old_client_id",
		ClientSecret:       "old_secret",
		OpenIDDiscoveryURL: "https://login.microsoftonline.com",
	}
	dbConn.Create(&source)

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	router := setupTestRouter(adminUser)

	t.Run("update keep client secret", func(t *testing.T) {
		reqPayload := AuthSourceRequest{
			Name:               "microsoft",
			Type:               "oidc",
			DisplayName:        "Microsoft Updated",
			IsActive:           true,
			ClientID:           "new_client_id",
			ClientSecret:       "", // empty implies keeping existing secret
			OpenIDDiscoveryURL: "https://login.microsoftonline.com",
		}
		body, _ := json.Marshal(reqPayload)
		req, _ := http.NewRequest("PUT", "/api/v1/admin/auth-sources/1", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}

		var src model.AuthSource
		dbConn.First(&src, 1)
		if src.DisplayName != "Microsoft Updated" {
			t.Errorf("expected display name update, got '%s'", src.DisplayName)
		}
		if src.ClientSecret != "old_secret" {
			t.Errorf("expected old secret to be preserved, got '%s'", src.ClientSecret)
		}
	})

	t.Run("update new client secret", func(t *testing.T) {
		reqPayload := AuthSourceRequest{
			Name:               "microsoft",
			Type:               "oidc",
			DisplayName:        "Microsoft Updated Again",
			IsActive:           true,
			ClientID:           "new_client_id",
			ClientSecret:       "brand_new_secret",
			OpenIDDiscoveryURL: "https://login.microsoftonline.com",
		}
		body, _ := json.Marshal(reqPayload)
		req, _ := http.NewRequest("PUT", "/api/v1/admin/auth-sources/microsoft", bytes.NewBuffer(body)) // Using Name instead of ID
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}

		var src model.AuthSource
		dbConn.First(&src, 1)
		if src.ClientSecret != "brand_new_secret" {
			t.Errorf("expected secret update, got '%s'", src.ClientSecret)
		}
	})
}

func TestToggleAuthSource(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	source := model.AuthSource{
		ID:                 1,
		Name:               "test_source",
		Type:               "oidc",
		DisplayName:        "Test Source",
		IsActive:           false,
		ClientID:           "",
		ClientSecret:       "",
		OpenIDDiscoveryURL: "https://test.discovery.url",
	}
	dbConn.Create(&source)

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	router := setupTestRouter(adminUser)

	t.Run("cannot activate without credentials", func(t *testing.T) {
		payload := ToggleAuthSourceRequest{IsActive: true}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("PUT", "/api/v1/admin/auth-sources/1/toggle", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request when activating without client_id/secret, got %d", w.Code)
		}
	})

	t.Run("toggle success after setting credentials", func(t *testing.T) {
		// Set credentials first
		dbConn.Model(&model.AuthSource{}).Where("id = ?", 1).Updates(map[string]interface{}{
			"client_id":     "id",
			"client_secret": "secret",
		})

		payload := ToggleAuthSourceRequest{IsActive: true}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("PUT", "/api/v1/admin/auth-sources/1/toggle", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}

		var src model.AuthSource
		dbConn.First(&src, 1)
		if !src.IsActive {
			t.Error("auth source should be activated")
		}
	})
}

func TestDeleteAuthSource(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	source := model.AuthSource{
		ID:                 1,
		Name:               "delete_me",
		Type:               "oidc",
		DisplayName:        "Delete Me",
		IsActive:           true,
		ClientID:           "id",
		ClientSecret:       "secret",
		OpenIDDiscoveryURL: "https://delete.me",
	}
	dbConn.Create(&source)

	externalAccount := model.ExternalAccount{
		ID:           10,
		AuthSourceID: 1,
		UserID:       50,
		ExternalID:   "ext_50",
	}
	dbConn.Create(&externalAccount)

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	router := setupTestRouter(adminUser)

	req, _ := http.NewRequest("DELETE", "/api/v1/admin/auth-sources/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}

	// Verify AuthSource is deleted
	var srcCount int64
	dbConn.Model(&model.AuthSource{}).Where("id = ?", 1).Count(&srcCount)
	if srcCount != 0 {
		t.Error("AuthSource should be deleted from the database")
	}

	// Verify ExternalAccount bindings are also deleted
	var extCount int64
	dbConn.Model(&model.ExternalAccount{}).Where("auth_source_id = ?", 1).Count(&extCount)
	if extCount != 0 {
		t.Error("related ExternalAccount bindings should be deleted")
	}
}
