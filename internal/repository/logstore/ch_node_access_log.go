// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

func (s *clickhouseLogStore) List(ctx context.Context, query model.OpenFlareAccessLogQuery) ([]*model.OpenFlareAccessLog, error) {
	conn, err := chConn()
	if err != nil {
		return nil, err
	}
	filter := toNodeAccessLogFilter(query)
	clause, args := buildCHNodeAccessLogFilterClause(filter)
	sql := fmt.Sprintf(`
SELECT %s
FROM %s
WHERE %s
ORDER BY %s`, (analyticsmodel.NodeAccessLog{}).InsertColumns(), chNodeAccessLogTableName(), clause, chNodeAccessLogOrderClause(filter.SortBy, filter.SortOrder))
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
	scanned, err := scanNodeAccessLogRows(rows)
	if err != nil {
		return nil, err
	}
	return fromAnalyticsNodeAccessLogs(scanned), nil
}

//nolint:dupl // scan shapes differ by model fields; shared helper would obscure CH column mapping
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
			&item.UserAgent,
			&item.CacheStatus,
			&item.StatusCode,
			&item.BytesSent,
			&item.RequestLength,
			&item.RequestTimeMs,
			&item.CreatedAt,
			&item.WAFAction,
			&item.WAFAudit,
			&item.ClientASN,
			&item.TCPRTTUs,
		); err != nil {
			return nil, fmt.Errorf("scan node access log row: %w", err)
		}
		item.LoggedAt = item.LoggedAt.UTC()
		item.CreatedAt = item.CreatedAt.UTC()
		result = append(result, item)
	}
	return result, nil
}

func (s *clickhouseLogStore) Count(ctx context.Context, query model.OpenFlareAccessLogQuery) (int64, int64, int64, error) {
	conn, err := chConn()
	if err != nil {
		return 0, 0, 0, err
	}
	clause, args := buildCHNodeAccessLogFilterClause(toNodeAccessLogFilter(query))
	countSQL := fmt.Sprintf(`
SELECT
	count() AS total_records,
	uniqExactIf(remote_addr, remote_addr != '') AS total_ips,
	sum(bytes_sent) AS total_bytes
FROM %s
WHERE %s`, chNodeAccessLogTableName(), clause)
	var totalRecords, totalIPs, totalBytes uint64
	if err := conn.QueryRow(ctx, countSQL, args...).Scan(&totalRecords, &totalIPs, &totalBytes); err != nil {
		return 0, 0, 0, fmt.Errorf("count node access logs: %w", err)
	}
	return safeInt64Count(totalRecords), safeInt64Count(totalIPs), safeInt64Count(totalBytes), nil
}

func (s *clickhouseLogStore) RegionCounts(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareAccessLogRegionCount, error) {
	conn, err := chConn()
	if err != nil {
		return nil, err
	}
	filter := analyticsmodel.NodeAccessLogFilter{NodeID: nodeID, Since: since}
	clause, args := buildCHNodeAccessLogFilterClause(filter)
	sql := fmt.Sprintf(`
SELECT trim(region) AS trimmed_region, count() AS count
FROM %s
WHERE %s AND trim(region) != ''
GROUP BY trimmed_region
ORDER BY count DESC, trimmed_region ASC`, chNodeAccessLogTableName(), clause)
	if limit > 0 {
		sql += clickHouseLimitClause
		args = append(args, limit)
	}
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("region counts node access logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []*model.OpenFlareAccessLogRegionCount
	for rows.Next() {
		var (
			region string
			count  uint64
		)
		if err := rows.Scan(&region, &count); err != nil {
			return nil, fmt.Errorf("scan region count row: %w", err)
		}
		result = append(result, &model.OpenFlareAccessLogRegionCount{
			Region: region,
			Count:  safeInt64Count(count),
		})
	}
	return result, nil
}

func (s *clickhouseLogStore) TrafficSummary(ctx context.Context, query model.OpenFlareAccessLogQuery) (model.OpenFlareAccessLogTrafficSummary, error) {
	conn, err := chConn()
	if err != nil {
		return model.OpenFlareAccessLogTrafficSummary{}, err
	}
	clause, args := buildCHNodeAccessLogFilterClause(toNodeAccessLogFilter(query))
	sql := fmt.Sprintf(`
SELECT
	count() AS request_count,
	countIf(status_code >= 400 AND status_code < 500) AS status_4xx_count,
	countIf(status_code >= 500) AS error_count,
	uniqExactIf(remote_addr, remote_addr != '') AS unique_ips,
	sum(bytes_sent) AS bytes_sent,
	sum(request_length) AS request_length,
	uniqExactIf(node_id, node_id != '') AS node_count
FROM %s
WHERE %s`, chNodeAccessLogTableName(), clause)
	var requestCount, status4xxCount, errorCount, uniqueIPs, bytesSent, requestLength, nodeCount uint64
	if err := conn.QueryRow(ctx, sql, args...).Scan(
		&requestCount, &status4xxCount, &errorCount, &uniqueIPs, &bytesSent, &requestLength, &nodeCount,
	); err != nil {
		return model.OpenFlareAccessLogTrafficSummary{}, fmt.Errorf("traffic summary node access logs: %w", err)
	}
	return model.OpenFlareAccessLogTrafficSummary{
		RequestCount:   safeInt64Count(requestCount),
		Status4xxCount: safeInt64Count(status4xxCount),
		ErrorCount:     safeInt64Count(errorCount),
		UniqueIPCount:  safeInt64Count(uniqueIPs),
		BytesSent:      safeInt64Count(bytesSent),
		RequestLength:  safeInt64Count(requestLength),
		NodeCount:      safeInt64Count(nodeCount),
	}, nil
}

func (s *clickhouseLogStore) ValueCounts(ctx context.Context, query model.OpenFlareAccessLogQuery, column string, limit int) ([]model.OpenFlareAccessLogValueCount, error) {
	conn, err := chConn()
	if err != nil {
		return nil, err
	}
	col := strings.TrimSpace(strings.ToLower(column))
	valueExpr, ok := chNodeAccessLogValueCountExpr(col)
	if !ok {
		return nil, fmt.Errorf("unsupported value count column: %s", column)
	}
	clause, args := buildCHNodeAccessLogFilterClause(toNodeAccessLogFilter(query))
	filterExpr := valueExpr + " != ''"
	if col == nodeAccessLogColumnStatusCode {
		filterExpr = "status_code >= 0"
	}
	sql := fmt.Sprintf(`
SELECT %s AS value, count() AS count
FROM %s
WHERE %s AND %s
GROUP BY value
ORDER BY count DESC, value ASC`, valueExpr, chNodeAccessLogTableName(), clause, filterExpr)
	if limit > 0 {
		sql += clickHouseLimitClause
		args = append(args, limit)
	}
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("value counts node access logs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var result []model.OpenFlareAccessLogValueCount
	for rows.Next() {
		var (
			value string
			count uint64
		)
		if err := rows.Scan(&value, &count); err != nil {
			return nil, fmt.Errorf("scan value count row: %w", err)
		}
		result = append(result, model.OpenFlareAccessLogValueCount{
			Value: value,
			Count: safeInt64Count(count),
		})
	}
	return result, nil
}

func chNodeAccessLogValueCountExpr(column string) (string, bool) {
	switch column {
	case nodeAccessLogColumnStatusCode:
		return "toString(" + nodeAccessLogColumnStatusCode + ")", true
	case nodeAccessLogColumnHost:
		return "trim(" + nodeAccessLogColumnHost + ")", true
	case nodeAccessLogColumnPath:
		return "trim(" + nodeAccessLogColumnPath + ")", true
	case nodeAccessLogColumnRemoteAddr:
		return "trim(" + nodeAccessLogColumnRemoteAddr + ")", true
	case nodeAccessLogColumnUserAgent:
		return "trim(" + nodeAccessLogColumnUserAgent + ")", true
	default:
		return "", false
	}
}

func (s *clickhouseLogStore) NodeAggregates(ctx context.Context, query model.OpenFlareAccessLogQuery) ([]model.OpenFlareAccessLogNodeAggregate, error) {
	conn, err := chConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildCHNodeAccessLogFilterClause(toNodeAccessLogFilter(query))
	sql := fmt.Sprintf(`
SELECT
	node_id,
	count() AS request_count,
	countIf(status_code >= 500) AS error_count,
	uniqExactIf(remote_addr, remote_addr != '') AS unique_ips
FROM %s
WHERE %s AND node_id != ''
GROUP BY node_id
ORDER BY request_count DESC, node_id ASC`, chNodeAccessLogTableName(), clause)
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("node aggregates node access logs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var result []model.OpenFlareAccessLogNodeAggregate
	for rows.Next() {
		var (
			nodeID                              string
			requestCount, errorCount, uniqueIPs uint64
		)
		if err := rows.Scan(&nodeID, &requestCount, &errorCount, &uniqueIPs); err != nil {
			return nil, fmt.Errorf("scan node aggregate row: %w", err)
		}
		result = append(result, model.OpenFlareAccessLogNodeAggregate{
			NodeID:        nodeID,
			RequestCount:  safeInt64Count(requestCount),
			ErrorCount:    safeInt64Count(errorCount),
			UniqueIPCount: safeInt64Count(uniqueIPs),
		})
	}
	return result, nil
}
