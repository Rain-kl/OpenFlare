// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// UploadListFilter filters paginated upload queries.
type UploadListFilter struct {
	UserID    uint64
	Keyword   string
	Type      string
	Extension string
	Page      int
	PageSize  int
}

// UploadStorageObject is a distinct active object path with aggregated metadata for migration.
type UploadStorageObject struct {
	FilePath string `gorm:"column:file_path"`
	FileSize int64  `gorm:"column:file_size"`
	MimeType string `gorm:"column:mime_type"`
	Hash     string `gorm:"column:hash"`
}

// RunInTransaction executes fn inside a database transaction.
// Prefer domain-specific repository methods when the full operation can live in repository.
// Upload package multi-step flows (lock + soft-delete + stats) use this boundary so apps
// do not call db.DB directly.
func RunInTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return db.DB(ctx).Transaction(fn)
}

// ListUploads returns paginated upload records matching the filter.
func ListUploads(ctx context.Context, filter UploadListFilter) (int64, []model.Upload, error) {
	query := db.DB(ctx).Model(&model.Upload{}).
		Where("status != ?", model.UploadStatusDeleted)

	if filter.UserID != 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.Keyword != "" {
		query = query.Where("LOWER(file_name) LIKE ?", "%"+strings.ToLower(filter.Keyword)+"%")
	}
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}
	if filter.Extension != "" {
		query = query.Where("extension = ?", strings.ToLower(filter.Extension))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}

	var items []model.Upload
	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(filter.PageSize).Find(&items).Error; err != nil {
		return 0, nil, err
	}
	return total, items, nil
}

// GetActiveUploadByID loads a non-deleted upload by ID.
func GetActiveUploadByID(ctx context.Context, id uint64) (model.Upload, error) {
	var upload model.Upload
	if err := db.DB(ctx).Where("id = ? AND status != ?", id, model.UploadStatusDeleted).First(&upload).Error; err != nil {
		return model.Upload{}, err
	}
	return upload, nil
}

// GetCacheableUploadByID loads a pending or used upload by ID (for metadata cache DB fallback).
func GetCacheableUploadByID(ctx context.Context, id uint64) (model.Upload, error) {
	var upload model.Upload
	if err := db.DB(ctx).
		Where("id = ? AND status IN (?, ?)", id, model.UploadStatusPending, model.UploadStatusUsed).
		First(&upload).Error; err != nil {
		return model.Upload{}, err
	}
	return upload, nil
}

// GetUploadByIDForUpdateTx loads and row-locks an upload by ID within an existing transaction.
func GetUploadByIDForUpdateTx(tx *gorm.DB, id uint64) (model.Upload, error) {
	var upload model.Upload
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).
		First(&upload).Error; err != nil {
		return model.Upload{}, err
	}
	return upload, nil
}

// SoftDeleteUpload marks an active upload as deleted and reports whether the row transitioned.
// External modules must use upload.Remove or upload.RemoveOwned; only internal/apps/upload may call this.
func SoftDeleteUpload(ctx context.Context, upload *model.Upload) (int64, error) {
	return SoftDeleteUploadTx(db.DB(ctx), upload)
}

// SoftDeleteUploadTx marks an active upload as deleted within an existing transaction.
// RowsAffected is one only for the single successful active-to-deleted transition.
func SoftDeleteUploadTx(tx *gorm.DB, upload *model.Upload) (int64, error) {
	result := tx.Model(&model.Upload{}).
		Where("id = ? AND status IN ?", upload.ID, []model.UploadStatus{
			model.UploadStatusPending,
			model.UploadStatusUsed,
		}).
		Update("status", model.UploadStatusDeleted)
	return result.RowsAffected, result.Error
}

// UpdateUpload applies partial field updates to an upload record.
func UpdateUpload(ctx context.Context, upload *model.Upload, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	return db.DB(ctx).Model(upload).Updates(updates).Error
}

// ListDistinctUploadTypes returns all distinct non-empty upload business types.
func ListDistinctUploadTypes(ctx context.Context) ([]string, error) {
	var types []string
	if err := db.DB(ctx).Model(&model.Upload{}).
		Where("type IS NOT NULL AND type != ''").
		Distinct().
		Pluck("type", &types).Error; err != nil {
		return nil, err
	}
	return types, nil
}

// FindReusableUploadByHash finds an existing upload with the same hash and size.
func FindReusableUploadByHash(ctx context.Context, hash string, size int64) (model.Upload, error) {
	var existing model.Upload
	err := db.DB(ctx).
		Where("hash = ? AND file_size = ? AND status IN (?, ?)", hash, size, model.UploadStatusPending, model.UploadStatusUsed).
		First(&existing).Error
	return existing, err
}

// CreateUpload persists a new upload record.
// External modules must use upload.Ingest; only internal/apps/upload may call this.
func CreateUpload(ctx context.Context, upload *model.Upload) error {
	return CreateUploadTx(db.DB(ctx), upload)
}

// CreateUploadTx persists a new upload record within an existing transaction.
func CreateUploadTx(tx *gorm.DB, upload *model.Upload) error {
	return tx.Create(upload).Error
}

// ListUploadsByIDs returns active uploads matching the given IDs.
func ListUploadsByIDs(ctx context.Context, ids []uint64) ([]model.Upload, error) {
	var uploads []model.Upload
	if err := db.DB(ctx).
		Where("id IN ? AND status IN (?, ?)", ids, model.UploadStatusPending, model.UploadStatusUsed).
		Find(&uploads).Error; err != nil {
		return nil, err
	}
	return uploads, nil
}

// CountActiveUploads returns the number of non-deleted upload records.
func CountActiveUploads(ctx context.Context) (int64, error) {
	var count int64
	err := db.DB(ctx).Model(&model.Upload{}).
		Where("status != ?", model.UploadStatusDeleted).
		Count(&count).Error
	return count, err
}

// ListPendingUploadsOlderThan returns pending uploads created before olderThan, after lastID, ordered by id.
func ListPendingUploadsOlderThan(ctx context.Context, lastID uint64, olderThan time.Time, limit int) ([]model.Upload, error) {
	var uploads []model.Upload
	err := db.DB(ctx).
		Where("id > ? AND status = ? AND created_at < ?", lastID, model.UploadStatusPending, olderThan).
		Order("id ASC").
		Limit(limit).
		Find(&uploads).Error
	return uploads, err
}

// ListActiveImageUploadsAfterID returns non-deleted image uploads with id greater than lastID.
func ListActiveImageUploadsAfterID(ctx context.Context, lastID uint64, limit int) ([]model.Upload, error) {
	var uploads []model.Upload
	err := db.DB(ctx).
		Where("id > ? AND status != ? AND (LOWER(mime_type) LIKE ? OR LOWER(extension) IN ?)",
			lastID,
			model.UploadStatusDeleted,
			"image/%",
			[]string{"jpg", "jpeg", "png", "webp", "gif"},
		).
		Order("id ASC").
		Limit(limit).
		Find(&uploads).Error
	return uploads, err
}

// CountDistinctActiveFilePaths returns the number of distinct non-deleted upload file paths.
func CountDistinctActiveFilePaths(ctx context.Context) (int64, error) {
	var count int64
	err := db.DB(ctx).Model(&model.Upload{}).
		Where("status != ?", model.UploadStatusDeleted).
		Distinct("file_path").
		Count(&count).Error
	return count, err
}

// ListDistinctActiveStorageObjects returns a page of distinct active file paths ordered by path.
// When afterFilePath is non-empty, only paths strictly greater than it are returned.
func ListDistinctActiveStorageObjects(ctx context.Context, afterFilePath string, limit int) ([]UploadStorageObject, error) {
	var objects []UploadStorageObject
	query := db.DB(ctx).Model(&model.Upload{}).
		Select("file_path, MAX(file_size) AS file_size, MAX(mime_type) AS mime_type, MAX(hash) AS hash").
		Where("status != ?", model.UploadStatusDeleted)
	if afterFilePath != "" {
		query = query.Where("file_path > ?", afterFilePath)
	}
	err := query.Group("file_path").
		Order("file_path ASC").
		Limit(limit).
		Scan(&objects).Error
	return objects, err
}

// UpdateActiveUploadsFilePath rewrites file_path for all non-deleted uploads matching oldPath.
func UpdateActiveUploadsFilePath(ctx context.Context, oldPath, newPath string) error {
	return db.DB(ctx).Model(&model.Upload{}).
		Where("file_path = ? AND status != ?", oldPath, model.UploadStatusDeleted).
		Update("file_path", newPath).Error
}

// MarkActiveUploadsDeletedByFilePath marks all non-deleted uploads with the given path as deleted
// and returns the rows that transitioned (for stats adjustment).
func MarkActiveUploadsDeletedByFilePath(ctx context.Context, filePath string) ([]model.Upload, error) {
	var affected []model.Upload
	err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where("file_path = ? AND status != ?", filePath, model.UploadStatusDeleted).
			Find(&affected).Error; err != nil {
			return err
		}
		if len(affected) == 0 {
			return nil
		}
		return tx.Model(&model.Upload{}).
			Where("file_path = ?", filePath).
			Update("status", model.UploadStatusDeleted).Error
	})
	if err != nil {
		return nil, err
	}
	return affected, nil
}

// ListActiveUploadsTx returns all non-deleted uploads within an existing transaction.
func ListActiveUploadsTx(tx *gorm.DB) ([]model.Upload, error) {
	var uploads []model.Upload
	if err := tx.Where("status != ?", model.UploadStatusDeleted).Find(&uploads).Error; err != nil {
		return nil, err
	}
	return uploads, nil
}

// UploadQuery returns a scoped GORM query for uploads.
func UploadQuery(ctx context.Context) *gorm.DB {
	return db.DB(ctx).Model(&model.Upload{})
}
