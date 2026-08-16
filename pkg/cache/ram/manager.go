// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package ram

import (
	"context"
	"errors"
	"maps"
	"sync"
	"time"
)

var (
	// ErrNotFound is returned by the Loader when the requested item is not found.
	ErrNotFound = errors.New("cache item not found in data source")

	managerCache *Cache[string, map[string]cacheEntry]

	writeLocks   = make(map[string]*sync.Mutex)
	writeLocksMu sync.Mutex
)

// CacheItem represents a unified cache entity.
type CacheItem struct {
	Key   string        `json:"key"`
	Value string        `json:"value"`
	Type  string        `json:"type"`
	TTL   time.Duration `json:"ttl"` // -1 means never expire
}

// Loader is an interface that the cache client must implement to handle database retrieval.
type Loader interface {
	LoadAll(ctx context.Context, configType string) ([]CacheItem, error)
	LoadOne(ctx context.Context, configType string, key string) (CacheItem, error)
}

type cacheEntry struct {
	item     CacheItem
	expireAt time.Time
}

func init() {
	// Initialize with a large maximum size since it only stores one entry per configType
	managerCache = MustNew[string, map[string]cacheEntry](Options{
		MaximumSize: 1000,
	})
}

func getWriteLock(configType string) *sync.Mutex {
	writeLocksMu.Lock()
	defer writeLocksMu.Unlock()
	lock, found := writeLocks[configType]
	if !found {
		lock = &sync.Mutex{}
		writeLocks[configType] = lock
	}
	return lock
}

// Get retrieves a cache item from the local cache store, checking for expiration.
// Reads are completely lock-free because maps stored in Otter are immutable.
func Get(configType string, key string) (CacheItem, bool) {
	m, ok := managerCache.GetIfPresent(configType)
	if !ok {
		return CacheItem{}, false
	}

	entry, found := m[key]
	if !found {
		return CacheItem{}, false
	}

	// Check expiration
	if entry.item.TTL != -1 && !entry.expireAt.IsZero() && time.Now().After(entry.expireAt) {
		// Asynchronously remove the expired item from the map and write back
		go deleteKeyIfExpired(configType, key, entry.expireAt)
		return CacheItem{}, false
	}

	return entry.item, true
}

func deleteKeyIfExpired(configType string, key string, expireAt time.Time) {
	lock := getWriteLock(configType)
	lock.Lock()
	defer lock.Unlock()

	currentMap, ok := managerCache.GetIfPresent(configType)
	if !ok {
		return
	}

	entry, found := currentMap[key]
	if !found {
		return
	}

	// Double-check expiration time to ensure we don't delete a newly updated key
	if entry.expireAt != expireAt || !time.Now().After(entry.expireAt) {
		return
	}

	newMap := make(map[string]cacheEntry, len(currentMap)-1)
	for k, v := range currentMap {
		if k != key {
			newMap[k] = v
		}
	}
	managerCache.Set(configType, newMap)
}

// Set stores a cache item in the local cache store.
// Writes are protected by a fine-grained lock per configType.
func Set(item CacheItem) {
	lock := getWriteLock(item.Type)
	lock.Lock()
	defer lock.Unlock()

	currentMap, ok := managerCache.GetIfPresent(item.Type)
	newMap := make(map[string]cacheEntry)
	if ok {
		maps.Copy(newMap, currentMap)
	}

	var expireAt time.Time
	if item.TTL != -1 {
		expireAt = time.Now().Add(item.TTL)
	}

	newMap[item.Key] = cacheEntry{
		item:     item,
		expireAt: expireAt,
	}
	managerCache.Set(item.Type, newMap)
}

// Delete removes a single item from the local cache store.
// Writes are protected by a fine-grained lock per configType.
func Delete(configType string, key string) {
	lock := getWriteLock(configType)
	lock.Lock()
	defer lock.Unlock()

	currentMap, ok := managerCache.GetIfPresent(configType)
	if !ok {
		return
	}

	newMap := make(map[string]cacheEntry, len(currentMap))
	for k, v := range currentMap {
		if k != key {
			newMap[k] = v
		}
	}
	managerCache.Set(configType, newMap)
}

// UpdateTypeItems replaces all cache items of a specific type atomically.
// Writes are protected by a fine-grained lock per configType.
func UpdateTypeItems(configType string, items []CacheItem) {
	lock := getWriteLock(configType)
	lock.Lock()
	defer lock.Unlock()

	newMap := make(map[string]cacheEntry, len(items))
	for _, item := range items {
		var expireAt time.Time
		if item.TTL != -1 {
			expireAt = time.Now().Add(item.TTL)
		}
		newMap[item.Key] = cacheEntry{
			item:     item,
			expireAt: expireAt,
		}
	}
	managerCache.Set(configType, newMap)
}

// GetTypeItems retrieves all unexpired cache items of a specific type.
// Reads are completely lock-free because maps stored in Otter are immutable.
func GetTypeItems(configType string) []CacheItem {
	currentMap, ok := managerCache.GetIfPresent(configType)
	if !ok {
		return nil
	}

	var list []CacheItem
	for _, entry := range currentMap {
		if entry.item.TTL == -1 || entry.expireAt.IsZero() || time.Now().Before(entry.expireAt) {
			list = append(list, entry.item)
		}
	}
	return list
}

// Refresh reloads configuration cache from database via the Loader.
func Refresh(ctx context.Context, configType string, key string, loader Loader) error {
	if configType == "" {
		return errors.New("type is required")
	}

	if key != "" {
		// Single key refresh: first fetch latest value from database
		item, err := loader.LoadOne(ctx, configType, key)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				Delete(configType, key)
				return nil
			}
			return err
		}
		Set(item)
		return nil
	}

	// All keys refresh: load all of that type from database first, then replace cache
	items, err := loader.LoadAll(ctx, configType)
	if err != nil {
		return err
	}
	UpdateTypeItems(configType, items)
	return nil
}

// ResetForTest clears the local store and locks.
func ResetForTest() {
	writeLocksMu.Lock()
	writeLocks = make(map[string]*sync.Mutex)
	writeLocksMu.Unlock()
	managerCache.InvalidateAll()
}
