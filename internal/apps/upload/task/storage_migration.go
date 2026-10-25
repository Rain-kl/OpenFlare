// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	uploadstats "github.com/Rain-kl/Wavelet/internal/apps/upload/stats"
	uploadstorage "github.com/Rain-kl/Wavelet/internal/apps/upload/storage"
	"github.com/Rain-kl/Wavelet/internal/infra/objectstore"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/util"
)

const (
	// StorageMigrationTask is the Asynq task name for storage migration.
	StorageMigrationTask = uploadstorage.StorageMigrationTask
	// TaskTypeStorageMigration is the task metadata type for storage migration.
	TaskTypeStorageMigration = "storage_migration"
)

// StorageMigrationMeta describes the manually dispatchable migration task.
var StorageMigrationMeta = task.TaskMeta{
	Type:         TaskTypeStorageMigration,
	AsynqTask:    StorageMigrationTask,
	Name:         "Migrate file storage",
	Description:  "Migrate files from active storage to the target backend; files remain read-only during migration",
	SupportsTime: false,
	MaxRetry:     task.DefaultMaxRetry,
	Queue:        task.QueueDefault,
	Retryable:    true,
	Params: []task.TaskParam{
		{
			Name:        "target",
			Label:       "Target storage configuration (JSON)",
			Type:        "text",
			Required:    true,
			Placeholder: `{"driver": "s3", "local": {"root": "."}, "s3": {"bucket": "my-bucket", ...}}`,
			Description: "Complete JSON configuration of the target storage backend",
		},
	},
}

// MigrationHandler copies stored objects and activates the target backend.
type MigrationHandler struct{}

// ValidatePayload rejects duplicate active migrations through the task framework.
func (h *MigrationHandler) ValidatePayload(payload []byte) ([]byte, error) {
	normalized, _, err := uploadstorage.NormalizeMigrationPayload(context.Background(), payload)
	if err != nil {
		return payload, err
	}
	active, err := hasUnresolvedMigrationTask(context.Background())
	if err != nil {
		return payload, err
	}
	if active {
		return payload, errors.New("storage migration task is already unresolved")
	}
	return normalized, nil
}

// Execute migrates all unique active-storage objects to the pending backend.
func (h *MigrationHandler) Execute(ctx context.Context, payload []byte) (*task.TaskResult, error) {
	if db.Redis != nil {
		const (
			cleanupTimeout  = 5 * time.Second
			renewalInterval = 10 * time.Minute
		)

		lockKey := db.PrefixedKey("lock:storage:migrate")
		ok, err := db.Redis.SetNX(ctx, lockKey, "locked", time.Hour).Result()
		if err != nil {
			return nil, fmt.Errorf("acquire migration lock: %w", err)
		}
		if !ok {
			return nil, errors.New("another storage migration task is already running")
		}

		stopRenewal := make(chan struct{})
		//nolint:contextcheck
		defer func() {
			close(stopRenewal)
			cleanupCtx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
			defer cancel()
			_ = db.Redis.Del(cleanupCtx, lockKey)
		}()

		//nolint:contextcheck,gosec
		util.Go(func() {
			ticker := time.NewTicker(renewalInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					renewCtx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
					_ = db.Redis.Expire(renewCtx, lockKey, time.Hour).Err()
					cancel()
				case <-stopRenewal:
					return
				case <-ctx.Done():
					return
				}
			}
		})
	}

	active, err := objectstore.LoadConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load active storage config: %w", err)
	}
	target, err := uploadstorage.ParseMigrationTargetConfig(ctx, payload)
	if err != nil {
		return nil, err
	}
	if target.Driver == active.Driver {
		if err := objectstore.SaveActiveConfig(ctx, target); err != nil {
			return nil, fmt.Errorf("activate same-driver storage config: %w", err)
		}
		message := fmt.Sprintf("Storage configuration updated; active storage remains %s", target.Driver)
		task.AppendLog(ctx, "%s", message)
		return &task.TaskResult{Message: message}, nil
	}

	total, err := countStorageObjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("count source objects: %w", err)
	}
	if total == 0 {
		if err := objectstore.SaveActiveConfig(ctx, target); err != nil {
			return nil, fmt.Errorf("activate empty storage config: %w", err)
		}
		message := fmt.Sprintf("Current storage has no objects to migrate; active storage switched to %s", target.Driver)
		task.AppendLog(ctx, "%s", message)
		return &task.TaskResult{Message: message}, nil
	}

	sourceBackend, err := objectstore.NewBackend(ctx, active, active.Driver)
	if err != nil {
		return nil, fmt.Errorf("create source storage: %w", err)
	}
	targetBackend, err := objectstore.NewBackend(ctx, target, target.Driver)
	if err != nil {
		return nil, fmt.Errorf("create target storage: %w", err)
	}

	task.AppendLog(ctx, "Starting storage migration: %s -> %s, total objects: %d", active.Driver, target.Driver, total)
	migrated, err := migrateObjects(ctx, sourceBackend, targetBackend, total)
	if err != nil {
		return nil, err
	}

	if err := objectstore.SaveActiveConfig(ctx, target); err != nil {
		return nil, fmt.Errorf("activate target storage: %w", err)
	}
	message := fmt.Sprintf("Storage migration completed; migrated %d objects, active storage switched to %s", migrated, target.Driver)
	task.AppendLog(ctx, "%s", message)
	return &task.TaskResult{Message: message}, nil
}

func countStorageObjects(ctx context.Context) (int64, error) {
	return repository.CountDistinctActiveFilePaths(ctx)
}

func hasUnresolvedMigrationTask(ctx context.Context) (bool, error) {
	execution, ok, err := uploadstorage.LatestMigrationExecution(ctx)
	if err != nil || !ok {
		return false, err
	}
	return execution.Status == model.TaskExecutionStatusPending || execution.Status == model.TaskExecutionStatusRunning, nil
}

func migrateObjects(
	ctx context.Context,
	sourceBackend objectstore.Backend,
	targetBackend objectstore.Backend,
	total int64,
) (int64, error) {
	const batchSize = 50
	const migrationConcurrency = 10
	const sha256HexLength = 64
	var migrated atomic.Int64
	var lastFilePath string
	for {
		if err := ctx.Err(); err != nil {
			return migrated.Load(), fmt.Errorf("storage migration canceled: %w", err)
		}

		task.AppendLog(ctx, "Querying next migration batch; migrated: %d/%d", migrated.Load(), total)

		objects, err := repository.ListDistinctActiveStorageObjects(ctx, lastFilePath, batchSize)
		if err != nil {
			return migrated.Load(), fmt.Errorf("query source objects: %w", err)
		}
		if len(objects) == 0 {
			task.AppendLog(ctx, "All objects have been migrated")
			break
		}

		lastFilePath = objects[len(objects)-1].FilePath
		task.AppendLog(ctx, "Fetched migration batch; requested size: %d, actual objects: %d", batchSize, len(objects))

		var g errgroup.Group
		g.SetLimit(migrationConcurrency)

		for _, object := range objects {
			obj := object
			g.Go(func() error {
				if err := migrateSingleObject(ctx, sourceBackend, targetBackend, obj, sha256HexLength); err != nil {
					return err
				}
				migrated.Add(1)
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return migrated.Load(), err
		}

		task.AppendLog(ctx, "Current migration batch completed. Progress: %d/%d", migrated.Load(), total)
	}
	return migrated.Load(), nil
}

func migrateSingleObject(
	ctx context.Context,
	sourceBackend objectstore.Backend,
	targetBackend objectstore.Backend,
	obj repository.UploadStorageObject,
	sha256HexLength int,
) error {
	if shouldSkipMigration(ctx, targetBackend, obj) {
		task.AppendLog(ctx, "[Migration skipped] Identical file already exists in target storage: %s", obj.FilePath)
		return nil
	}

	task.AppendLog(ctx, "[Migration started] Reading file from source storage: %s", obj.FilePath)
	source, err := sourceBackend.Get(ctx, obj.FilePath)
	if err != nil {
		if isNotFoundError(err) {
			return markMissingMigrationObjectDeleted(ctx, obj.FilePath, err)
		}
		return fmt.Errorf("open source object %q: %w", obj.FilePath, err)
	}
	task.AppendLog(ctx, "[Transfer] Uploading file to target storage: %s (size: %d bytes, type: %s)", obj.FilePath, obj.FileSize, obj.MimeType)
	targetResult, putErr := targetBackend.Put(ctx, obj.FilePath, source.Body, obj.FileSize, obj.MimeType)
	closeErr := source.Body.Close()
	if putErr != nil {
		return fmt.Errorf("copy object %q: %w", obj.FilePath, putErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close source object %q: %w", obj.FilePath, closeErr)
	}

	if len(obj.Hash) == sha256HexLength {
		task.AppendLog(ctx, "[Verification] Checking target file consistency (SHA-256): %s", targetResult.Key)
		targetObj, getErr := targetBackend.Get(ctx, targetResult.Key)
		if getErr != nil {
			return fmt.Errorf("retrieve target object for verification %q: %w", obj.FilePath, getErr)
		}
		if targetObj == nil || targetObj.Body == nil {
			return fmt.Errorf("retrieve target object for verification %q: object or body is nil", obj.FilePath)
		}
		h := sha256.New()
		if _, copyErr := io.Copy(h, targetObj.Body); copyErr != nil {
			_ = targetObj.Body.Close()
			return fmt.Errorf("read target object for verification %q: %w", obj.FilePath, copyErr)
		}
		_ = targetObj.Body.Close()
		computedHash := hex.EncodeToString(h.Sum(nil))
		if computedHash != obj.Hash {
			return fmt.Errorf("integrity check failed for %q: got hash %s, want %s", obj.FilePath, computedHash, obj.Hash)
		}
		task.AppendLog(ctx, "[Verified] File consistency check succeeded: %s", targetResult.Key)
	}

	if targetResult.Key != obj.FilePath {
		task.AppendLog(ctx, "[Database update] Updating file path: %s -> %s", obj.FilePath, targetResult.Key)
		if err := repository.UpdateActiveUploadsFilePath(ctx, obj.FilePath, targetResult.Key); err != nil {
			return fmt.Errorf("update migrated object %q: %w", obj.FilePath, err)
		}
	}
	task.AppendLog(ctx, "[Migration succeeded] File migration completed: %s", targetResult.Key)
	return nil
}

func shouldSkipMigration(
	ctx context.Context,
	targetBackend objectstore.Backend,
	obj repository.UploadStorageObject,
) bool {
	targetObj, err := targetBackend.Get(ctx, obj.FilePath)
	if err != nil || targetObj == nil || targetObj.Body == nil {
		return false
	}
	defer func() {
		_ = targetObj.Body.Close()
	}()

	return targetObj.ContentLength == obj.FileSize
}

func markMissingMigrationObjectDeleted(
	ctx context.Context,
	filePath string,
	sourceErr error,
) error {
	task.AppendLog(ctx, "Warning: source file is missing; marking it deleted and skipping: %s (error: %v)", filePath, sourceErr)

	affectedUploads, err := repository.MarkActiveUploadsDeletedByFilePath(ctx, filePath)
	if err != nil {
		return fmt.Errorf("mark missing object deleted %q: %w", filePath, err)
	}
	for i := range affectedUploads {
		uploadstats.RecordUploadStatsRemove(ctx, &affectedUploads[i])
	}
	return nil
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	errStr := strings.ToLower(err.Error())
	for _, sub := range []string{"not found", "nosuchkey", "nosuchbucket", "404", "does not exist"} {
		if strings.Contains(errStr, sub) {
			return true
		}
	}
	return false
}
