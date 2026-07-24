// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package stats

import (
	"context"

	"gorm.io/gorm"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

// ApplyUploadStatsAdd increments incremental stats for a newly active upload record.
func ApplyUploadStatsAdd(ctx context.Context, upload *model.Upload) error {
	return applyUploadStatsDelta(ctx, upload, 1)
}

// ApplyUploadStatsRemove decrements incremental stats for a removed active upload record.
func ApplyUploadStatsRemove(ctx context.Context, upload *model.Upload) error {
	return applyUploadStatsDelta(ctx, upload, -1)
}

// RebuildUploadStats rebuilds all incremental stats from current upload records.
func RebuildUploadStats(ctx context.Context) error {
	return repository.RebuildUploadStats(ctx, func(tx *gorm.DB, upload *model.Upload) error {
		return ApplyUploadStatsDeltaTx(tx, upload, 1)
	})
}

func applyUploadStatsDelta(ctx context.Context, upload *model.Upload, sign int64) error {
	if upload == nil || !isActiveUploadStatus(upload.Status) {
		return nil
	}
	return repository.RunInTransaction(ctx, func(tx *gorm.DB) error {
		return ApplyUploadStatsDeltaTx(tx, upload, sign)
	})
}

// ApplyUploadStatsDeltaTx applies incremental upload stats within an existing transaction.
func ApplyUploadStatsDeltaTx(tx *gorm.DB, upload *model.Upload, sign int64) error {
	if upload == nil || !isActiveUploadStatus(upload.Status) || sign == 0 {
		return nil
	}

	countDelta := sign
	sizeDelta := sign * upload.FileSize
	typeKey := upload.Type
	if typeKey == "" {
		typeKey = "generic"
	}

	entries := []struct {
		dimension string
		key       string
	}{
		{model.UploadStatDimensionTotal, ""},
		{model.UploadStatDimensionType, typeKey},
		{model.UploadStatDimensionCategory, GetFileCategory(upload.MimeType, upload.Extension)},
		{model.UploadStatDimensionTrend, upload.CreatedAt.Format("2006-01-02")},
	}

	for _, entry := range entries {
		if err := repository.UpsertUploadStatDeltaTx(tx, entry.dimension, entry.key, countDelta, sizeDelta); err != nil {
			return err
		}
	}
	return nil
}

// RecordUploadStatsAdd logs and applies upload stats increment.
func RecordUploadStatsAdd(ctx context.Context, upload *model.Upload) {
	if err := ApplyUploadStatsAdd(ctx, upload); err != nil {
		logger.WarnF(ctx, "increment upload stats failed: %v", err)
	}
}

// RecordUploadStatsRemove logs and applies upload stats decrement.
func RecordUploadStatsRemove(ctx context.Context, upload *model.Upload) {
	if err := ApplyUploadStatsRemove(ctx, upload); err != nil {
		logger.WarnF(ctx, "decrement upload stats failed: %v", err)
	}
}

func isActiveUploadStatus(status model.UploadStatus) bool {
	return status == model.UploadStatusPending || status == model.UploadStatusUsed
}
