// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"errors"
	"testing"
	"time"

	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

func TestMigratingReadsConfig(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, key string) (string, error) {
		if key == logMigrationKey {
			return "migrating", nil
		}
		return "", nil
	})
	if !Migrating(context.Background()) {
		t.Fatal("Migrating() = false, want true when key=migrating")
	}
	SetConfigReader(func(_ context.Context, key string) (string, error) {
		return "", nil
	})
	if Migrating(context.Background()) {
		t.Fatal("Migrating() = true, want false when key empty")
	}
}

func TestResolveDatabaseDefaults(t *testing.T) {
	ResetForTest()
	// 配置缺失（reader 返回空值）时按主库规则 seed（config.Config 默认值由既有测试基建决定）。
	SetConfigReader(func(_ context.Context, key string) (string, error) {
		return "", nil
	})
	got, err := resolveDatabase(context.Background())
	if err != nil {
		t.Fatalf("resolveDatabase: %v", err)
	}
	if got != "postgres" && got != "sqlite" && got != "clickhouse" {
		t.Fatalf("unexpected default log database: %s", got)
	}
}

func TestResolveDatabaseSurfacesReadError(t *testing.T) {
	ResetForTest()
	wantErr := errors.New("boom")
	SetConfigReader(func(_ context.Context, key string) (string, error) {
		return "", wantErr
	})
	if _, err := resolveDatabase(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("resolveDatabase error = %v, want %v", err, wantErr)
	}
}

func TestActiveBuildsStore(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, key string) (string, error) {
		if key == logDatabaseKey {
			return "sqlite", nil
		}
		return "", nil
	})
	store, err := Active(context.Background())
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if store == nil {
		t.Fatal("Active() returned nil store")
	}
	if store.AccessLogs == nil || store.Observability == nil || store.UserAccessLogs == nil || store.Status == nil {
		t.Fatalf("Active() store fields not fully wired: %+v", store)
	}
	// 再次调用应命中缓存。
	again, err := Active(context.Background())
	if err != nil {
		t.Fatalf("Active (cached): %v", err)
	}
	if again != store {
		t.Fatal("Active() did not return cached store")
	}
}

// TestClickHouseUserAccessLogBatchInsertFreeze 覆盖 CH 用户访问日志 flush 的冻结检查：
// 冻结期非空批次返回 ErrMigrating（在触碰 CH 连接之前），空批次直接成功。
func TestClickHouseUserAccessLogBatchInsertFreeze(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, key string) (string, error) {
		if key == logMigrationKey {
			return "migrating", nil
		}
		return "", nil
	})
	defer ResetForTest()
	s := newClickHouseUserAccessLogStore()
	ctx := context.Background()
	now := time.Now()

	if err := s.BatchInsert(ctx, []analyticsmodel.UserAccessLog{{UserID: 1, CreatedAt: now}}); !errors.Is(err, ErrMigrating) {
		t.Fatalf("BatchInsert during migration: want ErrMigrating, got %v", err)
	}
	if err := s.BatchInsert(ctx, nil); err != nil {
		t.Fatalf("BatchInsert empty batch: %v", err)
	}
}
