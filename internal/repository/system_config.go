// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package repository provides data access with caching and persistence boundaries.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/shopspring/decimal"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/cache/ram"
)

const (
	configTypeSystem                = "system"
	errDatabaseNotInitialized       = "database not initialized"
	errConfigIntParseFailed         = "配置 %s 的值 '%s' 无法转换为整数: %w"
	errConfigDecimalParseFailed     = "配置 %s 的值 '%s' 无法转换为decimal: %w"
	errConfigBoolParseFailed        = "配置 %s 的值 '%s' 无法转换为布尔值: %w"
	errParseMenuDisplayConfigFailed = "解析目录显示配置失败: %w"
)

// PreheatSystemConfigs loads all system configs from database.
// This function strictly performs database read and does not perform any cache read or write operations.
func PreheatSystemConfigs(ctx context.Context) ([]model.SystemConfig, error) {
	database := db.DB(ctx)
	if database == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}

	var configs []model.SystemConfig
	if err := database.Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

// PreheatSystemConfigByKey loads a single config key from database.
// This function strictly performs database read and does not perform any cache read or write operations.
func PreheatSystemConfigByKey(ctx context.Context, key string) (model.SystemConfig, error) {
	database := db.DB(ctx)
	if database == nil {
		return model.SystemConfig{}, errors.New(errDatabaseNotInitialized)
	}

	var sc model.SystemConfig
	if err := database.Where("key = ?", key).First(&sc).Error; err != nil {
		return model.SystemConfig{}, err
	}
	return sc, nil
}

// GetSystemConfigByGroup queries a configuration by Type and Key.
func GetSystemConfigByGroup(ctx context.Context, configType string, key string) (model.SystemConfig, error) {
	ensureSystemConfigCacheListener()

	if item, ok := ram.Get(configType, key); ok {
		var sc model.SystemConfig
		if err := json.Unmarshal([]byte(item.Value), &sc); err == nil {
			return sc, nil
		}
	}

	database := db.DB(ctx)
	if database == nil {
		return model.SystemConfig{}, errors.New(errDatabaseNotInitialized)
	}

	var sc model.SystemConfig
	if err := database.Where("key = ?", key).First(&sc).Error; err != nil {
		return model.SystemConfig{}, err
	}

	// Populate local cache directly on query miss
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

// GetSystemConfigByKey queries config by key (delegates to Type "config").
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

	database := db.DB(ctx)
	if database == nil {
		return nil, errors.New(errDatabaseNotInitialized)
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

	database := db.DB(ctx)
	if database == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}

	var configs []model.SystemConfig
	if err := database.Where("visibility = ?", model.ConfigVisibilityVisible).Find(&configs).Error; err != nil {
		return nil, err
	}

	// Populate visible configs to local cache store
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
		return 0, fmt.Errorf(errConfigIntParseFailed, key, sc.Value, err)
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
		return decimal.Zero, fmt.Errorf(errConfigDecimalParseFailed, key, sc.Value, err)
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
		return false, fmt.Errorf(errConfigBoolParseFailed, key, sc.Value, err)
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
		return nil, fmt.Errorf(errParseMenuDisplayConfigFailed, err)
	}

	return config, nil
}
