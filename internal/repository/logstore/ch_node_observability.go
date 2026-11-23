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

const chNodeMetricSnapshotSelectColumns = "id, node_id, captured_at, cpu_usage_percent, memory_used_bytes, memory_total_bytes, storage_used_bytes, storage_total_bytes, disk_read_bytes, disk_write_bytes, network_rx_bytes, network_tx_bytes, created_at"

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
	return listCHNodeObservability(
		ctx,
		chNodeMetricSnapshotTableName(),
		chNodeMetricSnapshotSelectColumns,
		"list latest node metric snapshots",
		analyticsmodel.NodeObservabilityFilter{NodeID: nodeID, Since: since},
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
		cpu_usage_percent,
		if(memory_total_bytes > 0, (memory_used_bytes * 100.0) / memory_total_bytes, 0) AS memory_usage_percent,
		network_rx_bytes - lagInFrame(network_rx_bytes, 1, network_rx_bytes) OVER (
			PARTITION BY node_id ORDER BY captured_at, id
			ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
		) AS network_rx_delta,
		network_tx_bytes - lagInFrame(network_tx_bytes, 1, network_tx_bytes) OVER (
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
