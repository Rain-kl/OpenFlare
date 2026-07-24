// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"

	"github.com/Rain-kl/Wavelet/internal/infra/objectstore"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

// ReadOnly checks if the storage system is in read-only maintenance mode.
func ReadOnly(ctx context.Context) bool {
	state := LoadMigrationAccessState(ctx)
	if state.LoadErr != nil {
		logger.ErrorF(ctx, "读取存储维护状态失败: %v", state.LoadErr)
		return true
	}
	return state.ReadOnly
}

// OpenStoredObject opens a stored upload object from the active storage backend.
func OpenStoredObject(ctx context.Context, upload *model.Upload) (*objectstore.Object, error) {
	_, backend, err := objectstore.Active(ctx)
	if err != nil {
		return nil, err
	}
	return backend.Get(ctx, upload.FilePath)
}
