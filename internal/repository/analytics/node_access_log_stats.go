// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// NodeAccessLogBucketAggregate is a folded bucket aggregate row.
type NodeAccessLogBucketAggregate struct {
	BucketEpoch      int64
	RequestCount     int64
	SuccessCount     int64
	ClientErrorCount int64
	ServerErrorCount int64
	UniqueIPCount    int64
	UniqueHostCount  int64
}

// NodeAccessLogWAFIPAggregate is a per-IP aggregate row for WAF automatic rules.
type NodeAccessLogWAFIPAggregate struct {
	RemoteAddr       string
	RequestCount     int64
	Status404Count   int64
	ClientErrorCount int64
	ServerErrorCount int64
	IPHostCount      int64
	LastSeenEpoch    int64
	StatusCounts     map[int]int64
}

// NodeAccessLogBucketDimension is a bucket dimension value.
type NodeAccessLogBucketDimension struct {
	BucketEpoch int64
	Value       string
}

// NodeAccessLogIPAggregate is an IP aggregate row.
type NodeAccessLogIPAggregate struct {
	RemoteAddr       string
	RequestCount     int64
	SuccessCount     int64
	ClientErrorCount int64
	ServerErrorCount int64
	LastSeenEpoch    int64
}

// NodeAccessLogIPSummary is an IP summary row.
type NodeAccessLogIPSummary struct {
	RemoteAddr     string
	TotalRequests  int64
	RecentRequests int64
	LastSeenEpoch  int64
}

// NodeAccessLogIPTrend is an IP trend bucket row.
type NodeAccessLogIPTrend struct {
	BucketEpoch  int64
	RequestCount int64
}

// BucketAggregatesNodeAccessLogs returns folded bucket aggregates with unique IP/host counts.
func BucketAggregatesNodeAccessLogs(ctx context.Context, filter NodeAccessLogFilter, bucketSeconds int64) ([]NodeAccessLogBucketAggregate, error) {
	conn, err := nodeAccessLogConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildNodeAccessLogFilterClause(filter)
	bucketExpr := nodeAccessLogBucketEpochExpr(bucketSeconds)
	tableName := nodeAccessLogTableName()
	sql := fmt.Sprintf(`
SELECT
	%s AS bucket_epoch,
	count() AS request_count,
	countIf(status_code < 400) AS success_count,
	countIf(status_code >= 400 AND status_code < 500) AS client_error_count,
	countIf(status_code >= 500) AS server_error_count,
	uniqExactIf(remote_addr, remote_addr != '') AS unique_ip_count,
	uniqExactIf(host, host != '') AS unique_host_count
FROM %s
WHERE %s
GROUP BY bucket_epoch
ORDER BY %s`, bucketExpr, tableName, clause, nodeAccessLogBucketOrderClause(filter.SortBy, filter.SortOrder))
	if filter.PageSize > 0 {
		if filter.Page < 0 {
			filter.Page = 0
		}
		sql += clickHouseLimitOffsetClause
		args = append(args, filter.PageSize, filter.Page*filter.PageSize)
	}
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("bucket aggregates node access logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []NodeAccessLogBucketAggregate
	for rows.Next() {
		var (
			bucketEpoch                                                                    int64
			requestCount, successCount, clientErrorCount, serverErrorCount, uniqueIPCount, uniqueHostCount uint64
		)
		if err := rows.Scan(&bucketEpoch, &requestCount, &successCount, &clientErrorCount, &serverErrorCount, &uniqueIPCount, &uniqueHostCount); err != nil {
			return nil, fmt.Errorf("scan bucket aggregate row: %w", err)
		}
		result = append(result, NodeAccessLogBucketAggregate{
			BucketEpoch:      bucketEpoch,
			RequestCount:     safeInt64Count(requestCount),
			SuccessCount:     safeInt64Count(successCount),
			ClientErrorCount: safeInt64Count(clientErrorCount),
			ServerErrorCount: safeInt64Count(serverErrorCount),
			UniqueIPCount:    safeInt64Count(uniqueIPCount),
			UniqueHostCount:  safeInt64Count(uniqueHostCount),
		})
	}
	return result, nil
}

// CountBucketAggregatesNodeAccessLogs returns the number of folded buckets matching filter.
func CountBucketAggregatesNodeAccessLogs(ctx context.Context, filter NodeAccessLogFilter, bucketSeconds int64) (int64, error) {
	conn, err := nodeAccessLogConn()
	if err != nil {
		return 0, err
	}
	clause, args := buildNodeAccessLogFilterClause(filter)
	bucketExpr := nodeAccessLogBucketEpochExpr(bucketSeconds)
	tableName := nodeAccessLogTableName()
	sql := fmt.Sprintf(`
SELECT count() FROM (
	SELECT 1
	FROM %s
	WHERE %s
	GROUP BY %s
)`, tableName, clause, bucketExpr)
	var totalBuckets uint64
	if err := conn.QueryRow(ctx, sql, args...).Scan(&totalBuckets); err != nil {
		return 0, fmt.Errorf("count bucket aggregates node access logs: %w", err)
	}
	return safeInt64Count(totalBuckets), nil
}

// BucketDimensionsNodeAccessLogs returns bucket dimension values.
func BucketDimensionsNodeAccessLogs(ctx context.Context, filter NodeAccessLogFilter, column string, bucketSeconds int64) ([]NodeAccessLogBucketDimension, error) {
	conn, err := nodeAccessLogConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildNodeAccessLogFilterClause(filter)
	bucketExpr := nodeAccessLogBucketEpochExpr(bucketSeconds)
	tableName := nodeAccessLogTableName()
	sql := fmt.Sprintf(`
SELECT
	%s AS bucket_epoch,
	trim(%s) AS value
FROM %s
WHERE %s AND trim(%s) != ''
GROUP BY bucket_epoch, trim(%s)`, bucketExpr, column, tableName, clause, column, column)
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("bucket dimensions node access logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []NodeAccessLogBucketDimension
	for rows.Next() {
		var item NodeAccessLogBucketDimension
		if err := rows.Scan(&item.BucketEpoch, &item.Value); err != nil {
			return nil, fmt.Errorf("scan bucket dimension row: %w", err)
		}
		result = append(result, item)
	}
	return result, nil
}

// IPAggregatesNodeAccessLogs returns IP aggregate rows.
func IPAggregatesNodeAccessLogs(ctx context.Context, filter NodeAccessLogFilter, exactRemoteAddr bool) ([]NodeAccessLogIPAggregate, error) {
	conn, err := nodeAccessLogConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildNodeAccessLogFilterClause(filter)
	queryClause := clause
	queryArgs := append([]any{}, args...)
	if exactRemoteAddr {
		trimmed := normalizeNodeAccessLogRemoteAddr(filter.RemoteAddr)
		if trimmed == "" {
			return []NodeAccessLogIPAggregate{}, nil
		}
		queryClause = combineNodeAccessLogSQLClauses(queryClause, "remote_addr = ?")
		queryArgs = append(queryArgs, trimmed)
	}
	lastSeenExpr := nodeAccessLogEpochExpr()
	tableName := nodeAccessLogTableName()
	sql := fmt.Sprintf(`
SELECT
	remote_addr,
	count() AS request_count,
	countIf(status_code < 400) AS success_count,
	countIf(status_code >= 400 AND status_code < 500) AS client_error_count,
	countIf(status_code >= 500) AS server_error_count,
	max(%s) AS last_seen_epoch
FROM %s
WHERE %s AND remote_addr != ''
GROUP BY remote_addr`, lastSeenExpr, tableName, queryClause)
	rows, err := conn.Query(ctx, sql, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("ip aggregates node access logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []NodeAccessLogIPAggregate
	for rows.Next() {
		var (
			remoteAddr                                       string
			lastSeenEpoch                                    int64
			requestCount, successCount, clientErrorCount, serverErrorCount uint64
		)
		if err := rows.Scan(&remoteAddr, &requestCount, &successCount, &clientErrorCount, &serverErrorCount, &lastSeenEpoch); err != nil {
			return nil, fmt.Errorf("scan ip aggregate row: %w", err)
		}
		result = append(result, NodeAccessLogIPAggregate{
			RemoteAddr:       remoteAddr,
			RequestCount:     safeInt64Count(requestCount),
			SuccessCount:     safeInt64Count(successCount),
			ClientErrorCount: safeInt64Count(clientErrorCount),
			ServerErrorCount: safeInt64Count(serverErrorCount),
			LastSeenEpoch:    lastSeenEpoch,
		})
	}
	return result, nil
}

// IPSummariesNodeAccessLogs returns paginated IP summary rows.
func IPSummariesNodeAccessLogs(ctx context.Context, filter NodeAccessLogFilter, recentSince time.Time) ([]NodeAccessLogIPSummary, error) {
	conn, err := nodeAccessLogConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildNodeAccessLogFilterClause(filter)
	lastSeenExpr := nodeAccessLogEpochExpr()
	recentClause := "0"
	queryArgs := make([]any, 0, len(args)+1)
	if !recentSince.IsZero() {
		recentClause = "if(logged_at >= ?, 1, 0)"
		queryArgs = append(queryArgs, recentSince)
	}
	queryArgs = append(queryArgs, args...)
	tableName := nodeAccessLogTableName()
	sql := fmt.Sprintf(`
SELECT
	remote_addr,
	count() AS total_requests,
	sum(%s) AS recent_requests,
	max(%s) AS last_seen_epoch
FROM %s
WHERE %s AND remote_addr != ''
GROUP BY remote_addr
ORDER BY %s`, recentClause, lastSeenExpr, tableName, clause, nodeAccessLogIPSummaryOrderClause(filter.SortBy, filter.SortOrder))
	if filter.PageSize > 0 {
		if filter.Page < 0 {
			filter.Page = 0
		}
		sql += clickHouseLimitOffsetClause
		queryArgs = append(queryArgs, filter.PageSize, filter.Page*filter.PageSize)
	}
	rows, err := conn.Query(ctx, sql, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("ip summaries node access logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []NodeAccessLogIPSummary
	for rows.Next() {
		var (
			remoteAddr                    string
			lastSeenEpoch                 int64
			totalRequests, recentRequests uint64
		)
		if err := rows.Scan(&remoteAddr, &totalRequests, &recentRequests, &lastSeenEpoch); err != nil {
			return nil, fmt.Errorf("scan ip summary row: %w", err)
		}
		result = append(result, NodeAccessLogIPSummary{
			RemoteAddr:     remoteAddr,
			TotalRequests:  safeInt64Count(totalRequests),
			RecentRequests: safeInt64Count(recentRequests),
			LastSeenEpoch:  lastSeenEpoch,
		})
	}
	return result, nil
}

// CountIPSummaryNodeAccessLogs returns the number of distinct IPs matching filter.
func CountIPSummaryNodeAccessLogs(ctx context.Context, filter NodeAccessLogFilter) (int64, error) {
	conn, err := nodeAccessLogConn()
	if err != nil {
		return 0, err
	}
	clause, args := buildNodeAccessLogFilterClause(filter)
	tableName := nodeAccessLogTableName()
	sql := fmt.Sprintf(`
SELECT count() FROM (
	SELECT 1
	FROM %s
	WHERE %s AND remote_addr != ''
	GROUP BY remote_addr
)`, tableName, clause)
	var totalIPs uint64
	if err := conn.QueryRow(ctx, sql, args...).Scan(&totalIPs); err != nil {
		return 0, fmt.Errorf("count ip summary node access logs: %w", err)
	}
	return safeInt64Count(totalIPs), nil
}

// IPAggregatesForWAFNodeAccessLogs returns per-IP aggregates for WAF automatic rules.
func IPAggregatesForWAFNodeAccessLogs(ctx context.Context, filter NodeAccessLogFilter) ([]NodeAccessLogWAFIPAggregate, error) {
	conn, err := nodeAccessLogConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildNodeAccessLogFilterClause(filter)
	lastSeenExpr := nodeAccessLogEpochExpr()
	hostIsIPExpr := nodeAccessLogHostIsIPLiteralExpr()
	tableName := nodeAccessLogTableName()
	sql := fmt.Sprintf(`
SELECT
	remote_addr,
	count() AS request_count,
	countIf(status_code = 404) AS status_404_count,
	countIf(status_code >= 400 AND status_code < 500) AS client_error_count,
	countIf(status_code >= 500) AS server_error_count,
	countIf(%s) AS ip_host_count,
	max(%s) AS last_seen_epoch
FROM %s
WHERE %s AND remote_addr != ''
GROUP BY remote_addr`, hostIsIPExpr, lastSeenExpr, tableName, clause)
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("ip aggregates for waf node access logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	aggregates := make(map[string]*NodeAccessLogWAFIPAggregate)
	order := make([]string, 0)
	for rows.Next() {
		var (
			remoteAddr                                                                 string
			lastSeenEpoch                                                              int64
			requestCount, status404Count, clientErrorCount, serverErrorCount, ipHostCount uint64
		)
		if err := rows.Scan(&remoteAddr, &requestCount, &status404Count, &clientErrorCount, &serverErrorCount, &ipHostCount, &lastSeenEpoch); err != nil {
			return nil, fmt.Errorf("scan waf ip aggregate row: %w", err)
		}
		remoteAddr = strings.TrimSpace(remoteAddr)
		if remoteAddr == "" {
			continue
		}
		aggregates[remoteAddr] = &NodeAccessLogWAFIPAggregate{
			RemoteAddr:       remoteAddr,
			RequestCount:     safeInt64Count(requestCount),
			Status404Count:   safeInt64Count(status404Count),
			ClientErrorCount: safeInt64Count(clientErrorCount),
			ServerErrorCount: safeInt64Count(serverErrorCount),
			IPHostCount:      safeInt64Count(ipHostCount),
			LastSeenEpoch:    lastSeenEpoch,
			StatusCounts:     make(map[int]int64),
		}
		order = append(order, remoteAddr)
	}
	if err := mergeWAFIPStatusCodeCounts(ctx, filter, aggregates); err != nil {
		return nil, err
	}
	result := make([]NodeAccessLogWAFIPAggregate, 0, len(order))
	for _, remoteAddr := range order {
		if aggregate := aggregates[remoteAddr]; aggregate != nil {
			result = append(result, *aggregate)
		}
	}
	return result, nil
}

func mergeWAFIPStatusCodeCounts(ctx context.Context, filter NodeAccessLogFilter, aggregates map[string]*NodeAccessLogWAFIPAggregate) error {
	if len(aggregates) == 0 {
		return nil
	}
	conn, err := nodeAccessLogConn()
	if err != nil {
		return err
	}
	clause, args := buildNodeAccessLogFilterClause(filter)
	tableName := nodeAccessLogTableName()
	sql := fmt.Sprintf(`
SELECT
	remote_addr,
	status_code,
	count() AS status_count
FROM %s
WHERE %s AND remote_addr != ''
GROUP BY remote_addr, status_code`, tableName, clause)
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("waf ip status code counts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var (
			remoteAddr  string
			statusCode  int32
			statusCount uint64
		)
		if err := rows.Scan(&remoteAddr, &statusCode, &statusCount); err != nil {
			return fmt.Errorf("scan waf ip status code row: %w", err)
		}
		remoteAddr = strings.TrimSpace(remoteAddr)
		aggregate := aggregates[remoteAddr]
		if aggregate == nil {
			continue
		}
		if aggregate.StatusCounts == nil {
			aggregate.StatusCounts = make(map[int]int64)
		}
		aggregate.StatusCounts[int(statusCode)] = safeInt64Count(statusCount)
	}
	return nil
}

// IPTrendNodeAccessLogs returns IP trend bucket rows.
func IPTrendNodeAccessLogs(ctx context.Context, filter NodeAccessLogFilter, bucketSeconds int64) ([]NodeAccessLogIPTrend, error) {
	conn, err := nodeAccessLogConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildNodeAccessLogFilterClause(filter)
	bucketExpr := nodeAccessLogBucketEpochExpr(bucketSeconds)
	tableName := nodeAccessLogTableName()
	sql := fmt.Sprintf(`
SELECT
	%s AS bucket_epoch,
	count() AS request_count
FROM %s
WHERE %s
GROUP BY bucket_epoch
ORDER BY bucket_epoch ASC`, bucketExpr, tableName, clause)
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("ip trend node access logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []NodeAccessLogIPTrend
	for rows.Next() {
		var (
			bucketEpoch  int64
			requestCount uint64
		)
		if err := rows.Scan(&bucketEpoch, &requestCount); err != nil {
			return nil, fmt.Errorf("scan ip trend row: %w", err)
		}
		result = append(result, NodeAccessLogIPTrend{
			BucketEpoch:  bucketEpoch,
			RequestCount: safeInt64Count(requestCount),
		})
	}
	return result, nil
}
