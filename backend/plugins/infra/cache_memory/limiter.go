// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cache_memory

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/limiter"
	"context"
)

type memoryLimiterAdapter struct {
	limiter *limiter.MemoryLimiter
}

func newMemoryLimiterService() contracts.LimiterService {
	return &memoryLimiterAdapter{
		limiter: limiter.NewMemoryLimiter(),
	}
}

func (a *memoryLimiterAdapter) Allow(ctx context.Context, key string, rate contracts.Rate) (*contracts.RateLimitResult, error) {
	res, err := a.limiter.Allow(ctx, key, limiter.Rate{
		Limit:  rate.Limit,
		Period: rate.Period,
	})
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

func (a *memoryLimiterAdapter) AllowN(ctx context.Context, key string, rate contracts.Rate, n int) (*contracts.RateLimitResult, error) {
	res, err := a.limiter.AllowN(ctx, key, limiter.Rate{
		Limit:  rate.Limit,
		Period: rate.Period,
	}, n)
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

func (a *memoryLimiterAdapter) Reset(ctx context.Context, key string) error {
	return a.limiter.Reset(ctx, key)
}
