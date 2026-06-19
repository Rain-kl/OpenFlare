// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package disk implements a platform-level disk-backed cache with size limit, TTL, and LRU eviction.
package disk

import (
	"container/list"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/peterbourgon/diskv/v3"
)

// ErrCacheMiss represents a cache miss.
var ErrCacheMiss = errors.New("cache miss")

// Constants for disk cache configuration and sizing
const (
	headerSize        = 8 // 8 bytes metadata prefix for expiration UnixNano timestamp
	defaultMaxSizeMB  = 100
	defaultTTLMinutes = 60
	cacheDirPerm      = 0750

	// DefaultExpiration applies the cache-wide default TTL.
	DefaultExpiration time.Duration = 0
	// NoExpiration stores the item without a TTL. Size limits and LRU eviction still apply.
	NoExpiration time.Duration = -1
)

// Status represents the runtime cache statistics.
type Status struct {
	TotalSize  int64  `json:"total_size"`
	KeysCount  int    `json:"keys_count"`
	MaxSizeMB  int64  `json:"max_size_mb"`
	TTLMinutes int64  `json:"ttl_minutes"`
	LRUEnabled bool   `json:"lru_enabled"`
	BasePath   string `json:"base_path"`
}

// Cache implements the disk-backed cache with size limits, TTL, and LRU eviction.
type Cache struct {
	mu         sync.RWMutex
	d          *diskv.Diskv
	basePath   string
	maxSize    int64 // in bytes
	defaultTTL time.Duration
	lruEnabled bool

	// LRU and Size tracking
	currentSize int64
	items       map[string]*list.Element
	evictList   *list.List
}

type cacheItem struct {
	key       string
	size      int64
	expiredAt time.Time
}

// New creates a new Cache instance.
func New(basePath string) *Cache {
	d := diskv.New(diskv.Options{
		BasePath:     basePath,
		Transform:    func(_ string) []string { return []string{} }, // flat structure for easy walk
		CacheSizeMax: 1024 * 1024,                                   // 1MB in-memory cache size for diskv itself
	})

	c := &Cache{
		d:          d,
		basePath:   basePath,
		maxSize:    defaultMaxSizeMB * 1024 * 1024,  // 100MB default
		defaultTTL: defaultTTLMinutes * time.Minute, // 60 minutes default
		lruEnabled: true,
		items:      make(map[string]*list.Element),
		evictList:  list.New(),
	}

	// Scan directory on startup to rebuild LRU and size tracking
	_ = c.loadTracker()
	return c
}

// Set stores a key-value pair in the cache.
// Use DefaultExpiration for the configured default TTL, NoExpiration for no
// TTL, or a positive duration for a business-specific TTL.
func (c *Cache) Set(key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ttl == DefaultExpiration {
		ttl = c.defaultTTL
	}

	var expiredAt time.Time
	if ttl > 0 {
		expiredAt = time.Now().Add(ttl)
	}

	// Prepare data layout: 8 bytes expiration timestamp + raw payload
	buf := make([]byte, headerSize+len(value))
	var expNano int64
	if !expiredAt.IsZero() {
		expNano = expiredAt.UnixNano()
	}
	binary.BigEndian.PutUint64(buf[0:headerSize], uint64(expNano))
	copy(buf[headerSize:], value)

	// Write to diskv
	if err := c.d.Write(key, buf); err != nil {
		return fmt.Errorf("failed to write key to disk: %w", err)
	}

	// Get file size on disk (approximate)
	size := int64(len(buf))

	// Update memory tracker
	if elem, ok := c.items[key]; ok {
		item := elem.Value.(*cacheItem)
		c.currentSize += size - item.size
		item.size = size
		item.expiredAt = expiredAt
		c.evictList.MoveToFront(elem)
	} else {
		item := &cacheItem{
			key:       key,
			size:      size,
			expiredAt: expiredAt,
		}
		elem := c.evictList.PushFront(item)
		c.items[key] = elem
		c.currentSize += size
	}

	// Evict items if size limit exceeded and LRU is enabled
	c.evict()

	return nil
}

// Get retrieves a key's value from the cache.
func (c *Cache) Get(key string) ([]byte, error) {
	c.mu.RLock()
	elem, ok := c.items[key]
	if !ok {
		c.mu.RUnlock()
		return nil, ErrCacheMiss
	}

	item := elem.Value.(*cacheItem)
	if !item.expiredAt.IsZero() && time.Now().After(item.expiredAt) {
		c.mu.RUnlock()
		return c.getAndDeleteIfExpired(key)
	}
	c.mu.RUnlock()

	// Read from disk outside the lock so concurrent cache hits do not serialize on I/O.
	data, err := c.d.Read(key)
	if err != nil {
		c.mu.Lock()
		defer c.mu.Unlock()
		if _, stillExists := c.items[key]; stillExists {
			_ = c.deleteUnlocked(key)
		}
		return nil, ErrCacheMiss
	}

	if len(data) < headerSize {
		c.mu.Lock()
		defer c.mu.Unlock()
		if _, stillExists := c.items[key]; stillExists {
			_ = c.deleteUnlocked(key)
		}
		return nil, ErrCacheMiss
	}

	payload := data[headerSize:]

	// Brief write lock only for LRU bookkeeping.
	c.mu.Lock()
	defer c.mu.Unlock()
	elem, ok = c.items[key]
	if !ok {
		return nil, ErrCacheMiss
	}
	c.evictList.MoveToFront(elem)

	return payload, nil
}

func (c *Cache) getAndDeleteIfExpired(key string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, ErrCacheMiss
	}

	item := elem.Value.(*cacheItem)
	if !item.expiredAt.IsZero() && time.Now().After(item.expiredAt) {
		_ = c.deleteUnlocked(key)
		return nil, ErrCacheMiss
	}

	data, err := c.d.Read(key)
	if err != nil {
		_ = c.deleteUnlocked(key)
		return nil, ErrCacheMiss
	}

	if len(data) < headerSize {
		_ = c.deleteUnlocked(key)
		return nil, ErrCacheMiss
	}

	c.evictList.MoveToFront(elem)
	return data[headerSize:], nil
}

// Delete removes a key-value pair from the cache.
func (c *Cache) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.deleteUnlocked(key)
}

func (c *Cache) deleteUnlocked(key string) error {
	if elem, ok := c.items[key]; ok {
		item := elem.Value.(*cacheItem)
		c.currentSize -= item.size
		c.evictList.Remove(elem)
		delete(c.items, key)
	}
	return c.d.Erase(key)
}

// Clear flushes all cached elements.
func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.currentSize = 0
	c.items = make(map[string]*list.Element)
	c.evictList.Init()

	return c.d.EraseAll()
}

// Status returns the cache status.
func (c *Cache) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return Status{
		TotalSize:  c.currentSize,
		KeysCount:  len(c.items),
		MaxSizeMB:  c.maxSize / (1024 * 1024),
		TTLMinutes: int64(c.defaultTTL.Minutes()),
		LRUEnabled: c.lruEnabled,
		BasePath:   c.basePath,
	}
}

// UpdatePolicy dynamically updates policies.
func (c *Cache) UpdatePolicy(maxSizeMB int64, ttlMinutes int64, lruEnabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.maxSize = maxSizeMB * 1024 * 1024
	c.defaultTTL = time.Duration(ttlMinutes) * time.Minute
	c.lruEnabled = lruEnabled
	c.evict()
}

// evict evicts oldest items if current size exceeds maxSize and LRU is enabled.
func (c *Cache) evict() {
	if !c.lruEnabled {
		return
	}

	for c.currentSize > c.maxSize && c.evictList.Len() > 0 {
		elem := c.evictList.Back()
		item := elem.Value.(*cacheItem)
		c.currentSize -= item.size
		c.evictList.Remove(elem)
		delete(c.items, item.key)
		_ = c.d.Erase(item.key)
	}
}

// loadTracker scans the cache directory on startup to rebuild memory state.
func (c *Cache) loadTracker() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Ensure directory exists
	if err := os.MkdirAll(c.basePath, cacheDirPerm); err != nil {
		return err
	}

	type loadedItem struct {
		key       string
		size      int64
		expiredAt time.Time
		modTime   time.Time
	}
	var loadedItems []loadedItem

	// Walk keys through diskv
	keysChan := c.d.Keys(nil)
	for key := range keysChan {
		// Read raw bytes to parse expiration prefix
		data, err := c.d.Read(key)
		if err != nil || len(data) < headerSize {
			_ = c.d.Erase(key) // corrupted file, wipe
			continue
		}

		expNano := int64(binary.BigEndian.Uint64(data[0:headerSize])) //nolint:gosec // false positive: UnixNano fits within int64
		var expiredAt time.Time
		if expNano > 0 {
			expiredAt = time.Unix(0, expNano)
		}

		// Check mod time for ordering
		path := filepath.Join(c.basePath, key)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		loadedItems = append(loadedItems, loadedItem{
			key:       key,
			size:      int64(len(data)),
			expiredAt: expiredAt,
			modTime:   info.ModTime(),
		})
	}

	// Sort by ModTime ascending (oldest first) so we rebuild LRU correctly
	sort.Slice(loadedItems, func(i, j int) bool {
		return loadedItems[i].modTime.Before(loadedItems[j].modTime)
	})

	// Populate LRU (PushFront so that newest items are at the front, oldest at the back)
	for _, item := range loadedItems {
		entry := &cacheItem{
			key:       item.key,
			size:      item.size,
			expiredAt: item.expiredAt,
		}
		element := c.evictList.PushFront(entry)
		c.items[item.key] = element
		c.currentSize += item.size
	}

	return nil
}

// StartCleanupWorker periodically cleans up expired cache items.
func (c *Cache) StartCleanupWorker(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		c.cleanExpired()
	}
}

// cleanExpired scans memory for expired items and removes them.
func (c *Cache) cleanExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, elem := range c.items {
		item := elem.Value.(*cacheItem)
		if !item.expiredAt.IsZero() && now.After(item.expiredAt) {
			c.currentSize -= item.size
			c.evictList.Remove(elem)
			delete(c.items, key)
			_ = c.d.Erase(key)
		}
	}
}
