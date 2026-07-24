// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"gorm.io/gorm"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/pkg/cache/ram"
)

const (
	// SystemConfigBroadcastChannel broadcasts system config cache updates across nodes.
	SystemConfigBroadcastChannel = "system:config_broadcast"

	// SystemConfigInvalidationChannel is kept as an alias for backward compatibility.
	SystemConfigInvalidationChannel = SystemConfigBroadcastChannel

	// SystemConfigRedisHashKey is kept for backward compatibility in tests.
	SystemConfigRedisHashKey = "system:system_configs"
	// SystemConfigVisibleListRedisKey is kept for backward compatibility in tests.
	SystemConfigVisibleListRedisKey = "system:visible_configs"

	// ConfigCacheType is the cache type for all system configs.
	ConfigCacheType = "config"
)

type systemConfigBroadcastMessage struct {
	Type string `json:"type"`
	Key  string `json:"key"`
}

// ConfigLoader loads configuration data from the database.
type ConfigLoader struct{}

// LoadAll loads all system configs from database as CacheItems.
func (ConfigLoader) LoadAll(ctx context.Context, configType string) ([]ram.CacheItem, error) {
	configs, err := PreheatSystemConfigs(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]ram.CacheItem, len(configs))
	for i, cfg := range configs {
		valBytes, err := json.Marshal(cfg)
		if err != nil {
			return nil, err
		}
		items[i] = ram.CacheItem{
			Key:   cfg.Key,
			Value: string(valBytes),
			Type:  configType,
			TTL:   determineTTL(cfg.Key),
		}
	}
	return items, nil
}

// LoadOne loads a single system config from database as a CacheItem.
func (ConfigLoader) LoadOne(ctx context.Context, configType string, key string) (ram.CacheItem, error) {
	cfg, err := PreheatSystemConfigByKey(ctx, key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ram.CacheItem{}, ram.ErrNotFound
		}
		return ram.CacheItem{}, err
	}

	valBytes, err := json.Marshal(cfg)
	if err != nil {
		return ram.CacheItem{}, err
	}

	return ram.CacheItem{
		Key:   cfg.Key,
		Value: string(valBytes),
		Type:  configType,
		TTL:   determineTTL(cfg.Key),
	}, nil
}

var (
	systemConfigListenerOnce   sync.Once
	systemConfigListenerCtx    context.Context
	systemConfigListenerCancel context.CancelFunc
)

func ensureSystemConfigCacheListener() {
	systemConfigListenerOnce.Do(startSystemConfigCacheInvalidationListener)
}

func startSystemConfigCacheInvalidationListener() {
	if db.Redis == nil {
		return
	}

	systemConfigListenerCtx, systemConfigListenerCancel = context.WithCancel(context.Background())

	go func() {
		pubsub := db.Redis.Subscribe(systemConfigListenerCtx, SystemConfigBroadcastChannel)
		defer func() {
			_ = pubsub.Close()
		}()

		go func() {
			<-systemConfigListenerCtx.Done()
			_ = pubsub.Close()
		}()

		for msg := range pubsub.Channel() {
			var payload systemConfigBroadcastMessage
			if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
				ram.UpdateTypeItems(ConfigCacheType, nil)
				continue
			}

			key := payload.Key
			if key == "*" || key == "" {
				ram.UpdateTypeItems(payload.Type, nil)
			} else {
				ram.Delete(payload.Type, key)
			}
		}
	}()
}

// StopSystemConfigCacheListener stops the Redis Pub/Sub subscription listener and resets the sync.Once guard.
func StopSystemConfigCacheListener() {
	if systemConfigListenerCancel != nil {
		systemConfigListenerCancel()
		systemConfigListenerCancel = nil
	}
	systemConfigListenerOnce = sync.Once{}
}

func determineTTL(_ string) time.Duration {
	// Program-determined TTL: -1 means never expire for all configs by default
	return -1
}

// InvalidateSystemConfigCache triggers a broadcast to refresh the cache for key.
func InvalidateSystemConfigCache(ctx context.Context, key string) error {
	ensureSystemConfigCacheListener()

	// Invalidate local cache synchronously first
	ram.Delete(ConfigCacheType, key)

	// Broadcast to other nodes and clean legacy Redis cache key
	if db.Redis != nil {
		_ = db.HDel(ctx, SystemConfigRedisHashKey, key)
		publishSystemConfigBroadcast(ctx, ConfigCacheType, key)
	}
	return nil
}

// InvalidateAllSystemConfigCaches triggers a broadcast to refresh the entire config cache.
func InvalidateAllSystemConfigCaches(ctx context.Context) error {
	ensureSystemConfigCacheListener()

	// Invalidate all items of type ConfigCacheType synchronously first
	ram.UpdateTypeItems(ConfigCacheType, nil)

	// Broadcast to other nodes and clean legacy Redis cache keys
	if db.Redis != nil {
		_ = db.Redis.Del(ctx, db.PrefixedKey(SystemConfigRedisHashKey), db.PrefixedKey(SystemConfigVisibleListRedisKey)).Err()
		publishSystemConfigBroadcast(ctx, ConfigCacheType, "*")
	}
	return nil
}

func publishSystemConfigBroadcast(ctx context.Context, configType string, key string) {
	if db.Redis == nil {
		return
	}
	payload, err := json.Marshal(systemConfigBroadcastMessage{Type: configType, Key: key})
	if err != nil {
		return
	}
	_ = db.Redis.Publish(ctx, SystemConfigBroadcastChannel, payload).Err()
}

// ResetSystemConfigRAMCacheForTest clears only the process-local RAM cache.
func ResetSystemConfigRAMCacheForTest() {
	ram.ResetForTest()
}
