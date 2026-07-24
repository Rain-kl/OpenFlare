// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"errors"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"gorm.io/gorm"
)

// ConfigVersionSummary is the list view for config versions.
type ConfigVersionSummary struct {
	ID        string    `json:"id" gorm:"-"`
	Version   string    `json:"version" gorm:"primaryKey;column:version"`
	Checksum  string    `json:"checksum"`
	IsActive  bool      `json:"is_active"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// AfterFind hook for ConfigVersionSummary.
func (cvs *ConfigVersionSummary) AfterFind(_ *gorm.DB) (err error) {
	cvs.ID = cvs.Version
	return
}

// ConfigVersion stores a published OpenResty configuration snapshot.
type ConfigVersion struct {
	ID               string    `json:"id" gorm:"-"`
	Version          string    `json:"version" gorm:"primaryKey;size:32;not null"`
	SnapshotJSON     string    `json:"snapshot_json" gorm:"type:text;not null"`
	MainConfig       string    `json:"main_config" gorm:"type:text;not null;default:''"`
	RenderedConfig   string    `json:"rendered_config" gorm:"type:text;not null"`
	SupportFilesJSON string    `json:"support_files_json" gorm:"type:text;not null;default:'[]'"`
	Checksum         string    `json:"checksum" gorm:"size:64;not null"`
	IsActive         bool      `json:"is_active" gorm:"not null;default:false;index"`
	CreatedBy        string    `json:"created_by" gorm:"size:64;not null"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// AfterFind hook for ConfigVersion.
func (cv *ConfigVersion) AfterFind(_ *gorm.DB) (err error) {
	cv.ID = cv.Version
	return
}

// AfterCreate hook for ConfigVersion.
func (cv *ConfigVersion) AfterCreate(_ *gorm.DB) (err error) {
	cv.ID = cv.Version
	return
}

// TableName returns the GORM table name.
func (ConfigVersion) TableName() string {
	return "of_config_versions"
}

// ListConfigVersionSummaries returns config version summaries ordered by created_at desc.
func ListConfigVersionSummaries(ctx context.Context) ([]*ConfigVersionSummary, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var versions []*ConfigVersionSummary
	err := conn.Model(&ConfigVersion{}).
		Select("version", "checksum", "is_active", "created_by", "created_at").
		Order("created_at desc, version desc").
		Find(&versions).Error
	return versions, err
}

// GetConfigVersionByVersion returns a config version by version string.
func GetConfigVersionByVersion(ctx context.Context, version string) (*ConfigVersion, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var cv ConfigVersion
	if err := conn.First(&cv, "version = ?", version).Error; err != nil {
		return nil, err
	}
	return &cv, nil
}

// GetActiveConfigVersion returns the currently active config version.
func GetActiveConfigVersion(ctx context.Context) (*ConfigVersion, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var version ConfigVersion
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
	var version ConfigVersion
	err := conn.Model(&ConfigVersion{}).
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
func CreateConfigVersion(ctx context.Context, version *ConfigVersion) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Create(version).Error
}

// PublishConfigVersionTx deactivates all versions and creates a new active version.
func PublishConfigVersionTx(ctx context.Context, version *ConfigVersion) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&ConfigVersion{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
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
		if err := tx.Model(&ConfigVersion{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
			return err
		}
		return tx.Model(&ConfigVersion{}).Where("version = ?", version).Update("is_active", true).Error
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
	result := conn.Where("version IN ?", versions).Delete(&ConfigVersion{})
	return result.RowsAffected, result.Error
}

// ListEnabledProxyRoutes returns enabled proxy routes ordered by id asc.
func ListEnabledProxyRoutes(ctx context.Context) ([]*ProxyRoute, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var routes []*ProxyRoute
	if err := conn.Where("enabled = ?", true).Order("id asc").Find(&routes).Error; err != nil {
		return nil, err
	}
	return routes, nil
}
