// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"fmt"
	"math"
	"time"

	"Wavelet/openflare/plugins/server/kernel/model"
	analyticsmodel "Wavelet/openflare/plugins/server/kernel/model/analytics"
	analyticsrepo "Wavelet/openflare/plugins/server/kernel/repository/analytics"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// clickhouseLogStore 实现 AccessLogStore / ObservabilityStore / StatusStore，
// 逐方法委托 analyticsrepo（CH 原生 batch 写入，零性能损耗）。
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

func chConn(ctx context.Context) (driver.Conn, error) {
	return analyticsrepo.ChConn(ctx)
}

// ensureWritable 迁移冻结期拒绝写入。
func (s *clickhouseLogStore) ensureWritable(ctx context.Context) error {
	if !s.skipFreeze && Migrating(ctx) {
		return ErrMigrating
	}
	return nil
}

// ---- AccessLogStore ----

// InsertBatch 节点访问日志写入入口：冻结检查后经 hook 入队（异步），不直接落库。
func (s *clickhouseLogStore) InsertBatch(ctx context.Context, records []*model.OpenFlareAccessLog) error {
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	rows := make([]analyticsmodel.NodeAccessLog, 0, len(records))
	for _, r := range records {
		if r == nil {
			continue
		}
		rows = append(rows, toAnalyticsNodeAccessLog(r))
	}
	if h := currentAccessLogHooks().QueueNodeAccessLogs; h != nil {
		h(rows)
	}
	return nil
}

// BatchInsertNodeAccessLogs 是 batchwriter flush 目标：CH 原生批量写入。
func (s *clickhouseLogStore) BatchInsertNodeAccessLogs(ctx context.Context, rows []analyticsmodel.NodeAccessLog) error {
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	return analyticsrepo.BatchInsertNodeAccessLogs(ctx, rows)
}

func (s *clickhouseLogStore) List(ctx context.Context, query model.OpenFlareAccessLogQuery) ([]*model.OpenFlareAccessLog, error) {
	rows, err := analyticsrepo.ListNodeAccessLogs(ctx, toNodeAccessLogFilter(query))
	if err != nil {
		return nil, err
	}
	return fromAnalyticsNodeAccessLogs(rows), nil
}

func (s *clickhouseLogStore) Count(ctx context.Context, query model.OpenFlareAccessLogQuery) (int64, int64, int64, error) {
	return analyticsrepo.CountNodeAccessLogs(ctx, toNodeAccessLogFilter(query))
}

func (s *clickhouseLogStore) RegionCounts(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareAccessLogRegionCount, error) {
	rows, err := analyticsrepo.RegionCountsNodeAccessLogs(ctx, nodeID, since, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*model.OpenFlareAccessLogRegionCount, len(rows))
	for i, r := range rows {
		out[i] = &model.OpenFlareAccessLogRegionCount{Region: r.Region, Count: r.Count}
	}
	return out, nil
}

func (s *clickhouseLogStore) BucketAggregates(ctx context.Context, query model.OpenFlareAccessLogQuery, bucketSeconds int64) ([]analyticsmodel.NodeAccessLogBucketAggregate, error) {
	return analyticsrepo.BucketAggregatesNodeAccessLogs(ctx, toNodeAccessLogFilter(query), bucketSeconds)
}

func (s *clickhouseLogStore) CountBuckets(ctx context.Context, query model.OpenFlareAccessLogQuery, bucketSeconds int64) (int64, error) {
	return analyticsrepo.CountBucketAggregatesNodeAccessLogs(ctx, toNodeAccessLogFilter(query), bucketSeconds)
}

func (s *clickhouseLogStore) BucketDimensions(ctx context.Context, query model.OpenFlareAccessLogQuery, column string, bucketSeconds int64) ([]analyticsmodel.NodeAccessLogBucketDimension, error) {
	return analyticsrepo.BucketDimensionsNodeAccessLogs(ctx, toNodeAccessLogFilter(query), column, bucketSeconds)
}

func (s *clickhouseLogStore) IPAggregates(ctx context.Context, query model.OpenFlareAccessLogQuery, exactRemoteAddr bool) ([]analyticsmodel.NodeAccessLogIPAggregate, error) {
	return analyticsrepo.IPAggregatesNodeAccessLogs(ctx, toNodeAccessLogFilter(query), exactRemoteAddr)
}

func (s *clickhouseLogStore) IPSummaries(ctx context.Context, query model.OpenFlareAccessLogQuery, recentSince time.Time) ([]analyticsmodel.NodeAccessLogIPSummary, error) {
	return analyticsrepo.IPSummariesNodeAccessLogs(ctx, toNodeAccessLogFilter(query), recentSince)
}

func (s *clickhouseLogStore) CountIPSummaries(ctx context.Context, query model.OpenFlareAccessLogQuery) (int64, error) {
	return analyticsrepo.CountIPSummaryNodeAccessLogs(ctx, toNodeAccessLogFilter(query))
}

func (s *clickhouseLogStore) WAFIPAggregates(ctx context.Context, query model.OpenFlareAccessLogQuery) ([]analyticsmodel.NodeAccessLogWAFIPAggregate, error) {
	return analyticsrepo.IPAggregatesForWAFNodeAccessLogs(ctx, toNodeAccessLogFilter(query))
}

func (s *clickhouseLogStore) IPTrend(ctx context.Context, query model.OpenFlareAccessLogQuery, bucketSeconds int64) ([]analyticsmodel.NodeAccessLogIPTrend, error) {
	return analyticsrepo.IPTrendNodeAccessLogs(ctx, toNodeAccessLogFilter(query), bucketSeconds)
}

func (s *clickhouseLogStore) TrafficSummary(ctx context.Context, query model.OpenFlareAccessLogQuery) (model.OpenFlareAccessLogTrafficSummary, error) {
	row, err := analyticsrepo.TrafficSummaryNodeAccessLogs(ctx, toNodeAccessLogFilter(query))
	if err != nil {
		return model.OpenFlareAccessLogTrafficSummary{}, err
	}
	return model.OpenFlareAccessLogTrafficSummary{
		RequestCount:  row.RequestCount,
		ErrorCount:    row.ErrorCount,
		UniqueIPCount: row.UniqueIPCount,
		BytesSent:     row.BytesSent,
		RequestLength: row.RequestLength,
		NodeCount:     row.NodeCount,
	}, nil
}

func (s *clickhouseLogStore) ValueCounts(ctx context.Context, query model.OpenFlareAccessLogQuery, column string, limit int) ([]model.OpenFlareAccessLogValueCount, error) {
	rows, err := analyticsrepo.ValueCountsNodeAccessLogs(ctx, toNodeAccessLogFilter(query), column, limit)
	if err != nil {
		return nil, err
	}
	out := make([]model.OpenFlareAccessLogValueCount, len(rows))
	for i, r := range rows {
		out[i] = model.OpenFlareAccessLogValueCount{Value: r.Value, Count: r.Count}
	}
	return out, nil
}

func (s *clickhouseLogStore) NodeAggregates(ctx context.Context, query model.OpenFlareAccessLogQuery) ([]model.OpenFlareAccessLogNodeAggregate, error) {
	rows, err := analyticsrepo.NodeAggregatesNodeAccessLogs(ctx, toNodeAccessLogFilter(query))
	if err != nil {
		return nil, err
	}
	out := make([]model.OpenFlareAccessLogNodeAggregate, len(rows))
	for i, r := range rows {
		out[i] = model.OpenFlareAccessLogNodeAggregate{NodeID: r.NodeID, RequestCount: r.RequestCount, ErrorCount: r.ErrorCount, UniqueIPCount: r.UniqueIPCount}
	}
	return out, nil
}

func (s *clickhouseLogStore) DeleteAll(ctx context.Context) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	return analyticsrepo.DeleteAllNodeAccessLogs(ctx)
}

func (s *clickhouseLogStore) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	return analyticsrepo.DeleteNodeAccessLogsBefore(ctx, cutoff)
}

func (s *clickhouseLogStore) DeleteByNodeBefore(ctx context.Context, nodeID string, before time.Time) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	return analyticsrepo.DeleteNodeAccessLogsByNodeBefore(ctx, nodeID, before)
}

// ListForMigration 按 id 升序分页读取（迁移复制用）：直接查询 CH 原生表。
func (s *clickhouseLogStore) ListForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeAccessLog, error) {
	conn, err := chConn(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := conn.Query(ctx, `
SELECT `+analyticsmodel.NodeAccessLog{}.InsertColumns()+`
FROM `+analyticsmodel.NodeAccessLog{}.TableName()+`
WHERE id > ?
ORDER BY id ASC
LIMIT ?`, afterID, limitOr(limit, migrationPageSize))
	if err != nil {
		return nil, fmt.Errorf("list node access logs for migration: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var result []analyticsmodel.NodeAccessLog
	for rows.Next() {
		var item analyticsmodel.NodeAccessLog
		if err := rows.Scan(
			&item.ID,
			&item.NodeID,
			&item.LoggedAt,
			&item.RemoteAddr,
			&item.Region,
			&item.Host,
			&item.Path,
			&item.UserAgent,
			&item.CacheStatus,
			&item.StatusCode,
			&item.BytesSent,
			&item.RequestLength,
			&item.RequestTimeMs,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan node access log row: %w", err)
		}
		item.LoggedAt = item.LoggedAt.UTC()
		item.CreatedAt = item.CreatedAt.UTC()
		result = append(result, item)
	}
	return result, nil
}

// ---- ObservabilityStore ----

// InsertMetricSnapshot 写入入口：冻结检查 + 经 hook 入队（异步），不直接落库。
func (s *clickhouseLogStore) InsertMetricSnapshot(ctx context.Context, record *model.OpenFlareMetricSnapshot) error {
	if record == nil {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	if h := currentObservabilityHooks().QueueMetricSnapshot; h != nil {
		h(toAnalyticsNodeMetricSnapshot(record))
	}
	return nil
}

func (s *clickhouseLogStore) ListMetricSnapshots(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareMetricSnapshot, error) {
	rows, err := analyticsrepo.ListNodeMetricSnapshots(ctx, toNodeObservabilityFilter(nodeID, since, limit))
	if err != nil {
		return nil, err
	}
	return fromAnalyticsNodeMetricSnapshots(rows), nil
}

func (s *clickhouseLogStore) DeleteAllMetricSnapshots(ctx context.Context) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	return analyticsrepo.DeleteAllNodeMetricSnapshots(ctx)
}

// ListTrafficHourly 委托 analyticsrepo 读 of_access_log_hourly rollup（M5 口径，UV 恒 0）。
func (s *clickhouseLogStore) ListTrafficHourly(ctx context.Context, nodeID string, since time.Time) ([]analyticsmodel.NodeTrafficHourly, error) {
	return analyticsrepo.ListNodeTrafficHourly(ctx, toNodeObservabilitySince(nodeID, since))
}

// ListAccessLogHourly 委托 analyticsrepo 读 of_access_log_hourly rollup。
func (s *clickhouseLogStore) ListAccessLogHourly(ctx context.Context, nodeID string, since time.Time) ([]analyticsmodel.AccessLogHourly, error) {
	return analyticsrepo.ListAccessLogHourly(ctx, toNodeObservabilitySince(nodeID, since))
}

// ListMetricHourly 委托 analyticsrepo ListNodeMetricHourly：rollup 覆盖窗口时读
// of_node_metric_capacity_hourly，否则按 mergeNodeMetricHourlyPreferRollup 合并 raw 兜底。
func (s *clickhouseLogStore) ListMetricHourly(ctx context.Context, nodeID string, since time.Time) ([]analyticsmodel.NodeMetricHourly, error) {
	return analyticsrepo.ListNodeMetricHourly(ctx, toNodeObservabilitySince(nodeID, since))
}

func (s *clickhouseLogStore) DeleteMetricSnapshotsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	return analyticsrepo.DeleteNodeMetricSnapshotsBefore(ctx, cutoff)
}

// BatchInsertNodeMetricSnapshots 是 batchwriter flush 目标：CH 原生批量写入。
func (s *clickhouseLogStore) BatchInsertNodeMetricSnapshots(ctx context.Context, rows []analyticsmodel.NodeMetricSnapshot) error {
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	return analyticsrepo.BatchInsertNodeMetricSnapshots(ctx, rows)
}

// InsertEdgeHealth 写入入口：冻结检查 + 经 hook 入队（异步），不直接落库。
func (s *clickhouseLogStore) InsertEdgeHealth(ctx context.Context, record *model.OpenFlareEdgeHealth) error {
	if record == nil {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	if h := currentObservabilityHooks().QueueEdgeHealth; h != nil {
		h(toAnalyticsNodeEdgeHealth(record))
	}
	return nil
}

func (s *clickhouseLogStore) ListEdgeHealth(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareEdgeHealth, error) {
	rows, err := analyticsrepo.ListNodeEdgeHealth(ctx, toNodeObservabilityFilter(nodeID, since, limit))
	if err != nil {
		return nil, err
	}
	return fromAnalyticsNodeEdgeHealths(rows), nil
}

func (s *clickhouseLogStore) DeleteAllEdgeHealth(ctx context.Context) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	return analyticsrepo.DeleteAllNodeEdgeHealth(ctx)
}

func (s *clickhouseLogStore) DeleteEdgeHealthBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	return analyticsrepo.DeleteNodeEdgeHealthBefore(ctx, cutoff)
}

// BatchInsertNodeEdgeHealth 是 batchwriter flush 目标：CH 原生批量写入。
func (s *clickhouseLogStore) BatchInsertNodeEdgeHealth(ctx context.Context, rows []analyticsmodel.NodeEdgeHealth) error {
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	return analyticsrepo.BatchInsertNodeEdgeHealth(ctx, rows)
}

// InsertNodeObservationFrps 写入入口：冻结检查 + 经 hook 入队（异步），不直接落库。
func (s *clickhouseLogStore) InsertNodeObservationFrps(ctx context.Context, record *model.OpenFlareNodeObservationFrps) error {
	if record == nil {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	if h := currentObservabilityHooks().QueueNodeObsFrps; h != nil {
		h(toAnalyticsNodeObsFrps(record))
	}
	return nil
}

func (s *clickhouseLogStore) ListNodeObservationFrps(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareNodeObservationFrps, error) {
	rows, err := analyticsrepo.ListNodeObsFrps(ctx, toNodeObservabilityFilter(nodeID, since, limit))
	if err != nil {
		return nil, err
	}
	return fromAnalyticsNodeObsFrps(rows), nil
}

func (s *clickhouseLogStore) DeleteAllNodeObservationFrps(ctx context.Context) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	return analyticsrepo.DeleteAllNodeObsFrps(ctx)
}

func (s *clickhouseLogStore) DeleteNodeObservationFrpsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	return analyticsrepo.DeleteNodeObsFrpsBefore(ctx, cutoff)
}

// BatchInsertNodeObsFrps 是 batchwriter flush 目标：CH 原生批量写入。
func (s *clickhouseLogStore) BatchInsertNodeObsFrps(ctx context.Context, rows []analyticsmodel.NodeObsFrps) error {
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	return analyticsrepo.BatchInsertNodeObsFrps(ctx, rows)
}

// InsertNodeObservationFrpc 写入入口：冻结检查 + 经 hook 入队（异步），不直接落库。
func (s *clickhouseLogStore) InsertNodeObservationFrpc(ctx context.Context, record *model.OpenFlareNodeObservationFrpc) error {
	if record == nil {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	if h := currentObservabilityHooks().QueueNodeObsFrpc; h != nil {
		h(toAnalyticsNodeObsFrpc(record))
	}
	return nil
}

func (s *clickhouseLogStore) ListNodeObservationFrpc(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareNodeObservationFrpc, error) {
	rows, err := analyticsrepo.ListNodeObsFrpc(ctx, toNodeObservabilityFilter(nodeID, since, limit))
	if err != nil {
		return nil, err
	}
	return fromAnalyticsNodeObsFrpc(rows), nil
}

func (s *clickhouseLogStore) DeleteAllNodeObservationFrpc(ctx context.Context) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	return analyticsrepo.DeleteAllNodeObsFrpc(ctx)
}

func (s *clickhouseLogStore) DeleteNodeObservationFrpcBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	return analyticsrepo.DeleteNodeObsFrpcBefore(ctx, cutoff)
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
	conn, err := chConn(ctx)
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

// BatchInsertNodeObsFrpc 是 batchwriter flush 目标：CH 原生批量写入。
func (s *clickhouseLogStore) BatchInsertNodeObsFrpc(ctx context.Context, rows []analyticsmodel.NodeObsFrpc) error {
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	return analyticsrepo.BatchInsertNodeObsFrpc(ctx, rows)
}

// ListMetricSnapshotsForMigration 按 id 升序分页读取（迁移复制用）。
func (s *clickhouseLogStore) ListMetricSnapshotsForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeMetricSnapshot, error) {
	return chListForMigration(ctx, afterID, limit,
		analyticsmodel.NodeMetricSnapshot{}.TableName(),
		analyticsmodel.NodeMetricSnapshot{}.InsertColumns(),
		func(rows driver.Rows) ([]analyticsmodel.NodeMetricSnapshot, error) {
			var result []analyticsmodel.NodeMetricSnapshot
			for rows.Next() {
				var item analyticsmodel.NodeMetricSnapshot
				if err := rows.Scan(
					&item.ID,
					&item.NodeID,
					&item.CapturedAt,
					&item.CPUUsagePercent,
					&item.MemoryUsedBytes,
					&item.MemoryTotalBytes,
					&item.StorageUsedBytes,
					&item.StorageTotalBytes,
					&item.DiskReadBytes,
					&item.DiskWriteBytes,
					&item.NetworkRxBytes,
					&item.NetworkTxBytes,
					&item.CreatedAt,
				); err != nil {
					return nil, fmt.Errorf("scan node metric snapshot row: %w", err)
				}
				item.CapturedAt = item.CapturedAt.UTC()
				item.CreatedAt = item.CreatedAt.UTC()
				result = append(result, item)
			}
			return result, nil
		})
}

// chObsRow 迁移读取共用的双字段观测行（字符串状态 + 数值计数）：
// edge_health（status/connections）与 obs_frpc（tunnel_status/connected_relays_count）同形状。
type chObsRow struct {
	ID         uint64
	NodeID     string
	CapturedAt time.Time
	Status     string
	Count      int64
	CreatedAt  time.Time
}

// countToInt32 将观测计数转为 int32（防御溢出；观测计数远小于 int32 上限）。
func countToInt32(v int64) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}

// scanChObsRow 扫描 chObsRow（含 UTC 归一化）。
func scanChObsRow(rows driver.Rows) ([]chObsRow, error) {
	var result []chObsRow
	for rows.Next() {
		var item chObsRow
		if err := rows.Scan(
			&item.ID,
			&item.NodeID,
			&item.CapturedAt,
			&item.Status,
			&item.Count,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan observation row: %w", err)
		}
		item.CapturedAt = item.CapturedAt.UTC()
		item.CreatedAt = item.CreatedAt.UTC()
		result = append(result, item)
	}
	return result, nil
}

// ListEdgeHealthForMigration 按 id 升序分页读取（迁移复制用）。
func (s *clickhouseLogStore) ListEdgeHealthForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeEdgeHealth, error) {
	rows, err := chListForMigration(ctx, afterID, limit,
		analyticsmodel.NodeEdgeHealth{}.TableName(),
		analyticsmodel.NodeEdgeHealth{}.InsertColumns(),
		scanChObsRow)
	if err != nil {
		return nil, err
	}
	out := make([]analyticsmodel.NodeEdgeHealth, len(rows))
	for i, r := range rows {
		out[i] = analyticsmodel.NodeEdgeHealth{ID: r.ID, NodeID: r.NodeID, CapturedAt: r.CapturedAt, Status: r.Status, Connections: r.Count, CreatedAt: r.CreatedAt}
	}
	return out, nil
}

// ListNodeObsFrpsForMigration 按 id 升序分页读取（迁移复制用）。
func (s *clickhouseLogStore) ListNodeObsFrpsForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeObsFrps, error) {
	return chListForMigration(ctx, afterID, limit,
		analyticsmodel.NodeObsFrps{}.TableName(),
		analyticsmodel.NodeObsFrps{}.InsertColumns(),
		func(rows driver.Rows) ([]analyticsmodel.NodeObsFrps, error) {
			var result []analyticsmodel.NodeObsFrps
			for rows.Next() {
				var item analyticsmodel.NodeObsFrps
				if err := rows.Scan(
					&item.ID,
					&item.NodeID,
					&item.CapturedAt,
					&item.FrpsConnections,
					&item.FrpsProxyCount,
					&item.FrpsClientCount,
					&item.FrpsProxies,
					&item.CreatedAt,
				); err != nil {
					return nil, fmt.Errorf("scan node frps observation row: %w", err)
				}
				item.CapturedAt = item.CapturedAt.UTC()
				item.CreatedAt = item.CreatedAt.UTC()
				result = append(result, item)
			}
			return result, nil
		})
}

// ListNodeObsFrpcForMigration 按 id 升序分页读取（迁移复制用）。
func (s *clickhouseLogStore) ListNodeObsFrpcForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeObsFrpc, error) {
	rows, err := chListForMigration(ctx, afterID, limit,
		analyticsmodel.NodeObsFrpc{}.TableName(),
		analyticsmodel.NodeObsFrpc{}.InsertColumns(),
		scanChObsRow)
	if err != nil {
		return nil, err
	}
	out := make([]analyticsmodel.NodeObsFrpc, len(rows))
	for i, r := range rows {
		out[i] = analyticsmodel.NodeObsFrpc{ID: r.ID, NodeID: r.NodeID, CapturedAt: r.CapturedAt, TunnelStatus: r.Status, ConnectedRelaysCount: countToInt32(r.Count), CreatedAt: r.CreatedAt}
	}
	return out, nil
}

// chListForMigration 执行按 id 升序分页的 CH 原生表查询，并交给 scanner 扫描。
func chListForMigration[T any](ctx context.Context, afterID uint64, limit int, table, columns string, scanner func(driver.Rows) ([]T, error)) ([]T, error) {
	conn, err := chConn(ctx)
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

// ClickHouseOperationalStats 委托 analyticsrepo 汇总 CH 运行状态。
func (s *clickhouseLogStore) ClickHouseOperationalStats(ctx context.Context) (*analyticsmodel.ClickHouseOperationalStats, error) {
	return analyticsrepo.GetClickHouseOperationalStats(ctx)
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

// BatchInsert 是 batchwriter flush 目标：CH 原生批量写入；冻结期拒绝写入，空批次直接返回。
func (s *clickhouseUserAccessLogStore) BatchInsert(ctx context.Context, logs []analyticsmodel.UserAccessLog) error {
	if len(logs) == 0 {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	return analyticsrepo.BatchInsert(ctx, logs)
}

// DeleteAll 清空全部用户访问日志（TRUNCATE 语义，迁移「覆盖目标库已有日志」幂等前提用）。
func (s *clickhouseUserAccessLogStore) DeleteAll(ctx context.Context) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	return analyticsrepo.DeleteAllUserAccessLogs(ctx)
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

func (s *clickhouseUserAccessLogStore) Count(ctx context.Context, filter analyticsmodel.AccessLogFilter) (uint64, error) {
	return analyticsrepo.CountAccessLogs(ctx, filter)
}

func (s *clickhouseUserAccessLogStore) List(ctx context.Context, filter analyticsmodel.AccessLogFilter, page, pageSize int) ([]analyticsmodel.UserAccessLog, uint64, error) {
	return analyticsrepo.ListAccessLogs(ctx, filter, page, pageSize)
}

func (s *clickhouseUserAccessLogStore) GetDailyTrend(ctx context.Context, days int) ([]analyticsmodel.DailyTrend, error) {
	return analyticsrepo.GetDailyTrend(ctx, days)
}

func (s *clickhouseUserAccessLogStore) GetBrowserDistribution(ctx context.Context, startTime time.Time) ([]analyticsmodel.BrowserShare, error) {
	return analyticsrepo.GetBrowserDistribution(ctx, startTime)
}

func (s *clickhouseUserAccessLogStore) GetTopActiveUsers(ctx context.Context, startTime time.Time, limit int) ([]analyticsmodel.TopUser, error) {
	return analyticsrepo.GetTopActiveUsers(ctx, startTime, limit)
}

// toNodeObservabilityFilter 构造 CH 可观测查询过滤器（limit<=0 表示不限制）。
func toNodeObservabilityFilter(nodeID string, since time.Time, limit int) analyticsmodel.NodeObservabilityFilter {
	return analyticsmodel.NodeObservabilityFilter{
		NodeID: nodeID,
		Since:  since,
		Limit:  limit,
	}
}

// toNodeObservabilitySince 构造不带 limit 的可观测查询过滤器
// （小时级聚合读无需分页，避免传无意义的 0）。
func toNodeObservabilitySince(nodeID string, since time.Time) analyticsmodel.NodeObservabilityFilter {
	return analyticsmodel.NodeObservabilityFilter{NodeID: nodeID, Since: since}
}
