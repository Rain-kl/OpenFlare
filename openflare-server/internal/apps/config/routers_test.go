// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-gonic/gin"

	"github.com/Rain-kl/Wavelet/internal/common/response"
)

func TestGetPublicConfigUsesVisibility(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	if err := dbConn.Create(&model.SystemConfig{
		Key:         "custom_public_key",
		Value:       "custom_public_value",
		Type:        "system",
		Visibility:  model.ConfigVisibilityVisible,
		Description: "custom public config",
	}).Error; err != nil {
		t.Fatalf("Create(custom_public_key) error = %v", err)
	}
	if err := dbConn.Model(&model.SystemConfig{}).
		Where("key = ?", model.ConfigKeySiteName).
		Update("visibility", model.ConfigVisibilityHidden).Error; err != nil {
		t.Fatalf("Update(%s.visibility) error = %v", model.ConfigKeySiteName, err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/config/public", GetPublicConfig)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/public", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GetPublicConfig() status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp response.Any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal(GetPublicConfig()) error = %v", err)
	}
	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("json.Marshal(GetPublicConfig().data) error = %v", err)
	}
	var configs map[string]string
	if err := json.Unmarshal(dataBytes, &configs); err != nil {
		t.Fatalf("json.Unmarshal(GetPublicConfig().data) error = %v", err)
	}

	if got := configs["custom_public_key"]; got != "custom_public_value" {
		t.Errorf("GetPublicConfig()[custom_public_key] = %q, want %q", got, "custom_public_value")
	}
	if _, ok := configs[model.ConfigKeySiteName]; ok {
		t.Errorf("GetPublicConfig()[%s] is present, want hidden", model.ConfigKeySiteName)
	}
}
