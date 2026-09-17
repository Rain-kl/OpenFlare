// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package cache provides the multi-tier caching infrastructure plugin for Cordis.
package cache

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/pkg/cache/ram"
	"Wavelet/pkg/util"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultRAMCapacity   = 10000
	defaultPubSubChannel = "wavelet:cache:invalidation"
)

type ramEntry struct {
	data     []byte
	expireAt time.Time
}

// Option configures the cache plugin.
type Option func(*Plugin)

// WithRedis sets an explicit Redis client instance.
func WithRedis(client redis.UniversalClient) Option {
	return func(p *Plugin) {
		p.redisClient = client
	}
}

// WithKeyPrefix sets a custom Redis key prefix.
func WithKeyPrefix(prefix string) Option {
	return func(p *Plugin) {
		p.keyPrefix = prefix
	}
}

// WithRAMCapacity sets the maximum capacity for the L1 RAM cache.
func WithRAMCapacity(capacity int) Option {
	return func(p *Plugin) {
		p.ramCapacity = capacity
	}
}

// Plugin implements core.Plugin to provide contracts.CacheService.
type Plugin struct {
	redisClient redis.UniversalClient
	keyPrefix   string
	ramCapacity int
}

// New creates a new cache infrastructure plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{
		ramCapacity: defaultRAMCapacity,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// Name returns the unique identifier of the cache plugin.
func (p *Plugin) Name() string {
	return "cache"
}

// DeclareConfig declares the configuration bindings consumed by this plugin.
func (p *Plugin) DeclareConfig() []core.ConfigBinding {
	return []core.ConfigBinding{
		{Prefix: "redis", Target: &RedisConfig{}},
	}
}

// ConfigEnabled gates plugin activation based on whether Redis is enabled.
func (p *Plugin) ConfigEnabled(view core.ConfigView) bool {
	return view.Bool("redis.enabled", false)
}

// Apply mounts the multi-layer cache service into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	var cfg RedisConfig
	if err := ctx.Config().Bind("redis", &cfg); err != nil {
		return err
	}

	redisClient := p.redisClient
	if redisClient == nil {
		if Redis == nil {
			var err error
			redisClient, err = InitRedisWithConfig(cfg)
			if err != nil {
				return err
			}
		} else {
			redisClient = Redis
		}
	}

	ramCache, err := ram.New[string, ramEntry](ram.Options{
		MaximumSize: p.ramCapacity,
	})
	if err != nil {
		return err
	}

	svc := &cacheServiceImpl{
		ramCache:      ramCache,
		redisClient:   redisClient,
		keyPrefix:     p.keyPrefix,
		pubSubChannel: defaultPubSubChannel,
		stopCh:        make(chan struct{}),
	}

	if redisClient != nil {
		svc.startPubSubListener()
		ctx.OnDispose(func() error {
			svc.stopPubSubListener()
			if p.redisClient == nil {
				Redis = nil
				if closeErr := redisClient.Close(); closeErr != nil && !errors.Is(closeErr, redis.ErrClosed) {
					return closeErr
				}
			}
			return nil
		})
	}

	core.Provide[contracts.CacheService](ctx, svc)

	var limiterSvc contracts.LimiterService
	if redisClient != nil {
		limiterSvc = newRedisLimiter(redisClient, p.keyPrefix)
	} else {
		limiterSvc = newMemoryLimiterFallback()
	}
	core.Provide[contracts.LimiterService](ctx, limiterSvc)

	return nil
}

type cacheServiceImpl struct {
	ramCache      *ram.Cache[string, ramEntry]
	redisClient   redis.UniversalClient
	keyPrefix     string
	pubSubChannel string

	subOnce  sync.Once
	stopOnce sync.Once
	stopCh   chan struct{}
	pubsub   *redis.PubSub
}

func (s *cacheServiceImpl) prefixedKey(key string) string {
	if s.keyPrefix != "" {
		return s.keyPrefix + key
	}
	return PrefixedKey(key)
}

func (s *cacheServiceImpl) startPubSubListener() {
	if s.redisClient == nil {
		return
	}

	s.subOnce.Do(func() {
		pubsub := s.redisClient.Subscribe(context.Background(), s.pubSubChannel)
		s.pubsub = pubsub

		util.Go(func() {
			ch := pubsub.Channel()
			for {
				select {
				case <-s.stopCh:
					return
				case msg, ok := <-ch:
					if !ok {
						return
					}
					if msg != nil && msg.Payload != "" {
						s.ramCache.Invalidate(msg.Payload)
					}
				}
			}
		})
	})
}

func (s *cacheServiceImpl) stopPubSubListener() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
		if s.pubsub != nil {
			_ = s.pubsub.Close()
		}
	})
}

func (s *cacheServiceImpl) Get(ctx context.Context, key string, target any) error {
	// 1. Check L1 RAM cache
	if entry, ok := s.ramCache.GetIfPresent(key); ok {
		if entry.expireAt.IsZero() || time.Now().Before(entry.expireAt) {
			return json.Unmarshal(entry.data, target)
		}
		// Expired in L1 RAM
		s.ramCache.Invalidate(key)
	}

	// 2. Check L2 Redis cache
	if s.redisClient != nil {
		data, err := s.redisClient.Get(ctx, s.prefixedKey(key)).Bytes()
		if err == nil {
			// Backfill L1 RAM cache
			s.ramCache.Set(key, ramEntry{
				data: data,
			})
			return json.Unmarshal(data, target)
		} else if !errors.Is(err, redis.Nil) {
			return err
		}
	}

	return contracts.ErrCacheMiss
}

func (s *cacheServiceImpl) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	var expireAt time.Time
	if ttl > 0 {
		expireAt = time.Now().Add(ttl)
	}

	// 1. Write L1 RAM cache
	s.ramCache.Set(key, ramEntry{
		data:     data,
		expireAt: expireAt,
	})

	// 2. Write L2 Redis cache
	if s.redisClient != nil {
		if err := s.redisClient.Set(ctx, s.prefixedKey(key), data, ttl).Err(); err != nil {
			return err
		}
	}

	return nil
}

func (s *cacheServiceImpl) Delete(ctx context.Context, key string) error {
	// 1. Evict L1 RAM
	s.ramCache.Invalidate(key)

	// 2. Evict L2 Redis and broadcast invalidation to cluster nodes
	if s.redisClient != nil {
		if err := s.redisClient.Del(ctx, s.prefixedKey(key)).Err(); err != nil && !errors.Is(err, redis.Nil) {
			return err
		}
		_ = s.redisClient.Publish(ctx, s.pubSubChannel, key).Err()
	}

	return nil
}

func (s *cacheServiceImpl) Invalidate(ctx context.Context, key string) error {
	return s.Delete(ctx, key)
}

func (s *cacheServiceImpl) GetOrSet(ctx context.Context, key string, target any, ttl time.Duration, loader func() (any, error)) error {
	err := s.Get(ctx, key, target)
	if err == nil {
		return nil
	}
	if !errors.Is(err, contracts.ErrCacheMiss) {
		return err
	}

	val, err := loader()
	if err != nil {
		return err
	}

	if err := s.Set(ctx, key, val, ttl); err != nil {
		return err
	}

	data, err := json.Marshal(val)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, target)
}
