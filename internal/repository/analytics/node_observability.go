// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"fmt"
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

// ListNodeRequestReports returns request reports matching filter.
func ListNodeRequestReports(ctx context.Context, filter NodeObservabilityFilter) ([]analyticsmodel.NodeRequestReport, error) {
	conn, err := observabilityConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildNodeObservabilityFilterClause(filter, "window_ended_at")
	tableName := nodeRequestReportTableName()
	sql := fmt.Sprintf(`
SELECT id, node_id, window_started_at, window_ended_at, request_count, error_count, unique_visitor_count, status_codes_json, top_domains_json, source_countries_json, created_at
FROM %s
WHERE %s
ORDER BY %s`, tableName, clause, nodeObservabilityWindowEndedAtOrderClause())
	if filter.Limit > 0 {
		sql += clickHouseLimitClause
		args = append(args, filter.Limit)
	}
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list node request reports: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanNodeRequestReportRows(rows)
}

// ListNodeObsOpenresty returns OpenResty observations matching filter.
func ListNodeObsOpenresty(ctx context.Context, filter NodeObservabilityFilter) ([]analyticsmodel.NodeObsOpenresty, error) {
	conn, err := observabilityConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildNodeObservabilityFilterClause(filter, "captured_at")
	tableName := nodeObsOpenrestyTableName()
	sql := fmt.Sprintf(`
SELECT id, node_id, captured_at, openresty_rx_bytes, openresty_tx_bytes, openresty_connections, created_at
FROM %s
WHERE %s
ORDER BY %s`, tableName, clause, nodeObservabilityCapturedAtOrderClause())
	if filter.Limit > 0 {
		sql += clickHouseLimitClause
		args = append(args, filter.Limit)
	}
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list node openresty observations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanNodeObsOpenrestyRows(rows)
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

func scanNodeRequestReportRows(rows driver.Rows) ([]analyticsmodel.NodeRequestReport, error) {
	var result []analyticsmodel.NodeRequestReport
	for rows.Next() {
		var item analyticsmodel.NodeRequestReport
		if err := rows.Scan(
			&item.ID,
			&item.NodeID,
			&item.WindowStartedAt,
			&item.WindowEndedAt,
			&item.RequestCount,
			&item.ErrorCount,
			&item.UniqueVisitorCount,
			&item.StatusCodesJSON,
			&item.TopDomainsJSON,
			&item.SourceCountriesJSON,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan node request report row: %w", err)
		}
		item.WindowStartedAt = item.WindowStartedAt.UTC()
		item.WindowEndedAt = item.WindowEndedAt.UTC()
		item.CreatedAt = item.CreatedAt.UTC()
		result = append(result, item)
	}
	return result, nil
}

func scanNodeObsOpenrestyRows(rows driver.Rows) ([]analyticsmodel.NodeObsOpenresty, error) {
	var result []analyticsmodel.NodeObsOpenresty
	for rows.Next() {
		var item analyticsmodel.NodeObsOpenresty
		if err := rows.Scan(
			&item.ID,
			&item.NodeID,
			&item.CapturedAt,
			&item.OpenrestyRxBytes,
			&item.OpenrestyTxBytes,
			&item.OpenrestyConnections,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan node openresty observation row: %w", err)
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

const nodeTrafficHourlyTableName = "of_node_traffic_hourly"

// NodeTrafficHourly is an hourly traffic rollup row.
type NodeTrafficHourly struct {
	NodeID             string
	Hour               time.Time
	RequestCount       int64
	ErrorCount         int64
	UniqueVisitorCount int64
}

// NodeMetricHourly is an hourly metric snapshot aggregation row.
//
// Disk and host network counters are cumulative; deltas use consecutive
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

// NodeOpenrestyHourly is an hourly OpenResty observation aggregation row.
type NodeOpenrestyHourly struct {
	Hour             time.Time
	OpenrestyRxBytes int64
	OpenrestyTxBytes int64
	ReportedNodes    int
}

// ListNodeTrafficHourly returns hourly traffic rollup rows matching filter.
func ListNodeTrafficHourly(ctx context.Context, filter NodeObservabilityFilter) ([]NodeTrafficHourly, error) {
	conn, err := observabilityConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildNodeObservabilityFilterClause(filter, "hour")
	sql := fmt.Sprintf(`
SELECT
	node_id,
	hour,
	sum(request_count) AS request_count,
	sum(error_count) AS error_count,
	sum(unique_visitor_count) AS unique_visitor_count
FROM %s
WHERE %s
GROUP BY node_id, hour
ORDER BY hour ASC`, nodeTrafficHourlyTableName, clause)
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list node traffic hourly: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make([]NodeTrafficHourly, 0)
	for rows.Next() {
		var (
			item                                           NodeTrafficHourly
			requestCount, errorCount, uniqueVisitorCount uint64
		)
		if err := rows.Scan(&item.NodeID, &item.Hour, &requestCount, &errorCount, &uniqueVisitorCount); err != nil {
			return nil, fmt.Errorf("scan node traffic hourly row: %w", err)
		}
		item.Hour = item.Hour.UTC()
		item.RequestCount = safeInt64Count(requestCount)
		item.ErrorCount = safeInt64Count(errorCount)
		item.UniqueVisitorCount = safeInt64Count(uniqueVisitorCount)
		result = append(result, item)
	}
	return result, nil
}

// ListNodeMetricHourly returns hourly metric snapshot aggregates matching filter.
// Capacity uses sample averages; network/disk use consecutive counter deltas via lagInFrame.
func ListNodeMetricHourly(ctx context.Context, filter NodeObservabilityFilter) ([]NodeMetricHourly, error) {
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

// ListNodeOpenrestyHourly returns hourly OpenResty observation aggregates matching filter.
func ListNodeOpenrestyHourly(ctx context.Context, filter NodeObservabilityFilter) ([]NodeOpenrestyHourly, error) {
	conn, err := observabilityConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildNodeObservabilityFilterClause(filter, "captured_at")
	tableName := nodeObsOpenrestyTableName()
	sql := fmt.Sprintf(`
SELECT
	hour,
	sum(if(openresty_rx_delta >= 0, openresty_rx_delta, 0)) AS openresty_rx_bytes,
	sum(if(openresty_tx_delta >= 0, openresty_tx_delta, 0)) AS openresty_tx_bytes,
	toUInt64(uniqExact(node_id)) AS reported_nodes
FROM (
	SELECT
		node_id,
		toStartOfHour(captured_at) AS hour,
		openresty_rx_bytes - lagInFrame(openresty_rx_bytes, 1, openresty_rx_bytes) OVER (
			PARTITION BY node_id ORDER BY captured_at, id
			ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
		) AS openresty_rx_delta,
		openresty_tx_bytes - lagInFrame(openresty_tx_bytes, 1, openresty_tx_bytes) OVER (
			PARTITION BY node_id ORDER BY captured_at, id
			ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
		) AS openresty_tx_delta
	FROM %s
	WHERE %s
)
GROUP BY hour
ORDER BY hour ASC`, tableName, clause)
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list node openresty hourly: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make([]NodeOpenrestyHourly, 0)
	for rows.Next() {
		var (
			item          NodeOpenrestyHourly
			reportedNodes uint64
			rx            int64
			tx            int64
		)
		if err := rows.Scan(&item.Hour, &rx, &tx, &reportedNodes); err != nil {
			return nil, fmt.Errorf("scan node openresty hourly row: %w", err)
		}
		item.Hour = item.Hour.UTC()
		item.OpenrestyRxBytes = rx
		item.OpenrestyTxBytes = tx
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
