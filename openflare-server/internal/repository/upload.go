// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
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

// SoftDeleteUpload marks an upload as deleted.
// External modules must use upload.Remove or upload.RemoveOwned; only internal/apps/upload may call this.
func SoftDeleteUpload(ctx context.Context, upload *model.Upload) error {
	return db.DB(ctx).Model(upload).Update("status", model.UploadStatusDeleted).Error
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
	return db.DB(ctx).Create(upload).Error
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

// UploadQuery returns a scoped GORM query for uploads.
func UploadQuery(ctx context.Context) *gorm.DB {
	return db.DB(ctx).Model(&model.Upload{})
}
