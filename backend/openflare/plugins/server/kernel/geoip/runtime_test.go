// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package geoip

import (
	"context"
	"testing"

	"Wavelet/openflare/plugins/server/kernel/model"
	"Wavelet/openflare/plugins/server/kernel/repository"
	pkggeoip "Wavelet/openflare/share/geoip"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestEnsureRuntimeProviderInitializesConfiguredProvider(t *testing.T) {
	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := sqliteDB.AutoMigrate(&model.SystemConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repository.SetDBForTest(sqliteDB)
	t.Cleanup(func() {
		repository.SetDBForTest(nil)
		ResetRuntimeForTest()
	})

	ctx := context.Background()
	ResetRuntimeForTest()
	// 通过 SystemConfig 设置 GeoIPProvider 配置
	if err := repository.DB(ctx).Create(&model.SystemConfig{
		Key:        model.ConfigKeyGeoIPProvider,
		Value:      pkggeoip.ProviderIPInfo,
		Type:       "business",
		Visibility: 0,
	}).Error; err != nil {
		t.Fatalf("create system config: %v", err)
	}

	if err := EnsureRuntimeProvider(ctx); err != nil {
		t.Fatalf("EnsureRuntimeProvider error = %v", err)
	}
	if pkggeoip.CurrentProvider == nil || pkggeoip.CurrentProvider.Name() == "EmptyProvider" {
		t.Fatalf("expected ipinfo provider, got %#v", pkggeoip.CurrentProvider)
	}
}
