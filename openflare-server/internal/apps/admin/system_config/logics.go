// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package system_config

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/storage"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"gorm.io/gorm"
)

func createSystemConfig(ctx context.Context, req CreateSystemConfigRequest) error {
	exists, err := repository.SystemConfigExists(ctx, req.Key)
	if err != nil {
		return err
	}
	if exists {
		return errors.New(ConfigKeyExists)
	}

	config := model.SystemConfig{
		Key:         req.Key,
		Value:       req.Value,
		Type:        req.Type,
		Visibility:  req.Visibility,
		Description: req.Description,
	}
	if err := repository.CreateSystemConfig(ctx, &config); err != nil {
		return err
	}

	invalidateSystemConfigCaches(ctx, req.Key)
	if err := repository.InvalidateVisibleSystemConfigsCache(ctx); err != nil {
		logger.WarnF(ctx, "清理公共配置列表缓存失败: %v", err)
	}
	return nil
}

func listSystemConfigs(ctx context.Context, configType string) ([]model.SystemConfig, error) {
	return repository.ListAdminSystemConfigs(ctx, configType)
}

func getSystemConfig(ctx context.Context, key string) (model.SystemConfig, error) {
	return repository.GetAdminSystemConfigByKey(ctx, key)
}

func updateSystemConfig(ctx context.Context, key string, req UpdateSystemConfigRequest) error {
	config, err := repository.GetAdminSystemConfigByKey(ctx, key)
	if err != nil {
		return err
	}

	var originalDriver storage.Driver
	if key == model.ConfigKeyStorageConfig {
		var currentCfg storage.Config
		if err := json.Unmarshal([]byte(config.Value), &currentCfg); err == nil {
			originalDriver = currentCfg.Driver
		}

		validatedVal, err := validateAndMergeStorageConfig(ctx, req.Value, config.Value)
		if err != nil {
			return err
		}
		req.Value = validatedVal
	}

	if err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"description": req.Description,
		}
		if req.Visibility != nil {
			updates["visibility"] = *req.Visibility
			config.Visibility = *req.Visibility
		}
		if key != model.ConfigKeySMTPPassword || req.Value != maskedConfigValue {
			updates["value"] = req.Value
			config.Value = req.Value
		}
		if err := tx.Model(&config).Updates(updates).Error; err != nil {
			return err
		}
		resolveStorageMigrationTasksOnDirectDriverUpdate(ctx, tx, key, originalDriver, req.Value)
		return nil
	}); err != nil {
		return err
	}

	invalidateCachesAfterConfigUpdate(ctx, key)
	return nil
}

func resolveStorageMigrationTasksOnDirectDriverUpdate(
	ctx context.Context,
	tx *gorm.DB,
	key string,
	originalDriver storage.Driver,
	newValue string,
) {
	if key != model.ConfigKeyStorageConfig || originalDriver == "" {
		return
	}

	var newCfg storage.Config
	if err := json.Unmarshal([]byte(newValue), &newCfg); err != nil {
		return
	}
	if newCfg.Driver != originalDriver {
		return
	}

	if err := tx.Model(&model.TaskExecution{}).
		Where("task_type = ? AND status = ?", "storage:migrate", model.TaskExecutionStatusFailed).
		Updates(map[string]any{
			"status":      model.TaskExecutionStatusSucceeded,
			"result":      "存储配置直接更新，故障迁移任务自动标记为已解决",
			"finished_at": time.Now(),
		}).Error; err != nil {
		logger.ErrorF(ctx, "自动更新迁移任务状态失败: %v", err)
	}
}
