// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package diskcache wraps the generic pkg/cache/disk to provide database configuration integration.
package diskcache

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	pkgcache "github.com/Rain-kl/Wavelet/pkg/cache/disk"
)

// Status represents the runtime cache statistics.
type Status = pkgcache.Status

const (
	defaultCacheDir        = "uploads/diskcache"
	defaultMaxSizeMB       = 100
	defaultTTLMinutes      = 60
	defaultCleanupInterval = 10

	// DefaultExpiration applies the cache-wide default TTL.
	DefaultExpiration = pkgcache.DefaultExpiration
	// NoExpiration stores the item without a TTL. Size limits and LRU eviction still apply.
	NoExpiration = pkgcache.NoExpiration
)

// ErrCacheMiss represents a cache miss.
var ErrCacheMiss = pkgcache.ErrCacheMiss

// DiskCache is a wrapper around the generic pkg/diskcache that integrates with the DB for configs.
type DiskCache struct {
	*pkgcache.Cache
}

var (
	globalCache     *DiskCache
	globalCacheOnce sync.Once
)

// GetGlobalCache returns the global singleton DiskCache instance.
func GetGlobalCache() *DiskCache {
	globalCacheOnce.Do(func() {
		pureCache := pkgcache.New(defaultCacheDir)
		globalCache = &DiskCache{pureCache}
		// Load initial configs from database
		globalCache.ReloadConfig(context.Background())
		// Start background routine to clean expired items every 10 minutes
		go globalCache.StartCleanupWorker(defaultCleanupInterval * time.Minute)
	})
	return globalCache
}

// New creates a new DiskCache wrapper.
func New(basePath string) *DiskCache {
	return &DiskCache{pkgcache.New(basePath)}
}

// ReloadConfig reloads policies from database configs dynamically.
func (c *DiskCache) ReloadConfig(ctx context.Context) {
	// Ensure DB is initialized before querying
	if db.DB(ctx) == nil {
		return
	}

	// 1. Max Size
	maxSizeMB := int64(defaultMaxSizeMB)
	if scMaxSize, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyDiskCacheMaxSizeMB); err == nil && scMaxSize.Value != "" {
		if val, err := strconv.ParseInt(scMaxSize.Value, 10, 64); err == nil && val > 0 {
			maxSizeMB = val
		}
	}

	// 2. Default TTL
	ttlMinutes := int64(defaultTTLMinutes)
	if scTTL, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyDiskCacheTTLMinutes); err == nil && scTTL.Value != "" {
		if val, err := strconv.ParseInt(scTTL.Value, 10, 64); err == nil && val >= 0 {
			ttlMinutes = val
		}
	}

	// 3. LRU Enabled
	lruEnabled := true
	if scLRU, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyDiskCacheLRUEnabled); err == nil && scLRU.Value != "" {
		if val, err := strconv.ParseBool(scLRU.Value); err == nil {
			lruEnabled = val
		}
	}

	c.UpdatePolicy(maxSizeMB, ttlMinutes, lruEnabled)
}
