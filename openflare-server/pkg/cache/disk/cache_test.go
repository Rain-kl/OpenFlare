// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package disk

import (
	"bytes"
	"os"
	"testing"
	"time"
)

func TestDiskCacheBasic(t *testing.T) {
	testDir := "uploads/test_diskcache_basic"
	defer func() { _ = os.RemoveAll(testDir) }()
	_ = os.RemoveAll(testDir)

	c := New(testDir)
	defer func() { _ = c.Clear() }()

	key := "key1"
	val := []byte("value1")

	// Get non-existent
	_, err := c.Get(key)
	if err != ErrCacheMiss {
		t.Fatalf("expected ErrCacheMiss, got %v", err)
	}

	// Set & Get
	err = c.Set(key, val, 10*time.Second)
	if err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	got, err := c.Get(key)
	if err != nil {
		t.Fatalf("failed to get cache: %v", err)
	}

	if !bytes.Equal(got, val) {
		t.Errorf("expected %s, got %s", val, got)
	}

	// Delete
	err = c.Delete(key)
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	_, err = c.Get(key)
	if err != ErrCacheMiss {
		t.Errorf("expected ErrCacheMiss after delete, got %v", err)
	}
}

func TestDiskCacheTTL(t *testing.T) {
	testDir := "uploads/test_diskcache_ttl"
	defer func() { _ = os.RemoveAll(testDir) }()
	_ = os.RemoveAll(testDir)

	c := New(testDir)
	defer func() { _ = c.Clear() }()

	key := "ttlkey"
	val := []byte("ttlval")

	// Set with 200ms TTL
	err := c.Set(key, val, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to set: %v", err)
	}

	// Immediate Get should succeed
	got, err := c.Get(key)
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}
	if !bytes.Equal(got, val) {
		t.Errorf("expected %s, got %s", val, got)
	}

	// Sleep 250ms to expire
	time.Sleep(250 * time.Millisecond)

	// Get should fail with cache miss
	_, err = c.Get(key)
	if err != ErrCacheMiss {
		t.Errorf("expected ErrCacheMiss after TTL expiration, got %v", err)
	}
}

func TestDiskCacheExpirationPolicies(t *testing.T) {
	testDir := "uploads/test_diskcache_expiration_policies"
	defer func() { _ = os.RemoveAll(testDir) }()
	_ = os.RemoveAll(testDir)

	c := New(testDir)
	defer func() { _ = c.Clear() }()
	c.defaultTTL = 50 * time.Millisecond

	if err := c.Set("default", []byte("default"), DefaultExpiration); err != nil {
		t.Fatalf("Set(default, DefaultExpiration) returned error: %v", err)
	}
	if err := c.Set("custom", []byte("custom"), 100*time.Millisecond); err != nil {
		t.Fatalf("Set(custom, 100ms) returned error: %v", err)
	}
	if err := c.Set("permanent", []byte("permanent"), NoExpiration); err != nil {
		t.Fatalf("Set(permanent, NoExpiration) returned error: %v", err)
	}

	time.Sleep(75 * time.Millisecond)

	if _, err := c.Get("default"); err != ErrCacheMiss {
		t.Errorf("Get(default) error = %v, want ErrCacheMiss", err)
	}
	if _, err := c.Get("custom"); err != nil {
		t.Errorf("Get(custom) returned error before custom TTL elapsed: %v", err)
	}
	if _, err := c.Get("permanent"); err != nil {
		t.Errorf("Get(permanent) returned error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if _, err := c.Get("custom"); err != ErrCacheMiss {
		t.Errorf("Get(custom) error = %v, want ErrCacheMiss", err)
	}
	if _, err := c.Get("permanent"); err != nil {
		t.Errorf("Get(permanent) returned error after other entries expired: %v", err)
	}
}

func TestDiskCacheNoExpirationSurvivesReload(t *testing.T) {
	testDir := "uploads/test_diskcache_no_expiration_reload"
	defer func() { _ = os.RemoveAll(testDir) }()
	_ = os.RemoveAll(testDir)

	c := New(testDir)
	if err := c.Set("permanent", []byte("value"), NoExpiration); err != nil {
		t.Fatalf("Set(permanent, NoExpiration) returned error: %v", err)
	}

	reloaded := New(testDir)
	defer func() { _ = reloaded.Clear() }()

	got, err := reloaded.Get("permanent")
	if err != nil {
		t.Fatalf("reloaded Get(permanent) returned error: %v", err)
	}
	if !bytes.Equal(got, []byte("value")) {
		t.Errorf("reloaded Get(permanent) = %q, want %q", got, "value")
	}
}

func TestDiskCacheLRUEviction(t *testing.T) {
	testDir := "uploads/test_diskcache_lru"
	defer func() { _ = os.RemoveAll(testDir) }()
	_ = os.RemoveAll(testDir)

	c := New(testDir)
	defer func() { _ = c.Clear() }()

	// Force a very small max size of 20 bytes for testing (8 bytes header + payload)
	// So 2 items of 2 bytes payload = 2 * (8 + 2) = 20 bytes max.
	c.maxSize = 20
	c.lruEnabled = true

	// Write item 1: 8 + 2 = 10 bytes
	err := c.Set("k1", []byte("v1"), DefaultExpiration)
	if err != nil {
		t.Fatalf("failed to set k1: %v", err)
	}

	// Write item 2: 8 + 2 = 10 bytes
	err = c.Set("k2", []byte("v2"), DefaultExpiration)
	if err != nil {
		t.Fatalf("failed to set k2: %v", err)
	}

	// Both should exist
	if _, err := c.Get("k1"); err != nil {
		t.Errorf("k1 should exist: %v", err)
	}
	if _, err := c.Get("k2"); err != nil {
		t.Errorf("k2 should exist: %v", err)
	}

	// Write item 3: 8 + 2 = 10 bytes -> total size would be 30, exceeding 20.
	// This should evict the oldest item. Since k1 was accessed, but then k2 was accessed,
	// wait, let's access k1 again to make it the most recently used, so k2 becomes oldest!
	_, _ = c.Get("k1") // k1 is now MRU, k2 is LRU

	err = c.Set("k3", []byte("v3"), DefaultExpiration)
	if err != nil {
		t.Fatalf("failed to set k3: %v", err)
	}

	// k2 should be evicted, k1 and k3 should exist
	_, err = c.Get("k2")
	if err != ErrCacheMiss {
		t.Errorf("expected k2 to be evicted, got error %v", err)
	}

	if _, err := c.Get("k1"); err != nil {
		t.Errorf("k1 should still exist: %v", err)
	}

	if _, err := c.Get("k3"); err != nil {
		t.Errorf("k3 should exist: %v", err)
	}
}
