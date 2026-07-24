// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/upload/shared"
	uploadstorage "github.com/Rain-kl/Wavelet/internal/apps/upload/storage"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
)

func TestLoadMigrationAccessStateCachesResult(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ResetAccessCaches()

	ctx := context.Background()
	first := uploadstorage.LoadMigrationAccessState(ctx)
	second := uploadstorage.LoadMigrationAccessState(ctx)

	if first.ReadOnly != second.ReadOnly {
		t.Fatalf("readOnly mismatch: first=%v second=%v", first.ReadOnly, second.ReadOnly)
	}
	if first.HasTarget != second.HasTarget {
		t.Fatalf("hasTarget mismatch: first=%v second=%v", first.HasTarget, second.HasTarget)
	}
}

func TestIsFilePublicUsesCachedWhitelist(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ResetAccessCaches()

	ctx := context.Background()
	if !IsFilePublic(ctx, "avatar") {
		t.Fatal("expected avatar to be public by default")
	}
	if IsFilePublic(ctx, "attachment") {
		t.Fatal("expected attachment to be private by default")
	}
	if !IsFilePublic(ctx, "AVATAR") {
		t.Fatal("expected whitelist lookup to be case-insensitive")
	}
}

func TestResetAccessCachesRefreshesWhitelist(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ResetAccessCaches()

	ctx := context.Background()
	if !IsFilePublic(ctx, "avatar") {
		t.Fatal("expected seeded avatar whitelist before reset")
	}

	var sc model.SystemConfig
	if err := dbConn.Where("key = ?", model.ConfigKeyFileAccessWhitelist).First(&sc).Error; err != nil {
		t.Fatalf("load whitelist config: %v", err)
	}
	sc.Value = `["attachment"]`
	if err := dbConn.Save(&sc).Error; err != nil {
		t.Fatalf("save whitelist config: %v", err)
	}
	if err := db.HSetJSON(ctx, repository.SystemConfigRedisHashKey, model.ConfigKeyFileAccessWhitelist, &sc); err != nil {
		t.Fatalf("refresh whitelist redis cache: %v", err)
	}
	repository.ResetSystemConfigRAMCacheForTest()

	ResetAccessCaches()
	if !IsFilePublic(ctx, "attachment") {
		t.Fatal("expected attachment to be public after whitelist refresh")
	}
	if IsFilePublic(ctx, "avatar") {
		t.Fatal("expected avatar to be private after whitelist refresh")
	}
}

func TestAccessCacheTTLExpires(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ResetAccessCaches()

	ctx := context.Background()
	_ = loadFileAccessWhitelist(ctx)

	fileAccessWhitelistMu.Lock()
	fileAccessWhitelistCheckedAt = time.Now().Add(-time.Duration(shared.AccessCacheTTL)*time.Second - time.Second)
	fileAccessWhitelistMu.Unlock()

	// Should still work after TTL by reloading from config.
	if !IsFilePublic(ctx, "avatar") {
		t.Fatal("expected whitelist reload after TTL expiration")
	}
}
