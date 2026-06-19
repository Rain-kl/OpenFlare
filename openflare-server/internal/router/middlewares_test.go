// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-gonic/gin"
)

func TestCORSMiddleware(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)

	clearConfigCache := func() {
		if err := repository.InvalidateAllSystemConfigCaches(context.Background()); err != nil {
			t.Fatalf("InvalidateAllSystemConfigCaches() error = %v", err)
		}
	}

	t.Run("missing server_address configuration returns no CORS headers", func(t *testing.T) {
		clearConfigCache()
		// Ensure it's empty in DB
		if err := dbConn.Model(&model.SystemConfig{}).Where("key = ?", model.ConfigKeyServerAddress).Update("value", "").Error; err != nil {
			t.Fatalf("failed to update config: %v", err)
		}
		clearConfigCache()

		r := gin.New()
		r.Use(corsMiddleware())
		r.GET("/test", func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})

		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "http://attacker.com")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		if val := w.Header().Get("Access-Control-Allow-Origin"); val != "" {
			t.Errorf("expected empty Access-Control-Allow-Origin header, got %q", val)
		}
		if val := w.Header().Get("Access-Control-Allow-Credentials"); val != "" {
			t.Errorf("expected empty Access-Control-Allow-Credentials header, got %q", val)
		}
	})

	t.Run("matching server_address allows origin and sets credential headers", func(t *testing.T) {
		clearConfigCache()
		if err := dbConn.Model(&model.SystemConfig{}).Where("key = ?", model.ConfigKeyServerAddress).Update("value", "https://trusted.com, http://localhost:3000/").Error; err != nil {
			t.Fatalf("failed to update config: %v", err)
		}
		clearConfigCache()

		r := gin.New()
		r.Use(corsMiddleware())
		r.GET("/test", func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})

		// Test trusted origin 1
		req1, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req1.Header.Set("Origin", "https://trusted.com")
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)

		if w1.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w1.Code)
		}
		if val := w1.Header().Get("Access-Control-Allow-Origin"); val != "https://trusted.com" {
			t.Errorf("expected Access-Control-Allow-Origin 'https://trusted.com', got %q", val)
		}
		if val := w1.Header().Get("Access-Control-Allow-Credentials"); val != "true" {
			t.Errorf("expected Access-Control-Allow-Credentials 'true', got %q", val)
		}

		// Test trusted origin 2 (trimmed trailing slash)
		req2, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req2.Header.Set("Origin", "http://localhost:3000")
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)

		if w2.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w2.Code)
		}
		if val := w2.Header().Get("Access-Control-Allow-Origin"); val != "http://localhost:3000" {
			t.Errorf("expected Access-Control-Allow-Origin 'http://localhost:3000', got %q", val)
		}
	})

	t.Run("non-matching origin is denied CORS headers", func(t *testing.T) {
		clearConfigCache()
		if err := dbConn.Model(&model.SystemConfig{}).Where("key = ?", model.ConfigKeyServerAddress).Update("value", "https://trusted.com").Error; err != nil {
			t.Fatalf("failed to update config: %v", err)
		}
		clearConfigCache()

		r := gin.New()
		r.Use(corsMiddleware())
		r.GET("/test", func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})

		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "https://attacker.com")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		if val := w.Header().Get("Access-Control-Allow-Origin"); val != "" {
			t.Errorf("expected empty Access-Control-Allow-Origin header, got %q", val)
		}
	})
}
