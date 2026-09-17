// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cache_memory_test

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/plugins/infra/cache_memory"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheMemoryPlugin(t *testing.T) {
	ctx := core.NewContext(context.Background())
	p := cache_memory.New(cache_memory.WithCapacity(100))

	assert.Equal(t, "cache_memory", p.Name())
	assert.Equal(t, "1.0.0", p.Manifest().Version)
	require.NoError(t, p.Apply(ctx))

	cacheSvc, err := core.Inject[contracts.CacheService](ctx)
	require.NoError(t, err)
	require.NotNil(t, cacheSvc)

	reqCtx := context.Background()

	// 1. Get non-existent
	var target string
	err = cacheSvc.Get(reqCtx, "non_existent", &target)
	assert.ErrorIs(t, err, contracts.ErrCacheMiss)

	// 2. Set and Get
	err = cacheSvc.Set(reqCtx, "user:100", "Alice", 10*time.Minute)
	require.NoError(t, err)

	var name string
	err = cacheSvc.Get(reqCtx, "user:100", &name)
	require.NoError(t, err)
	assert.Equal(t, "Alice", name)

	// 3. GetOrSet
	var count int
	var loaded int
	loader := func() (any, error) {
		loaded++
		return 42, nil
	}
	err = cacheSvc.GetOrSet(reqCtx, "counter", &count, 10*time.Minute, loader)
	require.NoError(t, err)
	assert.Equal(t, 42, count)
	assert.Equal(t, 1, loaded)

	// Second GetOrSet hits cache
	var count2 int
	err = cacheSvc.GetOrSet(reqCtx, "counter", &count2, 10*time.Minute, loader)
	require.NoError(t, err)
	assert.Equal(t, 42, count2)
	assert.Equal(t, 1, loaded)

	// 4. Delete & Invalidate
	require.NoError(t, cacheSvc.Delete(reqCtx, "user:100"))
	err = cacheSvc.Get(reqCtx, "user:100", &name)
	assert.ErrorIs(t, err, contracts.ErrCacheMiss)

	require.NoError(t, cacheSvc.Invalidate(reqCtx, "counter"))
	err = cacheSvc.Get(reqCtx, "counter", &count)
	assert.ErrorIs(t, err, contracts.ErrCacheMiss)

	// 5. Expiration
	err = cacheSvc.Set(reqCtx, "temp_key", "val", 50*time.Millisecond)
	require.NoError(t, err)
	time.Sleep(70 * time.Millisecond)
	var tempVal string
	err = cacheSvc.Get(reqCtx, "temp_key", &tempVal)
	assert.ErrorIs(t, err, contracts.ErrCacheMiss)

	// 6. LimiterService
	limiterSvc, err := core.Inject[contracts.LimiterService](ctx)
	require.NoError(t, err)
	require.NotNil(t, limiterSvc)

	rateRes, err := limiterSvc.Allow(reqCtx, "key_a", contracts.Rate{Limit: 2, Period: time.Minute})
	require.NoError(t, err)
	assert.True(t, rateRes.Allowed)
	assert.Equal(t, 1, rateRes.Remaining)
}
