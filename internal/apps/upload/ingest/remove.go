// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package ingest

import (
	"context"

	uploadcache "github.com/Rain-kl/Wavelet/internal/apps/upload/cache"
	uploadstats "github.com/Rain-kl/Wavelet/internal/apps/upload/stats"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"gorm.io/gorm"
)

// Remove soft-deletes an upload and decrements incremental stats.
func Remove(ctx context.Context, uploadID uint64) (model.Upload, error) {
	upload, err := repository.GetActiveUploadByID(ctx, uploadID)
	if err != nil {
		return model.Upload{}, err
	}
	if err := softDeleteUploadWithStats(ctx, &upload); err != nil {
		return model.Upload{}, err
	}
	upload.Status = model.UploadStatusDeleted
	return upload, nil
}

// RemoveOwned soft-deletes an upload owned by userID and decrements incremental stats.
func RemoveOwned(ctx context.Context, userID, uploadID uint64) (model.Upload, error) {
	upload, err := repository.GetActiveUploadByID(ctx, uploadID)
	if err != nil {
		return model.Upload{}, err
	}
	if upload.UserID != userID {
		return model.Upload{}, ErrForbidden
	}
	if err := softDeleteUploadWithStats(ctx, &upload); err != nil {
		return model.Upload{}, err
	}
	upload.Status = model.UploadStatusDeleted
	return upload, nil
}

func softDeleteUploadWithStats(ctx context.Context, upload *model.Upload) error {
	statsSnapshot := *upload
	if err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := repository.SoftDeleteUploadTx(tx, upload); err != nil {
			return err
		}
		return uploadstats.ApplyUploadStatsDeltaTx(tx, &statsSnapshot, -1)
	}); err != nil {
		return err
	}
	uploadcache.InvalidateUploadMetaCache(ctx, upload.ID)
	return nil
}
