// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

// ListAdminSystemConfigs returns all configs, optionally filtered by type.
func ListAdminSystemConfigs(ctx context.Context, configType string) ([]model.SystemConfig, error) {
	query := db.DB(ctx).Order("created_at DESC")
	if configType != "" {
		query = query.Where("type = ?", configType)
	}
	var configs []model.SystemConfig
	if err := query.Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

// GetAdminSystemConfigByKey loads a config directly from PostgreSQL.
func GetAdminSystemConfigByKey(ctx context.Context, key string) (model.SystemConfig, error) {
	var config model.SystemConfig
	if err := db.DB(ctx).Where("key = ?", key).First(&config).Error; err != nil {
		return model.SystemConfig{}, err
	}
	return config, nil
}

// SystemConfigExists reports whether a config key already exists.
func SystemConfigExists(ctx context.Context, key string) (bool, error) {
	var existing model.SystemConfig
	err := db.DB(ctx).Where("key = ?", key).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// CreateSystemConfig persists a new system config row.
func CreateSystemConfig(ctx context.Context, config *model.SystemConfig) error {
	return db.DB(ctx).Create(config).Error
}

// UpdateSystemConfigFields applies partial updates to a system config row.
func UpdateSystemConfigFields(ctx context.Context, config *model.SystemConfig, updates map[string]any) error {
	return db.DB(ctx).Model(config).Updates(updates).Error
}

// SaveOrUpdateSystemConfig creates or updates a config row and invalidates cache.
func SaveOrUpdateSystemConfig(ctx context.Context, key, value string) error {
	var sc model.SystemConfig
	err := db.DB(ctx).Where("key = ?", key).First(&sc).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		sc = model.SystemConfig{
			Key:        key,
			Value:      value,
			Type:       "system",
			Visibility: model.ConfigVisibilityHidden,
		}
		if err := db.DB(ctx).Create(&sc).Error; err != nil {
			return err
		}
	} else {
		sc.Value = value
		if err := db.DB(ctx).Save(&sc).Error; err != nil {
			return err
		}
	}
	return InvalidateSystemConfigCache(ctx, key)
}
