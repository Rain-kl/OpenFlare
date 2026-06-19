// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestStorageCache(t *testing.T) {
	// 1. Reset cache
	ResetCache()

	if activeConfigJSON != "" || activeDriver != "" || activeBackend != nil || !lastChecked.IsZero() {
		t.Fatal("ResetCache did not clear cache variables")
	}

	// 2. Set up cache manually
	expectedConfig := Config{
		Driver: DriverLocal,
		Local:  LocalConfig{Root: "/tmp/wavelet-test"},
	}
	cfgJSON, err := json.Marshal(expectedConfig)
	if err != nil {
		t.Fatalf("Marshal config failed: %v", err)
	}

	cacheMutex.Lock()
	activeConfigJSON = string(cfgJSON)
	lastChecked = time.Now()
	cacheMutex.Unlock()

	// 3. Call LoadConfig and verify it loads from cache (doesn't hit database, which would fail/panic because DB is not initialized)
	ctx := context.Background()
	loadedCfg, err := LoadConfig(ctx)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if loadedCfg.Driver != expectedConfig.Driver || loadedCfg.Local.Root != expectedConfig.Local.Root {
		t.Errorf("Loaded config %+v, expected %+v", loadedCfg, expectedConfig)
	}

	// 4. Test Active() returns cached driver and backend
	mockBnd := &functionBackend{
		put:    func(context.Context, string, io.Reader, int64, string) error { return nil },
		get:    func(context.Context, string) (*Object, error) { return nil, nil },
		delete: func(context.Context, string) error { return nil },
	}

	cacheMutex.Lock()
	activeBackend = mockBnd
	activeDriver = DriverLocal
	cacheMutex.Unlock()

	drv, bnd, err := Active(ctx)
	if err != nil {
		t.Fatalf("Active failed: %v", err)
	}
	if drv != DriverLocal || bnd != mockBnd {
		t.Errorf("Active returned driver %v, backend %v; expected %v, %v", drv, bnd, DriverLocal, mockBnd)
	}

	// 5. Test ResetCache again
	ResetCache()
	if activeConfigJSON != "" || activeDriver != "" || activeBackend != nil || !lastChecked.IsZero() {
		t.Fatal("ResetCache did not clear cache variables after setting them")
	}
}

func TestStorageCachePubSub(t *testing.T) {
	// 1. Start miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to run miniredis: %v", err)
	}
	defer mr.Close()

	// 2. Initialize Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer rdb.Close()

	// 3. Set db.Redis to our client
	oldRedis := db.Redis
	db.Redis = rdb
	defer func() {
		db.Redis = oldRedis
	}()

	// Reset cache and set some cached config
	ResetCache()
	cacheMutex.Lock()
	activeConfigJSON = "some_config"
	lastChecked = time.Now()
	cacheMutex.Unlock()

	// 4. Force trigger lazy initialization of subscription
	// Reset the once guard so it runs the listener
	pubSubOnce = sync.Once{}
	ctx := context.Background()

	// Create mock backend for Active call
	mockBnd := &functionBackend{
		put:    func(context.Context, string, io.Reader, int64, string) error { return nil },
		get:    func(context.Context, string) (*Object, error) { return nil, nil },
		delete: func(context.Context, string) error { return nil },
	}
	cacheMutex.Lock()
	activeBackend = mockBnd
	activeDriver = DriverLocal
	cacheMutex.Unlock()

	_, _, _ = Active(ctx) // This calls startPubSubListener()

	// Allow some time for subscriber connection
	time.Sleep(100 * time.Millisecond)

	// 5. Publish cache invalidation
	PublishCacheInvalidation(ctx)

	// Allow message propagation
	time.Sleep(100 * time.Millisecond)

	// 6. Verify cache was cleared
	cacheMutex.RLock()
	configJSON := activeConfigJSON
	cacheMutex.RUnlock()

	if configJSON != "" {
		t.Error("Memory cache was not cleared after Redis Pub/Sub broadcast")
	}
}
