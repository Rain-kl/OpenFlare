// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"go.opentelemetry.io/otel/attribute"
)

var (
	// Redis 全局 Redis 客户端实例
	Redis redis.UniversalClient
)

func init() {
	cfg := config.Config.Redis

	if !cfg.Enabled {
		log.Println("[Redis] is disabled, skipping Redis initialization")
		return
	}

	if cfg.ClusterMode {
		// Cluster 模式
		Redis = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:           cfg.Addrs,
			Username:        cfg.Username,
			Password:        cfg.Password,
			PoolSize:        cfg.PoolSize,
			MinIdleConns:    cfg.MinIdleConn,
			DialTimeout:     time.Duration(cfg.DialTimeout) * time.Second,
			ReadTimeout:     time.Duration(cfg.ReadTimeout) * time.Second,
			WriteTimeout:    time.Duration(cfg.WriteTimeout) * time.Second,
			MaxRetries:      cfg.MaxRetries,
			PoolTimeout:     time.Duration(cfg.PoolTimeout) * time.Second,
			ConnMaxIdleTime: time.Duration(cfg.ConnMaxIdleTime) * time.Second,
			MaintNotificationsConfig: &maintnotifications.Config{
				Mode: maintnotifications.ModeDisabled,
			},
		})
		log.Println("[Redis] initialized in Cluster mode")
	} else {
		// Standalone 或 Sentinel 模式
		Redis = redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs:           cfg.Addrs,
			MasterName:      cfg.MasterName, // 非空时启用 Sentinel
			Username:        cfg.Username,
			Password:        cfg.Password,
			DB:              cfg.DB,
			PoolSize:        cfg.PoolSize,
			MinIdleConns:    cfg.MinIdleConn,
			DialTimeout:     time.Duration(cfg.DialTimeout) * time.Second,
			ReadTimeout:     time.Duration(cfg.ReadTimeout) * time.Second,
			WriteTimeout:    time.Duration(cfg.WriteTimeout) * time.Second,
			MaxRetries:      cfg.MaxRetries,
			PoolTimeout:     time.Duration(cfg.PoolTimeout) * time.Second,
			ConnMaxIdleTime: time.Duration(cfg.ConnMaxIdleTime) * time.Second,
			MaintNotificationsConfig: &maintnotifications.Config{
				Mode: maintnotifications.ModeDisabled,
			},
		})
		if cfg.MasterName != "" {
			log.Println("[Redis] initialized in Sentinel mode")
		} else {
			log.Println("[Redis] initialized in Standalone mode")
		}
	}

	// OpenTelemetry 追踪（UniversalClient 兼容）
	if err := redisotel.InstrumentTracing(
		Redis,
		redisotel.WithAttributes(
			attribute.String("db.instance", fmt.Sprintf("%v", cfg.DB)),
			attribute.String("db.ip", strings.Join(cfg.Addrs, ",")),
			attribute.String("db.system", "Redis"),
		),
	); err != nil {
		log.Fatalf("[Redis] failed to init trace: %v\n", err)
	}

	// 测试连接
	_, err := Redis.Ping(context.Background()).Result()
	if err != nil {
		log.Fatalf("[Redis] failed to connect to redis: %v\n", err)
	}
}

// PrefixedKey 返回带前缀的 Key
func PrefixedKey(key string) string {
	prefix := config.Config.Redis.KeyPrefix
	if prefix == "" {
		return key
	}
	return prefix + key
}

// HSetJSON 将泛型数据序列化为 JSON 并设置到 Redis Hash
// ctx: 上下文
// hashKey: Redis Hash key
// fieldKey: Hash field key
// data: 要存储的数据（泛型）
func HSetJSON[T any](ctx context.Context, hashKey, fieldKey string, data T) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if err := Redis.HSet(ctx, PrefixedKey(hashKey), fieldKey, jsonData).Err(); err != nil {
		return fmt.Errorf(errRedisHashSetFailed, err)
	}

	return nil
}

// HDel removes one or more fields from a Redis Hash.
func HDel(ctx context.Context, hashKey string, fieldKeys ...string) error {
	if Redis == nil || len(fieldKeys) == 0 {
		return nil
	}
	if err := Redis.HDel(ctx, PrefixedKey(hashKey), fieldKeys...).Err(); err != nil {
		return fmt.Errorf(errRedisHashDeleteFailed, err)
	}
	return nil
}

// HGetJSON 从 Redis Hash 获取数据并反序列化为泛型类型
// ctx: 上下文
// hashKey: Redis Hash key
// fieldKey: Hash field key
// data: 用于接收数据的指针（泛型）
func HGetJSON[T any](ctx context.Context, hashKey, fieldKey string, data *T) error {
	val, err := Redis.HGet(ctx, PrefixedKey(hashKey), fieldKey).Result()
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(val), data); err != nil {
		return fmt.Errorf(errUnmarshalDataFailed, err)
	}

	return nil
}

// GetJSON 从Redis获取数据并反序列化为泛型类型
// ctx: 上下文
// key: Redis key
// data: 用于接收数据的指针（泛型）
func GetJSON[T any](ctx context.Context, key string, data *T) error {
	val, err := Redis.Get(ctx, PrefixedKey(key)).Bytes()
	if err != nil {
		return err
	}

	if err := json.Unmarshal(val, data); err != nil {
		return fmt.Errorf(errUnmarshalDataFailed, err)
	}

	return nil
}

// SetJSON 将泛型数据序列化为JSON并设置到Redis
// ctx: 上下文
// key: Redis key
// data: 要存储的数据（泛型）
// expiration: 过期时间
func SetJSON[T any](ctx context.Context, key string, data T, expiration time.Duration) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf(errMarshalDataFailed, err)
	}

	if err := Redis.Set(ctx, PrefixedKey(key), jsonData, expiration).Err(); err != nil {
		return fmt.Errorf(errRedisKeySetFailed, err)
	}

	return nil
}
