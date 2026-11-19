// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// ListConfigVersionSummaries returns config version summaries ordered by created_at desc.
func ListConfigVersionSummaries(ctx context.Context) ([]*model.ConfigVersionSummary, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var versions []*model.ConfigVersionSummary
	err := conn.Model(&model.ConfigVersion{}).
		Select("version", "checksum", "is_active", "engine", "created_by", "created_at").
		Order("created_at desc, version desc").
		Find(&versions).Error
	return versions, err
}

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
	return GetActiveConfigVersionByEngine(ctx, engineOpenResty)
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

const engineOpenResty = "openresty"

// GetActivePingoraConfigVersion returns the active Pingora release.
func GetActivePingoraConfigVersion(ctx context.Context) (*model.ConfigVersion, error) {
	return GetActiveConfigVersionByEngine(ctx, "pingora")
}

// GetLatestConfigVersionByPrefix returns the latest version string matching a date prefix.
func GetLatestConfigVersionByPrefix(ctx context.Context, prefix string) (string, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return "", errors.New(errDatabaseNotInitialized)
	}
	var version model.ConfigVersion
	err := conn.Model(&model.ConfigVersion{}).
		Select("version").
		Where("version LIKE ?", prefix+"-%").
		Order("version desc").
		First(&version).Error
	if err != nil {
		return "", err
	}
	return version.Version, nil
}

// PublishConfigVersionTx atomically advances the supplied engine release groups.
func PublishConfigVersionTx(ctx context.Context, versions ...*model.ConfigVersion) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		for _, version := range versions {
			if version == nil {
				continue
			}
			if version.Engine == "" {
				version.Engine = engineOpenResty
			}
			if err := deactivateActiveConfigVersion(tx, version.Engine); err != nil {
				return err
			}
			if version.Engine == enginePingora {
				if err := assignPingoraReleaseSequence(tx, version); err != nil {
					return err
				}
			}
			if err := tx.Create(version).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

const enginePingora = "pingora"

func deactivateActiveConfigVersion(tx *gorm.DB, engine string) error {
	return tx.Model(&model.ConfigVersion{}).
		Where("is_active = ? AND engine = ?", true, engine).
		Update("is_active", false).Error
}

func assignPingoraReleaseSequence(tx *gorm.DB, version *model.ConfigVersion) error {
	var latest int64
	if err := tx.Model(&model.ConfigVersion{}).Where("engine = ?", enginePingora).
		Select("COALESCE(MAX(release_sequence), 0)").Scan(&latest).Error; err != nil {
		return err
	}
	version.ReleaseSequence = latest + 1
	return nil
}

// ActivateConfigVersionTx restores an existing snapshot. Pingora rollbacks are new releases.
func ActivateConfigVersionTx(ctx context.Context, version string) (*model.ConfigVersion, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var target model.ConfigVersion
	err := conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&target, "version = ?", version).Error; err != nil {
			return err
		}
		if target.Engine == enginePingora && target.SourceVersion == "" {
			return errors.New("the Edge release does not reference a shared snapshot")
		}
		if err := deactivateActiveConfigVersion(tx, target.Engine); err != nil {
			return err
		}
		target.IsActive = true
		if target.Engine != enginePingora {
			return tx.Model(&model.ConfigVersion{}).Where("version = ?", version).Update("is_active", true).Error
		}
		if err := assignPingoraReleaseSequence(tx, &target); err != nil {
			return err
		}
		target.Version = fmt.Sprintf("pingora-r%d", target.ReleaseSequence)
		target.CreatedAt = time.Time{}
		return tx.Create(&target).Error
	})
	if err != nil {
		return nil, err
	}
	return &target, nil
}

// DeleteConfigVersionsByVersions removes config versions by versions.
func DeleteConfigVersionsByVersions(ctx context.Context, versions []string) (int64, error) {
	if len(versions) == 0 {
		return 0, nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return 0, errors.New(errDatabaseNotInitialized)
	}
	// Delete release records before considering their snapshots. A retained
	// release owns a reference even when its source is older than keepCount.
	var deleted int64
	err := conn.Transaction(func(tx *gorm.DB) error {
		references := tx.Model(&model.ConfigVersion{}).Select("source_version").Where("source_version <> ''")
		result := tx.Where("version IN ? AND is_active = ? AND version NOT IN (?)", versions, false, references).
			Delete(&model.ConfigVersion{})
		deleted = result.RowsAffected
		return result.Error
	})
	return deleted, err
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
