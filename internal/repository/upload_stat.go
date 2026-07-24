// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// ListUploadStats returns all upload statistics rows.
func ListUploadStats(ctx context.Context) ([]model.UploadStat, error) {
	var stats []model.UploadStat
	if err := db.DB(ctx).Find(&stats).Error; err != nil {
		return nil, err
	}
	return stats, nil
}

// GetTotalUploadStat returns the aggregate total-dimension stats row.
func GetTotalUploadStat(ctx context.Context) (model.UploadStat, error) {
	var total model.UploadStat
	if err := db.DB(ctx).
		Where("dimension = ? AND stat_key = ?", model.UploadStatDimensionTotal, "").
		First(&total).Error; err != nil {
		return model.UploadStat{}, err
	}
	return total, nil
}

// ListUploadStatsByDimension returns stats rows for a single dimension.
func ListUploadStatsByDimension(ctx context.Context, dimension string) ([]model.UploadStat, error) {
	var rows []model.UploadStat
	if err := db.DB(ctx).Where("dimension = ?", dimension).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// DeleteAllUploadStatsTx removes every row from w_upload_stats within a transaction.
func DeleteAllUploadStatsTx(tx *gorm.DB) error {
	return tx.Where("1 = 1").Delete(&model.UploadStat{}).Error
}

// UpsertUploadStatDeltaTx applies an incremental count/size delta for one dimension key.
func UpsertUploadStatDeltaTx(tx *gorm.DB, dimension, key string, countDelta, sizeDelta int64) error {
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "dimension"},
			{Name: "stat_key"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"file_count": gorm.Expr(
				"CASE WHEN w_upload_stats.file_count + ? < 0 THEN 0 ELSE w_upload_stats.file_count + ? END",
				countDelta,
				countDelta,
			),
			"file_size": gorm.Expr(
				"CASE WHEN w_upload_stats.file_size + ? < 0 THEN 0 ELSE w_upload_stats.file_size + ? END",
				sizeDelta,
				sizeDelta,
			),
			"updated_at": time.Now(),
		}),
	}).Create(&model.UploadStat{
		Dimension: dimension,
		StatKey:   key,
		FileCount: countDelta,
		FileSize:  sizeDelta,
	}).Error
}

// RebuildUploadStats clears w_upload_stats and re-applies deltas for every active upload
// inside a single transaction. applyDelta should apply +1 stats for one upload row.
func RebuildUploadStats(ctx context.Context, applyDelta func(tx *gorm.DB, upload *model.Upload) error) error {
	return db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := DeleteAllUploadStatsTx(tx); err != nil {
			return err
		}
		uploads, err := ListActiveUploadsTx(tx)
		if err != nil {
			return err
		}
		for i := range uploads {
			if err := applyDelta(tx, &uploads[i]); err != nil {
				return err
			}
		}
		return nil
	})
}
