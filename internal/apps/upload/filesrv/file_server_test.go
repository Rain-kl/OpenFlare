// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package filesrv

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/apps/upload/cache"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/shared"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/util"
	"github.com/Rain-kl/Wavelet/internal/infra/diskcache"
	"github.com/Rain-kl/Wavelet/internal/infra/objectstore"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	appshared "github.com/Rain-kl/Wavelet/internal/shared"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func init() {
	testhelper.RegisterCleanup(cache.ResetUploadMetaCacheForTest)
}

func TestServeFileByIDAccessControl(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	cache.ResetAccessCaches()

	tempDir := t.TempDir()
	configureLocalStorageRoot(t, dbConn, tempDir)

	// Create a user in DB
	user := model.User{
		ID:       12345,
		Username: "file_test_user",
		IsActive: true,
	}
	if err := dbConn.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Create an access token for this user
	tokenStr := "test-secret-token-123"
	tokenHash := model.HashToken(tokenStr)
	tokenRecord := model.AccessToken{
		UserID:    user.ID,
		Name:      "test_token",
		TokenHash: tokenHash,
	}
	if err := dbConn.Create(&tokenRecord).Error; err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// Create two files: one in whitelist (avatar), one not in whitelist (attachment)
	avatarFile := model.Upload{
		ID:         8001,
		UserID:     user.ID,
		FileName:   "avatar.png",
		FilePath:   "avatar.png",
		FileSize:   5,
		MimeType:   "image/png",
		Extension:  "png",
		Type:       "avatar",
		Status:     model.UploadStatusUsed,
		AccessMode: 1,
	}
	attachmentFile := model.Upload{
		ID:         8002,
		UserID:     user.ID,
		FileName:   "doc.pdf",
		FilePath:   "doc.pdf",
		FileSize:   5,
		MimeType:   "application/pdf",
		Extension:  "pdf",
		Type:       "attachment",
		Status:     model.UploadStatusUsed,
		AccessMode: 1,
	}

	if err := os.WriteFile(filepath.Join(tempDir, "avatar.png"), []byte("image"), 0644); err != nil {
		t.Fatalf("failed to write avatar file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "doc.pdf"), []byte("bytes"), 0644); err != nil {
		t.Fatalf("failed to write attachment file: %v", err)
	}

	dbConn.Create(&avatarFile)
	dbConn.Create(&attachmentFile)

	// Set up router
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(response.ErrorHandlerMiddleware())
	store := cookie.NewStore([]byte("secret"))
	r.Use(sessions.Sessions("test_session", store))
	r.GET("/f/:id", ServeFileByID)

	t.Run("whitelisted file type (avatar) accessed without authentication", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/f/8001", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d. Body: %s", w.Code, w.Body.String())
		}
		if w.Body.String() != "image" {
			t.Errorf("expected 'image', got %q", w.Body.String())
		}
	})

	t.Run("non-whitelisted file type (attachment) accessed without authentication returns 401", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/f/8002", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d. Body: %s", w.Code, w.Body.String())
		}

		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		if body["error_msg"] != appshared.UnAuthorized {
			t.Errorf("expected error_msg %q, got %v", appshared.UnAuthorized, body["error_msg"])
		}
	})

	t.Run("non-whitelisted file type (attachment) accessed with valid token succeeds", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/f/8002", nil)
		req.Header.Set("X-Access-Token", tokenStr)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d. Body: %s", w.Code, w.Body.String())
		}
		if w.Body.String() != "bytes" {
			t.Errorf("expected 'bytes', got %q", w.Body.String())
		}
	})

	t.Run("accessing non-existent file returns 404", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/f/9999", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})
}

func TestImageCompression(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	tempDir := t.TempDir()
	configureLocalStorageRoot(t, dbConn, tempDir)

	cache := diskcache.GetGlobalCache()
	if err := cache.Clear(); err != nil {
		t.Fatalf("failed to clear disk cache before test: %v", err)
	}

	defer func() {
		if err := cache.Clear(); err != nil {
			t.Errorf("failed to clear disk cache after test: %v", err)
		}
	}()

	// Create test user
	user := model.User{
		ID:       555,
		Username: "compress_tester",
		IsActive: true,
	}
	dbConn.Create(&user)

	// Create a 1x1 pixel PNG image
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatalf("failed to encode test png: %v", err)
	}

	filePath := filepath.Join(tempDir, "test_image.png")
	if err := os.WriteFile(filePath, pngBuf.Bytes(), 0644); err != nil {
		t.Fatalf("failed to write test png: %v", err)
	}

	// Save upload record to DB
	uploadRecord := model.Upload{
		ID:         3001,
		UserID:     user.ID,
		FileName:   "test_image.png",
		FilePath:   "test_image.png",
		FileSize:   int64(pngBuf.Len()),
		MimeType:   "image/png",
		Extension:  "png",
		Type:       "avatar", // Whitelisted by default
		Status:     model.UploadStatusUsed,
		AccessMode: 1,
	}
	dbConn.Create(&uploadRecord)

	// Setup Router
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/f/:id", ServeFileByID)

	t.Run("serve original file without compress parameter", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/f/3001", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		// Content-Type should be image/png (default local serving type)
		if w.Header().Get("Content-Type") != "image/png" {
			t.Errorf("expected Content-Type image/png, got %s", w.Header().Get("Content-Type"))
		}
		if len(w.Body.Bytes()) != pngBuf.Len() {
			t.Errorf("expected body size %d, got %d", pngBuf.Len(), len(w.Body.Bytes()))
		}
	})

	t.Run("serve compressed WebP file with medium quality", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/f/3001?quality=medium", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}
		// Content-Type should be image/webp
		if w.Header().Get("Content-Type") != "image/webp" {
			t.Errorf("expected Content-Type image/webp, got %s", w.Header().Get("Content-Type"))
		}

		cacheKey := ImageCompressionCacheKey(&uploadRecord, shared.ImageQualityMedium)
		cachedBytes, err := cache.Get(cacheKey)
		if err != nil {
			t.Fatalf("disk cache Get(%q) returned error: %v", cacheKey, err)
		}
		if !bytes.Equal(cachedBytes, w.Body.Bytes()) {
			t.Errorf("cached compressed image differs from response")
		}

		if err := os.Remove(filePath); err != nil {
			t.Fatalf("failed to remove source image before cache-hit request: %v", err)
		}
		t.Cleanup(func() {
			if err := os.WriteFile(filePath, pngBuf.Bytes(), 0644); err != nil {
				t.Errorf("failed to restore source image: %v", err)
			}
		})

		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req)
		if w2.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w2.Code)
		}
		if !bytes.Equal(w2.Body.Bytes(), cachedBytes) {
			t.Errorf("cache-hit response differs from cached compressed image")
		}
	})

	t.Run("serve compressed WebP file and check cache headers and 304 Not Modified", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/f/3001?quality=medium", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		etag := w.Header().Get("ETag")
		if etag == "" {
			t.Error("expected ETag header, got empty")
		}

		cacheControl := w.Header().Get("Cache-Control")
		if cacheControl != "public, max-age=31536000" {
			t.Errorf("expected Cache-Control 'public, max-age=31536000', got %q", cacheControl)
		}

		// Perform conditional GET request
		reqCond, _ := http.NewRequest("GET", "/f/3001?quality=medium", nil)
		reqCond.Header.Set("If-None-Match", etag)
		wCond := httptest.NewRecorder()
		r.ServeHTTP(wCond, reqCond)

		if wCond.Code != http.StatusNotModified {
			t.Errorf("expected status 304, got %d", wCond.Code)
		}
	})

	t.Run("serve original file with origin quality", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/f/3001?quality=origin", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		if w.Header().Get("Content-Type") != "image/png" {
			t.Errorf("expected Content-Type image/png, got %s", w.Header().Get("Content-Type"))
		}
		if !bytes.Equal(w.Body.Bytes(), pngBuf.Bytes()) {
			t.Errorf("origin-quality response differs from original image")
		}
	})
}

func TestNormalizeImageQuality(t *testing.T) {
	tests := []struct {
		name    string
		quality string
		want    string
	}{
		{name: shared.ImageQualityLow, quality: shared.ImageQualityLow, want: shared.ImageQualityLow},
		{name: shared.ImageQualityMedium, quality: shared.ImageQualityMedium, want: shared.ImageQualityMedium},
		{name: shared.ImageQualityHigh, quality: shared.ImageQualityHigh, want: shared.ImageQualityHigh},
		{name: "origin", quality: "origin", want: "origin"},
		{name: "uppercase", quality: "LOW", want: shared.ImageQualityLow},
		{name: "empty", quality: "", want: "origin"},
		{name: "invalid", quality: "maximum", want: "origin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := util.NormalizeImageQuality(tt.quality); got != tt.want {
				t.Errorf("NormalizeImageQuality(%q) = %q, want %q", tt.quality, got, tt.want)
			}
		})
	}
}

func configureLocalStorageRoot(t *testing.T, dbConn *gorm.DB, tempDir string) {
	var sc model.SystemConfig
	if err := dbConn.Where("key = ?", model.ConfigKeyStorageConfig).First(&sc).Error; err != nil {
		t.Fatalf("failed to find storage config: %v", err)
	}
	var cfg objectstore.Config
	if err := json.Unmarshal([]byte(sc.Value), &cfg); err != nil {
		t.Fatalf("failed to unmarshal storage config: %v", err)
	}
	cfg.Local.Root = tempDir
	newVal, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed to marshal storage config: %v", err)
	}
	sc.Value = string(newVal)
	if err := dbConn.Save(&sc).Error; err != nil {
		t.Fatalf("failed to save storage config: %v", err)
	}
	_ = db.HSetJSON(context.Background(), repository.SystemConfigRedisHashKey, sc.Key, &sc)
	repository.ResetSystemConfigRAMCacheForTest()
	objectstore.ResetCache()
}
