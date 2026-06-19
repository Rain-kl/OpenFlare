// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cap

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Store defines the storage interface for challenge nonces and verification tokens
type Store interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key string, val string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	// SetNX atomically sets key=val with the given TTL only when the key does not
	// exist yet. It returns true when the key was actually written (i.e. this
	// caller "won" the race), and false when the key already existed.
	SetNX(ctx context.Context, key string, val string, ttl time.Duration) (bool, error)
	// GetAndDelete atomically retrieves the value of key and removes it in a
	// single operation. Returns ("", false, nil) when the key does not exist.
	GetAndDelete(ctx context.Context, key string) (string, bool, error)
}

type memoryItem struct {
	value     string
	expiresAt time.Time
}

// MemoryStore is a thread-safe in-memory implementation of Store
type MemoryStore struct {
	items map[string]memoryItem
	mu    sync.Mutex // unified write-lock; promotes to exclusive for all ops
}

// NewMemoryStore creates and initializes a new MemoryStore
func NewMemoryStore(cleanupInterval time.Duration) *MemoryStore {
	store := &MemoryStore{
		items: make(map[string]memoryItem),
	}
	if cleanupInterval > 0 {
		go store.startCleanupLoop(cleanupInterval)
	}
	return store
}

// Get 从 MemoryStore 获取指定 key 的值
func (s *MemoryStore) Get(_ context.Context, key string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getLocked(key)
}

// getLocked is the internal helper – caller must hold s.mu.
func (s *MemoryStore) getLocked(key string) (string, bool, error) {
	item, found := s.items[key]
	if !found {
		return "", false, nil
	}
	if time.Now().After(item.expiresAt) {
		delete(s.items, key)
		return "", false, nil
	}
	return item.value, true, nil
}

// Set 向 MemoryStore 写入指定 key 的值
func (s *MemoryStore) Set(_ context.Context, key string, val string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = memoryItem{
		value:     val,
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

// Delete 从 MemoryStore 删除指定 key
func (s *MemoryStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
	return nil
}

// SetNX atomically sets key only when it is absent (or expired).
// Returns true if the key was written by this call.
func (s *MemoryStore) SetNX(_ context.Context, key string, val string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists, _ := s.getLocked(key)
	if exists {
		return false, nil
	}
	s.items[key] = memoryItem{
		value:     val,
		expiresAt: time.Now().Add(ttl),
	}
	return true, nil
}

// GetAndDelete atomically retrieves and removes key in one critical section.
func (s *MemoryStore) GetAndDelete(_ context.Context, key string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	val, exists, err := s.getLocked(key)
	if err != nil || !exists {
		return "", false, err
	}
	delete(s.items, key)
	return val, true, nil
}

func (s *MemoryStore) startCleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		s.cleanupExpired()
	}
}

func (s *MemoryStore) cleanupExpired() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.items {
		if now.After(v.expiresAt) {
			delete(s.items, k)
		}
	}
}

// RedisStore is a GORM-compatible/standalone Redis-backed implementation of Store
type RedisStore struct {
	client redis.UniversalClient
}

// NewRedisStore creates a new RedisStore wrapping a redis.UniversalClient
func NewRedisStore(client redis.UniversalClient) *RedisStore {
	return &RedisStore{
		client: client,
	}
}

// Get 从 RedisStore 获取指定 key 的值
func (s *RedisStore) Get(ctx context.Context, key string) (string, bool, error) {
	val, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

// Set 向 RedisStore 写入指定 key 的值
func (s *RedisStore) Set(ctx context.Context, key string, val string, ttl time.Duration) error {
	return s.client.Set(ctx, key, val, ttl).Err()
}

// Delete 从 RedisStore 删除指定 key
func (s *RedisStore) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

// SetNX wraps Redis SET NX – returns true only when the key was newly created.
func (s *RedisStore) SetNX(ctx context.Context, key string, val string, ttl time.Duration) (bool, error) {
	return s.client.SetNX(ctx, key, val, ttl).Result()
}

// GetAndDelete wraps Redis GETDEL (available since Redis 6.2).
func (s *RedisStore) GetAndDelete(ctx context.Context, key string) (string, bool, error) {
	val, err := s.client.GetDel(ctx, key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}
