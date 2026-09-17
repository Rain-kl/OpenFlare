// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package limiter_test

import (
	"Wavelet/pkg/limiter"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryLimiter_Basic(t *testing.T) {
	ctx := context.Background()
	lim := limiter.NewMemoryLimiter()

	rate := limiter.Rate{
		Limit:  3,
		Period: 100 * time.Millisecond,
	}

	// 1st request
	res, err := lim.Allow(ctx, "test_key", rate)
	require.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.Equal(t, 2, res.Remaining)

	// 2nd request
	res, err = lim.Allow(ctx, "test_key", rate)
	require.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.Equal(t, 1, res.Remaining)

	// 3rd request
	res, err = lim.Allow(ctx, "test_key", rate)
	require.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.Equal(t, 0, res.Remaining)

	// 4th request - should be blocked
	res, err = lim.Allow(ctx, "test_key", rate)
	require.NoError(t, err)
	assert.False(t, res.Allowed)
	assert.Equal(t, 0, res.Remaining)
	assert.Greater(t, res.RetryAfter, time.Duration(0))

	// Reset
	err = lim.Reset(ctx, "test_key")
	require.NoError(t, err)

	// Immediately allowed after reset
	res, err = lim.Allow(ctx, "test_key", rate)
	require.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.Equal(t, 2, res.Remaining)
}

func TestMemoryLimiter_WindowSlide(t *testing.T) {
	ctx := context.Background()
	lim := limiter.NewMemoryLimiter()

	rate := limiter.Rate{
		Limit:  2,
		Period: 50 * time.Millisecond,
	}

	res, err := lim.Allow(ctx, "slide_key", rate)
	require.NoError(t, err)
	assert.True(t, res.Allowed)

	res, err = lim.Allow(ctx, "slide_key", rate)
	require.NoError(t, err)
	assert.True(t, res.Allowed)

	res, err = lim.Allow(ctx, "slide_key", rate)
	require.NoError(t, err)
	assert.False(t, res.Allowed)

	// Wait for window to slide
	time.Sleep(60 * time.Millisecond)

	res, err = lim.Allow(ctx, "slide_key", rate)
	require.NoError(t, err)
	assert.True(t, res.Allowed)
}

func TestMemoryLimiter_Concurrency(t *testing.T) {
	ctx := context.Background()
	lim := limiter.NewMemoryLimiter()

	rate := limiter.Rate{
		Limit:  100,
		Period: time.Second,
	}

	var wg sync.WaitGroup
	allowedCount := int32(0)
	var mu sync.Mutex

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := lim.Allow(ctx, "concurrent_key", rate)
			if err == nil && res.Allowed {
				mu.Lock()
				allowedCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, int32(100), allowedCount)
}
