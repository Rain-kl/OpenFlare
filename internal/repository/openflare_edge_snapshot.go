// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"fmt"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

const edgeSnapshotAuthor = "system:edge-snapshot"

// GetEdgeSnapshotState reads the latest committed source revision from the primary.
func GetEdgeSnapshotState(ctx context.Context) (model.EdgeSnapshotState, error) {
	conn := db.DB(db.WithPrimaryReads(ctx))
	var state model.EdgeSnapshotState
	if conn == nil {
		return state, errors.New(errDatabaseNotInitialized)
	}
	err := conn.First(&state, 1).Error
	return state, err
}

// LoadEdgeSnapshotSettings reads the authoritative values used by the snapshot compiler.
func LoadEdgeSnapshotSettings(ctx context.Context, keys []string) (map[string]string, error) {
	conn := db.DB(db.WithPrimaryReads(ctx))
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var configs []model.SystemConfig
	err := conn.Select("key", "value").Where("key IN ?", keys).Find(&configs).Error
	values := make(map[string]string, len(configs))
	for _, config := range configs {
		values[config.Key] = config.Value
	}
	return values, err
}

// StoreEdgeSnapshot publishes a compiled snapshot only while its source revision
// is current. Concurrent compilations of the same revision cannot replace it.
func StoreEdgeSnapshot(ctx context.Context, revision int64, version *model.ConfigVersion) (bool, error) {
	conn := db.DB(db.WithPrimaryReads(ctx))
	if conn == nil {
		return false, errors.New(errDatabaseNotInitialized)
	}
	changed := false
	err := conn.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.EdgeSnapshotState{}).
			Where("id = 1 AND source_revision = ? AND compiled_revision < ?", revision, revision).
			UpdateColumn("compiled_revision", revision)
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}

		var active model.ConfigVersion
		result = tx.Select("payload_checksum").
			Where("is_active = ? AND engine = ?", true, enginePingora).Limit(1).Find(&active)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 0 && active.PayloadChecksum == version.PayloadChecksum {
			return nil
		}
		if err := tx.Model(&model.ConfigVersion{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
			return err
		}
		if err := assignPingoraReleaseSequence(tx, version); err != nil {
			return err
		}
		version.Engine = enginePingora
		version.IsActive = true
		version.CreatedBy = edgeSnapshotAuthor
		version.Version = fmt.Sprintf("edge-live-r%d", version.ReleaseSequence)
		if err := tx.Create(version).Error; err != nil {
			return err
		}
		changed = true
		// Nodes persist their own last healthy snapshot. Only the current automatically
		// assembled payload needs to be retained by the control plane.
		return tx.Where("is_active = ? AND created_by = ?", false, edgeSnapshotAuthor).Delete(&model.ConfigVersion{}).Error
	})
	return changed && err == nil, err
}
