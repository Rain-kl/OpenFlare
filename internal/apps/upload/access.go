// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package upload

import (
	"context"
	"errors"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
)

// GetActiveUpload loads an active upload record by ID through the upload metadata cache path.
func GetActiveUpload(ctx context.Context, uploadID uint64) (model.Upload, error) {
	if uploadID == 0 {
		return model.Upload{}, errors.New("upload id is required")
	}
	return GetUploadByID(ctx, uploadID)
}

// OpenStoredUpload opens the stored object for an active upload record.
func OpenStoredUpload(ctx context.Context, uploadID uint64) (OpenedUploadObject, error) {
	record, err := GetActiveUpload(ctx, uploadID)
	if err != nil {
		return OpenedUploadObject{}, err
	}
	obj, err := OpenStoredObject(ctx, &record)
	if err != nil {
		return OpenedUploadObject{}, err
	}
	return OpenedUploadObject{
		Body:          obj.Body,
		ContentType:   obj.ContentType,
		ContentLength: obj.ContentLength,
		Upload:        record,
	}, nil
}

// ActiveUploadHash returns the SHA-256 hash recorded for an active upload.
func ActiveUploadHash(ctx context.Context, uploadID uint64) (string, error) {
	record, err := GetActiveUpload(ctx, uploadID)
	if err != nil {
		return "", err
	}
	hash := strings.TrimSpace(record.Hash)
	if hash == "" {
		return "", errors.New("upload hash is empty")
	}
	return hash, nil
}
