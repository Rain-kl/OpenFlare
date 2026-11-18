// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/Rain-kl/Wavelet/internal/infra/objectstore"
	"github.com/Rain-kl/Wavelet/internal/infra/persistence/idgen"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

func normalizeRequest(req *IngestRequest) {
	req.Extension = strings.ToLower(strings.TrimSpace(req.Extension))
	if req.Extension == "" {
		req.Extension = "bin"
	}
	if req.Type == "" {
		req.Type = "generic"
	}
	if req.Status == "" {
		req.Status = model.UploadStatusUsed
	}
}

func resolveAccessMode(uploadType string, explicit *int) int {
	if explicit != nil {
		return *explicit
	}
	if uploadType == DefaultPublicUploadType {
		return 1
	}
	return 0
}

func validateAllowedExtension(ctx context.Context, ext string) error {
	sc, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyUploadAllowedExtensions)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if sc.Value == "" {
		return nil
	}

	allowedExts := strings.SplitSeq(strings.ToLower(sc.Value), ",")
	for allowedExt := range allowedExts {
		if strings.TrimSpace(allowedExt) == ext {
			return nil
		}
	}
	return errors.New(ErrUnsupportedFormat)
}

func defaultObjectKey(id uint64, ext string) string {
	return fmt.Sprintf("uploads/%s/%d.%s", time.Now().Format("2006/01/02"), id, ext)
}

func buildObjectKey(req IngestRequest, id uint64) string {
	if req.ObjectKeyFn != nil {
		return req.ObjectKeyFn(id, req.Extension)
	}
	return defaultObjectKey(id, req.Extension)
}

func storeObject(ctx context.Context, objectKey string, reader io.Reader, size int64, mimeType string, meta *model.UploadMetadata) (string, error) {
	if ReadOnly(ctx) {
		return "", ErrIngestStorageReadOnly
	}

	driver, backend, err := objectstore.Active(ctx)
	if err != nil {
		logger.ErrorF(ctx, "Failed to initialize active storage: %v", err)
		return "", errors.New(ErrSaveFileFailed)
	}

	result, err := backend.Put(ctx, objectKey, reader, size, mimeType)
	if err != nil {
		logger.ErrorF(ctx, "Failed to write to %s storage: %v", driver, err)
		return "", errors.New(ErrSaveFileFailed)
	}

	meta.Bucket = result.Bucket
	return result.Key, nil
}

func persistUploadRecord(ctx context.Context, upload *model.Upload, objectKey string, storedByRequest bool) error {
	if err := createUploadWithStats(ctx, upload); err != nil {
		if storedByRequest {
			cleanupUnpersistedObject(ctx, objectKey)
		}
		return err
	}
	SetUploadMetaCache(ctx, upload)
	return nil
}

func cleanupUnpersistedObject(ctx context.Context, objectKey string) {
	_, backend, err := objectstore.Active(ctx)
	if err != nil {
		return
	}
	if err := backend.Delete(ctx, objectKey); err != nil {
		logger.WarnF(ctx, "Failed to clean up upload object without a database record: %v", err)
	}
}

func createUploadWithStats(ctx context.Context, upload *model.Upload) error {
	// Multi-step: create upload row + apply incremental stats in one transaction.
	return repository.RunInTransaction(ctx, func(tx *gorm.DB) error {
		if err := repository.CreateUploadTx(tx, upload); err != nil {
			return err
		}
		return ApplyUploadStatsDeltaTx(tx, upload, 1)
	})
}

func createDedupRecord(ctx context.Context, existing model.Upload, req IngestRequest) (IngestResult, error) {
	accessMode := resolveAccessMode(req.Type, req.AccessMode)
	metadata := req.Metadata
	metadata.Bucket = existing.Metadata.Bucket
	newUpload := model.Upload{
		ID:         idgen.NextUint64ID(),
		UserID:     req.UserID,
		FileName:   req.FileName,
		FilePath:   existing.FilePath,
		FileSize:   req.Size,
		MimeType:   req.MimeType,
		Extension:  req.Extension,
		Hash:       req.Hash,
		Type:       req.Type,
		Status:     req.Status,
		AccessMode: accessMode,
		Metadata:   metadata,
	}
	if err := persistUploadRecord(ctx, &newUpload, existing.FilePath, false); err != nil {
		return IngestResult{}, err
	}
	logger.InfoF(ctx, "Upload deduplication succeeded ID: %d, Path: %s", newUpload.ID, existing.FilePath)
	return IngestResult{
		Upload:  newUpload,
		Created: true,
		Stored:  false,
	}, nil
}

func uploadstorageReadOnly(ctx context.Context) bool {
	return ReadOnly(ctx)
}

func createNewUpload(ctx context.Context, req IngestRequest) (IngestResult, error) {
	if uploadstorageReadOnly(ctx) {
		return IngestResult{}, ErrIngestStorageReadOnly
	}
	if !req.SkipExtensionCheck {
		if err := validateAllowedExtension(ctx, req.Extension); err != nil {
			return IngestResult{}, err
		}
	}

	id := idgen.NextUint64ID()
	objectKey := buildObjectKey(req, id)
	storedKey, err := storeObject(ctx, objectKey, req.Reader, req.Size, req.MimeType, &req.Metadata)
	if err != nil {
		return IngestResult{}, err
	}

	accessMode := resolveAccessMode(req.Type, req.AccessMode)
	upload := model.Upload{
		ID:         id,
		UserID:     req.UserID,
		FileName:   req.FileName,
		FilePath:   storedKey,
		FileSize:   req.Size,
		MimeType:   req.MimeType,
		Extension:  req.Extension,
		Hash:       req.Hash,
		Type:       req.Type,
		Status:     req.Status,
		AccessMode: accessMode,
		Metadata:   req.Metadata,
	}
	if err := persistUploadRecord(ctx, &upload, storedKey, true); err != nil {
		return IngestResult{}, err
	}

	return IngestResult{
		Upload:  upload,
		Created: true,
		Stored:  true,
	}, nil
}
