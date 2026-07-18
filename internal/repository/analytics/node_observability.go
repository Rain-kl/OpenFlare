// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/Rain-kl/Wavelet/internal/db"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

func observabilityConn() (driver.Conn, error) {
	if db.ChConn == nil {
		return nil, fmt.Errorf("clickhouse connection is not initialized")
	}
	return db.ChConn, nil
}

// ListNodeMetricSnapshots returns metric snapshots matching filter.
func ListNodeMetricSnapshots(ctx context.Context, filter NodeObservabilityFilter) ([]analyticsmodel.NodeMetricSnapshot, error) {
	conn, err := observabilityConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildNodeObservabilityFilterClause(filter, "captured_at")
	tableName := nodeMetricSnapshotTableName()
	sql := fmt.Sprintf(`
SELECT id, node_id, captured_at, cpu_usage_percent, memory_used_bytes, memory_total_bytes, storage_used_bytes, storage_total_bytes, disk_read_bytes, disk_write_bytes, network_rx_bytes, network_tx_bytes, created_at
FROM %s
WHERE %s
ORDER BY %s`, tableName, clause, nodeObservabilityCapturedAtOrderClause())
	if filter.Limit > 0 {
		sql += clickHouseLimitClause
		args = append(args, filter.Limit)
	}
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list node metric snapshots: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanNodeMetricSnapshotRows(rows)
}

// ListLatestNodeMetricSnapshots returns the latest metric snapshot per node_id.
// Uses ClickHouse LIMIT 1 BY so dashboard health does not depend on a global raw LIMIT.
func ListLatestNodeMetricSnapshots(ctx context.Context, filter NodeObservabilityFilter) ([]analyticsmodel.NodeMetricSnapshot, error) {
	conn, err := observabilityConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildNodeObservabilityFilterClause(filter, "captured_at")
	sql := fmt.Sprintf(`
SELECT id, node_id, captured_at, cpu_usage_percent, memory_used_bytes, memory_total_bytes, storage_used_bytes, storage_total_bytes, disk_read_bytes, disk_write_bytes, network_rx_bytes, network_tx_bytes, created_at
FROM %s
WHERE %s
ORDER BY %s%s`, nodeMetricSnapshotTableName(), clause, nodeObservabilityCapturedAtOrderClause(), clickHouseLimit1ByNodeIDClause)
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list latest node metric snapshots: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanNodeMetricSnapshotRows(rows)
}

// ListNodeEdgeHealth returns L2 OpenResty health snapshots.
func ListNodeEdgeHealth(ctx context.Context, filter NodeObservabilityFilter) ([]analyticsmodel.NodeEdgeHealth, error) {
	conn, err := observabilityConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildNodeObservabilityFilterClause(filter, "captured_at")
	tableName := nodeEdgeHealthTableName()
	sql := fmt.Sprintf(`
SELECT id, node_id, captured_at, status, connections, created_at
FROM %s
WHERE %s
ORDER BY %s`, tableName, clause, nodeObservabilityCapturedAtOrderClause())
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
	return result, nil
}

// ListNodeObsFrps returns FRPS observations matching filter.
func ListNodeObsFrps(ctx context.Context, filter NodeObservabilityFilter) ([]analyticsmodel.NodeObsFrps, error) {
	conn, err := observabilityConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildNodeObservabilityFilterClause(filter, "captured_at")
	tableName := nodeObsFrpsTableName()
	sql := fmt.Sprintf(`
SELECT id, node_id, captured_at, frps_connections, frps_proxy_count, frps_client_count, frps_proxies, created_at
FROM %s
WHERE %s
ORDER BY %s`, tableName, clause, nodeObservabilityCapturedAtOrderClause())
	if filter.Limit > 0 {
		sql += clickHouseLimitClause
		args = append(args, filter.Limit)
	}
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list node frps observations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanNodeObsFrpsRows(rows)
}

// ListNodeObsFrpc returns FRPC observations matching filter.
func ListNodeObsFrpc(ctx context.Context, filter NodeObservabilityFilter) ([]analyticsmodel.NodeObsFrpc, error) {
	conn, err := observabilityConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildNodeObservabilityFilterClause(filter, "captured_at")
	tableName := nodeObsFrpcTableName()
	sql := fmt.Sprintf(`
SELECT id, node_id, captured_at, tunnel_status, connected_relays_count, created_at
FROM %s
WHERE %s
ORDER BY %s`, tableName, clause, nodeObservabilityCapturedAtOrderClause())
	if filter.Limit > 0 {
		sql += clickHouseLimitClause
		args = append(args, filter.Limit)
	}
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list node frpc observations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanNodeObsFrpcRows(rows)
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

func scanNodeObsFrpsRows(rows driver.Rows) ([]analyticsmodel.NodeObsFrps, error) {
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
}

// NodeTrafficHourly is an hourly traffic rollup row.
//
// UniqueVisitorCount is always 0 when sourced from of_access_log_hourly
// (true UV requires raw uniqExact on access logs).
type NodeTrafficHourly struct {
	NodeID             string
	Hour               time.Time
	RequestCount       int64
	ErrorCount         int64
	UniqueVisitorCount int64
}

// NodeMetricHourly is an hourly metric snapshot aggregation row.
//
// Disk and host network counters are cumulative. Prefer pre-aggregated min/max
// deltas from of_node_metric_capacity_hourly; raw fallback uses consecutive
// lagInFrame samples per node (negative deltas after counter reset are dropped).
type NodeMetricHourly struct {
	Hour                      time.Time
	AverageCPUUsagePercent    float64
	AverageMemoryUsagePercent float64
	NetworkRxBytes            int64
	NetworkTxBytes            int64
	DiskReadBytes             int64
	DiskWriteBytes            int64
	ReportedNodes             int
}

// ListNodeTrafficHourly returns hourly traffic from of_access_log_hourly (M5).
// UniqueVisitorCount is always 0 here (UV requires raw uniqExact on access logs).
func ListNodeTrafficHourly(ctx context.Context, filter NodeObservabilityFilter) ([]NodeTrafficHourly, error) {
	rows, err := ListAccessLogHourly(ctx, filter)
	if err != nil {
		return nil, err
	}
	// Aggregate across hosts per node/hour.
	type key struct {
		node string
		hour int64
	}
	merged := make(map[key]*NodeTrafficHourly)
	order := make([]key, 0)
	for _, row := range rows {
		k := key{node: row.NodeID, hour: row.Hour.UTC().Unix()}
		item := merged[k]
		if item == nil {
			item = &NodeTrafficHourly{NodeID: row.NodeID, Hour: row.Hour.UTC()}
			merged[k] = item
			order = append(order, k)
		}
		item.RequestCount += row.RequestCount
		item.ErrorCount += row.ErrorCount
	}
	result := make([]NodeTrafficHourly, 0, len(order))
	for _, k := range order {
		result = append(result, *merged[k])
	}
	return result, nil
}

// ListAccessLogHourly returns Server-side access log hourly rollups.
func ListAccessLogHourly(ctx context.Context, filter NodeObservabilityFilter) ([]analyticsmodel.AccessLogHourly, error) {
	conn, err := observabilityConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildNodeObservabilityFilterClause(filter, "hour")
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
ORDER BY hour ASC, node_id ASC, host ASC`, accessLogHourlyTableName(), clause)
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

// hourlyRollupCoversWindow reports whether rollup coverage starts near the requested window.
// rows must be ordered by hour ascending.
func hourlyRollupCoversWindow(earliestHour time.Time, since time.Time) bool {
	if since.IsZero() {
		return true
	}
	sinceHour := since.UTC().Truncate(time.Hour)
	earliest := earliestHour.UTC().Truncate(time.Hour)
	return !earliest.After(sinceHour.Add(hourlyRollupMaxLead))
}

// ListNodeMetricHourly returns hourly metric snapshot aggregates matching filter.
//
// Strategy (optimal for correctness + cost):
//  1. Load of_node_metric_capacity_hourly rollup.
//  2. If rollup spans the window from filter.Since, return it alone (cheap path).
//  3. Otherwise load raw lagInFrame aggregates and merge by hour: rollup wins on
//     overlap, raw fills historical gaps (MV never backfills pre-creation data).
func ListNodeMetricHourly(ctx context.Context, filter NodeObservabilityFilter) ([]NodeMetricHourly, error) {
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
	// Partial rollup (or rollupErr with empty slice): merge; raw fills historical gaps.
	return mergeNodeMetricHourlyPreferRollup(rollup, raw), nil
}

// mergeNodeMetricHourlyPreferRollup unions two hour series (both ASC by Hour).
// Rollup values replace raw for the same hour; raw supplies missing hours.
func mergeNodeMetricHourlyPreferRollup(rollup, raw []NodeMetricHourly) []NodeMetricHourly {
	byHour := make(map[int64]NodeMetricHourly, len(raw)+len(rollup))
	order := make([]int64, 0, len(raw)+len(rollup))
	add := func(row NodeMetricHourly, overwrite bool) {
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
	result := make([]NodeMetricHourly, 0, len(order))
	// Keep chronological order of first-seen keys; re-sort by hour for stability.
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	for _, key := range order {
		result = append(result, byHour[key])
	}
	return result
}

func listNodeMetricHourlyFromRollup(ctx context.Context, filter NodeObservabilityFilter) ([]NodeMetricHourly, error) {
	conn, err := observabilityConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildNodeObservabilityFilterClause(filter, "hour")
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
ORDER BY hour ASC`, nodeMetricCapacityHourlyTableName(), clause)
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list node metric hourly from rollup: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanNodeMetricHourlyRows(rows)
}

func listNodeMetricHourlyFromRaw(ctx context.Context, filter NodeObservabilityFilter) ([]NodeMetricHourly, error) {
	conn, err := observabilityConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildNodeObservabilityFilterClause(filter, "captured_at")
	tableName := nodeMetricSnapshotTableName()
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
ORDER BY hour ASC`, tableName, clause)
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list node metric hourly: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanNodeMetricHourlyRows(rows)
}

func scanNodeMetricHourlyRows(rows driver.Rows) ([]NodeMetricHourly, error) {
	result := make([]NodeMetricHourly, 0)
	for rows.Next() {
		var (
			item          NodeMetricHourly
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

func scanNodeObsFrpcRows(rows driver.Rows) ([]analyticsmodel.NodeObsFrpc, error) {
	var result []analyticsmodel.NodeObsFrpc
	for rows.Next() {
		var item analyticsmodel.NodeObsFrpc
		if err := rows.Scan(
			&item.ID,
			&item.NodeID,
			&item.CapturedAt,
			&item.TunnelStatus,
			&item.ConnectedRelaysCount,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan node frpc observation row: %w", err)
		}
		item.CapturedAt = item.CapturedAt.UTC()
		item.CreatedAt = item.CreatedAt.UTC()
		result = append(result, item)
	}
	return result, nil
}
