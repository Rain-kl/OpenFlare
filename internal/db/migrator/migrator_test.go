// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package migrator

import (
	"context"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"gorm.io/gorm"
)

// expectedMigratedSystemConfigCount 包含初始 32 项系统配置，以及 202606220004
// 从 of_options 迁移过来的 48 项业务配置（OpenFlare/UptimeKuma/OpenResty）。
const expectedMigratedSystemConfigCount = 80

func TestMigrateInitializesSQLiteDatabase(t *testing.T) {
	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("gorm.Open(sqlite) error = %v", err)
	}

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
		MaintNotificationsConfig: &maintnotifications.Config{
			Mode: maintnotifications.ModeDisabled,
		},
	})

	previousDBEnabled := config.Config.Database.Enabled
	config.Config.Database.Enabled = false
	db.SetDB(sqliteDB)
	t.Cleanup(func() {
		config.Config.Database.Enabled = previousDBEnabled
		db.SetDB(nil)
		_ = redisClient.Close()
		mr.Close()
	})

	Migrate()

	var systemConfigCount int64
	if err := sqliteDB.Table("w_system_configs").Count(&systemConfigCount).Error; err != nil {
		t.Fatalf("Migrate() count w_system_configs error = %v", err)
	}
	if systemConfigCount != expectedMigratedSystemConfigCount {
		t.Errorf("Migrate() w_system_configs count = %d, want %d", systemConfigCount, expectedMigratedSystemConfigCount)
	}

	var adminCount int64
	if err := sqliteDB.Table("w_users").Where("username = ?", "admin").Count(&adminCount).Error; err != nil {
		t.Fatalf("Migrate() count admin user error = %v", err)
	}
	if adminCount != 1 {
		t.Errorf("Migrate() admin user count = %d, want %d", adminCount, 1)
	}

	var templateCount int64
	if err := sqliteDB.Table("w_templates").Count(&templateCount).Error; err != nil {
		t.Fatalf("Migrate() count templates error = %v", err)
	}
	if templateCount != 2 {
		t.Errorf("Migrate() templates count = %d, want %d", templateCount, 2)
	}

	if !sqliteDB.Migrator().HasTable("of_zones") {
		t.Error("Migrate() did not create of_zones")
	}
	if !sqliteDB.Migrator().HasTable("of_zone_domains") {
		t.Error("Migrate() did not create of_zone_domains")
	}
	if sqliteDB.Migrator().HasTable("of_managed_domains") {
		t.Error("Migrate() should drop of_managed_domains after phase-2 cleanup")
	}
	if sqliteDB.Migrator().HasColumn(&model.ProxyRoute{}, "domain") {
		t.Error("Migrate() should drop of_proxy_routes.domain after phase-2 cleanup")
	}
	if sqliteDB.Migrator().HasColumn(&model.ProxyRoute{}, "domains") {
		t.Error("Migrate() should drop of_proxy_routes.domains after phase-2 cleanup")
	}
	if sqliteDB.Migrator().HasColumn(&model.ProxyRoute{}, "cert_id") {
		t.Error("Migrate() should drop of_proxy_routes.cert_id after phase-2 cleanup")
	}
	if sqliteDB.Migrator().HasColumn(&model.ProxyRoute{}, "cert_ids") {
		t.Error("Migrate() should drop of_proxy_routes.cert_ids after phase-2 cleanup")
	}
	if sqliteDB.Migrator().HasColumn(&model.ProxyRoute{}, "domain_cert_ids") {
		t.Error("Migrate() should drop of_proxy_routes.domain_cert_ids after phase-2 cleanup")
	}

	zone := model.Zone{Domain: "example.com"}
	if err := sqliteDB.Create(&zone).Error; err != nil {
		t.Fatalf("Migrate() create Zone error = %v", err)
	}
	if err := sqliteDB.Create(&model.Zone{Domain: zone.Domain}).Error; err == nil {
		t.Error("Migrate() allowed duplicate of_zones.domain")
	}

	domain := model.ZoneDomain{ZoneID: zone.ID, Domain: "api.example.com"}
	if err := sqliteDB.Create(&domain).Error; err != nil {
		t.Fatalf("Migrate() create ZoneDomain error = %v", err)
	}
	if err := sqliteDB.Create(&model.ZoneDomain{ZoneID: zone.ID, Domain: domain.Domain}).Error; err == nil {
		t.Error("Migrate() allowed duplicate of_zone_domains.domain")
	}
}

func TestMigrateClearsStaleSystemConfigCache(t *testing.T) {
	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("gorm.Open(sqlite) error = %v", err)
	}

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	previousDBEnabled := config.Config.Database.Enabled
	previousRedis := db.Redis
	config.Config.Database.Enabled = false
	db.SetDB(sqliteDB)
	db.Redis = redisClient
	t.Cleanup(func() {
		config.Config.Database.Enabled = previousDBEnabled
		db.SetDB(nil)
		db.Redis = previousRedis
		_ = redisClient.Close()
		mr.Close()
	})

	staleConfig := model.SystemConfig{
		Key:   model.ConfigKeyEmailLoginVerificationEnabled,
		Value: "true",
		Type:  "system",
	}
	if err := db.HSetJSON(context.Background(), repository.SystemConfigRedisHashKey, model.ConfigKeyEmailLoginVerificationEnabled, &staleConfig); err != nil {
		t.Fatalf("HSetJSON() error = %v", err)
	}

	Migrate()

	exists, err := db.Redis.Exists(context.Background(), db.PrefixedKey(repository.SystemConfigRedisHashKey)).Result()
	if err != nil {
		t.Fatalf("Redis.Exists() error = %v", err)
	}
	if exists != 0 {
		t.Fatalf("system config cache exists = %d, want 0", exists)
	}

	enabled, err := repository.GetBoolByKey(context.Background(), model.ConfigKeyEmailLoginVerificationEnabled)
	if err != nil {
		t.Fatalf("GetBoolByKey(%s) error = %v", model.ConfigKeyEmailLoginVerificationEnabled, err)
	}
	if enabled {
		t.Fatalf("GetBoolByKey(%s) = true, want false", model.ConfigKeyEmailLoginVerificationEnabled)
	}
}
