// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package template

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

	adminGroup.GET("/templates", ListTemplates)
	adminGroup.POST("/templates", CreateTemplate)

	templateRouter := adminGroup.Group("/templates/:key")
	{
		templateRouter.GET("", GetTemplate)
		templateRouter.PUT("", UpdateTemplate)
		templateRouter.DELETE("", DeleteTemplate)
	}

	return r
}

func TestCreateTemplate(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	router := setupTestRouter(adminUser)

	t.Run("create successfully", func(t *testing.T) {
		payload := CreateTemplateRequest{
			Key:         "test_template",
			Name:        "Test Template",
			Type:        "email",
			Subject:     "Test Subject",
			Content:     "Hello {{.Name}}",
			Description: "Test Desc",
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/v1/admin/templates", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}

		var tmpl model.Template
		err := dbConn.Where("key = ?", "test_template").First(&tmpl).Error
		if err != nil {
			t.Fatalf("failed to find template in DB: %v", err)
		}
		if tmpl.Name != "Test Template" {
			t.Errorf("expected Name 'Test Template', got '%s'", tmpl.Name)
		}
	})

	t.Run("create duplicate key error", func(t *testing.T) {
		payload := CreateTemplateRequest{
			Key:         "test_template",
			Name:        "Another Name",
			Type:        "email",
			Subject:     "Another Subject",
			Content:     "Hello",
			Description: "desc",
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/v1/admin/templates", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request on duplicate key, got %d", w.Code)
		}
	})
}

func TestListTemplates(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	// Seed system templates manually for testing
	t1 := model.Template{Key: "login_email", Name: "Login Code", Type: "email", Content: "code {{.Code}}", IsSystem: true}
	t2 := model.Template{Key: "register_email", Name: "Register Code", Type: "email", Content: "code {{.Code}}", IsSystem: true}
	dbConn.Create(&t1)
	dbConn.Create(&t2)

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	router := setupTestRouter(adminUser)

	t.Run("list templates", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/templates", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp response.Any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		dataBytes, _ := json.Marshal(resp.Data)
		var templates []model.Template
		_ = json.Unmarshal(dataBytes, &templates)

		if len(templates) != 2 {
			t.Errorf("expected 2 templates, got %d", len(templates))
		}
	})
}

func TestGetTemplate(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	t1 := model.Template{Key: "login_email", Name: "Login Code", Type: "email", Content: "code {{.Code}}", IsSystem: true}
	dbConn.Create(&t1)

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	router := setupTestRouter(adminUser)

	t.Run("get existing", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/templates/login_email", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
	})

	t.Run("get non-existent", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/templates/non_existent", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d", w.Code)
		}
	})
}

func TestUpdateTemplate(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	t1 := model.Template{Key: "login_email", Name: "Login Code", Type: "email", Content: "code {{.Code}}", IsSystem: true}
	dbConn.Create(&t1)

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	router := setupTestRouter(adminUser)

	t.Run("update successfully", func(t *testing.T) {
		payload := UpdateTemplateRequest{
			Name:        "Updated Login Code",
			Type:        "email",
			Subject:     "New Subject",
			Content:     "new code {{.Code}}",
			Description: "new desc",
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("PUT", "/api/v1/admin/templates/login_email", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}

		var tmpl model.Template
		dbConn.Where("key = ?", "login_email").First(&tmpl)
		if tmpl.Name != "Updated Login Code" || tmpl.Subject != "New Subject" {
			t.Errorf("database values not updated: %+v", tmpl)
		}
	})
}

func TestDeleteTemplate(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	t1 := model.Template{Key: "login_email", Name: "Login Code", Type: "email", Content: "code {{.Code}}", IsSystem: true}
	t2 := model.Template{Key: "custom_tmpl", Name: "Custom", Type: "email", Content: "hi", IsSystem: false}
	dbConn.Create(&t1)
	dbConn.Create(&t2)

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	router := setupTestRouter(adminUser)

	t.Run("delete system template should fail", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/v1/admin/templates/login_email", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request when deleting system template, got %d", w.Code)
		}
	})

	t.Run("delete custom template should succeed", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", "/api/v1/admin/templates/custom_tmpl", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var count int64
		dbConn.Model(&model.Template{}).Where("key = ?", "custom_tmpl").Count(&count)
		if count != 0 {
			t.Error("custom template was not deleted from DB")
		}
	})
}
