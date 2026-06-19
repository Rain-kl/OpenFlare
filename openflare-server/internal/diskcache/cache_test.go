// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package diskcache

import (
	"context"
	"os"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
)

func TestDiskCacheReloadConfig(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	testDir := "uploads/test_diskcache_reload"
	defer func() { _ = os.RemoveAll(testDir) }()
	_ = os.RemoveAll(testDir)

	c := New(testDir)
	defer func() { _ = c.Clear() }()

	// Update DB config values
	dbConn.Model(&model.SystemConfig{}).Where("key = ?", model.ConfigKeyDiskCacheMaxSizeMB).Update("value", "250")
	dbConn.Model(&model.SystemConfig{}).Where("key = ?", model.ConfigKeyDiskCacheTTLMinutes).Update("value", "120")
	dbConn.Model(&model.SystemConfig{}).Where("key = ?", model.ConfigKeyDiskCacheLRUEnabled).Update("value", "false")

	// Invalidate Redis config cache to force DB reload
	if db.Redis != nil {
		db.Redis.Del(context.Background(), db.PrefixedKey(repository.SystemConfigRedisHashKey))
	}

	// Reload config
	c.ReloadConfig(context.Background())

	status := c.Status()
	if status.MaxSizeMB != 250 {
		t.Errorf("expected MaxSizeMB to be 250, got %d", status.MaxSizeMB)
	}
	if status.TTLMinutes != 120 {
		t.Errorf("expected TTLMinutes to be 120, got %d", status.TTLMinutes)
	}
	if status.LRUEnabled != false {
		t.Errorf("expected LRUEnabled to be false, got %t", status.LRUEnabled)
	}
}
