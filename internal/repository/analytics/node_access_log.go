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

// NodeAccessLogRegionCount aggregates access log regions.
type NodeAccessLogRegionCount struct {
	Region string
	Count  int64
}

func nodeAccessLogConn() (driver.Conn, error) {
	if db.ChConn == nil {
		return nil, fmt.Errorf("clickhouse connection is not initialized")
	}
	return db.ChConn, nil
}

// ListNodeAccessLogs returns access logs matching filter.
func ListNodeAccessLogs(ctx context.Context, filter NodeAccessLogFilter) ([]analyticsmodel.NodeAccessLog, error) {
	conn, err := nodeAccessLogConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildNodeAccessLogFilterClause(filter)
	tableName := nodeAccessLogTableName()
	sql := fmt.Sprintf(`
SELECT id, node_id, logged_at, remote_addr, region, host, path, status_code, bytes_sent, created_at
FROM %s
WHERE %s
ORDER BY %s`, tableName, clause, nodeAccessLogOrderClause(filter.SortBy, filter.SortOrder))
	if filter.PageSize > 0 {
		if filter.Page < 0 {
			filter.Page = 0
		}
		sql += clickHouseLimitOffsetClause
		args = append(args, filter.PageSize, filter.Page*filter.PageSize)
	}
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list node access logs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanNodeAccessLogRows(rows)
}

func scanNodeAccessLogRows(rows driver.Rows) ([]analyticsmodel.NodeAccessLog, error) {
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
			&item.StatusCode,
			&item.BytesSent,
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

// CountNodeAccessLogs returns total records, distinct IPs, and total bytes sent matching filter.
func CountNodeAccessLogs(ctx context.Context, filter NodeAccessLogFilter) (int64, int64, int64, error) {
	conn, err := nodeAccessLogConn()
	if err != nil {
		return 0, 0, 0, err
	}
	clause, args := buildNodeAccessLogFilterClause(filter)
	tableName := nodeAccessLogTableName()

	countSQL := fmt.Sprintf(`
SELECT
	count() AS total_records,
	uniqExactIf(remote_addr, remote_addr != '') AS total_ips,
	sum(bytes_sent) AS total_bytes
FROM %s
WHERE %s`, tableName, clause)
	var totalRecords, totalIPs, totalBytes uint64
	if err := conn.QueryRow(ctx, countSQL, args...).Scan(&totalRecords, &totalIPs, &totalBytes); err != nil {
		return 0, 0, 0, fmt.Errorf("count node access logs: %w", err)
	}
	return safeInt64Count(totalRecords), safeInt64Count(totalIPs), safeInt64Count(totalBytes), nil
}

// RegionCountsNodeAccessLogs returns region counts for a node since a time.
func RegionCountsNodeAccessLogs(ctx context.Context, nodeID string, since time.Time, limit int) ([]NodeAccessLogRegionCount, error) {
	conn, err := nodeAccessLogConn()
	if err != nil {
		return nil, err
	}
	filter := NodeAccessLogFilter{NodeID: nodeID, Since: since}
	clause, args := buildNodeAccessLogFilterClause(filter)
	tableName := nodeAccessLogTableName()
	sql := fmt.Sprintf(`
SELECT trim(region) AS trimmed_region, count() AS count
FROM %s
WHERE %s AND trim(region) != ''
GROUP BY trimmed_region
ORDER BY count DESC, trimmed_region ASC`, tableName, clause)
	if limit > 0 {
		sql += clickHouseLimitClause
		args = append(args, limit)
	}
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("region counts node access logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []NodeAccessLogRegionCount
	for rows.Next() {
		var (
			region string
			count  uint64
		)
		if err := rows.Scan(&region, &count); err != nil {
			return nil, fmt.Errorf("scan region count row: %w", err)
		}
		result = append(result, NodeAccessLogRegionCount{
			Region: region,
			Count:  safeInt64Count(count),
		})
	}
	return result, nil
}
