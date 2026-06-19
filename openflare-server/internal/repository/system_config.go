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

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
)

const (
	errDatabaseNotInitialized       = "database not initialized"
	errConfigIntParseFailed         = "配置 %s 的值 '%s' 无法转换为整数: %w"
	errConfigDecimalParseFailed     = "配置 %s 的值 '%s' 无法转换为decimal: %w"
	errConfigBoolParseFailed        = "配置 %s 的值 '%s' 无法转换为布尔值: %w"
	errParseMenuDisplayConfigFailed = "解析目录显示配置失败: %w"
)

// GetSystemConfigByKey 通过 key 查询配置（带 RAM + Redis 缓存）。
func GetSystemConfigByKey(ctx context.Context, key string) (model.SystemConfig, error) {
	ensureSystemConfigCacheListener()

	if cached, ok := systemConfigRAMCache.GetIfPresent(key); ok {
		return cloneSystemConfig(cached), nil
	}

	var sc model.SystemConfig
	if db.Redis != nil {
		if err := db.HGetJSON(ctx, SystemConfigRedisHashKey, key, &sc); err == nil {
			systemConfigRAMCache.Set(key, cloneSystemConfig(sc))
			return sc, nil
		} else if !errors.Is(err, redis.Nil) {
			return model.SystemConfig{}, err
		}
	}

	database := db.DB(ctx)
	if database == nil {
		return model.SystemConfig{}, errors.New(errDatabaseNotInitialized)
	}

	if err := database.Where("key = ?", key).First(&sc).Error; err != nil {
		return model.SystemConfig{}, err
	}

	populateSystemConfigCache(ctx, sc)
	return sc, nil
}

// ListSystemConfigsByKeys loads multiple config keys in one database round trip.
func ListSystemConfigsByKeys(ctx context.Context, keys []string) (map[string]model.SystemConfig, error) {
	if len(keys) == 0 {
		return map[string]model.SystemConfig{}, nil
	}

	ensureSystemConfigCacheListener()

	result := make(map[string]model.SystemConfig, len(keys))
	missing := make([]string, 0, len(keys))
	for _, key := range keys {
		if cached, ok := systemConfigRAMCache.GetIfPresent(key); ok {
			result[key] = cloneSystemConfig(cached)
			continue
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
		populateSystemConfigCache(ctx, configs[i])
		result[configs[i].Key] = cloneSystemConfig(configs[i])
	}

	return result, nil
}

// InvalidateVisibleSystemConfigsCache clears the cached public config list.
func InvalidateVisibleSystemConfigsCache(ctx context.Context) error {
	if db.Redis == nil {
		return nil
	}
	return db.Redis.Del(ctx, db.PrefixedKey(SystemConfigVisibleListRedisKey)).Err()
}

// ListVisibleSystemConfigs 查询所有可通过公共配置接口暴露的配置（带 Redis 列表缓存）。
func ListVisibleSystemConfigs(ctx context.Context) ([]model.SystemConfig, error) {
	if db.Redis != nil {
		var cached []model.SystemConfig
		if err := db.GetJSON(ctx, SystemConfigVisibleListRedisKey, &cached); err == nil {
			return cached, nil
		} else if !errors.Is(err, redis.Nil) {
			return nil, err
		}
	}

	database := db.DB(ctx)
	if database == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}

	var configs []model.SystemConfig
	if err := database.Where("visibility = ?", model.ConfigVisibilityVisible).Find(&configs).Error; err != nil {
		return nil, err
	}

	if db.Redis != nil {
		_ = db.SetJSON(ctx, SystemConfigVisibleListRedisKey, configs, 0)
	}

	return configs, nil
}

// GetIntByKey 通过 key 查询配置并转换为 int 类型。
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

// GetDecimalByKey 通过 key 查询配置并转换为 decimal.Decimal 类型。
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

// GetBoolByKey 通过 key 查询配置并转换为 bool 类型。
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

// GetMenuDisplayConfig 获取目录显示配置，解析为 map[string]bool。
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
