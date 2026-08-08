// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

// TestEnsurePartitionsPostgresInsertAcrossMonths 需要 TEST_POSTGRES_DSN（未设置时跳过）：
// 验证 EnsurePartitions 预建任意月份范围分区后，跨月历史数据可写入 PG 分区表
// （对应迁移任务从 CH/SQLite 复制历史日志到 PG 时先预建分区的场景）。
func TestEnsurePartitionsPostgresInsertAcrossMonths(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}

	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	schema := fmt.Sprintf("logstore_partition_%d", time.Now().UnixNano())
	if !regexp.MustCompile(`^[a-z0-9_]+$`).MatchString(schema) {
		t.Fatalf("invalid schema: %s", schema)
	}
	if err := gdb.Exec(`CREATE SCHEMA "` + schema + `"`).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if err := gdb.Exec(`SET search_path TO "` + schema + `"`).Error; err != nil {
		t.Fatalf("set search_path: %v", err)
	}
	t.Cleanup(func() {
		_ = gdb.Exec("SET search_path TO public").Error
		_ = gdb.Exec(`DROP SCHEMA IF EXISTS "` + schema + `" CASCADE`).Error
		_ = sqlDB.Close()
	})

	// 与 goose/postgres/202608080001_create_log_tables.sql 保持一致的分区父表 DDL。
	for _, ddl := range []string{postgresNodeAccessLogsDDL, postgresUserAccessLogsDDL} {
		if err := gdb.Exec(ddl).Error; err != nil {
			t.Fatalf("create partitioned table: %v", err)
		}
	}

	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	defer ResetForTest()

	ctx := context.Background()
	store := newGormStore(gdb)
	ua := newUserAccessLogGormStore(gdb)

	// 源范围跨 3 个月：2026-01-10 ~ 2026-03-20；to+1 月兜底生成 202601..202604 分区。
	from := time.Date(2026, 1, 10, 8, 0, 0, 0, time.UTC)
	max := time.Date(2026, 3, 20, 9, 30, 0, 0, time.UTC)
	if err := store.EnsurePartitions(ctx, from, max.AddDate(0, 1, 0)); err != nil {
		t.Fatalf("EnsurePartitions: %v", err)
	}

	// 幂等：重复调用不报错（CREATE TABLE IF NOT EXISTS ... PARTITION OF）。
	if err := store.EnsurePartitions(ctx, from, max.AddDate(0, 1, 0)); err != nil {
		t.Fatalf("EnsurePartitions idempotent: %v", err)
	}

	var partitionCount int64
	if err := gdb.Raw(
		"SELECT count(*) FROM pg_inherits WHERE inhrelid = to_regclass('of_node_access_logs')",
	).Scan(&partitionCount).Error; err != nil {
		t.Fatalf("count partitions: %v", err)
	}
	if partitionCount != 4 {
		t.Fatalf("of_node_access_logs partitions = %d, want 4", partitionCount)
	}

	// 跨月插入：1/2/3 月各 2 条节点访问日志 + 2 条用户访问日志，均应命中已有分区。
	nodeRows := []analyticsmodel.NodeAccessLog{
		{ID: 1, NodeID: "n1", LoggedAt: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC), RemoteAddr: "1.1.1.1"},
		{ID: 2, NodeID: "n1", LoggedAt: time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC), RemoteAddr: "1.1.1.2"},
		{ID: 3, NodeID: "n2", LoggedAt: time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC), RemoteAddr: "2.2.2.2"},
		{ID: 4, NodeID: "n2", LoggedAt: time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC), RemoteAddr: "2.2.2.3"},
		{ID: 5, NodeID: "n1", LoggedAt: time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC), RemoteAddr: "3.3.3.3"},
		{ID: 6, NodeID: "n1", LoggedAt: time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC), RemoteAddr: "3.3.3.4"},
	}
	if err := store.BatchInsertNodeAccessLogs(ctx, nodeRows); err != nil {
		t.Fatalf("insert node access logs across months: %v", err)
	}

	userRows := []analyticsmodel.UserAccessLog{
		{ID: 1, UserID: 101, Path: "/a", CreatedAt: time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC)},
		{ID: 2, UserID: 102, Path: "/b", CreatedAt: time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)},
	}
	if err := ua.BatchInsert(ctx, userRows); err != nil {
		t.Fatalf("insert user access logs across months: %v", err)
	}

	var nodeCount, userCount int64
	if err := gdb.Model(&analyticsmodel.NodeAccessLog{}).Count(&nodeCount).Error; err != nil {
		t.Fatalf("count node access logs: %v", err)
	}
	if err := gdb.Model(&analyticsmodel.UserAccessLog{}).Count(&userCount).Error; err != nil {
		t.Fatalf("count user access logs: %v", err)
	}
	if nodeCount != 6 {
		t.Fatalf("node access log count = %d, want 6", nodeCount)
	}
	if userCount != 2 {
		t.Fatalf("user access log count = %d, want 2", userCount)
	}

	// MigrationRange 返回跨月范围（覆盖两表）。
	gotFrom, gotTo, err := store.MigrationRange(ctx)
	if err != nil {
		t.Fatalf("node MigrationRange: %v", err)
	}
	if !gotFrom.Equal(time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)) || !gotTo.Equal(time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("node MigrationRange = %s ~ %s, want 2026-01-15 ~ 2026-03-18", gotFrom, gotTo)
	}
	uaFrom, uaTo, err := ua.MigrationRange(ctx)
	if err != nil {
		t.Fatalf("user MigrationRange: %v", err)
	}
	if !uaFrom.Equal(time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC)) || !uaTo.Equal(time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("user MigrationRange = %s ~ %s", uaFrom, uaTo)
	}
}

// postgresNodeAccessLogsDDL 与 goose/postgres/202608080001_create_log_tables.sql 对齐。
const postgresNodeAccessLogsDDL = `
CREATE TABLE IF NOT EXISTS of_node_access_logs (
    id              BIGINT NOT NULL,
    node_id         VARCHAR(64) NOT NULL DEFAULT '',
    logged_at       TIMESTAMPTZ NOT NULL,
    remote_addr     VARCHAR(128) NOT NULL DEFAULT '',
    region          VARCHAR(128) NOT NULL DEFAULT '',
    host            VARCHAR(255) NOT NULL DEFAULT '',
    path            VARCHAR(2048) NOT NULL DEFAULT '',
    user_agent      TEXT NOT NULL DEFAULT '',
    cache_status    VARCHAR(64) NOT NULL DEFAULT '',
    status_code     INTEGER NOT NULL DEFAULT 0,
    bytes_sent      BIGINT NOT NULL DEFAULT 0,
    request_length  BIGINT NOT NULL DEFAULT 0,
    request_time_ms INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, logged_at)
) PARTITION BY RANGE (logged_at)`

// postgresUserAccessLogsDDL 与 goose/postgres/202608080001_create_log_tables.sql 对齐。
const postgresUserAccessLogsDDL = `
CREATE TABLE IF NOT EXISTS w_user_access_logs (
    id          BIGINT NOT NULL,
    user_id     BIGINT NOT NULL DEFAULT 0,
    path        VARCHAR(2048) NOT NULL DEFAULT '',
    method      VARCHAR(16) NOT NULL DEFAULT '',
    ip          VARCHAR(128) NOT NULL DEFAULT '',
    user_agent  TEXT NOT NULL DEFAULT '',
    headers     TEXT NOT NULL DEFAULT '',
    status      INTEGER NOT NULL DEFAULT 0,
    latency     BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at)`
