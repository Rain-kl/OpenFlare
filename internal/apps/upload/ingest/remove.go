// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package ingest

import (
	"context"

	uploadcache "github.com/Rain-kl/Wavelet/internal/apps/upload/cache"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/shared"
	uploadstats "github.com/Rain-kl/Wavelet/internal/apps/upload/stats"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Remove soft-deletes an ordinary upload and decrements incremental stats once.
func Remove(ctx context.Context, uploadID uint64) (model.Upload, error) {
	upload, err := remove(ctx, 0, uploadID, false)
	if err != nil {
		return model.Upload{}, err
	}
	return upload, nil
}

// RemoveOwned soft-deletes an ordinary upload owned by userID and decrements incremental stats once.
func RemoveOwned(ctx context.Context, userID, uploadID uint64) (model.Upload, error) {
	upload, err := remove(ctx, userID, uploadID, true)
	if err != nil {
		return model.Upload{}, err
	}
	return upload, nil
}

func remove(ctx context.Context, userID, uploadID uint64, owned bool) (model.Upload, error) {
	var upload model.Upload
	if err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", uploadID).
			First(&upload).Error; err != nil {
			return err
		}
		if owned && upload.UserID != userID {
			return ErrForbidden
		}
		if upload.Type == shared.ReservedPagesDeploymentType {
			return ErrReservedUploadType
		}
		_, err := RemoveLockedTx(tx, &upload)
		return err
	}); err != nil {
		return model.Upload{}, err
	}

	InvalidateUploadMetaCache(ctx, uploadID)
	upload.Status = model.UploadStatusDeleted
	return upload, nil
}

// RemoveLockedTx performs the idempotent active-to-deleted transition for a row
// that the caller has already locked in its surrounding transaction.
func RemoveLockedTx(tx *gorm.DB, upload *model.Upload) (bool, error) {
	rowsAffected, err := repository.SoftDeleteUploadTx(tx, upload)
	if err != nil {
		return false, err
	}
	if rowsAffected == 0 {
		return false, nil
	}
	if err := uploadstats.ApplyUploadStatsDeltaTx(tx, upload, -1); err != nil {
		return false, err
	}
	upload.Status = model.UploadStatusDeleted
	return true, nil
}

// InvalidateUploadMetaCache invalidates upload metadata after the caller commits its transaction.
func InvalidateUploadMetaCache(ctx context.Context, uploadID uint64) {
	uploadcache.InvalidateUploadMetaCache(ctx, uploadID)
}
