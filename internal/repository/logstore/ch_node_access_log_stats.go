// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

func (s *clickhouseLogStore) BucketAggregates(ctx context.Context, query model.OpenFlareAccessLogQuery, bucketSeconds int64) ([]analyticsmodel.NodeAccessLogBucketAggregate, error) {
	conn, err := chConn()
	if err != nil {
		return nil, err
	}
	filter := toNodeAccessLogFilter(query)
	clause, args := buildCHNodeAccessLogFilterClause(filter)
	bucketExpr := chNodeAccessLogBucketEpochExpr(bucketSeconds)
	sql := fmt.Sprintf(`
SELECT
	%s AS bucket_epoch,
	count() AS request_count,
	countIf(status_code < 400) AS success_count,
	countIf(status_code >= 400 AND status_code < 500) AS client_error_count,
	countIf(status_code >= 500) AS server_error_count,
	countIf(status_code >= 200 AND status_code < 300) AS status_2xx_count,
	countIf(status_code >= 400 AND status_code < 500) AS status_4xx_count,
	countIf(status_code >= 500) AS status_5xx_count,
	uniqExactIf(remote_addr, remote_addr != '') AS unique_ip_count,
	uniqExactIf(host, host != '') AS unique_host_count,
	sum(bytes_sent) AS bytes_sent,
	sum(request_length) AS request_length
FROM %s
WHERE %s
GROUP BY bucket_epoch
ORDER BY %s`, bucketExpr, chNodeAccessLogTableName(), clause, chNodeAccessLogBucketOrderClause(filter.SortBy, filter.SortOrder))
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

	var result []analyticsmodel.NodeAccessLogBucketAggregate
	for rows.Next() {
		var (
			bucketEpoch                                                                                                                                                              int64
			requestCount, successCount, clientErrorCount, serverErrorCount, status2xxCount, status4xxCount, status5xxCount, uniqueIPCount, uniqueHostCount, bytesSent, requestLength uint64
		)
		if err := rows.Scan(&bucketEpoch, &requestCount, &successCount, &clientErrorCount, &serverErrorCount, &status2xxCount, &status4xxCount, &status5xxCount, &uniqueIPCount, &uniqueHostCount, &bytesSent, &requestLength); err != nil {
			return nil, fmt.Errorf("scan bucket aggregate row: %w", err)
		}
		result = append(result, analyticsmodel.NodeAccessLogBucketAggregate{
			BucketEpoch:      bucketEpoch,
			RequestCount:     safeInt64Count(requestCount),
			SuccessCount:     safeInt64Count(successCount),
			ClientErrorCount: safeInt64Count(clientErrorCount),
			ServerErrorCount: safeInt64Count(serverErrorCount),
			Status2xxCount:   safeInt64Count(status2xxCount),
			Status4xxCount:   safeInt64Count(status4xxCount),
			Status5xxCount:   safeInt64Count(status5xxCount),
			UniqueIPCount:    safeInt64Count(uniqueIPCount),
			UniqueHostCount:  safeInt64Count(uniqueHostCount),
			BytesSent:        safeInt64Count(bytesSent),
			RequestLength:    safeInt64Count(requestLength),
		})
	}
	return result, nil
}

func (s *clickhouseLogStore) CountBuckets(ctx context.Context, query model.OpenFlareAccessLogQuery, bucketSeconds int64) (int64, error) {
	conn, err := chConn()
	if err != nil {
		return 0, err
	}
	clause, args := buildCHNodeAccessLogFilterClause(toNodeAccessLogFilter(query))
	bucketExpr := chNodeAccessLogBucketEpochExpr(bucketSeconds)
	sql := fmt.Sprintf(`
SELECT count() FROM (
	SELECT 1
	FROM %s
	WHERE %s
	GROUP BY %s
)`, chNodeAccessLogTableName(), clause, bucketExpr)
	var totalBuckets uint64
	if err := conn.QueryRow(ctx, sql, args...).Scan(&totalBuckets); err != nil {
		return 0, fmt.Errorf("count bucket aggregates node access logs: %w", err)
	}
	return safeInt64Count(totalBuckets), nil
}

func (s *clickhouseLogStore) BucketDimensions(ctx context.Context, query model.OpenFlareAccessLogQuery, column string, bucketSeconds int64) ([]analyticsmodel.NodeAccessLogBucketDimension, error) {
	conn, err := chConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildCHNodeAccessLogFilterClause(toNodeAccessLogFilter(query))
	bucketExpr := chNodeAccessLogBucketEpochExpr(bucketSeconds)
	sql := fmt.Sprintf(`
SELECT
	%s AS bucket_epoch,
	trim(%s) AS value
FROM %s
WHERE %s AND trim(%s) != ''
GROUP BY bucket_epoch, trim(%s)`, bucketExpr, column, chNodeAccessLogTableName(), clause, column, column)
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("bucket dimensions node access logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []analyticsmodel.NodeAccessLogBucketDimension
	for rows.Next() {
		var item analyticsmodel.NodeAccessLogBucketDimension
		if err := rows.Scan(&item.BucketEpoch, &item.Value); err != nil {
			return nil, fmt.Errorf("scan bucket dimension row: %w", err)
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *clickhouseLogStore) IPAggregates(ctx context.Context, query model.OpenFlareAccessLogQuery, exactRemoteAddr bool) ([]analyticsmodel.NodeAccessLogIPAggregate, error) {
	conn, err := chConn()
	if err != nil {
		return nil, err
	}
	filter := toNodeAccessLogFilter(query)
	clause, args := buildCHNodeAccessLogFilterClause(filter)
	queryClause := clause
	queryArgs := append([]any{}, args...)
	if exactRemoteAddr {
		trimmed := strings.TrimSpace(filter.RemoteAddr)
		if trimmed == "" {
			return []analyticsmodel.NodeAccessLogIPAggregate{}, nil
		}
		queryClause = combineCHNodeAccessLogSQLClauses(queryClause, "remote_addr = ?")
		queryArgs = append(queryArgs, trimmed)
	}
	lastSeenExpr := chNodeAccessLogEpochExpr()
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
GROUP BY remote_addr`, lastSeenExpr, chNodeAccessLogTableName(), queryClause)
	rows, err := conn.Query(ctx, sql, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("ip aggregates node access logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []analyticsmodel.NodeAccessLogIPAggregate
	for rows.Next() {
		var (
			remoteAddr                                                     string
			lastSeenEpoch                                                  int64
			requestCount, successCount, clientErrorCount, serverErrorCount uint64
		)
		if err := rows.Scan(&remoteAddr, &requestCount, &successCount, &clientErrorCount, &serverErrorCount, &lastSeenEpoch); err != nil {
			return nil, fmt.Errorf("scan ip aggregate row: %w", err)
		}
		result = append(result, analyticsmodel.NodeAccessLogIPAggregate{
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

func (s *clickhouseLogStore) IPSummaries(ctx context.Context, query model.OpenFlareAccessLogQuery, _ time.Time) ([]analyticsmodel.NodeAccessLogIPSummary, error) {
	conn, err := chConn()
	if err != nil {
		return nil, err
	}
	filter := toNodeAccessLogFilter(query)
	clause, args := buildCHNodeAccessLogFilterClause(filter)
	lastSeenExpr := chNodeAccessLogEpochExpr()
	queryArgs := append([]any{}, args...)
	tableName := chNodeAccessLogTableName()
	// Outer query allows ORDER BY success_ratio without repeating countIf.
	innerSQL := fmt.Sprintf(`
SELECT
	remote_addr,
	argMax(region, logged_at) AS region,
	count() AS total_requests,
	countIf(status_code >= 200 AND status_code < 300) AS success_2xx_count,
	sum(request_length) AS request_length,
	sum(bytes_sent) AS bytes_sent,
	max(%s) AS last_seen_epoch
FROM %s
WHERE %s AND remote_addr != ''
GROUP BY remote_addr`, lastSeenExpr, tableName, clause)
	sql := fmt.Sprintf(`
SELECT
	remote_addr,
	region,
	total_requests,
	success_2xx_count,
	if(total_requests = 0, 0., toFloat64(success_2xx_count) / toFloat64(total_requests)) AS success_ratio,
	request_length,
	bytes_sent,
	last_seen_epoch
FROM (%s)
ORDER BY %s`, innerSQL, chNodeAccessLogIPSummaryOrderClause(filter.SortBy, filter.SortOrder))
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

	var result []analyticsmodel.NodeAccessLogIPSummary
	for rows.Next() {
		var (
			remoteAddr, region                              string
			lastSeenEpoch                                   int64
			successRatio                                    float64
			totalRequests, success2xx, bytesReceived, bytes uint64
		)
		if err := rows.Scan(
			&remoteAddr,
			&region,
			&totalRequests,
			&success2xx,
			&successRatio,
			&bytesReceived,
			&bytes,
			&lastSeenEpoch,
		); err != nil {
			return nil, fmt.Errorf("scan ip summary row: %w", err)
		}
		result = append(result, analyticsmodel.NodeAccessLogIPSummary{
			RemoteAddr:      remoteAddr,
			Region:          region,
			TotalRequests:   safeInt64Count(totalRequests),
			Success2xxCount: safeInt64Count(success2xx),
			SuccessRatio:    successRatio,
			BytesReceived:   safeInt64Count(bytesReceived),
			BytesSent:       safeInt64Count(bytes),
			RecentRequests:  0,
			LastSeenEpoch:   lastSeenEpoch,
		})
	}
	return result, nil
}

func (s *clickhouseLogStore) CountIPSummaries(ctx context.Context, query model.OpenFlareAccessLogQuery) (int64, error) {
	conn, err := chConn()
	if err != nil {
		return 0, err
	}
	clause, args := buildCHNodeAccessLogFilterClause(toNodeAccessLogFilter(query))
	sql := fmt.Sprintf(`
SELECT count() FROM (
	SELECT 1
	FROM %s
	WHERE %s AND remote_addr != ''
	GROUP BY remote_addr
)`, chNodeAccessLogTableName(), clause)
	var totalIPs uint64
	if err := conn.QueryRow(ctx, sql, args...).Scan(&totalIPs); err != nil {
		return 0, fmt.Errorf("count ip summary node access logs: %w", err)
	}
	return safeInt64Count(totalIPs), nil
}

func (s *clickhouseLogStore) WAFIPAggregates(ctx context.Context, query model.OpenFlareAccessLogQuery) ([]analyticsmodel.NodeAccessLogWAFIPAggregate, error) {
	conn, err := chConn()
	if err != nil {
		return nil, err
	}
	filter := toNodeAccessLogFilter(query)
	clause, args := buildCHNodeAccessLogFilterClause(filter)
	lastSeenExpr := chNodeAccessLogEpochExpr()
	hostIsIPExpr := chNodeAccessLogHostIsIPLiteralExpr()
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
GROUP BY remote_addr`, hostIsIPExpr, lastSeenExpr, chNodeAccessLogTableName(), clause)
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("ip aggregates for waf node access logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	aggregates := make(map[string]*analyticsmodel.NodeAccessLogWAFIPAggregate)
	order := make([]string, 0)
	for rows.Next() {
		var (
			remoteAddr                                                                    string
			lastSeenEpoch                                                                 int64
			requestCount, status404Count, clientErrorCount, serverErrorCount, ipHostCount uint64
		)
		if err := rows.Scan(&remoteAddr, &requestCount, &status404Count, &clientErrorCount, &serverErrorCount, &ipHostCount, &lastSeenEpoch); err != nil {
			return nil, fmt.Errorf("scan waf ip aggregate row: %w", err)
		}
		remoteAddr = strings.TrimSpace(remoteAddr)
		if remoteAddr == "" {
			continue
		}
		aggregates[remoteAddr] = &analyticsmodel.NodeAccessLogWAFIPAggregate{
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
	if err := mergeCHWAFIPStatusCodeCounts(ctx, filter, aggregates); err != nil {
		return nil, err
	}
	result := make([]analyticsmodel.NodeAccessLogWAFIPAggregate, 0, len(order))
	for _, remoteAddr := range order {
		if aggregate := aggregates[remoteAddr]; aggregate != nil {
			result = append(result, *aggregate)
		}
	}
	return result, nil
}

func mergeCHWAFIPStatusCodeCounts(ctx context.Context, filter analyticsmodel.NodeAccessLogFilter, aggregates map[string]*analyticsmodel.NodeAccessLogWAFIPAggregate) error {
	if len(aggregates) == 0 {
		return nil
	}
	conn, err := chConn()
	if err != nil {
		return err
	}
	clause, args := buildCHNodeAccessLogFilterClause(filter)
	sql := fmt.Sprintf(`
SELECT
	remote_addr,
	status_code,
	count() AS status_count
FROM %s
WHERE %s AND remote_addr != ''
GROUP BY remote_addr, status_code`, chNodeAccessLogTableName(), clause)
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

func (s *clickhouseLogStore) IPTrend(ctx context.Context, query model.OpenFlareAccessLogQuery, bucketSeconds int64) ([]analyticsmodel.NodeAccessLogIPTrend, error) {
	conn, err := chConn()
	if err != nil {
		return nil, err
	}
	clause, args := buildCHNodeAccessLogFilterClause(toNodeAccessLogFilter(query))
	bucketExpr := chNodeAccessLogBucketEpochExpr(bucketSeconds)
	sql := fmt.Sprintf(`
SELECT
	%s AS bucket_epoch,
	count() AS request_count
FROM %s
WHERE %s
GROUP BY bucket_epoch
ORDER BY bucket_epoch ASC`, bucketExpr, chNodeAccessLogTableName(), clause)
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("ip trend node access logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []analyticsmodel.NodeAccessLogIPTrend
	for rows.Next() {
		var (
			bucketEpoch  int64
			requestCount uint64
		)
		if err := rows.Scan(&bucketEpoch, &requestCount); err != nil {
			return nil, fmt.Errorf("scan ip trend row: %w", err)
		}
		result = append(result, analyticsmodel.NodeAccessLogIPTrend{
			BucketEpoch:  bucketEpoch,
			RequestCount: safeInt64Count(requestCount),
		})
	}
	return result, nil
}
