// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"

	"github.com/Rain-kl/Wavelet/internal/db"
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
