// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"gorm.io/gorm"
)

func init() {
	testhelper.RegisterCleanup(func() {
		StopUploadMetaCacheListener()
		ResetUploadMetaCacheForTest()
	})
}

func seedUpload(t *testing.T, dbConn *gorm.DB, upload model.Upload) {
	t.Helper()
	if err := dbConn.Create(&upload).Error; err != nil {
		t.Fatalf("create upload: %v", err)
	}
}

func TestGetUploadByIDLoadsFromDBAndPopulatesCache(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ResetUploadMetaCacheForTest()

	ctx := context.Background()
	upload := model.Upload{
		ID:         91001,
		UserID:     1,
		FileName:   "cached.png",
		FilePath:   "cached.png",
		FileSize:   12,
		MimeType:   "image/png",
		Extension:  "png",
		Type:       "avatar",
		Status:     model.UploadStatusUsed,
		AccessMode: 1,
	}
	seedUpload(t, dbConn, upload)

	got, err := GetUploadByID(ctx, upload.ID)
	if err != nil {
		t.Fatalf("GetUploadByID: %v", err)
	}
	if got.ID != upload.ID || got.FileName != upload.FileName {
		t.Fatalf("unexpected upload: %+v", got)
	}

	var redisUpload model.Upload
	if err := db.GetJSON(ctx, uploadMetaRedisKey(upload.ID), &redisUpload); err != nil {
		t.Fatalf("redis cache miss after DB load: %v", err)
	}
	if redisUpload.ID != upload.ID {
		t.Fatalf("redis upload id mismatch: got=%d want=%d", redisUpload.ID, upload.ID)
	}

	if err := dbConn.Delete(&model.Upload{}, upload.ID).Error; err != nil {
		t.Fatalf("delete upload from db: %v", err)
	}

	gotCached, err := GetUploadByID(ctx, upload.ID)
	if err != nil {
		t.Fatalf("GetUploadByID from RAM cache: %v", err)
	}
	if gotCached.ID != upload.ID {
		t.Fatalf("expected RAM cache hit for upload %d", upload.ID)
	}
}

func TestGetUploadByIDReadsFromRedisWhenRAMEmpty(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ResetUploadMetaCacheForTest()

	ctx := context.Background()
	upload := model.Upload{
		ID:         91002,
		UserID:     1,
		FileName:   "redis.png",
		FilePath:   "redis.png",
		FileSize:   8,
		MimeType:   "image/png",
		Extension:  "png",
		Type:       "avatar",
		Status:     model.UploadStatusPending,
		AccessMode: 0,
	}
	seedUpload(t, dbConn, upload)
	SetUploadMetaCache(ctx, &upload)
	ResetUploadMetaCacheForTest()

	if err := dbConn.Delete(&model.Upload{}, upload.ID).Error; err != nil {
		t.Fatalf("delete upload from db: %v", err)
	}

	got, err := GetUploadByID(ctx, upload.ID)
	if err != nil {
		t.Fatalf("GetUploadByID from redis: %v", err)
	}
	if got.ID != upload.ID || got.FileName != upload.FileName {
		t.Fatalf("unexpected upload from redis: %+v", got)
	}
}

func TestInvalidateUploadMetaCacheClearsRAMAndRedis(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ResetUploadMetaCacheForTest()

	ctx := context.Background()
	upload := model.Upload{
		ID:         91003,
		UserID:     1,
		FileName:   "invalidate.png",
		FilePath:   "invalidate.png",
		FileSize:   4,
		MimeType:   "image/png",
		Extension:  "png",
		Type:       "avatar",
		Status:     model.UploadStatusUsed,
		AccessMode: 1,
	}
	seedUpload(t, dbConn, upload)
	SetUploadMetaCache(ctx, &upload)

	InvalidateUploadMetaCache(ctx, upload.ID)

	var redisUpload model.Upload
	if err := db.GetJSON(ctx, uploadMetaRedisKey(upload.ID), &redisUpload); err == nil {
		t.Fatal("expected redis cache to be invalidated")
	}

	got, err := GetUploadByID(ctx, upload.ID)
	if err != nil {
		t.Fatalf("GetUploadByID after invalidate should reload from DB: %v", err)
	}
	if got.ID != upload.ID {
		t.Fatalf("unexpected upload reloaded from DB: %+v", got)
	}
}

func TestUploadMetaInvalidationPubSubClearsPeerRAM(t *testing.T) {
	StopUploadMetaCacheListener()
	defer StopUploadMetaCacheListener()

	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ResetUploadMetaCacheForTest()

	ctx := context.Background()
	upload := model.Upload{
		ID:         91006,
		UserID:     1,
		FileName:   "pubsub.png",
		FilePath:   "pubsub.png",
		FileSize:   4,
		MimeType:   "image/png",
		Extension:  "png",
		Type:       "avatar",
		Status:     model.UploadStatusUsed,
		AccessMode: 1,
	}
	seedUpload(t, dbConn, upload)

	if _, err := GetUploadByID(ctx, upload.ID); err != nil {
		t.Fatalf("GetUploadByID: %v", err)
	}
	time.Sleep(50 * time.Millisecond) // allow pub/sub listener to subscribe
	if err := dbConn.Delete(&model.Upload{}, upload.ID).Error; err != nil {
		t.Fatalf("delete upload from db: %v", err)
	}
	if _, err := GetUploadByID(ctx, upload.ID); err != nil {
		t.Fatalf("expected cache hit before pub/sub invalidation: %v", err)
	}

	payload, err := json.Marshal(uploadMetaInvalidationMessage{ID: upload.ID})
	if err != nil {
		t.Fatalf("marshal invalidation payload: %v", err)
	}
	if err := db.Redis.Publish(ctx, uploadMetaInvalidationChan, string(payload)).Err(); err != nil {
		t.Fatalf("publish invalidation: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	ramCleared := false
	for time.Now().Before(deadline) {
		if _, ok := uploadMetaRAM.GetIfPresent(upload.ID); !ok {
			ramCleared = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ramCleared {
		t.Fatal("expected peer RAM cache to be cleared by pub/sub")
	}

	if err := db.Redis.Del(ctx, db.PrefixedKey(uploadMetaRedisKey(upload.ID))).Err(); err != nil {
		t.Fatalf("delete redis cache: %v", err)
	}
	if _, err := GetUploadByID(ctx, upload.ID); err == nil {
		t.Fatal("expected cache miss after pub/sub RAM eviction and redis delete")
	}
}

func TestGetUploadByIDSkipsDeletedUploads(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ResetUploadMetaCacheForTest()

	ctx := context.Background()
	upload := model.Upload{
		ID:         91004,
		UserID:     1,
		FileName:   "deleted.png",
		FilePath:   "deleted.png",
		FileSize:   4,
		MimeType:   "image/png",
		Extension:  "png",
		Type:       "avatar",
		Status:     model.UploadStatusDeleted,
		AccessMode: 1,
	}
	seedUpload(t, dbConn, upload)

	if _, err := GetUploadByID(ctx, upload.ID); err == nil {
		t.Fatal("expected error for deleted upload")
	}
}

func TestGetUploadByIDWorksWithRedisDisabled(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ResetUploadMetaCacheForTest()

	redisClient := db.Redis
	db.Redis = nil
	t.Cleanup(func() {
		db.Redis = redisClient
		StopUploadMetaCacheListener()
	})

	ctx := context.Background()
	upload := model.Upload{
		ID:         91005,
		UserID:     1,
		FileName:   "ram-only.png",
		FilePath:   "ram-only.png",
		FileSize:   6,
		MimeType:   "image/png",
		Extension:  "png",
		Type:       "avatar",
		Status:     model.UploadStatusUsed,
		AccessMode: 1,
	}
	seedUpload(t, dbConn, upload)

	got, err := GetUploadByID(ctx, upload.ID)
	if err != nil {
		t.Fatalf("GetUploadByID without redis: %v", err)
	}
	if got.ID != upload.ID {
		t.Fatalf("unexpected upload: %+v", got)
	}

	if err := dbConn.Delete(&model.Upload{}, upload.ID).Error; err != nil {
		t.Fatalf("delete upload from db: %v", err)
	}

	gotCached, err := GetUploadByID(ctx, upload.ID)
	if err != nil {
		t.Fatalf("GetUploadByID from RAM without redis: %v", err)
	}
	if gotCached.ID != upload.ID {
		t.Fatal("expected RAM cache hit when redis is disabled")
	}
}
