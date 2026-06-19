// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/cache/ram"
)

const (
	// SystemConfigInvalidationChannel broadcasts RAM cache eviction across nodes.
	SystemConfigInvalidationChannel = "system:config_invalidation"
	// SystemConfigRedisHashKey Redis Hash key，存储所有系统配置。
	SystemConfigRedisHashKey = "system:system_configs"
	// SystemConfigVisibleListRedisKey Redis key，缓存所有 visibility=1 的公共配置列表。
	SystemConfigVisibleListRedisKey = "system:visible_configs"

	systemConfigInvalidateAllToken = "*"
	systemConfigRAMMaximumSize     = 512
)

type systemConfigInvalidationMessage struct {
	Key string `json:"key"`
}

var (
	systemConfigRAMCache     = ram.MustNew[string, model.SystemConfig](ram.Options{MaximumSize: systemConfigRAMMaximumSize})
	systemConfigListenerOnce sync.Once
)

func ensureSystemConfigCacheListener() {
	systemConfigListenerOnce.Do(startSystemConfigCacheInvalidationListener)
}

func startSystemConfigCacheInvalidationListener() {
	if db.Redis == nil {
		return
	}

	go func() {
		pubsub := db.Redis.Subscribe(context.Background(), SystemConfigInvalidationChannel)
		defer func() {
			_ = pubsub.Close()
		}()

		for msg := range pubsub.Channel() {
			var payload systemConfigInvalidationMessage
			if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
				systemConfigRAMCache.InvalidateAll()
				continue
			}
			if payload.Key == "" || payload.Key == systemConfigInvalidateAllToken {
				systemConfigRAMCache.InvalidateAll()
				continue
			}
			systemConfigRAMCache.Invalidate(payload.Key)
		}
	}()
}

func cloneSystemConfig(sc model.SystemConfig) model.SystemConfig {
	return sc
}

func populateSystemConfigCache(ctx context.Context, sc model.SystemConfig) {
	systemConfigRAMCache.Set(sc.Key, cloneSystemConfig(sc))
	if db.Redis != nil {
		_ = db.HSetJSON(ctx, SystemConfigRedisHashKey, sc.Key, &sc)
	}
}

func publishSystemConfigRAMInvalidation(ctx context.Context, key string) {
	if db.Redis == nil {
		return
	}
	payload, err := json.Marshal(systemConfigInvalidationMessage{Key: key})
	if err != nil {
		return
	}
	_ = db.Redis.Publish(ctx, SystemConfigInvalidationChannel, payload).Err()
}

// InvalidateSystemConfigCache evicts one config key from local RAM and Redis.
func InvalidateSystemConfigCache(ctx context.Context, key string) error {
	ensureSystemConfigCacheListener()

	systemConfigRAMCache.Invalidate(key)
	if db.Redis != nil {
		if err := db.HDel(ctx, SystemConfigRedisHashKey, key); err != nil {
			return err
		}
	}
	publishSystemConfigRAMInvalidation(ctx, key)
	return nil
}

// InvalidateAllSystemConfigCaches evicts all config entries from local RAM and Redis.
func InvalidateAllSystemConfigCaches(ctx context.Context) error {
	ensureSystemConfigCacheListener()

	systemConfigRAMCache.InvalidateAll()
	if db.Redis != nil {
		if err := db.Redis.Del(ctx, db.PrefixedKey(SystemConfigRedisHashKey)).Err(); err != nil {
			return err
		}
	}
	publishSystemConfigRAMInvalidation(ctx, systemConfigInvalidateAllToken)
	return nil
}

// ResetSystemConfigRAMCacheForTest clears only the process-local RAM cache.
func ResetSystemConfigRAMCacheForTest() {
	systemConfigRAMCache.InvalidateAll()
}
