// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package repository provides persistence operations for the admin domain.
package repository

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/pkg/cache/ram"
	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/admin/errs"
	"Wavelet/plugins/domain/admin/model"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	configTypeSystem = "system"
)

var (
	repoMu       sync.RWMutex
	dbService    contracts.DBService
	cacheService contracts.CacheService
)

// SetDBService injects the DBService contract.
func SetDBService(s contracts.DBService) {
	repoMu.Lock()
	defer repoMu.Unlock()
	dbService = s
}

// SetCacheService injects the CacheService contract.
func SetCacheService(s contracts.CacheService) {
	repoMu.Lock()
	defer repoMu.Unlock()
	cacheService = s
}

// ResetServices clears injected persistence services.
func ResetServices() {
	repoMu.Lock()
	defer repoMu.Unlock()
	dbService = nil
	cacheService = nil
}

// GetDB returns the GORM DB instance bound to the context if available.
func GetDB(ctx context.Context) *gorm.DB {
	if s, err := core.InjectFrom[contracts.DBService](ctx); err == nil && s != nil {
		return s.DB(ctx)
	}
	repoMu.RLock()
	defer repoMu.RUnlock()
	if dbService == nil {
		return nil
	}
	return dbService.DB(ctx)
}

// GetCache returns the unified CacheService instance.
func GetCache(ctx context.Context) contracts.CacheService {
	if s, err := core.InjectFrom[contracts.CacheService](ctx); err == nil && s != nil {
		return s
	}
	repoMu.RLock()
	defer repoMu.RUnlock()
	return cacheService
}

// PreheatSystemConfigs loads all system configs from database.
func PreheatSystemConfigs(ctx context.Context) ([]model.SystemConfig, error) {
	database := GetDB(ctx)
	if database == nil {
		return nil, errors.New(errs.ErrDatabaseNotInitialized)
	}

	var configs []model.SystemConfig
	if err := database.Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

// PreheatSystemConfigByKey loads a single config key from database.
func PreheatSystemConfigByKey(ctx context.Context, key string) (model.SystemConfig, error) {
	database := GetDB(ctx)
	if database == nil {
		return model.SystemConfig{}, errors.New(errs.ErrDatabaseNotInitialized)
	}

	var sc model.SystemConfig
	if err := database.Where("key = ?", key).First(&sc).Error; err != nil {
		return model.SystemConfig{}, err
	}
	return sc, nil
}

// GetSystemConfigByGroup queries a configuration by Type and Key.
func GetSystemConfigByGroup(ctx context.Context, configType, key string) (model.SystemConfig, error) {
	ensureSystemConfigCacheListener()

	if item, ok := ram.Get(configType, key); ok {
		var sc model.SystemConfig
		if err := json.Unmarshal([]byte(item.Value), &sc); err == nil {
			return sc, nil
		}
	}

	database := GetDB(ctx)
	if database == nil {
		return model.SystemConfig{}, errors.New(errs.ErrDatabaseNotInitialized)
	}

	var sc model.SystemConfig
	if err := database.Where("key = ?", key).First(&sc).Error; err != nil {
		return model.SystemConfig{}, err
	}

	valBytes, err := json.Marshal(sc)
	if err == nil {
		ram.Set(ram.CacheItem{
			Key:   sc.Key,
			Value: string(valBytes),
			Type:  configType,
			TTL:   determineTTL(sc.Key),
		})
	}

	return sc, nil
}

// GetSystemConfigByKey queries config by key.
func GetSystemConfigByKey(ctx context.Context, key string) (model.SystemConfig, error) {
	return GetSystemConfigByGroup(ctx, ConfigCacheType, key)
}

// ListSystemConfigsByKeys loads multiple config keys.
func ListSystemConfigsByKeys(ctx context.Context, keys []string) (map[string]model.SystemConfig, error) {
	if len(keys) == 0 {
		return map[string]model.SystemConfig{}, nil
	}

	ensureSystemConfigCacheListener()

	result := make(map[string]model.SystemConfig, len(keys))
	missing := make([]string, 0, len(keys))

	for _, key := range keys {
		if item, ok := ram.Get(ConfigCacheType, key); ok {
			var sc model.SystemConfig
			if err := json.Unmarshal([]byte(item.Value), &sc); err == nil {
				result[key] = sc
				continue
			}
		}
		missing = append(missing, key)
	}

	if len(missing) == 0 {
		return result, nil
	}

	database := GetDB(ctx)
	if database == nil {
		return nil, errors.New(errs.ErrDatabaseNotInitialized)
	}

	var configs []model.SystemConfig
	if err := database.Where("key IN ?", missing).Find(&configs).Error; err != nil {
		return nil, err
	}

	for i := range configs {
		valBytes, err := json.Marshal(configs[i])
		if err == nil {
			ram.Set(ram.CacheItem{
				Key:   configs[i].Key,
				Value: string(valBytes),
				Type:  ConfigCacheType,
				TTL:   determineTTL(configs[i].Key),
			})
		}
		result[configs[i].Key] = configs[i]
	}

	return result, nil
}

// InvalidateVisibleSystemConfigsCache clears the cached public config list.
func InvalidateVisibleSystemConfigsCache(ctx context.Context) error {
	return InvalidateAllSystemConfigCaches(ctx)
}

// ListVisibleSystemConfigs queries visible configs using local cache store.
func ListVisibleSystemConfigs(ctx context.Context) ([]model.SystemConfig, error) {
	ensureSystemConfigCacheListener()

	items := ram.GetTypeItems(ConfigCacheType)
	if len(items) > 0 {
		var list []model.SystemConfig
		for _, item := range items {
			var sc model.SystemConfig
			if err := json.Unmarshal([]byte(item.Value), &sc); err == nil {
				if sc.Visibility == model.ConfigVisibilityVisible {
					list = append(list, sc)
				}
			}
		}
		return list, nil
	}

	database := GetDB(ctx)
	if database == nil {
		return nil, errors.New(errs.ErrDatabaseNotInitialized)
	}

	var configs []model.SystemConfig
	if err := database.Where("visibility = ?", model.ConfigVisibilityVisible).Find(&configs).Error; err != nil {
		return nil, err
	}

	for _, cfg := range configs {
		valBytes, err := json.Marshal(cfg)
		if err == nil {
			ram.Set(ram.CacheItem{
				Key:   cfg.Key,
				Value: string(valBytes),
				Type:  ConfigCacheType,
				TTL:   determineTTL(cfg.Key),
			})
		}
	}

	return configs, nil
}

// GetIntByKey queries config and converts to int.
func GetIntByKey(ctx context.Context, key string) (int, error) {
	sc, err := GetSystemConfigByKey(ctx, key)
	if err != nil {
		return 0, err
	}

	value, err := strconv.Atoi(sc.Value)
	if err != nil {
		return 0, fmt.Errorf(errs.ErrConfigIntParseFailed, key, sc.Value, err)
	}

	return value, nil
}

// GetDecimalByKey queries config and converts to decimal.Decimal.
func GetDecimalByKey(ctx context.Context, key string, precision int32) (decimal.Decimal, error) {
	sc, err := GetSystemConfigByKey(ctx, key)
	if err != nil {
		return decimal.Zero, err
	}

	value, err := decimal.NewFromString(sc.Value)
	if err != nil {
		return decimal.Zero, fmt.Errorf(errs.ErrConfigDecimalParseFailed, key, sc.Value, err)
	}

	return value.Truncate(precision), nil
}

// GetBoolByKey queries config and converts to bool.
func GetBoolByKey(ctx context.Context, key string) (bool, error) {
	sc, err := GetSystemConfigByKey(ctx, key)
	if err != nil {
		return false, err
	}

	value, err := strconv.ParseBool(sc.Value)
	if err != nil {
		return false, fmt.Errorf(errs.ErrConfigBoolParseFailed, key, sc.Value, err)
	}

	return value, nil
}

// GetMenuDisplayConfig queries and parses menu config.
func GetMenuDisplayConfig(ctx context.Context) (map[string]bool, error) {
	sc, err := GetSystemConfigByKey(ctx, model.ConfigKeyMenuDisplayConfig)
	if err != nil {
		return nil, err
	}

	config := make(map[string]bool)
	if sc.Value == "" || sc.Value == "{}" {
		return config, nil
	}

	if err := json.Unmarshal([]byte(sc.Value), &config); err != nil {
		return nil, fmt.Errorf(errs.ErrParseMenuDisplayConfigFailed, err)
	}

	return config, nil
}

// ListAdminSystemConfigs returns all configs, optionally filtered by type.
func ListAdminSystemConfigs(ctx context.Context, configType string) ([]model.SystemConfig, error) {
	query := GetDB(ctx).Order("created_at DESC")
	if configType != "" {
		query = query.Where("type = ?", configType)
	}
	var configs []model.SystemConfig
	if err := query.Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

// GetAdminSystemConfigByKey loads a config directly from DB.
func GetAdminSystemConfigByKey(ctx context.Context, key string) (model.SystemConfig, error) {
	var config model.SystemConfig
	if err := GetDB(ctx).Where("key = ?", key).First(&config).Error; err != nil {
		return model.SystemConfig{}, err
	}
	return config, nil
}

// SystemConfigExists reports whether a config key already exists.
func SystemConfigExists(ctx context.Context, key string) (bool, error) {
	var existing model.SystemConfig
	err := GetDB(ctx).Where("key = ?", key).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// CreateSystemConfigRecord persists a new system config row.
func CreateSystemConfigRecord(ctx context.Context, config *model.SystemConfig) error {
	return GetDB(ctx).Create(config).Error
}

// UpdateSystemConfigFields applies partial updates to a system config row.
func UpdateSystemConfigFields(ctx context.Context, config *model.SystemConfig, updates map[string]any) error {
	return GetDB(ctx).Model(config).Updates(updates).Error
}

// UpdateSystemConfigTx applies the config row updates inside a transaction and, when
// resolveTaskType is not empty, marks that task type's failed executions as succeeded
// within the same transaction.
func UpdateSystemConfigTx(
	ctx context.Context,
	config *model.SystemConfig,
	updates map[string]any,
	resolveTaskType string,
	resolveResult string,
) error {
	database := GetDB(ctx)
	if database == nil {
		return errors.New(errs.ErrDatabaseServiceNotAvailable)
	}

	return database.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(config).Updates(updates).Error; err != nil {
			return err
		}
		if resolveTaskType == "" {
			return nil
		}
		if err := MarkFailedTaskExecutionsSucceededTx(tx, resolveTaskType, resolveResult, time.Now()); err != nil {
			logger.ErrorF(ctx, errs.ErrAutoResolveMigrationTaskFailed, err)
		}
		return nil
	})
}

// SaveOrUpdateSystemConfig creates or updates a config row and invalidates cache.
func SaveOrUpdateSystemConfig(ctx context.Context, key, value string) error {
	var sc model.SystemConfig
	err := GetDB(ctx).Where("key = ?", key).First(&sc).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		sc = model.SystemConfig{
			Key:        key,
			Value:      value,
			Type:       configTypeSystem,
			Visibility: model.ConfigVisibilityHidden,
		}
		if err := GetDB(ctx).Create(&sc).Error; err != nil {
			return err
		}
	} else {
		sc.Value = value
		if err := GetDB(ctx).Save(&sc).Error; err != nil {
			return err
		}
	}
	return InvalidateSystemConfigCache(ctx, key)
}

// CountActiveUploads counts non-deleted rows of the storage upload table. A missing
// database handle yields zero, matching the pre-refactor guard behaviour.
func CountActiveUploads(ctx context.Context) (int64, error) {
	gormDB := GetDB(ctx)
	if gormDB == nil {
		return 0, nil
	}

	var uploadCount int64
	if err := gormDB.Table("w_uploads").
		Where("status != ?", "deleted").
		Count(&uploadCount).Error; err != nil {
		return 0, fmt.Errorf(errs.ErrCheckExistingUploadsFailed, err)
	}
	return uploadCount, nil
}
