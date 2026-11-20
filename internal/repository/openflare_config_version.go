// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// GetConfigVersionByVersion returns a config version by version string.
func GetConfigVersionByVersion(ctx context.Context, version string) (*model.ConfigVersion, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var cv model.ConfigVersion
	if err := conn.First(&cv, "version = ?", version).Error; err != nil {
		return nil, err
	}
	return &cv, nil
}

// GetConfigVersionSource resolves the shared snapshot referenced by a release.
func GetConfigVersionSource(ctx context.Context, version *model.ConfigVersion) (*model.ConfigVersion, error) {
	if version.SourceVersion == "" {
		return version, nil
	}
	return GetConfigVersionByVersion(ctx, version.SourceVersion)
}

// GetActiveConfigVersion returns the currently active config version.
func GetActiveConfigVersion(ctx context.Context) (*model.ConfigVersion, error) {
	active, err := GetActivePingoraConfigVersion(ctx)
	if err != nil {
		return nil, err
	}
	source, err := GetConfigVersionSource(ctx, active)
	if err != nil {
		return nil, err
	}
	active.SnapshotJSON = source.SnapshotJSON
	active.MainConfig = source.MainConfig
	active.RenderedConfig = source.RenderedConfig
	active.SupportFilesJSON = source.SupportFilesJSON
	return active, nil
}

// GetActiveConfigVersionByEngine returns the active version for one release engine.
func GetActiveConfigVersionByEngine(ctx context.Context, engine string) (*model.ConfigVersion, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var version model.ConfigVersion
	if err := conn.
		Where("is_active = ? AND engine = ?", true, engine).
		Order("release_sequence desc, version desc").
		First(&version).Error; err != nil {
		return nil, err
	}
	return &version, nil
}

// GetActivePingoraConfigVersion returns the active Pingora release.
func GetActivePingoraConfigVersion(ctx context.Context) (*model.ConfigVersion, error) {
	return GetActiveConfigVersionByEngine(ctx, "pingora")
}

// GetActivePingoraConfigMetadata reads only the fields carried in a configuration manifest.
func GetActivePingoraConfigMetadata(ctx context.Context) (*model.ConfigVersion, error) {
	conn := db.DB(db.WithPrimaryReads(ctx))
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var version model.ConfigVersion
	err := conn.Select("version", "checksum", "engine", "release_sequence", "payload_checksum").
		Where("is_active = ? AND engine = ?", true, enginePingora).
		Order("release_sequence desc, version desc").Take(&version).Error
	return &version, err
}

const enginePingora = "pingora"

func assignPingoraReleaseSequence(tx *gorm.DB, version *model.ConfigVersion) error {
	var latest int64
	if err := tx.Model(&model.ConfigVersion{}).Where("engine = ?", enginePingora).
		Select("COALESCE(MAX(release_sequence), 0)").Scan(&latest).Error; err != nil {
		return err
	}
	version.ReleaseSequence = latest + 1
	return nil
}

// ListEnabledProxyRoutes returns enabled proxy routes ordered by id asc.
func ListEnabledProxyRoutes(ctx context.Context) ([]*model.ProxyRoute, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var routes []*model.ProxyRoute
	if err := conn.Where("enabled = ?", true).Order("id asc").Find(&routes).Error; err != nil {
		return nil, err
	}
	return routes, nil
}
