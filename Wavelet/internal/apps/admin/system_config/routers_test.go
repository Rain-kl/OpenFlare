// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package system_config

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/storage"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-gonic/gin"

	"github.com/Rain-kl/Wavelet/internal/common/response"
)

const expectedDefaultConfigsCount = 30

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

	adminGroup.POST("/system-configs", CreateSystemConfig)
	adminGroup.GET("/system-configs", ListSystemConfigs)

	systemConfigRouter := adminGroup.Group("/system-configs/:key")
	{
		systemConfigRouter.GET("", GetSystemConfig)
		systemConfigRouter.PUT("", UpdateSystemConfig)
	}

	return r
}

func TestCreateSystemConfig(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	router := setupTestRouter(adminUser)

	t.Run("create successfully", func(t *testing.T) {
		payload := CreateSystemConfigRequest{
			Key:         "custom_key",
			Value:       "custom_value",
			Type:        "system",
			Visibility:  model.ConfigVisibilityVisible,
			Description: "desc",
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/v1/admin/system-configs", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}

		// Verify database
		var cfg model.SystemConfig
		err := dbConn.Where("key = ?", "custom_key").First(&cfg).Error
		if err != nil {
			t.Fatalf("failed to find system config in DB: %v", err)
		}

		// Verify caches are invalidated after create and repopulate on read
		_, err = db.Redis.HGet(
			context.Background(),
			db.PrefixedKey(repository.SystemConfigRedisHashKey),
			"custom_key",
		).Result()
		if err == nil {
			t.Fatal("expected redis cache miss immediately after create")
		}

		loaded, err := repository.GetSystemConfigByKey(context.Background(), "custom_key")
		if err != nil {
			t.Fatalf("GetSystemConfigByKey(custom_key) error = %v", err)
		}
		if loaded.Value != "custom_value" {
			t.Errorf("GetSystemConfigByKey(custom_key).Value = %q, want %q", loaded.Value, "custom_value")
		}
		if loaded.Visibility != model.ConfigVisibilityVisible {
			t.Errorf("GetSystemConfigByKey(custom_key).Visibility = %d, want %d", loaded.Visibility, model.ConfigVisibilityVisible)
		}
	})

	t.Run("create duplicate key error", func(t *testing.T) {
		// Key "custom_key" already exists from previous test
		payload := CreateSystemConfigRequest{
			Key:         "custom_key",
			Value:       "another_value",
			Type:        "system",
			Description: "desc",
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/v1/admin/system-configs", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request on duplicate key, got %d", w.Code)
		}
	})
}

func TestListSystemConfigs(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	router := setupTestRouter(adminUser)

	t.Run("list all seeded configurations", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/system-configs", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp response.Any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		dataBytes, _ := json.Marshal(resp.Data)
		var configs []model.SystemConfig
		_ = json.Unmarshal(dataBytes, &configs)

		// Defaults seed configurations
		if len(configs) != expectedDefaultConfigsCount {
			t.Errorf("expected %d default configs, got %d", expectedDefaultConfigsCount, len(configs))
		}
	})

	t.Run("filter by type business", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/system-configs?type=business", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var resp response.Any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		dataBytes, _ := json.Marshal(resp.Data)
		var configs []model.SystemConfig
		_ = json.Unmarshal(dataBytes, &configs)

		if len(configs) != 1 || configs[0].Key != model.ConfigKeyMaxAPIKeysPerUser {
			t.Errorf("expected 1 business config (max_api_keys_per_user), got %d: %v", len(configs), configs)
		}
	})
}

func TestGetSystemConfig(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	router := setupTestRouter(adminUser)

	t.Run("get existing configuration", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/system-configs/"+model.ConfigKeySiteName, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}

		var resp response.Any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		dataBytes, _ := json.Marshal(resp.Data)
		var cfg model.SystemConfig
		_ = json.Unmarshal(dataBytes, &cfg)

		if cfg.Value != "OpenFlare" {
			t.Errorf("expected 'OpenFlare', got '%s'", cfg.Value)
		}
	})

	t.Run("get non-existent config", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/system-configs/non_existent_key", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d", w.Code)
		}
	})
}

func TestUpdateSystemConfig(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	router := setupTestRouter(adminUser)

	t.Run("update successfully", func(t *testing.T) {
		hidden := model.ConfigVisibilityHidden
		payload := UpdateSystemConfigRequest{
			Value:       "Super Site Name",
			Visibility:  &hidden,
			Description: "Updated Description",
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("PUT", "/api/v1/admin/system-configs/"+model.ConfigKeySiteName, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}

		// Verify database
		var cfg model.SystemConfig
		dbConn.Where("key = ?", model.ConfigKeySiteName).First(&cfg)
		if cfg.Value != "Super Site Name" || cfg.Description != "Updated Description" || cfg.Visibility != model.ConfigVisibilityHidden {
			t.Errorf("database values not updated: %+v", cfg)
		}

		// Verify caches are invalidated after update and repopulate on read
		_, err := db.Redis.HGet(
			context.Background(),
			db.PrefixedKey(repository.SystemConfigRedisHashKey),
			model.ConfigKeySiteName,
		).Result()
		if err == nil {
			t.Fatal("expected redis cache miss immediately after update")
		}

		loaded, err := repository.GetSystemConfigByKey(context.Background(), model.ConfigKeySiteName)
		if err != nil {
			t.Fatalf("GetSystemConfigByKey(site_name) error = %v", err)
		}
		if loaded.Value != "Super Site Name" {
			t.Errorf("GetSystemConfigByKey(site_name).Value = %q, want %q", loaded.Value, "Super Site Name")
		}
		if loaded.Visibility != model.ConfigVisibilityHidden {
			t.Errorf("GetSystemConfigByKey(site_name).Visibility = %d, want %d", loaded.Visibility, model.ConfigVisibilityHidden)
		}
	})

	t.Run("update non-existent config", func(t *testing.T) {
		payload := UpdateSystemConfigRequest{
			Value: "New Value",
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("PUT", "/api/v1/admin/system-configs/invalid_key", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d", w.Code)
		}
	})
}

func TestTestSMTP(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	r := setupTestRouter(adminUser)
	r.POST("/api/v1/admin/system-configs/smtp/test", TestSMTP)

	// Start a mock SMTP server
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock smtp server: %v", err)
	}
	defer func() { _ = l.Close() }()

	port := l.Addr().(*net.TCPAddr).Port

	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		writer := bufio.NewWriter(conn)
		reader := bufio.NewReader(conn)
		tp := textproto.NewReader(reader)

		// 220 Ready
		_, _ = writer.WriteString("220 mock.smtp.com SMTP Ready\r\n")
		_ = writer.Flush()

		// Read HELO/EHLO
		_, _ = tp.ReadLine()
		_, _ = writer.WriteString("250-mock.smtp.com\r\n250 AUTH PLAIN\r\n")
		_ = writer.Flush()

		// Read AUTH PLAIN
		_, _ = tp.ReadLine()
		_, _ = writer.WriteString("235 Authentication successful\r\n")
		_ = writer.Flush()

		// Read MAIL FROM
		_, _ = tp.ReadLine()
		_, _ = writer.WriteString("250 OK\r\n")
		_ = writer.Flush()

		// Read RCPT TO
		_, _ = tp.ReadLine()
		_, _ = writer.WriteString("250 OK\r\n")
		_ = writer.Flush()

		// Read DATA
		_, _ = tp.ReadLine()
		_, _ = writer.WriteString("354 Start mail input\r\n")
		_ = writer.Flush()

		// Read body lines until dot
		for {
			line, err := tp.ReadLine()
			if err != nil || line == "." {
				break
			}
		}
		_, _ = writer.WriteString("250 OK\r\n")
		_ = writer.Flush()

		// Read QUIT
		_, _ = tp.ReadLine()
		_, _ = writer.WriteString("221 Bye\r\n")
		_ = writer.Flush()
	}()

	payload := TestSMTPRequest{
		SMTPHost:     "127.0.0.1",
		SMTPPort:     port,
		SMTPUsername: "sender@example.com",
		SMTPPassword: "password",
		To:           "recipient@example.com",
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/v1/admin/system-configs/smtp/test", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp response.Any
	json.Unmarshal(w.Body.Bytes(), &resp)

	dataBytes, _ := json.Marshal(resp.Data)
	var testResp TestSMTPResponse
	json.Unmarshal(dataBytes, &testResp)

	if !testResp.Success {
		t.Errorf("expected test success, got failed: %s. Log: %s", testResp.Error, testResp.Log)
	}
}

func TestUpdateStorageConfigValidation(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	router := setupTestRouter(adminUser)

	t.Run("update storage config successfully", func(t *testing.T) {
		tempDir := t.TempDir()
		cfg := storage.DefaultConfig()
		cfg.Local.Root = tempDir

		cfgBytes, _ := json.Marshal(cfg)
		payload := UpdateSystemConfigRequest{
			Value: string(cfgBytes),
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("PUT", "/api/v1/admin/system-configs/storage_config", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}

		// Verify database
		var dbCfg model.SystemConfig
		dbConn.Where("key = ?", "storage_config").First(&dbCfg)
		var savedCfg storage.Config
		_ = json.Unmarshal([]byte(dbCfg.Value), &savedCfg)
		if savedCfg.Local.Root != tempDir {
			t.Errorf("expected local root to be updated to %s, got %s", tempDir, savedCfg.Local.Root)
		}
	})

	t.Run("update storage config failed connectivity check", func(t *testing.T) {
		cfg := storage.DefaultConfig()
		cfg.Driver = storage.DriverS3
		cfg.S3.Bucket = "non-existent-bucket"
		cfg.S3.Endpoint = "http://127.0.0.1:9999" // Will fail connectivity check

		cfgBytes, _ := json.Marshal(cfg)
		payload := UpdateSystemConfigRequest{
			Value: string(cfgBytes),
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("PUT", "/api/v1/admin/system-configs/storage_config", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("reject driver switch when uploads exist", func(t *testing.T) {
		upload := model.Upload{
			ID:        88001,
			UserID:    1,
			FileName:  "keep.txt",
			FilePath:  "uploads/keep.txt",
			FileSize:  4,
			MimeType:  "text/plain",
			Extension: "txt",
			Type:      "attachment",
			Status:    model.UploadStatusUsed,
		}
		if err := dbConn.Create(&upload).Error; err != nil {
			t.Fatalf("seed upload failed: %v", err)
		}

		tempDir := t.TempDir()
		cfg := storage.DefaultConfig()
		cfg.Driver = storage.DriverS3
		cfg.S3.Endpoint = "http://127.0.0.1:19998"
		cfg.S3.Region = "us-east-1"
		cfg.S3.Bucket = "wavelet"
		cfg.S3.AccessKeyID = "test"
		cfg.S3.SecretAccessKey = "test"
		cfg.Local.Root = tempDir

		cfgBytes, _ := json.Marshal(cfg)
		payload := UpdateSystemConfigRequest{Value: string(cfgBytes)}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("PUT", "/api/v1/admin/system-configs/storage_config", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d. Body: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), StorageDriverSwitchRequiresMigration) {
			t.Fatalf("expected migration-required error, got: %s", w.Body.String())
		}
	})

	t.Run("switch to local while active s3 is unreachable", func(t *testing.T) {
		if err := dbConn.Where("1 = 1").Delete(&model.Upload{}).Error; err != nil {
			t.Fatalf("clear uploads failed: %v", err)
		}

		activeCfg := storage.DefaultConfig()
		activeCfg.Driver = storage.DriverS3
		activeCfg.S3.Endpoint = "http://127.0.0.1:9999"
		activeCfg.S3.Region = "us-east-1"
		activeCfg.S3.Bucket = "wavelet"
		activeCfg.S3.AccessKeyID = "test"
		activeCfg.S3.SecretAccessKey = "test"
		activeBytes, _ := json.Marshal(activeCfg)
		seedCfg := model.SystemConfig{
			Key:   "storage_config",
			Value: string(activeBytes),
			Type:  "system",
		}
		if err := dbConn.Where("key = ?", "storage_config").
			Assign(map[string]any{"value": seedCfg.Value, "type": seedCfg.Type}).
			FirstOrCreate(&seedCfg).Error; err != nil {
			t.Fatalf("seed active storage config failed: %v", err)
		}

		tempDir := t.TempDir()
		stagedCfg := activeCfg
		stagedCfg.Driver = storage.DriverLocal
		stagedCfg.Local.Root = tempDir

		cfgBytes, _ := json.Marshal(stagedCfg)
		payload := UpdateSystemConfigRequest{Value: string(cfgBytes)}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("PUT", "/api/v1/admin/system-configs/storage_config", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}

		var dbCfg model.SystemConfig
		if err := dbConn.Where("key = ?", "storage_config").First(&dbCfg).Error; err != nil {
			t.Fatalf("load saved storage config failed: %v", err)
		}
		var savedCfg storage.Config
		if err := json.Unmarshal([]byte(dbCfg.Value), &savedCfg); err != nil {
			t.Fatalf("parse saved storage config failed: %v", err)
		}
		if savedCfg.Driver != storage.DriverLocal {
			t.Fatalf("active driver = %q, want %q after save", savedCfg.Driver, storage.DriverLocal)
		}
		if savedCfg.Local.Root != tempDir {
			t.Fatalf("staged local root = %q, want %q", savedCfg.Local.Root, tempDir)
		}
	})
}
