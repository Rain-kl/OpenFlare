// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service_test

import (
	"Wavelet/plugins/domain/admin/model"
	"Wavelet/plugins/domain/admin/repository"
	"Wavelet/plugins/domain/admin/service"
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type testDBService struct {
	db *gorm.DB
}

func (s *testDBService) DB(ctx context.Context) *gorm.DB {
	return s.db
}

func (s *testDBService) MasterDB(ctx context.Context) *gorm.DB {
	return s.db
}

func (s *testDBService) GORM() *gorm.DB {
	return s.db
}

func (s *testDBService) Named(_ string) *gorm.DB {
	return s.db
}

func setupSystemConfigTest(t *testing.T) (*gorm.DB, func()) {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("gorm.Open(sqlite) error = %v", err)
	}
	if err := sqliteDB.AutoMigrate(&model.SystemConfig{}); err != nil {
		t.Fatalf("AutoMigrate(SystemConfig) error = %v", err)
	}

	siteConfig := model.SystemConfig{
		Key:         model.ConfigKeySiteName,
		Value:       "Wavelet",
		Type:        "system",
		Description: "系统平台的展示名称",
	}
	if err := sqliteDB.Create(&siteConfig).Error; err != nil {
		t.Fatalf("Create(site_name) error = %v", err)
	}

	service.SetDBService(&testDBService{db: sqliteDB})

	cleanup := func() {
		repository.StopSystemConfigCacheListener()
		repository.ResetSystemConfigRAMCacheForTest()
		service.ResetServices()
	}

	return sqliteDB, cleanup
}

func TestPublicSystemConfigsExposesVisibleKeys(t *testing.T) {
	dbConn, cleanup := setupSystemConfigTest(t)
	defer cleanup()
	repository.ResetSystemConfigRAMCacheForTest()
	ctx := context.Background()

	hidden := model.SystemConfig{
		Key:        "secret_key",
		Value:      "nope",
		Type:       "system",
		Visibility: model.ConfigVisibilityHidden,
	}
	visible := model.SystemConfig{
		Key:        model.ConfigKeyCapLoginEnabled,
		Value:      "true",
		Type:       "system",
		Visibility: model.ConfigVisibilityVisible,
	}
	if err := dbConn.Create(&hidden).Error; err != nil {
		t.Fatalf("Create(hidden) error = %v", err)
	}
	if err := dbConn.Create(&visible).Error; err != nil {
		t.Fatalf("Create(visible) error = %v", err)
	}

	got, err := service.PublicSystemConfigs(ctx)
	if err != nil {
		t.Fatalf("PublicSystemConfigs() error = %v", err)
	}
	if got[model.ConfigKeyCapLoginEnabled] != "true" {
		t.Fatalf("PublicSystemConfigs()[%s] = %q, want %q", model.ConfigKeyCapLoginEnabled, got[model.ConfigKeyCapLoginEnabled], "true")
	}
	if _, ok := got["secret_key"]; ok {
		t.Fatalf("PublicSystemConfigs() leaked hidden key secret_key")
	}

	viaProvider, err := service.PublicConfigAdapter{}.PublicConfig(ctx)
	if err != nil {
		t.Fatalf("PublicConfigAdapter.PublicConfig() error = %v", err)
	}
	if viaProvider[model.ConfigKeyCapLoginEnabled] != "true" {
		t.Fatalf("PublicConfigAdapter.PublicConfig()[%s] = %q, want %q", model.ConfigKeyCapLoginEnabled, viaProvider[model.ConfigKeyCapLoginEnabled], "true")
	}
}

func TestListSystemConfigsByKeys_EmptyKeys(t *testing.T) {
	result, err := repository.ListSystemConfigsByKeys(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListSystemConfigsByKeys(nil) error = %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("ListSystemConfigsByKeys(nil) = %#v, want empty map", result)
	}
}

func TestListSystemConfigsByKeys_LoadsFromRAMCache(t *testing.T) {
	dbConn, cleanup := setupSystemConfigTest(t)
	defer cleanup()
	ctx := context.Background()

	repository.ResetSystemConfigRAMCacheForTest()

	// Initial load
	warm, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeySiteName)
	if err != nil {
		t.Fatalf("GetSystemConfigByKey(site_name) warm error = %v", err)
	}
	if warm.Value != "Wavelet" {
		t.Fatalf("GetSystemConfigByKey(site_name).Value = %q, want %q", warm.Value, "Wavelet")
	}

	// Update DB directly
	if err := dbConn.Model(&model.SystemConfig{}).
		Where("key = ?", model.ConfigKeySiteName).
		Update("value", "db_only_value").Error; err != nil {
		t.Fatalf("Update(site_name) error = %v", err)
	}

	// Fetch via ListSystemConfigsByKeys should serve from local store (meaning the old value "Wavelet")
	configs, err := repository.ListSystemConfigsByKeys(ctx, []string{model.ConfigKeySiteName})
	if err != nil {
		t.Fatalf("ListSystemConfigsByKeys(site_name) error = %v", err)
	}

	sc, ok := configs[model.ConfigKeySiteName]
	if !ok {
		t.Fatal("ListSystemConfigsByKeys(site_name) missing site_name entry")
	}
	if sc.Value != "Wavelet" {
		t.Fatalf("ListSystemConfigsByKeys(site_name).Value = %q, want cached value %q", sc.Value, "Wavelet")
	}
}

func TestGetSystemConfigByGroupAndInvalidation(t *testing.T) {
	dbConn, cleanup := setupSystemConfigTest(t)
	defer cleanup()
	ctx := context.Background()

	repository.ResetSystemConfigRAMCacheForTest()

	// Get via specific group/type
	cfg, err := repository.GetSystemConfigByGroup(ctx, repository.ConfigCacheType, model.ConfigKeySiteName)
	if err != nil {
		t.Fatalf("GetSystemConfigByGroup error = %v", err)
	}
	if cfg.Value != "Wavelet" {
		t.Fatalf("value = %q, want %q", cfg.Value, "Wavelet")
	}

	// Direct DB update
	if err := dbConn.Model(&model.SystemConfig{}).
		Where("key = ?", model.ConfigKeySiteName).
		Update("value", "new_site_name").Error; err != nil {
		t.Fatalf("DB Update error = %v", err)
	}

	// Invalidate
	if err := repository.InvalidateSystemConfigCache(ctx, model.ConfigKeySiteName); err != nil {
		t.Fatalf("InvalidateSystemConfigCache error = %v", err)
	}

	// Wait for broadcast execution
	time.Sleep(100 * time.Millisecond)

	// Fetch again
	updated, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeySiteName)
	if err != nil {
		t.Fatalf("GetSystemConfigByKey error = %v", err)
	}
	if updated.Value != "new_site_name" {
		t.Fatalf("value = %q, want %q", updated.Value, "new_site_name")
	}
}
