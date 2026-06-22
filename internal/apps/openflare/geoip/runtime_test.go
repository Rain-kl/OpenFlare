package geoip

import (
	"context"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	pkggeoip "github.com/Rain-kl/Wavelet/pkg/geoip"
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
	db.SetDB(sqliteDB)
	t.Cleanup(func() {
		db.SetDB(nil)
		ResetRuntimeForTest()
	})

	ctx := context.Background()
	ResetRuntimeForTest()
	// 通过 SystemConfig 设置 GeoIPProvider 配置
	if err := db.DB(ctx).Create(&model.SystemConfig{
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
