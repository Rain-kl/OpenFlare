// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package ingest

import (
	"context"
	"errors"
	"strings"

	uploadcache "github.com/Rain-kl/Wavelet/internal/apps/upload/cache"
	uploadstorage "github.com/Rain-kl/Wavelet/internal/apps/upload/storage"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// GetActive loads an active upload record by ID through the upload metadata cache path.
func GetActive(ctx context.Context, uploadID uint64) (model.Upload, error) {
	if uploadID == 0 {
		return model.Upload{}, errors.New("upload id is required")
	}
	return uploadcache.GetUploadByID(ctx, uploadID)
}

// OpenActiveObject opens the stored object for an active upload record.
func OpenActiveObject(ctx context.Context, uploadID uint64) (OpenedObject, error) {
	record, err := GetActive(ctx, uploadID)
	if err != nil {
		return OpenedObject{}, err
	}
	obj, err := uploadstorage.OpenStoredObject(ctx, &record)
	if err != nil {
		return OpenedObject{}, err
	}
	return OpenedObject{
		Body:          obj.Body,
		ContentType:   obj.ContentType,
		ContentLength: obj.ContentLength,
		Upload:        record,
	}, nil
}

// ActiveHash returns the SHA-256 hash recorded for an active upload.
func ActiveHash(ctx context.Context, uploadID uint64) (string, error) {
	record, err := GetActive(ctx, uploadID)
	if err != nil {
		return "", err
	}
	hash := strings.TrimSpace(record.Hash)
	if hash == "" {
		return "", errors.New("upload hash is empty")
	}
	return hash, nil
}
