// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

const chNodeMetricSnapshotSelectColumns = "id, node_id, captured_at, cpu, gpu, ram, ram_total, swap, swap_total, load, load5, load15, temp, disk, disk_total, disk_read_bytes, disk_write_bytes, net_in, net_out, net_total_up, net_total_down, process, connections, connections_udp, uptime, created_at"

func listCHNodeObservability[A any, M any](
	ctx context.Context,
	tableName, columns, opName string,
	filter analyticsmodel.NodeObservabilityFilter,
	extraSuffix string,
	scan func(driver.Rows) ([]A, error),
	convert func([]A) []*M,
) ([]*M, error) {
	conn, err := chConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildCHNodeObservabilityFilterClause(filter, "captured_at")
	sql := fmt.Sprintf(`
SELECT %s
FROM %s
WHERE %s
ORDER BY %s%s`, columns, tableName, clause, chNodeObservabilityCapturedAtOrderClause(), extraSuffix)
	if filter.Limit > 0 {
		sql += clickHouseLimitClause
		args = append(args, filter.Limit)
	}
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", opName, err)
	}
	defer func() { _ = rows.Close() }()
	scanned, err := scan(rows)
	if err != nil {
		return nil, err
	}
	return convert(scanned), nil
}

func (s *clickhouseLogStore) ListMetricSnapshots(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareMetricSnapshot, error) {
	return listCHNodeObservability(
		ctx,
		chNodeMetricSnapshotTableName(),
		chNodeMetricSnapshotSelectColumns,
		"list node metric snapshots",
		analyticsmodel.NodeObservabilityFilter{NodeID: nodeID, Since: since, Limit: limit},
		"",
		scanNodeMetricSnapshotRows,
		fromAnalyticsNodeMetricSnapshots,
	)
}

// ListLatestMetricSnapshots uses ClickHouse LIMIT 1 BY node_id so dashboard health does not depend on a global raw LIMIT.
func (s *clickhouseLogStore) ListLatestMetricSnapshots(ctx context.Context, nodeID string, since time.Time) ([]*model.OpenFlareMetricSnapshot, error) {
	return listCHLatestMetricSnapshots(ctx, analyticsmodel.NodeObservabilityFilter{NodeID: nodeID, Since: since})
}

func (s *clickhouseLogStore) ListLatestMetricSnapshotsByNodeIDs(ctx context.Context, nodeIDs []string, since time.Time) ([]*model.OpenFlareMetricSnapshot, error) {
	if len(nodeIDs) == 0 {
		return nil, nil
	}
	return listCHLatestMetricSnapshots(ctx, analyticsmodel.NodeObservabilityFilter{NodeIDs: nodeIDs, Since: since})
}

func listCHLatestMetricSnapshots(ctx context.Context, filter analyticsmodel.NodeObservabilityFilter) ([]*model.OpenFlareMetricSnapshot, error) {
	return listCHNodeObservability(
		ctx,
		chNodeMetricSnapshotTableName(),
		chNodeMetricSnapshotSelectColumns,
		"list latest node metric snapshots",
		filter,
		clickHouseLimit1ByNodeIDClause,
		scanNodeMetricSnapshotRows,
		fromAnalyticsNodeMetricSnapshots,
	)
}

func (s *clickhouseLogStore) ListEdgeHealth(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareEdgeHealth, error) {
	conn, err := chConn()
	if err != nil {
		return nil, err
	}
	filter := analyticsmodel.NodeObservabilityFilter{NodeID: nodeID, Since: since, Limit: limit}
	clause, args := buildCHNodeObservabilityFilterClause(filter, "captured_at")
	sql := fmt.Sprintf(`
SELECT id, node_id, captured_at, status, connections, created_at
FROM %s
WHERE %s
ORDER BY %s`, chNodeEdgeHealthTableName(), clause, chNodeObservabilityCapturedAtOrderClause())
	if filter.Limit > 0 {
		sql += clickHouseLimitClause
		args = append(args, filter.Limit)
	}
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list node edge health: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var result []analyticsmodel.NodeEdgeHealth
	for rows.Next() {
		var item analyticsmodel.NodeEdgeHealth
		if err := rows.Scan(
			&item.ID,
			&item.NodeID,
			&item.CapturedAt,
			&item.Status,
			&item.Connections,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan node edge health row: %w", err)
		}
		item.CapturedAt = item.CapturedAt.UTC()
		item.CreatedAt = item.CreatedAt.UTC()
		result = append(result, item)
	}
	return fromAnalyticsNodeEdgeHealths(result), nil
}

//nolint:dupl // scan shapes differ by model fields; shared helper would obscure CH column mapping
func scanNodeMetricSnapshotRows(rows driver.Rows) ([]analyticsmodel.NodeMetricSnapshot, error) {
	var result []analyticsmodel.NodeMetricSnapshot
	for rows.Next() {
		var item analyticsmodel.NodeMetricSnapshot
		if err := rows.Scan(
			&item.ID,
			&item.NodeID,
			&item.CapturedAt,
			&item.CPUUsagePercent,
			&item.GPUUsagePercent,
			&item.MemoryUsedBytes,
			&item.MemoryTotalBytes,
			&item.SwapUsedBytes,
			&item.SwapTotalBytes,
			&item.Load1,
			&item.Load5,
			&item.Load15,
			&item.Temp,
			&item.StorageUsedBytes,
			&item.StorageTotalBytes,
			&item.DiskReadBytes,
			&item.DiskWriteBytes,
			&item.NetIn,
			&item.NetOut,
			&item.NetworkTxBytes,
			&item.NetworkRxBytes,
			&item.ProcessCount,
			&item.TCPConnections,
			&item.UDPConnections,
			&item.UptimeSeconds,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan node metric snapshot row: %w", err)
		}
		item.CapturedAt = item.CapturedAt.UTC()
		item.CreatedAt = item.CreatedAt.UTC()
		result = append(result, item)
	}
	return result, nil
}

func (s *clickhouseLogStore) ListPingRecords(ctx context.Context, nodeID string, taskID uint, since time.Time, limit int) ([]*model.OpenFlarePingRecord, error) {
	clause, args := buildCHNodeObservabilityFilterClause(analyticsmodel.NodeObservabilityFilter{NodeID: nodeID, Since: since}, "recorded_at")
	if taskID > 0 {
		clause += " AND task_id = ?"
		args = append(args, uint64(taskID))
	}
	suffix := ""
	if limit > 0 {
		suffix = clickHouseLimitClause
		args = append(args, limit)
	}
	return listCHPingRecords(ctx, clause, suffix, args)
}

func (s *clickhouseLogStore) ListRecentPingRecordsByTask(ctx context.Context, tasks map[uint]analyticsmodel.PingTaskWindow, limit int) ([]*model.OpenFlarePingRecord, error) {
	clause, args := pingTaskNodesFilter(tasks)
	if clause == "" || limit <= 0 {
		return nil, nil
	}
	return listCHPingRecords(ctx, clause, " LIMIT ? BY node_id, task_id", append(args, limit))
}

func listCHPingRecords(ctx context.Context, clause, suffix string, args []any) ([]*model.OpenFlarePingRecord, error) {
	conn, err := chConn()
	if err != nil {
		return nil, err
	}
	sql := fmt.Sprintf(`
SELECT id, node_id, task_id, recorded_at, value, created_at
FROM %s
WHERE %s
ORDER BY recorded_at DESC, id DESC%s`, chNodePingRecordTableName(), clause, suffix)
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list node ping records: %w", err)
	}
	defer func() { _ = rows.Close() }()
	scanned, err := scanNodePingRecordRows(rows)
	if err != nil {
		return nil, err
	}
	return fromAnalyticsNodePingRecords(scanned), nil
}

func scanNodePingRecordRows(rows driver.Rows) ([]analyticsmodel.NodePingRecord, error) {
	var result []analyticsmodel.NodePingRecord
	for rows.Next() {
		var item analyticsmodel.NodePingRecord
		if err := rows.Scan(
			&item.ID,
			&item.NodeID,
			&item.TaskID,
			&item.RecordedAt,
			&item.Value,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan node ping record row: %w", err)
		}
		item.RecordedAt = item.RecordedAt.UTC()
		item.CreatedAt = item.CreatedAt.UTC()
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *clickhouseLogStore) ListNetworkQualitySamplesSince(ctx context.Context, since time.Time, limit int) ([]analyticsmodel.NetworkQualitySample, error) {
	conn, err := chConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildCHNodeObservabilityFilterClause(analyticsmodel.NodeObservabilityFilter{Since: since}, "captured_at")
	sql := fmt.Sprintf(`
SELECT %s
FROM %s
WHERE %s
ORDER BY captured_at DESC, id DESC`, analyticsmodel.NetworkQualitySample{}.InsertColumns(), chNetworkQualitySampleTableName(), clause)
	if limit > 0 {
		sql += clickHouseLimitClause
		args = append(args, limit)
	}
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list network quality samples: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanNetworkQualitySampleRows(rows)
}

func scanNetworkQualitySampleRows(rows driver.Rows) ([]analyticsmodel.NetworkQualitySample, error) {
	var result []analyticsmodel.NetworkQualitySample
	for rows.Next() {
		var item analyticsmodel.NetworkQualitySample
		if err := rows.Scan(
			&item.ID,
			&item.NodeID,
			&item.NodeASN,
			&item.ClientASN,
			&item.ClientCountry,
			&item.ClientRegion,
			&item.IPFamily,
			&item.StatsVersion,
			&item.Source,
			&item.SampleCount,
			&item.RTTMinMs,
			&item.RTTP50Ms,
			&item.RTTP95Ms,
			&item.RTTHist,
			&item.PrefixSketch,
			&item.DistinctPrefixCount,
			&item.ClientLat,
			&item.ClientLon,
			&item.GeoAccuracyKm,
			&item.GeoSource,
			&item.RetransRate,
			&item.LossRate,
			&item.HopCount,
			&item.CapturedAt,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan network quality sample row: %w", err)
		}
		item.CapturedAt = item.CapturedAt.UTC()
		item.CreatedAt = item.CreatedAt.UTC()
		result = append(result, item)
	}
	return result, rows.Err()
}

// ListTrafficHourly 读 of_access_log_hourly rollup（M5 口径，UV 恒 0）。
func (s *clickhouseLogStore) ListTrafficHourly(ctx context.Context, nodeID string, since time.Time) ([]analyticsmodel.NodeTrafficHourly, error) {
	rows, err := s.ListAccessLogHourly(ctx, nodeID, since)
	if err != nil {
		return nil, err
	}
	type key struct {
		node string
		hour int64
	}
	merged := make(map[key]*analyticsmodel.NodeTrafficHourly)
	order := make([]key, 0)
	for _, row := range rows {
		k := key{node: row.NodeID, hour: row.Hour.UTC().Unix()}
		item := merged[k]
		if item == nil {
			item = &analyticsmodel.NodeTrafficHourly{NodeID: row.NodeID, Hour: row.Hour.UTC()}
			merged[k] = item
			order = append(order, k)
		}
		item.RequestCount += row.RequestCount
		item.ErrorCount += row.ErrorCount
	}
	result := make([]analyticsmodel.NodeTrafficHourly, 0, len(order))
	for _, k := range order {
		result = append(result, *merged[k])
	}
	return result, nil
}

// ListAccessLogHourly 读 of_access_log_hourly rollup。
func (s *clickhouseLogStore) ListAccessLogHourly(ctx context.Context, nodeID string, since time.Time) ([]analyticsmodel.AccessLogHourly, error) {
	conn, err := chConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildCHNodeObservabilityFilterClause(analyticsmodel.NodeObservabilityFilter{NodeID: nodeID, Since: since}, "hour")
	sql := fmt.Sprintf(`
SELECT
	node_id,
	hour,
	host,
	sum(request_count) AS request_count,
	sum(error_count) AS error_count,
	sum(bytes_sent) AS bytes_sent,
	sum(request_length) AS request_length
FROM %s
WHERE %s
GROUP BY node_id, hour, host
ORDER BY hour ASC, node_id ASC, host ASC`, chAccessLogHourlyTableName(), clause)
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list access log hourly: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var result []analyticsmodel.AccessLogHourly
	for rows.Next() {
		var (
			item                                               analyticsmodel.AccessLogHourly
			requestCount, errorCount, bytesSent, requestLength uint64
		)
		if err := rows.Scan(
			&item.NodeID,
			&item.Hour,
			&item.Host,
			&requestCount,
			&errorCount,
			&bytesSent,
			&requestLength,
		); err != nil {
			return nil, fmt.Errorf("scan access log hourly row: %w", err)
		}
		item.Hour = item.Hour.UTC()
		item.RequestCount = safeInt64Count(requestCount)
		item.ErrorCount = safeInt64Count(errorCount)
		item.BytesSent = safeInt64Count(bytesSent)
		item.RequestLength = safeInt64Count(requestLength)
		result = append(result, item)
	}
	return result, nil
}

// hourlyRollupMaxLead is how far after filter.Since the earliest rollup bucket may start
// while still treating pre-aggregated tables as a complete window (skip raw query).
const hourlyRollupMaxLead = 2 * time.Hour

func hourlyRollupCoversWindow(earliestHour, since time.Time) bool {
	if since.IsZero() {
		return true
	}
	sinceHour := since.UTC().Truncate(time.Hour)
	earliest := earliestHour.UTC().Truncate(time.Hour)
	return !earliest.After(sinceHour.Add(hourlyRollupMaxLead))
}

// ListMetricHourly rollup 覆盖窗口时读 of_node_metric_capacity_hourly，否则合并 raw 兜底。
func (s *clickhouseLogStore) ListMetricHourly(ctx context.Context, nodeID string, since time.Time) ([]analyticsmodel.NodeMetricHourly, error) {
	filter := analyticsmodel.NodeObservabilityFilter{NodeID: nodeID, Since: since}
	rollup, rollupErr := listNodeMetricHourlyFromRollup(ctx, filter)
	if rollupErr == nil && len(rollup) > 0 && hourlyRollupCoversWindow(rollup[0].Hour, filter.Since) {
		return rollup, nil
	}

	raw, rawErr := listNodeMetricHourlyFromRaw(ctx, filter)
	if rawErr != nil {
		if rollupErr == nil && len(rollup) > 0 {
			return rollup, nil
		}
		return nil, rawErr
	}
	if len(rollup) == 0 {
		return raw, nil
	}
	return mergeNodeMetricHourlyPreferRollup(rollup, raw), nil
}

func mergeNodeMetricHourlyPreferRollup(rollup, raw []analyticsmodel.NodeMetricHourly) []analyticsmodel.NodeMetricHourly {
	byHour := make(map[int64]analyticsmodel.NodeMetricHourly, len(raw)+len(rollup))
	order := make([]int64, 0, len(raw)+len(rollup))
	add := func(row analyticsmodel.NodeMetricHourly, overwrite bool) {
		key := row.Hour.UTC().Truncate(time.Hour).Unix()
		if _, exists := byHour[key]; !exists {
			order = append(order, key)
			byHour[key] = row
			return
		}
		if overwrite {
			byHour[key] = row
		}
	}
	for _, row := range raw {
		add(row, false)
	}
	for _, row := range rollup {
		add(row, true)
	}
	result := make([]analyticsmodel.NodeMetricHourly, 0, len(order))
	slices.Sort(order)
	for _, key := range order {
		result = append(result, byHour[key])
	}
	return result
}

func listNodeMetricHourlyFromRollup(ctx context.Context, filter analyticsmodel.NodeObservabilityFilter) ([]analyticsmodel.NodeMetricHourly, error) {
	conn, err := chConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildCHNodeObservabilityFilterClause(filter, "hour")
	sql := fmt.Sprintf(`
SELECT
	hour,
	if(sum(cpu_usage_count) > 0, sum(cpu_usage_sum) / sum(cpu_usage_count), 0) AS average_cpu_usage_percent,
	if(sum(memory_usage_count) > 0, sum(memory_usage_sum) / sum(memory_usage_count), 0) AS average_memory_usage_percent,
	sum(greatest(network_rx_max - network_rx_min, 0)) AS network_rx_bytes,
	sum(greatest(network_tx_max - network_tx_min, 0)) AS network_tx_bytes,
	sum(greatest(disk_read_max - disk_read_min, 0)) AS disk_read_bytes,
	sum(greatest(disk_write_max - disk_write_min, 0)) AS disk_write_bytes,
	toUInt64(uniqExact(node_id)) AS reported_nodes
FROM %s
WHERE %s
GROUP BY hour
ORDER BY hour ASC`, chNodeMetricCapacityHourlyTableName(), clause)
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list node metric hourly from rollup: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanNodeMetricHourlyRows(rows)
}

func listNodeMetricHourlyFromRaw(ctx context.Context, filter analyticsmodel.NodeObservabilityFilter) ([]analyticsmodel.NodeMetricHourly, error) {
	conn, err := chConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildCHNodeObservabilityFilterClause(filter, "captured_at")
	sql := fmt.Sprintf(`
SELECT
	hour,
	avg(cpu_usage_percent) AS average_cpu_usage_percent,
	avg(memory_usage_percent) AS average_memory_usage_percent,
	sum(if(network_rx_delta >= 0, network_rx_delta, 0)) AS network_rx_bytes,
	sum(if(network_tx_delta >= 0, network_tx_delta, 0)) AS network_tx_bytes,
	sum(if(disk_read_delta >= 0, disk_read_delta, 0)) AS disk_read_bytes,
	sum(if(disk_write_delta >= 0, disk_write_delta, 0)) AS disk_write_bytes,
	toUInt64(uniqExact(node_id)) AS reported_nodes
FROM (
	SELECT
		node_id,
		toStartOfHour(captured_at) AS hour,
		cpu AS cpu_usage_percent,
		if(ram_total > 0, (ram * 100.0) / ram_total, 0) AS memory_usage_percent,
		net_total_down - lagInFrame(net_total_down, 1, net_total_down) OVER (
			PARTITION BY node_id ORDER BY captured_at, id
			ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
		) AS network_rx_delta,
		net_total_up - lagInFrame(net_total_up, 1, net_total_up) OVER (
			PARTITION BY node_id ORDER BY captured_at, id
			ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
		) AS network_tx_delta,
		disk_read_bytes - lagInFrame(disk_read_bytes, 1, disk_read_bytes) OVER (
			PARTITION BY node_id ORDER BY captured_at, id
			ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
		) AS disk_read_delta,
		disk_write_bytes - lagInFrame(disk_write_bytes, 1, disk_write_bytes) OVER (
			PARTITION BY node_id ORDER BY captured_at, id
			ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
		) AS disk_write_delta
	FROM %s
	WHERE %s
)
GROUP BY hour
ORDER BY hour ASC`, chNodeMetricSnapshotTableName(), clause)
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list node metric hourly: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanNodeMetricHourlyRows(rows)
}

func scanNodeMetricHourlyRows(rows driver.Rows) ([]analyticsmodel.NodeMetricHourly, error) {
	result := make([]analyticsmodel.NodeMetricHourly, 0)
	for rows.Next() {
		var (
			item          analyticsmodel.NodeMetricHourly
			reportedNodes uint64
			networkRx     int64
			networkTx     int64
			diskRead      int64
			diskWrite     int64
		)
		if err := rows.Scan(
			&item.Hour,
			&item.AverageCPUUsagePercent,
			&item.AverageMemoryUsagePercent,
			&networkRx,
			&networkTx,
			&diskRead,
			&diskWrite,
			&reportedNodes,
		); err != nil {
			return nil, fmt.Errorf("scan node metric hourly row: %w", err)
		}
		item.Hour = item.Hour.UTC()
		item.NetworkRxBytes = networkRx
		item.NetworkTxBytes = networkTx
		item.DiskReadBytes = diskRead
		item.DiskWriteBytes = diskWrite
		item.ReportedNodes = int(safeInt64Count(reportedNodes))
		result = append(result, item)
	}
	return result, nil
}
