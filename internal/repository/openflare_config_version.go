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

// ListConfigVersionSummaries returns config version summaries ordered by created_at desc.
func ListConfigVersionSummaries(ctx context.Context) ([]*model.ConfigVersionSummary, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var versions []*model.ConfigVersionSummary
	err := conn.Model(&model.ConfigVersion{}).
		Select("version", "checksum", "is_active", "created_by", "created_at").
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

// GetActiveConfigVersion returns the currently active config version.
func GetActiveConfigVersion(ctx context.Context) (*model.ConfigVersion, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var version model.ConfigVersion
	if err := conn.Where("is_active = ?", true).Order("version desc").First(&version).Error; err != nil {
		return nil, err
	}
	return &version, nil
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

// CreateConfigVersion inserts a new config version record.
func CreateConfigVersion(ctx context.Context, version *model.ConfigVersion) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Create(version).Error
}

// PublishConfigVersionTx deactivates all versions and creates a new active version.
func PublishConfigVersionTx(ctx context.Context, version *model.ConfigVersion) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ConfigVersion{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
			return err
		}
		return tx.Create(version).Error
	})
}

// ActivateConfigVersionTx marks the given version active and deactivates others.
func ActivateConfigVersionTx(ctx context.Context, version string) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ConfigVersion{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
			return err
		}
		return tx.Model(&model.ConfigVersion{}).Where("version = ?", version).Update("is_active", true).Error
	})
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
	result := conn.Where("version IN ?", versions).Delete(&model.ConfigVersion{})
	return result.RowsAffected, result.Error
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
