// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package logstore 提供日志/分析存储抽象：上层只面向本包接口，
// 禁止直接 import internal/repository/analytics 或触碰 db.ChConn/db.ChDB。
package logstore

import (
	"context"
	"errors"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

// ErrMigrating 表示日志数据库正在迁移，当前禁止写入。
var ErrMigrating = errors.New("log database is migrating, writes are disabled")

// AccessLogStore 节点访问日志（of_node_access_logs）。
type AccessLogStore interface {
	// InsertBatch 为写入入口：冻结检查 + 经 hook 入队（异步），不直接落库。
	InsertBatch(ctx context.Context, records []*model.OpenFlareAccessLog) error
	// BatchInsertNodeAccessLogs 为 batchwriter flush 目标：直接批量写入当前存储。
	BatchInsertNodeAccessLogs(ctx context.Context, rows []analyticsmodel.NodeAccessLog) error

	List(ctx context.Context, query model.OpenFlareAccessLogQuery) ([]*model.OpenFlareAccessLog, error)
	Count(ctx context.Context, query model.OpenFlareAccessLogQuery) (int64, int64, int64, error)
	RegionCounts(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareAccessLogRegionCount, error)
	BucketAggregates(ctx context.Context, filter model.OpenFlareAccessLogQuery, bucketSeconds int64) ([]analyticsmodel.NodeAccessLogBucketAggregate, error)
	CountBuckets(ctx context.Context, filter model.OpenFlareAccessLogQuery, bucketSeconds int64) (int64, error)
	BucketDimensions(ctx context.Context, filter model.OpenFlareAccessLogQuery, column string, bucketSeconds int64) ([]analyticsmodel.NodeAccessLogBucketDimension, error)
	IPAggregates(ctx context.Context, filter model.OpenFlareAccessLogQuery, exactRemoteAddr bool) ([]analyticsmodel.NodeAccessLogIPAggregate, error)
	IPSummaries(ctx context.Context, filter model.OpenFlareAccessLogQuery, recentSince time.Time) ([]analyticsmodel.NodeAccessLogIPSummary, error)
	CountIPSummaries(ctx context.Context, filter model.OpenFlareAccessLogQuery) (int64, error)
	WAFIPAggregates(ctx context.Context, filter model.OpenFlareAccessLogQuery) ([]analyticsmodel.NodeAccessLogWAFIPAggregate, error)
	IPTrend(ctx context.Context, filter model.OpenFlareAccessLogQuery, bucketSeconds int64) ([]analyticsmodel.NodeAccessLogIPTrend, error)
	TrafficSummary(ctx context.Context, filter model.OpenFlareAccessLogQuery) (model.OpenFlareAccessLogTrafficSummary, error)
	ValueCounts(ctx context.Context, filter model.OpenFlareAccessLogQuery, column string, limit int) ([]model.OpenFlareAccessLogValueCount, error)
	NodeAggregates(ctx context.Context, filter model.OpenFlareAccessLogQuery) ([]model.OpenFlareAccessLogNodeAggregate, error)
	DeleteAll(ctx context.Context) (int64, error)
	DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error)
	DeleteByNodeBefore(ctx context.Context, nodeID string, before time.Time) (int64, error)
	// ListForMigration 按 id 升序分页读取（迁移复制用）。
	ListForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeAccessLog, error)
	// MigrationRange 返回源表 logged_at 的最小/最大值（空表返回零值），迁移预建分区用。
	MigrationRange(ctx context.Context) (from, to time.Time, err error)
	// EnsurePartitions 幂等预建 PG 分区（按月），覆盖 [from, to] 月份；CH/SQLite 为 no-op。
	// 目标为 PG 的迁移在复制前调用，避免历史数据写入报 "no partition of relation found"。
	EnsurePartitions(ctx context.Context, from, to time.Time) error
	// DropEmptyPartitions 幂等清理 PG 空分区表：删除 before 月份之前、且无任何数据的按月分区；
	// CH/SQLite 为 no-op（CH 分区随数据删除自动消失、SQLite 无分区）。
	DropEmptyPartitions(ctx context.Context, before time.Time) error
	// DropExpiredPartitions 直接删除完全过期的 PG 整月分区（候选为月份早于 cutoff 月的分区，
	// 删除前校验分区内无保留期内数据，避免时区偏移下误删；迁移冻结期间拒绝执行）；CH/SQLite 为 no-op。
	DropExpiredPartitions(ctx context.Context, cutoff time.Time) error
}

// ObservabilityStore 可观测 4 表（metric snapshots / edge health / frps / frpc）。
type ObservabilityStore interface {
	InsertMetricSnapshot(ctx context.Context, record *model.OpenFlareMetricSnapshot) error
	ListMetricSnapshots(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareMetricSnapshot, error)
	DeleteAllMetricSnapshots(ctx context.Context) (int64, error)
	DeleteMetricSnapshotsBefore(ctx context.Context, cutoff time.Time) (int64, error)
	BatchInsertNodeMetricSnapshots(ctx context.Context, rows []analyticsmodel.NodeMetricSnapshot) error

	// ListTrafficHourly 返回小时级流量汇总（按 node/hour 聚合，unique_visitor_count 恒 0）。
	// CH 后端读 of_access_log_hourly rollup；PG/SQLite 从 of_node_access_logs 实时聚合。
	ListTrafficHourly(ctx context.Context, nodeID string, since time.Time) ([]analyticsmodel.NodeTrafficHourly, error)
	// ListAccessLogHourly 返回按 node/hour/host 的小时级访问日志汇总。
	ListAccessLogHourly(ctx context.Context, nodeID string, since time.Time) ([]analyticsmodel.AccessLogHourly, error)
	// ListMetricHourly 返回小时级指标聚合（avg cpu/memory + 计数器增量，reported_nodes 去重节点数）。
	ListMetricHourly(ctx context.Context, nodeID string, since time.Time) ([]analyticsmodel.NodeMetricHourly, error)

	InsertEdgeHealth(ctx context.Context, record *model.OpenFlareEdgeHealth) error
	ListEdgeHealth(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareEdgeHealth, error)
	DeleteAllEdgeHealth(ctx context.Context) (int64, error)
	DeleteEdgeHealthBefore(ctx context.Context, cutoff time.Time) (int64, error)
	BatchInsertNodeEdgeHealth(ctx context.Context, rows []analyticsmodel.NodeEdgeHealth) error

	InsertNodeObservationFrps(ctx context.Context, record *model.OpenFlareNodeObservationFrps) error
	ListNodeObservationFrps(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareNodeObservationFrps, error)
	DeleteAllNodeObservationFrps(ctx context.Context) (int64, error)
	DeleteNodeObservationFrpsBefore(ctx context.Context, cutoff time.Time) (int64, error)
	BatchInsertNodeObsFrps(ctx context.Context, rows []analyticsmodel.NodeObsFrps) error

	InsertNodeObservationFrpc(ctx context.Context, record *model.OpenFlareNodeObservationFrpc) error
	ListNodeObservationFrpc(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareNodeObservationFrpc, error)
	DeleteAllNodeObservationFrpc(ctx context.Context) (int64, error)
	DeleteNodeObservationFrpcBefore(ctx context.Context, cutoff time.Time) (int64, error)
	BatchInsertNodeObsFrpc(ctx context.Context, rows []analyticsmodel.NodeObsFrpc) error

	// 迁移复制用：按 id 升序分页读取。
	ListMetricSnapshotsForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeMetricSnapshot, error)
	ListEdgeHealthForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeEdgeHealth, error)
	ListNodeObsFrpsForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeObsFrps, error)
	ListNodeObsFrpcForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeObsFrpc, error)
}

// UserAccessLogStore 用户访问日志（w_user_access_logs）。
type UserAccessLogStore interface {
	BatchInsert(ctx context.Context, logs []analyticsmodel.UserAccessLog) error
	// DeleteAll 清空全部用户访问日志（迁移「覆盖目标库已有日志」幂等前提用）。
	DeleteAll(ctx context.Context) (int64, error)
	Count(ctx context.Context, filter analyticsmodel.AccessLogFilter) (uint64, error)
	List(ctx context.Context, filter analyticsmodel.AccessLogFilter, page, pageSize int) ([]analyticsmodel.UserAccessLog, uint64, error)
	GetDailyTrend(ctx context.Context, days int) ([]analyticsmodel.DailyTrend, error)
	GetBrowserDistribution(ctx context.Context, startTime time.Time) ([]analyticsmodel.BrowserShare, error)
	GetTopActiveUsers(ctx context.Context, startTime time.Time, limit int) ([]analyticsmodel.TopUser, error)
	// ListForMigration 按 id 升序分页读取（迁移复制用）。
	ListForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.UserAccessLog, error)
	// MigrationRange 返回源表 created_at 的最小/最大值（空表返回零值），迁移预建分区用。
	MigrationRange(ctx context.Context) (from, to time.Time, err error)
}

// StatusStore 日志库状态（供管理端状态端点）。
type StatusStore interface {
	ActiveDatabase(ctx context.Context) (string, error)
	ClickHouseOperationalStats(ctx context.Context) (*analyticsmodel.ClickHouseOperationalStats, error) // 仅 CH 激活时非 nil
}

// Store 聚合当前生效日志库的全部域存储。
type Store struct {
	AccessLogs     AccessLogStore
	Observability  ObservabilityStore
	UserAccessLogs UserAccessLogStore
	Status         StatusStore
}
