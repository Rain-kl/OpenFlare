// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
)

type redisClientConnOpt struct {
	options redis.Options
}

func (opt redisClientConnOpt) MakeRedisClient() interface{} {
	return redis.NewClient(&opt.options)
}

type redisClusterConnOpt struct {
	options redis.ClusterOptions
}

func (opt redisClusterConnOpt) MakeRedisClient() interface{} {
	return redis.NewClusterClient(&opt.options)
}

type redisFailoverConnOpt struct {
	options                   redis.FailoverOptions
	maintNotificationsEnabled bool
}

func (opt redisFailoverConnOpt) MakeRedisClient() interface{} {
	client := redis.NewFailoverClient(&opt.options)
	// go-redis v9.16 does not expose maintenance notification settings on
	// FailoverOptions, so apply the configured mode before the client is used.
	client.Options().MaintNotificationsConfig = maintNotificationsConfig(opt.maintNotificationsEnabled)
	return client
}

func maintNotificationsConfig(enabled bool) *maintnotifications.Config {
	mode := maintnotifications.ModeDisabled
	if enabled {
		mode = maintnotifications.ModeAuto
	}
	return &maintnotifications.Config{Mode: mode}
}

// RedisOpt asynq Redis 连接配置（兼容 Standalone/Sentinel/Cluster）
var RedisOpt asynq.RedisConnOpt

// AsynqClient asynq 客户端，用于任务入队
var AsynqClient *asynq.Client

func init() {
	RedisOpt = NewRedisConnOpt()
	AsynqClient = asynq.NewClient(RedisOpt)
}

// NewRedisConnOpt 根据配置返回对应的 asynq Redis 连接选项
func NewRedisConnOpt() asynq.RedisConnOpt {
	cfg := config.Config.Redis
	addrs := cfg.Addrs

	if cfg.ClusterMode {
		return redisClusterConnOpt{
			options: redis.ClusterOptions{
				Addrs:                    addrs,
				Username:                 cfg.Username,
				Password:                 cfg.Password,
				MaintNotificationsConfig: maintNotificationsConfig(cfg.MaintNotifications),
			},
		}
	}

	if cfg.MasterName != "" {
		return redisFailoverConnOpt{
			maintNotificationsEnabled: cfg.MaintNotifications,
			options: redis.FailoverOptions{
				MasterName:    cfg.MasterName,
				SentinelAddrs: addrs,
				Username:      cfg.Username,
				Password:      cfg.Password,
				DB:            cfg.DB,
			},
		}
	}

	addr := "localhost:6379"
	if len(addrs) > 0 {
		addr = addrs[0]
	}
	return redisClientConnOpt{
		options: redis.Options{
			Addr:                     addr,
			Username:                 cfg.Username,
			Password:                 cfg.Password,
			DB:                       cfg.DB,
			PoolSize:                 cfg.PoolSize,
			MaintNotificationsConfig: maintNotificationsConfig(cfg.MaintNotifications),
		},
	}
}

// PrefixedQueue 返回带前缀的队列名，用于 Cluster 模式隔离
func PrefixedQueue(queue string) string {
	prefix := config.Config.Redis.KeyPrefix
	if prefix == "" {
		return queue
	}
	return prefix + queue
}
