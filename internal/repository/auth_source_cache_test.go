// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"gorm.io/gorm"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

func setupAuthSourceCacheTest(t *testing.T) (*gorm.DB, *miniredis.Miniredis, func()) {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("failed to open in-memory SQLite db: %v", err)
	}
	if err := sqliteDB.AutoMigrate(&model.AuthSource{}); err != nil {
		t.Fatalf("failed to migrate auth sources: %v", err)
	}

	miniRedis, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	db.SetDB(sqliteDB)
	db.Redis = redis.NewClient(&redis.Options{
		Addr: miniRedis.Addr(),
		MaintNotificationsConfig: &maintnotifications.Config{
			Mode: maintnotifications.ModeDisabled,
		},
	})

	ResetAuthSourceRAMCacheForTest()

	cleanup := func() {
		StopAuthSourceCacheListener()
		ResetAuthSourceRAMCacheForTest()
		db.Redis.Close()
		miniRedis.Close()
		db.Redis = nil
	}
	return sqliteDB, miniRedis, cleanup
}

func TestGetActiveAuthSourcesCached_LoadsFromRedisBeforeDB(t *testing.T) {
	dbConn, _, cleanup := setupAuthSourceCacheTest(t)
	defer cleanup()
	ctx := context.Background()

	if err := InvalidateAuthSourceCache(ctx); err != nil {
		t.Fatalf("InvalidateAuthSourceCache() error = %v", err)
	}

	source := model.AuthSource{
		Name:               "cached-source",
		Type:               model.AuthSourceTypeOIDC,
		DisplayName:        "Cached Source",
		IsActive:           true,
		ClientID:           "client-id",
		ClientSecret:       "client-secret",
		OpenIDDiscoveryURL: "https://issuer.example.com",
	}
	if err := model.CreateAuthSource(ctx, &source); err != nil {
		t.Fatalf("CreateAuthSource() error = %v", err)
	}

	warmed, err := GetActiveAuthSourcesCached(ctx)
	if err != nil {
		t.Fatalf("GetActiveAuthSourcesCached() warm error = %v", err)
	}
	if len(warmed) == 0 || warmed[0].Name != source.Name {
		t.Fatalf("GetActiveAuthSourcesCached() warm = %#v, want source %q", warmed, source.Name)
	}

	if err := dbConn.Delete(&model.AuthSource{}, "id = ?", source.ID).Error; err != nil {
		t.Fatalf("Delete(auth source) error = %v", err)
	}

	ResetAuthSourceRAMCacheForTest()

	cached, err := GetActiveAuthSourcesCached(ctx)
	if err != nil {
		t.Fatalf("GetActiveAuthSourcesCached() cached error = %v", err)
	}
	if len(cached) == 0 || cached[0].Name != source.Name {
		t.Fatalf("GetActiveAuthSourcesCached() = %#v, want redis-backed source %q", cached, source.Name)
	}
}

func TestGetAuthSourceByNameCached_LoadsFromRedisBeforeDB(t *testing.T) {
	dbConn, _, cleanup := setupAuthSourceCacheTest(t)
	defer cleanup()
	ctx := context.Background()

	if err := InvalidateAuthSourceCache(ctx); err != nil {
		t.Fatalf("InvalidateAuthSourceCache() error = %v", err)
	}

	source := model.AuthSource{
		Name:               "by-name-source",
		Type:               model.AuthSourceTypeOIDC,
		DisplayName:        "By Name Source",
		IsActive:           true,
		ClientID:           "client-id",
		ClientSecret:       "client-secret",
		OpenIDDiscoveryURL: "https://issuer.example.com",
	}
	if err := model.CreateAuthSource(ctx, &source); err != nil {
		t.Fatalf("CreateAuthSource() error = %v", err)
	}

	warmed, err := GetAuthSourceByNameCached(ctx, source.Name)
	if err != nil {
		t.Fatalf("GetAuthSourceByNameCached() warm error = %v", err)
	}
	if warmed.Name != source.Name || warmed.ClientSecret != source.ClientSecret {
		t.Fatalf("GetAuthSourceByNameCached() warm = %#v, want %#v", warmed, source)
	}

	if err := dbConn.Delete(&model.AuthSource{}, "id = ?", source.ID).Error; err != nil {
		t.Fatalf("Delete(auth source) error = %v", err)
	}

	ResetAuthSourceRAMCacheForTest()

	cached, err := GetAuthSourceByNameCached(ctx, source.Name)
	if err != nil {
		t.Fatalf("GetAuthSourceByNameCached() cached error = %v", err)
	}
	if cached.Name != source.Name || cached.ClientSecret != source.ClientSecret {
		t.Fatalf("GetAuthSourceByNameCached() = %#v, want redis-backed source %#v", cached, source)
	}
}

func TestInvalidateAuthSourceCache_ClearsRedisKeys(t *testing.T) {
	_, _, cleanup := setupAuthSourceCacheTest(t)
	defer cleanup()
	ctx := context.Background()

	if err := InvalidateAuthSourceCache(ctx); err != nil {
		t.Fatalf("InvalidateAuthSourceCache() initial error = %v", err)
	}

	source := model.AuthSource{
		Name:               "invalidate-source",
		Type:               model.AuthSourceTypeOIDC,
		DisplayName:        "Invalidate Source",
		IsActive:           true,
		ClientID:           "client-id",
		ClientSecret:       "client-secret",
		OpenIDDiscoveryURL: "https://issuer.example.com",
	}
	if err := model.CreateAuthSource(ctx, &source); err != nil {
		t.Fatalf("CreateAuthSource() error = %v", err)
	}
	if _, err := GetActiveAuthSourcesCached(ctx); err != nil {
		t.Fatalf("GetActiveAuthSourcesCached() warm error = %v", err)
	}
	if _, err := GetAuthSourceByNameCached(ctx, source.Name); err != nil {
		t.Fatalf("GetAuthSourceByNameCached() warm error = %v", err)
	}

	if err := InvalidateAuthSourceCache(ctx); err != nil {
		t.Fatalf("InvalidateAuthSourceCache() error = %v", err)
	}

	activeExists, err := db.Redis.Exists(ctx, db.PrefixedKey(authSourceActiveRedisKey)).Result()
	if err != nil {
		t.Fatalf("Exists(active key) error = %v", err)
	}
	if activeExists != 0 {
		t.Fatalf("active redis key still exists after invalidation")
	}

	byNameExists, err := db.Redis.Exists(ctx, db.PrefixedKey(authSourceByNameRedisKey(source.Name))).Result()
	if err != nil {
		t.Fatalf("Exists(by-name key) error = %v", err)
	}
	if byNameExists != 0 {
		t.Fatalf("by-name redis key still exists after invalidation")
	}
}

func TestAuthSourceInvalidationPubSubClearsPeerRAM(t *testing.T) {
	dbConn, _, cleanup := setupAuthSourceCacheTest(t)
	defer cleanup()
	ctx := context.Background()

	source := model.AuthSource{
		Name:               "pubsub-source",
		Type:               model.AuthSourceTypeOIDC,
		DisplayName:        "PubSub Source",
		IsActive:           true,
		ClientID:           "client-id",
		ClientSecret:       "client-secret",
		OpenIDDiscoveryURL: "https://issuer.example.com",
	}
	if err := model.CreateAuthSource(ctx, &source); err != nil {
		t.Fatalf("CreateAuthSource() error = %v", err)
	}

	if _, err := GetActiveAuthSourcesCached(ctx); err != nil {
		t.Fatalf("GetActiveAuthSourcesCached() error = %v", err)
	}
	if err := dbConn.Delete(&model.AuthSource{}, "id = ?", source.ID).Error; err != nil {
		t.Fatalf("Delete(auth source) error = %v", err)
	}
	if _, err := GetActiveAuthSourcesCached(ctx); err != nil {
		t.Fatalf("expected RAM cache hit before pub/sub invalidation: %v", err)
	}

	if err := db.Redis.Publish(ctx, authSourceInvalidationChannel, "reset").Err(); err != nil {
		t.Fatalf("publish invalidation: %v", err)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if _, ok := authSourceActiveRAM.GetIfPresent(authSourceActiveRAMKey); !ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, ok := authSourceActiveRAM.GetIfPresent(authSourceActiveRAMKey); ok {
		t.Fatal("expected peer RAM cache to be cleared by pub/sub")
	}
}
