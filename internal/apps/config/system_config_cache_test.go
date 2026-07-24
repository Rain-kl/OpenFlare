// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
)

func TestSystemConfigRAMCacheServesUntilInvalidated(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	repository.ResetSystemConfigRAMCacheForTest()
	if err := repository.InvalidateAllSystemConfigCaches(ctx); err != nil {
		t.Fatalf("InvalidateAllSystemConfigCaches() error = %v", err)
	}
	time.Sleep(50 * time.Millisecond) // Wait for async Redis broadcast to be processed

	warm, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeySiteName)
	if err != nil {
		t.Fatalf("GetSystemConfigByKey(site_name) warm error = %v", err)
	}
	if warm.Value != "OpenFlare" {
		t.Fatalf("GetSystemConfigByKey(site_name).Value = %q, want %q", warm.Value, "OpenFlare")
	}

	if err := dbConn.Model(&model.SystemConfig{}).
		Where("key = ?", model.ConfigKeySiteName).
		Update("value", "ram_probe_value").Error; err != nil {
		t.Fatalf("Update(site_name) error = %v", err)
	}
	if err := db.HDel(ctx, repository.SystemConfigRedisHashKey, model.ConfigKeySiteName); err != nil {
		t.Fatalf("HDel(site_name) error = %v", err)
	}

	cached, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeySiteName)
	if err != nil {
		t.Fatalf("GetSystemConfigByKey(site_name) cached error = %v", err)
	}
	if cached.Value != "OpenFlare" {
		t.Fatalf("GetSystemConfigByKey(site_name).Value = %q, want stale RAM value %q", cached.Value, "OpenFlare")
	}

	if err := repository.InvalidateSystemConfigCache(ctx, model.ConfigKeySiteName); err != nil {
		t.Fatalf("InvalidateSystemConfigCache(site_name) error = %v", err)
	}
	time.Sleep(50 * time.Millisecond) // Wait for async Redis broadcast to be processed

	refreshed, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeySiteName)
	if err != nil {
		t.Fatalf("GetSystemConfigByKey(site_name) refreshed error = %v", err)
	}
	if refreshed.Value != "ram_probe_value" {
		t.Fatalf("GetSystemConfigByKey(site_name).Value = %q, want %q", refreshed.Value, "ram_probe_value")
	}

	// Since system configs are now purely cached in process-local RAM (L1) and not written to Redis (L2),
	// we do not verify if the Redis hash field is repopulated.
}

func TestInvalidateSystemConfigCacheClearsRedisField(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	sc, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeySiteName)
	if err != nil {
		t.Fatalf("GetSystemConfigByKey(site_name) error = %v", err)
	}
	_ = sc

	if err := repository.InvalidateSystemConfigCache(ctx, model.ConfigKeySiteName); err != nil {
		t.Fatalf("InvalidateSystemConfigCache(site_name) error = %v", err)
	}
	time.Sleep(50 * time.Millisecond) // Wait for async Redis broadcast to be processed

	_, err = db.Redis.HGet(ctx, db.PrefixedKey(repository.SystemConfigRedisHashKey), model.ConfigKeySiteName).Result()
	if !errors.Is(err, redis.Nil) {
		t.Fatalf("HGet(site_name) error = %v, want redis.Nil", err)
	}

	if err := dbConn.Model(&model.SystemConfig{}).
		Where("key = ?", model.ConfigKeySiteName).
		Update("value", "after_invalidate").Error; err != nil {
		t.Fatalf("Update(site_name) error = %v", err)
	}

	refreshed, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeySiteName)
	if err != nil {
		t.Fatalf("GetSystemConfigByKey(site_name) refreshed error = %v", err)
	}
	if refreshed.Value != "after_invalidate" {
		t.Fatalf("GetSystemConfigByKey(site_name).Value = %q, want %q", refreshed.Value, "after_invalidate")
	}
}
