// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"context"
	"strings"
	"testing"

	admin_push "github.com/Rain-kl/Wavelet/internal/apps/admin/push"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/repository/logstore"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
)

func TestInitSyncsPushEventsOnce(t *testing.T) {
	ResetInitRuntimeOnceForTest()
	t.Cleanup(ResetInitRuntimeOnceForTest)

	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	if err := dbConn.AutoMigrate(&model.PushEvent{}); err != nil {
		t.Fatalf("auto migrate push events failed: %v", err)
	}

	RegisterPushDomainEvents()

	wantCount := len(admin_push.BuiltInEvents)
	if wantCount < 1 {
		t.Fatalf("built-in push events = %d, want at least 1", wantCount)
	}

	ctx := context.Background()
	Init(ctx, Options{API: true})
	Init(ctx, Options{}) // second Init must not duplicate events (initRuntimeOnce)

	var count int64
	if err := dbConn.Model(&model.PushEvent{}).Count(&count).Error; err != nil {
		t.Fatalf("count push events failed: %v", err)
	}
	if count != int64(wantCount) {
		t.Fatalf("push event count = %d, want %d", count, wantCount)
	}

	var adminLogin model.PushEvent
	if err := dbConn.Where("event_key = ?", "admin_login").First(&adminLogin).Error; err != nil {
		t.Fatalf("admin_login event not found after Init: %v", err)
	}
	if adminLogin.Name != "管理员登录" {
		t.Fatalf("admin_login name = %q, want %q", adminLogin.Name, "管理员登录")
	}
}

func TestValidateAndSeedLogDatabaseSeedsDefault(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	prevDB := config.Config.Database.Enabled
	prevCH := config.Config.ClickHouse.Enabled
	t.Cleanup(func() {
		config.Config.Database.Enabled = prevDB
		config.Config.ClickHouse.Enabled = prevCH
	})

	tests := []struct {
		name      string
		dbEnabled bool
		chEnabled bool
		want      string
	}{
		{name: "sqlite default", dbEnabled: false, chEnabled: false, want: "sqlite"},
		{name: "postgres default", dbEnabled: true, chEnabled: false, want: "postgres"},
		{name: "clickhouse default", dbEnabled: true, chEnabled: true, want: "clickhouse"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Config.Database.Enabled = tt.dbEnabled
			config.Config.ClickHouse.Enabled = tt.chEnabled

			ctx := context.Background()
			// 清掉标记行，模拟首次启动。
			if err := db.DB(ctx).Where("key = ?", model.ConfigKeyLogDatabase).Delete(&model.SystemConfig{}).Error; err != nil {
				t.Fatalf("delete log_database marker failed: %v", err)
			}
			repository.ResetSystemConfigRAMCacheForTest()

			if err := validateAndSeedLogDatabase(ctx); err != nil {
				t.Fatalf("validateAndSeedLogDatabase() error = %v", err)
			}

			repository.ResetSystemConfigRAMCacheForTest()
			cfg, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyLogDatabase)
			if err != nil {
				t.Fatalf("GetSystemConfigByKey(%s) error = %v", model.ConfigKeyLogDatabase, err)
			}
			if cfg.Value != tt.want {
				t.Fatalf("seeded log_database = %q, want %q", cfg.Value, tt.want)
			}
		})
	}
}

func TestValidateAndSeedLogDatabaseUpdatesEmptyMarker(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	dbPrev := config.Config.Database.Enabled
	chPrev := config.Config.ClickHouse.Enabled
	config.Config.Database.Enabled = true
	config.Config.ClickHouse.Enabled = false
	t.Cleanup(func() {
		config.Config.Database.Enabled = dbPrev
		config.Config.ClickHouse.Enabled = chPrev
	})

	ctx := context.Background()
	// 标记行已存在但值为空，等同首次启动，应写入默认值（走更新路径）。
	if err := repository.CreateSystemConfig(ctx, &model.SystemConfig{Key: model.ConfigKeyLogDatabase, Value: "", Type: "system"}); err != nil {
		t.Fatalf("create empty marker failed: %v", err)
	}
	repository.ResetSystemConfigRAMCacheForTest()

	if err := validateAndSeedLogDatabase(ctx); err != nil {
		t.Fatalf("validateAndSeedLogDatabase() error = %v", err)
	}

	repository.ResetSystemConfigRAMCacheForTest()
	cfg, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyLogDatabase)
	if err != nil {
		t.Fatalf("GetSystemConfigByKey(%s) error = %v", model.ConfigKeyLogDatabase, err)
	}
	want := "postgres"
	if cfg.Value != want {
		t.Fatalf("log_database = %q, want %q", cfg.Value, want)
	}
}

func TestValidateAndSeedLogDatabaseRejectsInconsistentConfig(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	prevDB := config.Config.Database.Enabled
	prevCH := config.Config.ClickHouse.Enabled
	t.Cleanup(func() {
		config.Config.Database.Enabled = prevDB
		config.Config.ClickHouse.Enabled = prevCH
	})

	seedMarker := func(t *testing.T, value string) {
		t.Helper()
		ctx := context.Background()
		if err := db.DB(ctx).Where("key = ?", model.ConfigKeyLogDatabase).Delete(&model.SystemConfig{}).Error; err != nil {
			t.Fatalf("delete log_database marker failed: %v", err)
		}
		if err := repository.CreateSystemConfig(ctx, &model.SystemConfig{Key: model.ConfigKeyLogDatabase, Value: value, Type: "system"}); err != nil {
			t.Fatalf("create log_database marker failed: %v", err)
		}
		repository.ResetSystemConfigRAMCacheForTest()
	}

	tests := []struct {
		name      string
		marker    string
		dbEnabled bool
		chEnabled bool
		wantErr   string
	}{
		{name: "clickhouse marker but disabled", marker: "clickhouse", dbEnabled: true, chEnabled: false, wantErr: "ClickHouse 未启用"},
		{name: "postgres marker but disabled", marker: "postgres", dbEnabled: false, chEnabled: false, wantErr: "PostgreSQL 未启用"},
		{name: "sqlite marker but postgres primary", marker: "sqlite", dbEnabled: true, chEnabled: false, wantErr: "SQLite"},
		{name: "unknown marker", marker: "mysql", dbEnabled: false, chEnabled: false, wantErr: "未知的日志主库配置"},
		{name: "consistent sqlite", marker: "sqlite", dbEnabled: false, chEnabled: false, wantErr: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Config.Database.Enabled = tt.dbEnabled
			config.Config.ClickHouse.Enabled = tt.chEnabled
			seedMarker(t, tt.marker)

			err := validateAndSeedLogDatabase(context.Background())
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateAndSeedLogDatabase() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateAndSeedLogDatabase() = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateAndSeedLogDatabase() error = %q, want contains %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestInitWiresLogstoreConfigReader(t *testing.T) {
	ResetInitRuntimeOnceForTest()
	t.Cleanup(ResetInitRuntimeOnceForTest)
	logstore.ResetForTest()
	t.Cleanup(logstore.ResetForTest)

	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()
	// 插入迁移标记：bootstrap 注入的 reader 应能经 repository 读到该值（区分未装配时的兜底行为）。
	if err := repository.CreateSystemConfig(ctx, &model.SystemConfig{Key: model.ConfigKeyLogDBMigration, Value: "migrating", Type: "system"}); err != nil {
		t.Fatalf("create log_db_migration marker failed: %v", err)
	}
	repository.ResetSystemConfigRAMCacheForTest()

	Init(ctx, Options{})

	repository.ResetSystemConfigRAMCacheForTest()
	if !logstore.Migrating(ctx) {
		t.Fatal("logstore config reader not wired after bootstrap.Init: Migrating() = false, want true")
	}
}
