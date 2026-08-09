// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

// cleanupTestModels 清理涉及的 5 张日志/可观测表。
func cleanupTestModels() []any {
	return []any{
		&analyticsmodel.NodeAccessLog{},
		&analyticsmodel.NodeMetricSnapshot{},
		&analyticsmodel.NodeEdgeHealth{},
		&analyticsmodel.NodeObsFrps{},
		&analyticsmodel.NodeObsFrpc{},
	}
}

// newCleanupTestDB 构造内存 sqlite 库并注入 db.DB（CleanupExpired 经 Active → buildStore 使用）。
func newCleanupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:logstore-cleanup-%d?mode=memory&cache=shared", atomic.AddInt64(&testGormStoreSeq, 1))
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := gdb.AutoMigrate(cleanupTestModels()...); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	db.SetDB(gdb)
	t.Cleanup(func() { db.SetDB(nil) })
	return gdb
}

// TestCleanupExpiredSQLite 验证 sqlite 激活库的过期日志清理：
// 注入 log_retention_days_sqlite=30，40 天前的 5 表记录被删、昨天的保留。
func TestCleanupExpiredSQLite(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, key string) (string, error) {
		switch key {
		case logDatabaseKey:
			return "sqlite", nil
		case model.ConfigKeyLogRetentionDaysSQLite:
			return "30", nil
		}
		return "", nil
	})
	defer ResetForTest()

	gdb := newCleanupTestDB(t)
	ctx := context.Background()
	old := time.Now().AddDate(0, 0, -40).UTC()
	recent := time.Now().AddDate(0, 0, -1).UTC()

	if err := gdb.Create([]analyticsmodel.NodeAccessLog{
		{ID: 1, NodeID: "n1", LoggedAt: old, RemoteAddr: "1.1.1.1"},
		{ID: 2, NodeID: "n1", LoggedAt: recent, RemoteAddr: "2.2.2.2"},
	}).Error; err != nil {
		t.Fatalf("seed node access logs: %v", err)
	}
	if err := gdb.Create([]analyticsmodel.NodeMetricSnapshot{
		{ID: 1, NodeID: "n1", CapturedAt: old},
		{ID: 2, NodeID: "n1", CapturedAt: recent},
	}).Error; err != nil {
		t.Fatalf("seed metric snapshots: %v", err)
	}
	if err := gdb.Create([]analyticsmodel.NodeEdgeHealth{
		{ID: 1, NodeID: "n1", CapturedAt: old},
		{ID: 2, NodeID: "n1", CapturedAt: recent},
	}).Error; err != nil {
		t.Fatalf("seed edge health: %v", err)
	}
	if err := gdb.Create([]analyticsmodel.NodeObsFrps{
		{ID: 1, NodeID: "n1", CapturedAt: old},
		{ID: 2, NodeID: "n1", CapturedAt: recent},
	}).Error; err != nil {
		t.Fatalf("seed obs frps: %v", err)
	}
	if err := gdb.Create([]analyticsmodel.NodeObsFrpc{
		{ID: 1, NodeID: "n1", CapturedAt: old},
		{ID: 2, NodeID: "n1", CapturedAt: recent},
	}).Error; err != nil {
		t.Fatalf("seed obs frpc: %v", err)
	}

	summary, err := CleanupExpired(ctx)
	if err != nil {
		t.Fatalf("CleanupExpired: %v", err)
	}
	if summary.ActiveDatabase != "sqlite" {
		t.Fatalf("ActiveDatabase = %q, want sqlite", summary.ActiveDatabase)
	}
	if summary.RetentionDays != 30 {
		t.Fatalf("RetentionDays = %d, want 30", summary.RetentionDays)
	}
	if summary.Deleted != 5 {
		t.Fatalf("Deleted = %d, want 5", summary.Deleted)
	}
	if len(summary.Tables) != 5 {
		t.Fatalf("Tables = %v, want 5 tables", summary.Tables)
	}

	assertCount := func(m any, want int64, label string) {
		t.Helper()
		var n int64
		if err := gdb.Model(m).Count(&n).Error; err != nil {
			t.Fatalf("count %s: %v", label, err)
		}
		if n != want {
			t.Fatalf("%s count = %d, want %d", label, n, want)
		}
	}
	assertCount(&analyticsmodel.NodeAccessLog{}, 1, "node_access_logs")
	assertCount(&analyticsmodel.NodeMetricSnapshot{}, 1, "metric_snapshots")
	assertCount(&analyticsmodel.NodeEdgeHealth{}, 1, "edge_health")
	assertCount(&analyticsmodel.NodeObsFrps{}, 1, "obs_frps")
	assertCount(&analyticsmodel.NodeObsFrpc{}, 1, "obs_frpc")

	var kept analyticsmodel.NodeAccessLog
	if err := gdb.First(&kept).Error; err != nil {
		t.Fatalf("recent node access log missing: %v", err)
	}
	if kept.ID != 2 {
		t.Fatalf("kept log ID = %d, want 2 (recent)", kept.ID)
	}
}

// TestRetentionDaysForDatabase 覆盖保留天数读取：按激活库选 key、非法值回退默认 90。
func TestRetentionDaysForDatabase(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, key string) (string, error) {
		switch key {
		case logDatabaseKey:
			return "sqlite", nil
		case model.ConfigKeyLogRetentionDaysSQLite:
			return "30", nil
		}
		return "", nil
	})
	if got := retentionDaysForDatabase(context.Background(), "sqlite"); got != 30 {
		t.Fatalf("retentionDaysForDatabase = %d, want 30", got)
	}

	// 非法值（非数字/<=0）回退默认 90。
	SetConfigReader(func(_ context.Context, key string) (string, error) {
		switch key {
		case logDatabaseKey:
			return "postgres", nil
		case model.ConfigKeyLogRetentionDaysPostgres:
			return "abc", nil
		}
		return "", nil
	})
	if got := retentionDaysForDatabase(context.Background(), "postgres"); got != 90 {
		t.Fatalf("retentionDaysForDatabase invalid value = %d, want 90", got)
	}

	// reader 报错回退默认 90。
	SetConfigReader(func(_ context.Context, _ string) (string, error) {
		return "", fmt.Errorf("boom")
	})
	if got := retentionDaysForDatabase(context.Background(), "postgres"); got != 90 {
		t.Fatalf("retentionDaysForDatabase reader error = %d, want 90", got)
	}
}

// TestPartitionStatements 验证 PG 分区 DDL 生成：当前月 + 未来 2 个月 × 2 表，
// 幂等 PARTITION OF 语句与迁移 SQL 命名一致（含跨年）。
func TestPartitionStatements(t *testing.T) {
	now := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
	stmts := partitionStatementsRange(now, now.AddDate(0, 2, 0))
	if len(stmts) != 6 {
		t.Fatalf("partitionStatements len = %d, want 6", len(stmts))
	}
	want := []string{
		"CREATE TABLE IF NOT EXISTS of_node_access_logs_202608 PARTITION OF of_node_access_logs FOR VALUES FROM ('2026-08-01') TO ('2026-09-01')",
		"CREATE TABLE IF NOT EXISTS w_user_access_logs_202608 PARTITION OF w_user_access_logs FOR VALUES FROM ('2026-08-01') TO ('2026-09-01')",
		"CREATE TABLE IF NOT EXISTS of_node_access_logs_202609 PARTITION OF of_node_access_logs FOR VALUES FROM ('2026-09-01') TO ('2026-10-01')",
		"CREATE TABLE IF NOT EXISTS w_user_access_logs_202609 PARTITION OF w_user_access_logs FOR VALUES FROM ('2026-09-01') TO ('2026-10-01')",
		"CREATE TABLE IF NOT EXISTS of_node_access_logs_202610 PARTITION OF of_node_access_logs FOR VALUES FROM ('2026-10-01') TO ('2026-11-01')",
		"CREATE TABLE IF NOT EXISTS w_user_access_logs_202610 PARTITION OF w_user_access_logs FOR VALUES FROM ('2026-10-01') TO ('2026-11-01')",
	}
	for i, w := range want {
		if stmts[i] != w {
			t.Fatalf("stmt[%d] = %q, want %q", i, stmts[i], w)
		}
	}

	// 跨年：2026-11 → 202611, 202612, 202701。
	nov := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	suffixes := []string{"202611", "202612", "202701"}
	for _, stmt := range partitionStatementsRange(nov, nov.AddDate(0, 2, 0)) {
		if !hasAnySuffix(stmt, suffixes) {
			t.Fatalf("statement lacks expected month suffix: %s", stmt)
		}
	}
}

func hasAnySuffix(stmt string, suffixes []string) bool {
	for _, table := range []string{"of_node_access_logs", "w_user_access_logs"} {
		for _, suf := range suffixes {
			if strings.Contains(stmt, table+"_"+suf) {
				return true
			}
		}
	}
	return false
}
