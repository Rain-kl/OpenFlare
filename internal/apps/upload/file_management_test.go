// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package upload

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-gonic/gin"
)

func TestGetDistinctUploadTypes(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	user := model.User{ID: 2222, Username: "test_user_2"}
	dbConn.Create(&user)

	customUpload := model.Upload{
		ID:        9001,
		UserID:    user.ID,
		FileName:  "custom.txt",
		FilePath:  "uploads/custom.txt",
		FileSize:  10,
		MimeType:  "text/plain",
		Extension: "txt",
		Type:      "custom_type_xyz",
		Status:    model.UploadStatusUsed,
	}
	dbConn.Create(&customUpload)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/admin/uploads/types", GetDistinctUploadTypes)

	req, _ := http.NewRequest("GET", "/api/v1/admin/uploads/types", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		ErrorMsg string   `json:"error_msg"`
		Data     []string `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if resp.ErrorMsg != "" {
		t.Fatalf("unexpected error: %s", resp.ErrorMsg)
	}

	if len(resp.Data) != 1 || resp.Data[0] != "custom_type_xyz" {
		t.Errorf("expected only custom_type_xyz in types list, got: %v", resp.Data)
	}
}
