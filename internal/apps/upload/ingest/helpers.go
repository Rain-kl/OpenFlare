// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package ingest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	uploadcache "github.com/Rain-kl/Wavelet/internal/apps/upload/cache"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/shared"
	uploadstats "github.com/Rain-kl/Wavelet/internal/apps/upload/stats"
	uploadstorage "github.com/Rain-kl/Wavelet/internal/apps/upload/storage"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/db/idgen"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/storage"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"gorm.io/gorm"
)

func normalizeRequest(req *Request) {
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
	if uploadType == shared.DefaultPublicUploadType {
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

	allowedExts := strings.Split(strings.ToLower(sc.Value), ",")
	for _, allowedExt := range allowedExts {
		if strings.TrimSpace(allowedExt) == ext {
			return nil
		}
	}
	return errors.New(shared.ErrUnsupportedFormat)
}

func defaultObjectKey(id uint64, ext string) string {
	return fmt.Sprintf("uploads/%s/%d.%s", time.Now().Format("2006/01/02"), id, ext)
}

func buildObjectKey(req Request, id uint64) string {
	if req.ObjectKeyFn != nil {
		return req.ObjectKeyFn(id, req.Extension)
	}
	return defaultObjectKey(id, req.Extension)
}

func storeObject(ctx context.Context, objectKey string, reader io.Reader, size int64, mimeType string, meta *model.UploadMetadata) (string, error) {
	if uploadstorage.ReadOnly(ctx) {
		return "", ErrStorageReadOnly
	}

	driver, backend, err := storage.Active(ctx)
	if err != nil {
		logger.ErrorF(ctx, "初始化活动存储失败: %v", err)
		return "", errors.New(shared.ErrSaveFileFailed)
	}

	result, err := backend.Put(ctx, objectKey, reader, size, mimeType)
	if err != nil {
		logger.ErrorF(ctx, "写入 %s 存储失败: %v", driver, err)
		return "", errors.New(shared.ErrSaveFileFailed)
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
	uploadcache.SetUploadMetaCache(ctx, upload)
	return nil
}

func cleanupUnpersistedObject(ctx context.Context, objectKey string) {
	_, backend, err := storage.Active(ctx)
	if err != nil {
		return
	}
	if err := backend.Delete(ctx, objectKey); err != nil {
		logger.WarnF(ctx, "清理未写入数据库的上传对象失败: %v", err)
	}
}

func createUploadWithStats(ctx context.Context, upload *model.Upload) error {
	return db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := repository.CreateUploadTx(tx, upload); err != nil {
			return err
		}
		return uploadstats.ApplyUploadStatsDeltaTx(tx, upload, 1)
	})
}

func createDedupRecord(ctx context.Context, existing model.Upload, req Request) (Result, error) {
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
		return Result{}, err
	}
	logger.InfoF(ctx, "文件触发秒传成功! ID: %d, Path: %s", newUpload.ID, existing.FilePath)
	return Result{
		Upload:  newUpload,
		Created: true,
		Stored:  false,
	}, nil
}

func uploadstorageReadOnly(ctx context.Context) bool {
	return uploadstorage.ReadOnly(ctx)
}

func createNewUpload(ctx context.Context, req Request) (Result, error) {
	if uploadstorageReadOnly(ctx) {
		return Result{}, ErrStorageReadOnly
	}
	if !req.SkipExtensionCheck {
		if err := validateAllowedExtension(ctx, req.Extension); err != nil {
			return Result{}, err
		}
	}

	id := idgen.NextUint64ID()
	objectKey := buildObjectKey(req, id)
	storedKey, err := storeObject(ctx, objectKey, req.Reader, req.Size, req.MimeType, &req.Metadata)
	if err != nil {
		return Result{}, err
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
		return Result{}, err
	}

	return Result{
		Upload:  upload,
		Created: true,
		Stored:  true,
	}, nil
}
