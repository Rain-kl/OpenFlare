// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/plugins/infra/cache"
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisLimiterService(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer func() { _ = rdb.Close() }()

	p := cache.New(
		cache.WithRedis(rdb),
		cache.WithKeyPrefix("test:"),
	)
	ctx := core.NewContext(context.Background())
	ctx.Config().SetSource(core.NewMapSource(map[string]any{
		"redis.enabled": true,
	}))
	require.NoError(t, ctx.Config().Resolve())
	require.NoError(t, p.Apply(ctx))

	limiter, err := core.Inject[contracts.LimiterService](ctx)
	require.NoError(t, err)
	require.NotNil(t, limiter)

	testCtx := context.Background()
	rate := contracts.Rate{
		Limit:  3,
		Period: time.Minute,
	}

	// 1st request
	res, err := limiter.Allow(testCtx, "user:123", rate)
	require.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.Equal(t, 2, res.Remaining)

	// 2nd request
	res, err = limiter.Allow(testCtx, "user:123", rate)
	require.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.Equal(t, 1, res.Remaining)

	// 3rd request
	res, err = limiter.Allow(testCtx, "user:123", rate)
	require.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.Equal(t, 0, res.Remaining)

	// 4th request - blocked
	res, err = limiter.Allow(testCtx, "user:123", rate)
	require.NoError(t, err)
	assert.False(t, res.Allowed)
	assert.Equal(t, 0, res.Remaining)
	assert.Greater(t, res.RetryAfter, time.Duration(0))

	// Reset
	err = limiter.Reset(testCtx, "user:123")
	require.NoError(t, err)

	// Allowed again after reset
	res, err = limiter.Allow(testCtx, "user:123", rate)
	require.NoError(t, err)
	assert.True(t, res.Allowed)
}
