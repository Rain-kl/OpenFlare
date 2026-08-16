// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package ingest

import (
	"context"
	"errors"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"gorm.io/gorm"
)

// Ingest stores or resolves an upload using the configured policy and side effects.
func Ingest(ctx context.Context, req Request) (Result, error) {
	normalizeRequest(&req)
	if req.Hash == "" {
		return Result{}, errors.New("ingest hash is required")
	}
	if req.Reader == nil {
		return Result{}, errors.New("ingest reader is required")
	}
	if req.Size < 0 {
		return Result{}, errors.New("ingest size must be non-negative")
	}

	switch req.Policy {
	case PolicyDedupNewRecord, PolicyResolveExisting:
		return ingestWithHashPolicy(ctx, req)
	case PolicyCreate:
		return createNewUpload(ctx, req)
	default:
		return Result{}, errors.New("unsupported ingest policy")
	}
}

// FindByHash returns a reusable active upload with the same hash and size.
func FindByHash(ctx context.Context, hash string, size int64) (model.Upload, error) {
	return repository.FindReusableUploadByHash(ctx, hash, size)
}

func ingestWithHashPolicy(ctx context.Context, req Request) (Result, error) {
	existing, err := repository.FindReusableUploadByHash(ctx, req.Hash, req.Size)
	if err == nil {
		switch req.Policy {
		case PolicyResolveExisting:
			return Result{
				Upload:   existing,
				Resolved: true,
			}, nil
		case PolicyDedupNewRecord:
			if uploadstorageReadOnly(ctx) {
				return Result{}, ErrStorageReadOnly
			}
			return createDedupRecord(ctx, existing, req)
		case PolicyCreate:
			// 忽略已存在记录，走下方的新建流程。
		}
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return Result{}, err
	}

	return createNewUpload(ctx, req)
}
