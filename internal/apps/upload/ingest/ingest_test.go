// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package ingest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	uploadcache "github.com/Rain-kl/Wavelet/internal/apps/upload/cache"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/shared"
	"github.com/Rain-kl/Wavelet/internal/infra/objectstore"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"gorm.io/gorm"
)

func TestIngestPolicyCreateIncrementsStats(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	content := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01")
	hash := sha256.Sum256(content)

	restoreStorage, disableStorage := setupMockStorage(t, nil)
	defer restoreStorage()
	defer disableStorage()

	result, err := Ingest(ctx, Request{
		UserID:    1001,
		Reader:    bytes.NewReader(content),
		Size:      int64(len(content)),
		FileName:  "mirror.png",
		MimeType:  "image/png",
		Extension: "png",
		Hash:      hex.EncodeToString(hash[:]),
		Type:      "pixez_mirror",
		Policy:    PolicyCreate,
	})
	if err != nil {
		t.Fatalf("Ingest(PolicyCreate) returned error: %v", err)
	}
	if !result.Created || !result.Stored || result.Resolved {
		t.Fatalf("Ingest(PolicyCreate) = %+v, want Created+Stored without Resolved", result)
	}

	stats, err := loadTotalStats(ctx)
	if err != nil {
		t.Fatalf("loadTotalStats returned error: %v", err)
	}
	if stats.TotalCount != 1 || stats.TotalSize != int64(len(content)) {
		t.Fatalf("loadTotalStats() = count %d size %d, want count 1 size %d", stats.TotalCount, stats.TotalSize, len(content))
	}
}

func TestIngestPolicyResolveExistingSkipsStatsOnHit(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	content := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01")
	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:])

	existing := model.Upload{
		ID:        88001,
		UserID:    42,
		FileName:  "existing.png",
		FilePath:  "uploads/existing.png",
		FileSize:  int64(len(content)),
		MimeType:  "image/png",
		Extension: "png",
		Hash:      hashStr,
		Type:      "pixez_mirror",
		Status:    model.UploadStatusUsed,
		CreatedAt: time.Now(),
	}
	if err := dbConn.Create(&existing).Error; err != nil {
		t.Fatalf("seed upload failed: %v", err)
	}

	restoreStorage, disableStorage := setupMockStorage(t, nil)
	defer restoreStorage()
	defer disableStorage()

	result, err := Ingest(ctx, Request{
		UserID:    1001,
		Reader:    bytes.NewReader(content),
		Size:      int64(len(content)),
		FileName:  "mirror.png",
		MimeType:  "image/png",
		Extension: "png",
		Hash:      hashStr,
		Type:      "pixez_mirror",
		Policy:    PolicyResolveExisting,
	})
	if err != nil {
		t.Fatalf("Ingest(PolicyResolveExisting) returned error: %v", err)
	}
	if !result.Resolved || result.Created || result.Stored {
		t.Fatalf("Ingest(PolicyResolveExisting) = %+v, want Resolved only", result)
	}
	if result.Upload.ID != existing.ID {
		t.Fatalf("Ingest(PolicyResolveExisting).Upload.ID = %d, want %d", result.Upload.ID, existing.ID)
	}

	stats, err := loadTotalStats(ctx)
	if err != nil {
		t.Fatalf("loadTotalStats returned error: %v", err)
	}
	if stats.TotalCount != 0 || stats.TotalSize != 0 {
		t.Fatalf("loadTotalStats() = count %d size %d, want zero stats for resolved upload", stats.TotalCount, stats.TotalSize)
	}
}

func TestIngestPolicyDedupNewRecordCreatesSecondRecord(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	content := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01")
	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:])
	putCount := 0

	restoreStorage, disableStorage := setupMockStorage(t, &putCount)
	defer restoreStorage()
	defer disableStorage()

	first, err := Ingest(ctx, Request{
		UserID:    1001,
		Reader:    bytes.NewReader(content),
		Size:      int64(len(content)),
		FileName:  "first.png",
		MimeType:  "image/png",
		Extension: "png",
		Hash:      hashStr,
		Type:      "avatar",
		Metadata: model.UploadMetadata{
			UserAgent: "first-agent",
			Extra:     map[string]any{"record": "first"},
		},
		Policy: PolicyDedupNewRecord,
	})
	if err != nil {
		t.Fatalf("first Ingest returned error: %v", err)
	}
	if putCount != 1 {
		t.Fatalf("putCount after first ingest = %d, want 1", putCount)
	}
	first.Upload.Metadata.Bucket = "shared-bucket"
	if err := dbConn.Save(&first.Upload).Error; err != nil {
		t.Fatalf("update first upload metadata failed: %v", err)
	}

	second, err := Ingest(ctx, Request{
		UserID:    1002,
		Reader:    bytes.NewReader(content),
		Size:      int64(len(content)),
		FileName:  "second.png",
		MimeType:  "image/png",
		Extension: "png",
		Hash:      hashStr,
		Type:      "avatar",
		Metadata: model.UploadMetadata{
			UserAgent: "second-agent",
			Bucket:    "caller-bucket-must-not-survive",
			Extra:     map[string]any{"record": "second"},
		},
		Policy: PolicyDedupNewRecord,
	})
	if err != nil {
		t.Fatalf("second Ingest returned error: %v", err)
	}
	if putCount != 1 {
		t.Fatalf("putCount after dedup ingest = %d, want 1", putCount)
	}
	if first.Upload.FilePath != second.Upload.FilePath {
		t.Fatalf("dedup file paths differ: %s vs %s", first.Upload.FilePath, second.Upload.FilePath)
	}
	if first.Upload.ID == second.Upload.ID {
		t.Fatal("dedup records should have unique IDs")
	}
	if second.Upload.Metadata.Bucket != "shared-bucket" {
		t.Fatalf("dedup bucket = %q, want inherited shared-bucket", second.Upload.Metadata.Bucket)
	}
	if second.Upload.Metadata.UserAgent != "second-agent" {
		t.Fatalf("dedup user agent = %q, want caller metadata", second.Upload.Metadata.UserAgent)
	}
	if second.Upload.Metadata.Extra["record"] != "second" {
		t.Fatalf("dedup extra metadata = %#v, want caller metadata", second.Upload.Metadata.Extra)
	}

	var count int64
	if err := dbConn.Model(&model.Upload{}).Where("hash = ?", hashStr).Count(&count).Error; err != nil {
		t.Fatalf("count uploads failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("upload count = %d, want 2", count)
	}
}

func TestDedupRecordFailureDoesNotDeleteSharedObject(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	content := []byte("\x89PNG\r\n\x1a\nshared-object")
	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:])
	deleteCount := 0
	restoreStorage, disableStorage := setupMockStorageWithDeleteCount(t, nil, &deleteCount)
	defer restoreStorage()
	defer disableStorage()

	first, err := Ingest(ctx, Request{
		UserID:    1001,
		Reader:    bytes.NewReader(content),
		Size:      int64(len(content)),
		FileName:  "shared.png",
		MimeType:  "image/png",
		Extension: "png",
		Hash:      hashStr,
		Type:      "avatar",
		Policy:    PolicyDedupNewRecord,
	})
	if err != nil {
		t.Fatalf("first Ingest returned error: %v", err)
	}

	const callbackName = "test:reject_dedup_upload_record"
	if err := dbConn.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		upload, ok := tx.Statement.Dest.(*model.Upload)
		if ok && upload.FileName == "dedup-fail.png" {
			tx.AddError(errors.New("injected upload create failure"))
		}
	}); err != nil {
		t.Fatalf("register create failure callback: %v", err)
	}
	defer func() { _ = dbConn.Callback().Create().Remove(callbackName) }()

	_, err = Ingest(ctx, Request{
		UserID:    1002,
		Reader:    bytes.NewReader(content),
		Size:      int64(len(content)),
		FileName:  "dedup-fail.png",
		MimeType:  "image/png",
		Extension: "png",
		Hash:      hashStr,
		Type:      "avatar",
		Metadata: model.UploadMetadata{
			Extra: map[string]any{"record": "dedup-failure"},
		},
		Policy: PolicyDedupNewRecord,
	})
	if err == nil {
		t.Fatal("dedup Ingest expected injected persistence error")
	}
	if deleteCount != 0 {
		t.Fatalf("shared object delete count = %d, want 0", deleteCount)
	}

	_, backend, err := objectstore.Active(ctx)
	if err != nil {
		t.Fatalf("load active storage: %v", err)
	}
	obj, err := backend.Get(ctx, first.Upload.FilePath)
	if err != nil {
		t.Fatalf("shared object became unreadable after dedup failure: %v", err)
	}
	_ = obj.Body.Close()
}

func TestCreateUploadWithStatsRollsBackOnCreateFailure(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	existing := model.Upload{
		ID:        99001,
		UserID:    1001,
		FileName:  "existing.png",
		FilePath:  "uploads/existing.png",
		FileSize:  64,
		MimeType:  "image/png",
		Extension: "png",
		Type:      "generic",
		Status:    model.UploadStatusUsed,
		CreatedAt: time.Now(),
	}
	if err := dbConn.Create(&existing).Error; err != nil {
		t.Fatalf("seed upload failed: %v", err)
	}

	duplicate := &model.Upload{
		ID:        existing.ID,
		UserID:    1002,
		FileName:  "duplicate.png",
		FilePath:  "uploads/duplicate.png",
		FileSize:  128,
		MimeType:  "image/png",
		Extension: "png",
		Type:      "generic",
		Status:    model.UploadStatusUsed,
		CreatedAt: time.Now(),
	}
	if err := createUploadWithStats(ctx, duplicate); err == nil {
		t.Fatal("createUploadWithStats with duplicate ID expected error")
	}

	stats, err := loadTotalStats(ctx)
	if err != nil {
		t.Fatalf("loadTotalStats returned error: %v", err)
	}
	if stats.TotalCount != 0 || stats.TotalSize != 0 {
		t.Fatalf("loadTotalStats() = count %d size %d, want zero after rolled-back stats", stats.TotalCount, stats.TotalSize)
	}
}

func TestRemoveDecrementsStats(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	content := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01")
	hash := sha256.Sum256(content)

	restoreStorage, disableStorage := setupMockStorage(t, nil)
	defer restoreStorage()
	defer disableStorage()

	result, err := Ingest(ctx, Request{
		UserID:    1001,
		Reader:    bytes.NewReader(content),
		Size:      int64(len(content)),
		FileName:  "delete-me.png",
		MimeType:  "image/png",
		Extension: "png",
		Hash:      hex.EncodeToString(hash[:]),
		Type:      "generic",
		Policy:    PolicyCreate,
	})
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}

	if _, err := Remove(ctx, result.Upload.ID); err != nil {
		t.Fatalf("Remove(%d) returned error: %v", result.Upload.ID, err)
	}
	stale := result.Upload
	uploadcache.SetUploadMetaCache(ctx, &stale)
	removedAgain, err := Remove(ctx, result.Upload.ID)
	if err != nil {
		t.Fatalf("second Remove(%d) returned error: %v", result.Upload.ID, err)
	}
	if removedAgain.Status != model.UploadStatusDeleted {
		t.Fatalf("second Remove status = %s, want deleted", removedAgain.Status)
	}
	if _, err := uploadcache.GetUploadByID(ctx, result.Upload.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cache lookup after idempotent Remove error = %v, want record not found", err)
	}

	stats, err := loadTotalStats(ctx)
	if err != nil {
		t.Fatalf("loadTotalStats returned error: %v", err)
	}
	if stats.TotalCount != 0 || stats.TotalSize != 0 {
		t.Fatalf("loadTotalStats() after remove = count %d size %d, want zero", stats.TotalCount, stats.TotalSize)
	}
}

func TestConcurrentRemoveDecrementsStatsOnce(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	content := []byte("\x89PNG\r\n\x1a\nconcurrent-remove")
	hash := sha256.Sum256(content)
	restoreStorage, disableStorage := setupMockStorage(t, nil)
	defer restoreStorage()
	defer disableStorage()

	result, err := Ingest(ctx, Request{
		UserID:    1001,
		Reader:    bytes.NewReader(content),
		Size:      int64(len(content)),
		FileName:  "concurrent.png",
		MimeType:  "image/png",
		Extension: "png",
		Hash:      hex.EncodeToString(hash[:]),
		Type:      "generic",
		Policy:    PolicyCreate,
	})
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}

	const workers = 8
	start := make(chan struct{})
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, removeErr := Remove(ctx, result.Upload.ID)
			errs <- removeErr
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for removeErr := range errs {
		if removeErr != nil {
			t.Fatalf("concurrent Remove returned error: %v", removeErr)
		}
	}

	stats, err := loadTotalStats(ctx)
	if err != nil {
		t.Fatalf("loadTotalStats returned error: %v", err)
	}
	if stats.TotalCount != 0 || stats.TotalSize != 0 {
		t.Fatalf("stats after concurrent remove = count %d size %d, want zero", stats.TotalCount, stats.TotalSize)
	}
}

func TestRemoveOwnedAndReservedTypeBoundaries(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	ordinary := model.Upload{
		ID:        99101,
		UserID:    1001,
		FileName:  "owned.txt",
		FilePath:  "uploads/owned.txt",
		FileSize:  16,
		MimeType:  "text/plain",
		Extension: "txt",
		Hash:      "owned-hash",
		Type:      "generic",
		Status:    model.UploadStatusUsed,
		CreatedAt: time.Now(),
	}
	reserved := model.Upload{
		ID:        99102,
		UserID:    1001,
		FileName:  "pages.zip",
		FilePath:  "uploads/pages.zip",
		FileSize:  32,
		MimeType:  "application/zip",
		Extension: "zip",
		Hash:      "reserved-hash",
		Type:      shared.ReservedPagesDeploymentType,
		Status:    model.UploadStatusUsed,
		CreatedAt: time.Now(),
	}
	if err := dbConn.Create(&ordinary).Error; err != nil {
		t.Fatalf("seed ordinary upload: %v", err)
	}
	if err := dbConn.Create(&reserved).Error; err != nil {
		t.Fatalf("seed reserved upload: %v", err)
	}

	if _, err := RemoveOwned(ctx, 2002, ordinary.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("RemoveOwned non-owner error = %v, want ErrForbidden", err)
	}
	if _, err := Remove(ctx, reserved.ID); !errors.Is(err, ErrReservedUploadType) {
		t.Fatalf("Remove reserved error = %v, want ErrReservedUploadType", err)
	}
	if _, err := RemoveOwned(ctx, reserved.UserID, reserved.ID); !errors.Is(err, ErrReservedUploadType) {
		t.Fatalf("RemoveOwned reserved error = %v, want ErrReservedUploadType", err)
	}

	var persisted model.Upload
	if err := dbConn.First(&persisted, reserved.ID).Error; err != nil {
		t.Fatalf("reload reserved upload: %v", err)
	}
	if persisted.Status != model.UploadStatusUsed {
		t.Fatalf("reserved upload status = %s, want used", persisted.Status)
	}
}

type totalStatsSnapshot struct {
	TotalCount int64
	TotalSize  int64
}

func loadTotalStats(ctx context.Context) (totalStatsSnapshot, error) {
	var rows []model.UploadStat
	if err := db.DB(ctx).Where("dimension = ?", model.UploadStatDimensionTotal).Find(&rows).Error; err != nil {
		return totalStatsSnapshot{}, err
	}
	if len(rows) == 0 {
		return totalStatsSnapshot{}, nil
	}
	return totalStatsSnapshot{
		TotalCount: rows[0].FileCount,
		TotalSize:  rows[0].FileSize,
	}, nil
}

func setupMockStorage(t *testing.T, putCount *int) (restore func(), disable func()) {
	return setupMockStorageWithDeleteCount(t, putCount, nil)
}

func setupMockStorageWithDeleteCount(t *testing.T, putCount, deleteCount *int) (restore func(), disable func()) {
	t.Helper()
	mockFiles := make(map[string][]byte)
	restore = objectstore.MockStorage(
		func(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
			data, err := io.ReadAll(body)
			if err != nil {
				return err
			}
			mockFiles[key] = data
			if putCount != nil {
				*putCount++
			}
			return nil
		},
		func(ctx context.Context, key string) (*objectstore.Object, error) {
			data, ok := mockFiles[key]
			if !ok {
				return nil, os.ErrNotExist
			}
			return &objectstore.Object{
				Body:          io.NopCloser(bytes.NewReader(data)),
				ContentLength: int64(len(data)),
				ContentType:   "application/octet-stream",
			}, nil
		},
		func(ctx context.Context, key string) error {
			delete(mockFiles, key)
			if deleteCount != nil {
				*deleteCount++
			}
			return nil
		},
	)
	objectstore.IsEnabledFunc = func() bool { return true }
	objectstore.ResetCache()
	disable = func() {
		objectstore.IsEnabledFunc = func() bool { return false }
		objectstore.ResetCache()
	}
	return restore, disable
}
