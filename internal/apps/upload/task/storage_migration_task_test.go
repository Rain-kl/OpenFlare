// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/infra/objectstore"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestMigrationHandlerExecute(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	sourceRoot := t.TempDir()
	sourcePath := filepath.Join(sourceRoot, "uploads", "test.txt")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0755); err != nil {
		t.Fatalf("MkdirAll(%q) returned error: %v", sourcePath, err)
	}
	const content = "storage migration"
	if err := os.WriteFile(sourcePath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile(%q) returned error: %v", sourcePath, err)
	}

	ctx := context.Background()
	active := objectstore.DefaultConfig()
	active.Local.Root = sourceRoot
	if err := objectstore.SaveActiveConfig(ctx, active); err != nil {
		t.Fatalf("SaveActiveConfig() returned error: %v", err)
	}
	target := objectstore.DefaultConfig()
	target.Driver = objectstore.DriverS3
	target.S3 = objectstore.ObjectConfig{
		Region:          "us-east-1",
		Bucket:          "target",
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
	}
	payload, err := json.Marshal(struct {
		Target objectstore.Config `json:"target"`
	}{Target: target})
	if err != nil {
		t.Fatalf("Marshal(storageMigrationPayload) returned error: %v", err)
	}

	upload := model.Upload{
		ID:        99101,
		UserID:    1,
		FileName:  "test.txt",
		FilePath:  "uploads/test.txt",
		FileSize:  int64(len(content)),
		MimeType:  "text/plain",
		Extension: "txt",
		Hash:      "hash",
		Type:      "attachment",
		Status:    model.UploadStatusUsed,
	}
	if err := dbConn.Create(&upload).Error; err != nil {
		t.Fatalf("Create(upload) returned error: %v", err)
	}

	var copied bytes.Buffer
	restore := objectstore.MockStorage(
		func(_ context.Context, _ string, body io.Reader, _ int64, _ string) error {
			_, err := io.Copy(&copied, body)
			return err
		},
		func(context.Context, string) (*objectstore.Object, error) {
			return nil, nil
		},
		func(context.Context, string) error {
			return nil
		},
	)
	defer restore()

	result, err := (&MigrationHandler{}).Execute(ctx, payload)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() result = nil, want non-nil")
	}
	if copied.String() != content {
		t.Errorf("migrated content = %q, want %q", copied.String(), content)
	}

	var migrated model.Upload
	if err := dbConn.First(&migrated, upload.ID).Error; err != nil {
		t.Fatalf("First(upload) returned error: %v", err)
	}
	current, err := objectstore.LoadConfig(ctx)
	if err != nil {
		t.Fatalf("LoadConfig() returned error: %v", err)
	}
	if current.Driver != objectstore.DriverS3 {
		t.Errorf("active driver = %q, want %q", current.Driver, objectstore.DriverS3)
	}
}

func TestMigrationHandlerExecuteWithHashValidation(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	sourceRoot := t.TempDir()
	sourcePath := filepath.Join(sourceRoot, "uploads", "test-hash.txt")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0755); err != nil {
		t.Fatalf("MkdirAll(%q) returned error: %v", sourcePath, err)
	}
	const content = "storage migration integrity check content"
	if err := os.WriteFile(sourcePath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile(%q) returned error: %v", sourcePath, err)
	}

	// Calculate correct SHA-256 hash
	h := sha256.New()
	h.Write([]byte(content))
	correctHash := hex.EncodeToString(h.Sum(nil))

	ctx := context.Background()
	active := objectstore.DefaultConfig()
	active.Local.Root = sourceRoot
	if err := objectstore.SaveActiveConfig(ctx, active); err != nil {
		t.Fatalf("SaveActiveConfig() returned error: %v", err)
	}

	target := objectstore.DefaultConfig()
	target.Driver = objectstore.DriverS3
	target.S3 = objectstore.ObjectConfig{
		Region:          "us-east-1",
		Bucket:          "target",
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
	}
	payload, err := json.Marshal(struct {
		Target objectstore.Config `json:"target"`
	}{Target: target})
	if err != nil {
		t.Fatalf("Marshal(storageMigrationPayload) returned error: %v", err)
	}

	// Case 1: Incorrect Hash (should fail validation)
	uploadIncorrect := model.Upload{
		ID:        99102,
		UserID:    1,
		FileName:  "test-hash.txt",
		FilePath:  "uploads/test-hash.txt",
		FileSize:  int64(len(content)),
		MimeType:  "text/plain",
		Extension: "txt",
		Hash:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // Invalid hash
		Type:      "attachment",
		Status:    model.UploadStatusUsed,
	}
	if err := dbConn.Create(&uploadIncorrect).Error; err != nil {
		t.Fatalf("Create(uploadIncorrect) returned error: %v", err)
	}

	var copied bytes.Buffer
	restore := objectstore.MockStorage(
		func(_ context.Context, _ string, body io.Reader, _ int64, _ string) error {
			copied.Reset()
			_, err := io.Copy(&copied, body)
			return err
		},
		func(context.Context, string) (*objectstore.Object, error) {
			return &objectstore.Object{
				Body:          io.NopCloser(bytes.NewBuffer(copied.Bytes())),
				ContentLength: int64(copied.Len()),
				ContentType:   "text/plain",
			}, nil
		},
		func(context.Context, string) error {
			return nil
		},
	)
	defer restore()

	// Running execution with incorrect hash should fail with integrity error
	_, err = (&MigrationHandler{}).Execute(ctx, payload)
	if err == nil {
		t.Fatal("Execute() succeeded with incorrect hash, want error")
	}
	if !strings.Contains(err.Error(), "integrity check failed") {
		t.Errorf("expected integrity check failed error, got: %v", err)
	}

	// Case 2: Correct Hash (should succeed)
	if err := dbConn.Model(&model.Upload{}).Where("id = ?", uploadIncorrect.ID).Update("hash", correctHash).Error; err != nil {
		t.Fatalf("Update hash to correct value returned error: %v", err)
	}

	// Run execution with correct hash should succeed
	result, err := (&MigrationHandler{}).Execute(ctx, payload)
	if err != nil {
		t.Fatalf("Execute() with correct hash failed: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() result = nil, want non-nil")
	}

	var migrated model.Upload
	if err := dbConn.First(&migrated, uploadIncorrect.ID).Error; err != nil {
		t.Fatalf("First(upload) returned error: %v", err)
	}
	if migrated.FilePath != "uploads/test-hash.txt" {
		t.Errorf("FilePath = %q, want %q", migrated.FilePath, "uploads/test-hash.txt")
	}
}

func TestMigrationHandlerExecuteWithRedisLock(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to run miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer rdb.Close()

	oldRedis := db.Redis
	db.Redis = rdb
	defer func() {
		db.Redis = oldRedis
	}()

	ctx := context.Background()

	// Acquire lock manually
	lockKey := db.PrefixedKey("lock:storage:migrate")
	if err := rdb.Set(ctx, lockKey, "locked", time.Hour).Err(); err != nil {
		t.Fatalf("Failed to set manual lock in Redis: %v", err)
	}

	active := objectstore.DefaultConfig()
	if err := objectstore.SaveActiveConfig(ctx, active); err != nil {
		t.Fatalf("SaveActiveConfig() returned error: %v", err)
	}

	payload, err := json.Marshal(struct {
		Target objectstore.Config `json:"target"`
	}{Target: active})
	if err != nil {
		t.Fatalf("Marshal payload failed: %v", err)
	}

	// Execution should fail because lock is already acquired
	_, err = (&MigrationHandler{}).Execute(ctx, payload)
	if err == nil {
		t.Fatal("Execute() succeeded when lock was held, want error")
	}
	if !strings.Contains(err.Error(), "另一个存储迁移任务正在运行中") {
		t.Errorf("expected lock warning, got: %v", err)
	}

	// Release lock and run again, should succeed
	if err := rdb.Del(ctx, lockKey).Err(); err != nil {
		t.Fatalf("Failed to delete lock: %v", err)
	}

	_, err = (&MigrationHandler{}).Execute(ctx, payload)
	if err != nil {
		t.Fatalf("Execute() failed after lock released: %v", err)
	}
}
