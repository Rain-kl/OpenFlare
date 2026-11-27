// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

// clickhouseLogStore 实现 AccessLogStore / ObservabilityStore / StatusStore。
// UserAccessLogStore 由 clickhouseUserAccessLogStore 实现（List/Count 方法名已被
// AccessLogStore 占用，Go 不允许同名不同签名方法）。
type clickhouseLogStore struct {
	// skipFreeze 为 true 时跳过迁移冻结检查（仅迁移目标 store 使用）。
	skipFreeze bool
}

func newClickHouseStore() *clickhouseLogStore { return &clickhouseLogStore{} }

// 编译期断言。
var (
	_ AccessLogStore     = (*clickhouseLogStore)(nil)
	_ ObservabilityStore = (*clickhouseLogStore)(nil)
	_ StatusStore        = (*clickhouseLogStore)(nil)
	_ UserAccessLogStore = (*clickhouseUserAccessLogStore)(nil)
)

func chConnErr() error {
	if !db.ChConnReady() {
		return errors.New("clickhouse connection is not initialized")
	}
	return nil
}

func chConn() (driver.Conn, error) {
	if err := chConnErr(); err != nil {
		return nil, err
	}
	return db.ChConn, nil
}

// ensureWritable 迁移冻结期拒绝写入。
func (s *clickhouseLogStore) ensureWritable(ctx context.Context) error {
	if !s.skipFreeze && Migrating(ctx) {
		return ErrMigrating
	}
	return nil
}

// ListForMigration 按 id 升序分页读取（迁移复制用）：直接查询 CH 原生表。
func (s *clickhouseLogStore) ListForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeAccessLog, error) {
	return chListForMigration(ctx, afterID, limit,
		analyticsmodel.NodeAccessLog{}.TableName(),
		analyticsmodel.NodeAccessLog{}.InsertColumns(),
		scanNodeAccessLogRows)
}

// MigrationRange 返回 of_node_access_logs.logged_at 的最小/最大值（空表返回零值）。
func (s *clickhouseLogStore) MigrationRange(ctx context.Context) (time.Time, time.Time, error) {
	return chMigrationRange(ctx, analyticsmodel.NodeAccessLog{}.TableName(), "logged_at")
}

// EnsurePartitions 是 CH 分支 no-op（CH 无 PG 式分区）。
func (s *clickhouseLogStore) EnsurePartitions(_ context.Context, _, _ time.Time) error {
	return nil
}

// DropEmptyPartitions 是 CH 分支 no-op（CH 分区随数据删除自动消失，无独立分区表）。
func (s *clickhouseLogStore) DropEmptyPartitions(_ context.Context, _ time.Time) error {
	return nil
}

// DropExpiredPartitions 是 CH 分支 no-op（CH 无 PG 式分区，retention 仍走 DeleteBefore）。
func (s *clickhouseLogStore) DropExpiredPartitions(_ context.Context, _ time.Time) error {
	return nil
}

// chMigrationRange 查询 CH 表时间列 MIN/MAX；空表（NULL）返回零值。
func chMigrationRange(ctx context.Context, table, column string) (time.Time, time.Time, error) {
	conn, err := chConn()
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	var minTime, maxTime *time.Time
	if err := conn.QueryRow(ctx,
		"SELECT min("+column+"), max("+column+") FROM "+table,
	).Scan(&minTime, &maxTime); err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("query migration range %s: %w", table, err)
	}
	if minTime == nil || maxTime == nil {
		return time.Time{}, time.Time{}, nil
	}
	return minTime.UTC(), maxTime.UTC(), nil
}

// ListMetricSnapshotsForMigration 按 id 升序分页读取（迁移复制用）。
func (s *clickhouseLogStore) ListMetricSnapshotsForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeMetricSnapshot, error) {
	return chListForMigration(ctx, afterID, limit,
		analyticsmodel.NodeMetricSnapshot{}.TableName(),
		analyticsmodel.NodeMetricSnapshot{}.InsertColumns(),
		scanNodeMetricSnapshotRows)
}

// ListPingRecordsForMigration 按 id 升序分页读取（迁移复制用）。
func (s *clickhouseLogStore) ListPingRecordsForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodePingRecord, error) {
	return chListForMigration(ctx, afterID, limit,
		analyticsmodel.NodePingRecord{}.TableName(),
		analyticsmodel.NodePingRecord{}.InsertColumns(),
		scanNodePingRecordRows)
}

// ListNetworkQualitySamplesForMigration 按 id 升序分页读取（迁移复制用）。
func (s *clickhouseLogStore) ListNetworkQualitySamplesForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NetworkQualitySample, error) {
	return chListForMigration(ctx, afterID, limit,
		analyticsmodel.NetworkQualitySample{}.TableName(),
		analyticsmodel.NetworkQualitySample{}.InsertColumns(),
		scanNetworkQualitySampleRows)
}

// chListForMigration 执行按 id 升序分页的 CH 原生表查询，并交给 scanner 扫描。
func chListForMigration[T any](ctx context.Context, afterID uint64, limit int, table, columns string, scanner func(driver.Rows) ([]T, error)) ([]T, error) {
	conn, err := chConn()
	if err != nil {
		return nil, err
	}
	rows, err := conn.Query(ctx, `
SELECT `+columns+`
FROM `+table+`
WHERE id > ?
ORDER BY id ASC
LIMIT ?`, afterID, limitOr(limit, migrationPageSize))
	if err != nil {
		return nil, fmt.Errorf("list %s for migration: %w", table, err)
	}
	defer func() { _ = rows.Close() }()
	return scanner(rows)
}

// ---- StatusStore ----

// ActiveDatabase 返回当前日志主库名（CH 分支固定 clickhouse）。
func (s *clickhouseLogStore) ActiveDatabase(_ context.Context) (string, error) {
	return dbNameClickHouse, nil
}

// ---- UserAccessLogStore ----

// clickhouseUserAccessLogStore 实现 UserAccessLogStore。clickhouseLogStore 已占用
// List/Count 方法名（AccessLogStore 接口），Go 不允许同名不同签名方法，故用户访问日志
// 用独立类型嵌入同一 clickhouseLogStore（与 userAccessLogGormStore 同构），复用 ensureWritable。
type clickhouseUserAccessLogStore struct {
	*clickhouseLogStore
}

func newClickHouseUserAccessLogStore() *clickhouseUserAccessLogStore {
	return &clickhouseUserAccessLogStore{clickhouseLogStore: newClickHouseStore()}
}

// ListForMigration 按 id 升序分页读取（迁移复制用）。
func (s *clickhouseUserAccessLogStore) ListForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.UserAccessLog, error) {
	return chListForMigration(ctx, afterID, limit,
		analyticsmodel.UserAccessLog{}.TableName(),
		analyticsmodel.UserAccessLog{}.InsertColumns(),
		func(rows driver.Rows) ([]analyticsmodel.UserAccessLog, error) {
			var result []analyticsmodel.UserAccessLog
			for rows.Next() {
				var item analyticsmodel.UserAccessLog
				if err := rows.Scan(
					&item.ID,
					&item.UserID,
					&item.Path,
					&item.Method,
					&item.IP,
					&item.UserAgent,
					&item.Headers,
					&item.Status,
					&item.Latency,
					&item.CreatedAt,
				); err != nil {
					return nil, fmt.Errorf("scan user access log row: %w", err)
				}
				item.CreatedAt = item.CreatedAt.UTC()
				result = append(result, item)
			}
			return result, nil
		})
}

// MigrationRange 返回 w_user_access_logs.created_at 的最小/最大值（空表返回零值）。
func (s *clickhouseUserAccessLogStore) MigrationRange(ctx context.Context) (time.Time, time.Time, error) {
	return chMigrationRange(ctx, analyticsmodel.UserAccessLog{}.TableName(), "created_at")
}
