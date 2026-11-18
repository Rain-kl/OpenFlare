// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package upload

import (
	"context"
	"errors"
	"sort"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"gorm.io/gorm"
)

func listUploadFiles(ctx context.Context, filter repository.UploadListFilter) (int64, []model.Upload, error) {
	return repository.ListUploads(ctx, filter)
}

func listMyUploadFiles(ctx context.Context, userID uint64, filter repository.UploadListFilter) (int64, []model.Upload, error) {
	filter.UserID = userID
	return repository.ListUploads(ctx, filter)
}

func softDeleteUpload(ctx context.Context, uploadID uint64) (model.Upload, error) {
	return Remove(ctx, uploadID)
}

func softDeleteOwnedUpload(ctx context.Context, userID, uploadID uint64) (model.Upload, error) {
	return RemoveOwned(ctx, userID, uploadID)
}

func listDistinctUploadTypes(ctx context.Context) ([]string, error) {
	types, err := repository.ListDistinctUploadTypes(ctx)
	if err != nil {
		return nil, err
	}
	sort.Strings(types)
	return types, nil
}

type updateMyUploadInput struct {
	FileName   string
	AccessMode *int
}

func updateOwnedUpload(ctx context.Context, userID, uploadID uint64, input updateMyUploadInput) (model.Upload, error) {
	upload, err := repository.GetActiveUploadByID(ctx, uploadID)
	if err != nil {
		return model.Upload{}, err
	}
	if upload.UserID != userID {
		return model.Upload{}, ErrIngestForbidden
	}

	updates := make(map[string]any)
	if input.FileName != "" {
		updates["file_name"] = input.FileName
	}
	if input.AccessMode != nil {
		updates["access_mode"] = *input.AccessMode
	}
	if err := repository.UpdateUpload(ctx, &upload, updates); err != nil {
		return model.Upload{}, err
	}
	if name, ok := updates["file_name"].(string); ok {
		upload.FileName = name
	}
	if mode, ok := updates["access_mode"].(int); ok {
		upload.AccessMode = mode
	}
	return upload, nil
}

func listUploadsForBatchDownload(ctx context.Context, ids []uint64) ([]model.Upload, error) {
	return repository.ListUploadsByIDs(ctx, ids)
}

func loadUploadStats(ctx context.Context) ([]model.UploadStat, error) {
	return repository.ListUploadStats(ctx)
}

func isRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
