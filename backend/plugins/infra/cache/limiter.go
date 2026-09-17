// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/limiter"
	"context"

	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
)

type redisLimiterImpl struct {
	limiter   *redis_rate.Limiter
	keyPrefix string
}

func newRedisLimiter(client redis.UniversalClient, keyPrefix string) contracts.LimiterService {
	return &redisLimiterImpl{
		limiter:   redis_rate.NewLimiter(client),
		keyPrefix: keyPrefix,
	}
}

func (r *redisLimiterImpl) prefixedKey(key string) string {
	if r.keyPrefix != "" {
		return r.keyPrefix + "limiter:" + key
	}
	return PrefixedKey("limiter:" + key)
}

func (r *redisLimiterImpl) Allow(ctx context.Context, key string, rate contracts.Rate) (*contracts.RateLimitResult, error) {
	return r.AllowN(ctx, key, rate, 1)
}

func (r *redisLimiterImpl) AllowN(ctx context.Context, key string, rate contracts.Rate, n int) (*contracts.RateLimitResult, error) {
	limit := redis_rate.Limit{
		Rate:   rate.Limit,
		Period: rate.Period,
		Burst:  rate.Limit,
	}

	res, err := r.limiter.AllowN(ctx, r.prefixedKey(key), limit, n)
	if err != nil {
		return nil, err
	}

	return &contracts.RateLimitResult{
		Allowed:    res.Allowed > 0,
		Remaining:  res.Remaining,
		ResetAfter: res.ResetAfter,
		RetryAfter: res.RetryAfter,
	}, nil
}

func (r *redisLimiterImpl) Reset(ctx context.Context, key string) error {
	return r.limiter.Reset(ctx, r.prefixedKey(key))
}

type memoryLimiterFallback struct {
	limiter *limiter.MemoryLimiter
}

func newMemoryLimiterFallback() contracts.LimiterService {
	return &memoryLimiterFallback{
		limiter: limiter.NewMemoryLimiter(),
	}
}

func (m *memoryLimiterFallback) Allow(ctx context.Context, key string, rate contracts.Rate) (*contracts.RateLimitResult, error) {
	res, err := m.limiter.Allow(ctx, key, limiter.Rate{Limit: rate.Limit, Period: rate.Period})
	if err != nil {
		return nil, err
	}
	return &contracts.RateLimitResult{
		Allowed:    res.Allowed,
		Remaining:  res.Remaining,
		ResetAfter: res.ResetAfter,
		RetryAfter: res.RetryAfter,
	}, nil
}

func (m *memoryLimiterFallback) AllowN(ctx context.Context, key string, rate contracts.Rate, n int) (*contracts.RateLimitResult, error) {
	res, err := m.limiter.AllowN(ctx, key, limiter.Rate{Limit: rate.Limit, Period: rate.Period}, n)
	if err != nil {
		return nil, err
	}
	return &contracts.RateLimitResult{
		Allowed:    res.Allowed,
		Remaining:  res.Remaining,
		ResetAfter: res.ResetAfter,
		RetryAfter: res.RetryAfter,
	}, nil
}

func (m *memoryLimiterFallback) Reset(ctx context.Context, key string) error {
	return m.limiter.Reset(ctx, key)
}
