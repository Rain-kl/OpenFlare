// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package storage provides upload storage backend operations and migration state.
package storage

import (
	"context"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/upload/shared"
	"github.com/Rain-kl/Wavelet/internal/infra/objectstore"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// MigrationAccessState captures cached migration maintenance state.
type MigrationAccessState struct {
	ReadOnly  bool
	Target    objectstore.Config
	HasTarget bool
	TargetErr error
	LoadErr   error
}

var (
	migrationAccessMu        sync.RWMutex
	migrationAccessCached    MigrationAccessState
	migrationAccessValid     bool
	migrationAccessCheckedAt time.Time
)

// ResetMigrationAccessCache clears the in-process migration access cache.
func ResetMigrationAccessCache() {
	migrationAccessMu.Lock()
	migrationAccessValid = false
	migrationAccessMu.Unlock()
}

// LoadMigrationAccessState returns cached migration maintenance state.
func LoadMigrationAccessState(ctx context.Context) MigrationAccessState {
	migrationAccessMu.RLock()
	if migrationAccessValid && time.Since(migrationAccessCheckedAt) < time.Duration(shared.AccessCacheTTL)*time.Second {
		state := migrationAccessCached
		migrationAccessMu.RUnlock()
		return state
	}
	migrationAccessMu.RUnlock()

	migrationAccessMu.Lock()
	defer migrationAccessMu.Unlock()

	if migrationAccessValid && time.Since(migrationAccessCheckedAt) < time.Duration(shared.AccessCacheTTL)*time.Second {
		return migrationAccessCached
	}

	migrationAccessCached = buildMigrationAccessState(ctx)
	migrationAccessValid = true
	migrationAccessCheckedAt = time.Now()
	return migrationAccessCached
}

func buildMigrationAccessState(ctx context.Context) MigrationAccessState {
	execution, ok, err := LatestMigrationExecution(ctx)
	if err != nil {
		return MigrationAccessState{LoadErr: err, ReadOnly: true}
	}
	if !ok {
		return MigrationAccessState{}
	}

	state := MigrationAccessState{
		ReadOnly: execution.Status != model.TaskExecutionStatusSucceeded,
	}
	if execution.Status == model.TaskExecutionStatusSucceeded {
		return state
	}

	target, err := ParseMigrationTargetConfig(ctx, []byte(execution.Payload))
	if err != nil {
		state.TargetErr = err
		return state
	}

	state.Target = target
	state.HasTarget = true
	return state
}
