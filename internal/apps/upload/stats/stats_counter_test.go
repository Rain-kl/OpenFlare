// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package stats

import (
	"context"
	"testing"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"gorm.io/gorm"
)

func TestApplyUploadStatsDeltaTxWithinTransaction(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	upload := &model.Upload{
		ID:        42002,
		FileSize:  256,
		MimeType:  "image/jpeg",
		Extension: "jpg",
		Type:      "avatar",
		Status:    model.UploadStatusUsed,
		CreatedAt: time.Now(),
	}

	if err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		return ApplyUploadStatsDeltaTx(tx, upload, 1)
	}); err != nil {
		t.Fatalf("ApplyUploadStatsDeltaTx returned error: %v", err)
	}

	stats, err := loadUploadStats(ctx)
	if err != nil {
		t.Fatalf("loadUploadStats returned error: %v", err)
	}
	if stats.TotalCount != 1 || stats.TotalSize != 256 {
		t.Fatalf("unexpected total stats: count=%d size=%d", stats.TotalCount, stats.TotalSize)
	}
}

func TestApplyUploadStatsAddAndRemove(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	upload := &model.Upload{
		ID:        42001,
		FileSize:  128,
		MimeType:  "image/png",
		Extension: "png",
		Type:      "avatar",
		Status:    model.UploadStatusUsed,
		CreatedAt: time.Now(),
	}
	if err := ApplyUploadStatsAdd(ctx, upload); err != nil {
		t.Fatalf("ApplyUploadStatsAdd returned error: %v", err)
	}

	stats, err := loadUploadStats(ctx)
	if err != nil {
		t.Fatalf("loadUploadStats returned error: %v", err)
	}
	if stats.TotalCount != 1 || stats.TotalSize != 128 {
		t.Fatalf("unexpected total stats: count=%d size=%d", stats.TotalCount, stats.TotalSize)
	}

	if err := ApplyUploadStatsRemove(ctx, upload); err != nil {
		t.Fatalf("ApplyUploadStatsRemove returned error: %v", err)
	}

	stats, err = loadUploadStats(ctx)
	if err != nil {
		t.Fatalf("loadUploadStats after remove returned error: %v", err)
	}
	if stats.TotalCount != 0 || stats.TotalSize != 0 {
		t.Fatalf("expected zeroed total stats, got count=%d size=%d", stats.TotalCount, stats.TotalSize)
	}
}

type uploadStatsSnapshot struct {
	TotalCount int64
	TotalSize  int64
}

func loadUploadStats(ctx context.Context) (uploadStatsSnapshot, error) {
	var rows []model.UploadStat
	if err := db.DB(ctx).Where("dimension = ?", model.UploadStatDimensionTotal).Find(&rows).Error; err != nil {
		return uploadStatsSnapshot{}, err
	}
	if len(rows) == 0 {
		return uploadStatsSnapshot{}, nil
	}
	return uploadStatsSnapshot{
		TotalCount: rows[0].FileCount,
		TotalSize:  rows[0].FileSize,
	}, nil
}
