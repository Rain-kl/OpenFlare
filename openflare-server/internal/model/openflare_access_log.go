// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"gorm.io/gorm"
)

const openFlareAccessLogTable = "of_node_access_logs"

type openFlareAccessLogBucketAggregateRow struct {
	BucketEpoch      int64 `gorm:"column:bucket_epoch"`
	RequestCount     int64 `gorm:"column:request_count"`
	SuccessCount     int64 `gorm:"column:success_count"`
	ClientErrorCount int64 `gorm:"column:client_error_count"`
	ServerErrorCount int64 `gorm:"column:server_error_count"`
}

type openFlareAccessLogBucketDimensionRow struct {
	BucketEpoch int64  `gorm:"column:bucket_epoch"`
	Value       string `gorm:"column:value"`
}

type openFlareAccessLogIPAggregateRow struct {
	RemoteAddr       string `gorm:"column:remote_addr"`
	RequestCount     int64  `gorm:"column:request_count"`
	SuccessCount     int64  `gorm:"column:success_count"`
	ClientErrorCount int64  `gorm:"column:client_error_count"`
	ServerErrorCount int64  `gorm:"column:server_error_count"`
	LastSeenEpoch    int64  `gorm:"column:last_seen_epoch"`
}

type openFlareAccessLogIPSummaryRow struct {
	RemoteAddr     string `gorm:"column:remote_addr"`
	TotalRequests  int64  `gorm:"column:total_requests"`
	RecentRequests int64  `gorm:"column:recent_requests"`
	LastSeenEpoch  int64  `gorm:"column:last_seen_epoch"`
}

type openFlareAccessLogIPTrendRow struct {
	BucketEpoch  int64 `gorm:"column:bucket_epoch"`
	RequestCount int64 `gorm:"column:request_count"`
}

// ListOpenFlareAccessLogsForWAFIPGroup lists access logs in a time window for automatic IP group rules.
func ListOpenFlareAccessLogsForWAFIPGroup(ctx context.Context, query OpenFlareAccessLogQuery) ([]*OpenFlareAccessLog, error) {
	return ListOpenFlareAccessLogs(ctx, query)
}

// ListOpenFlareAccessLogs lists access logs matching the query.
func ListOpenFlareAccessLogs(ctx context.Context, query OpenFlareAccessLogQuery) ([]*OpenFlareAccessLog, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	tx := applyOpenFlareAccessLogFilters(conn.Model(&OpenFlareAccessLog{}), query)
	tx = tx.Order(openFlareAccessLogOrderClause(query.SortBy, query.SortOrder))
	if query.PageSize > 0 {
		if query.Page < 0 {
			query.Page = 0
		}
		tx = tx.Offset(query.Page * query.PageSize).Limit(query.PageSize)
	}
	var rows []*OpenFlareAccessLog
	if err := tx.Find(&rows).Error; err != nil {
		if isMissingTableError(err) {
			return []*OpenFlareAccessLog{}, nil
		}
		return nil, err
	}
	return rows, nil
}

// CountOpenFlareAccessLogs counts access logs and distinct IPs matching the query.
func CountOpenFlareAccessLogs(ctx context.Context, query OpenFlareAccessLogQuery) (int64, int64, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return 0, 0, errors.New(errDatabaseNotInitialized)
	}
	totalRecords, err := countOpenFlareAccessLogRecords(conn, query)
	if err != nil {
		if isMissingTableError(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	totalIPs, err := countDistinctOpenFlareAccessLogIPs(conn, query)
	if err != nil {
		if isMissingTableError(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	return totalRecords, totalIPs, nil
}

// ListOpenFlareAccessLogRegionCounts returns region counts for access logs.
func ListOpenFlareAccessLogRegionCounts(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareAccessLogRegionCount, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	filter := OpenFlareAccessLogQuery{
		NodeID: nodeID,
		Since:  since,
	}
	clause, args := buildOpenFlareAccessLogFilterClause(filter)
	sql := fmt.Sprintf(`
SELECT TRIM(region) AS region, COUNT(*) AS count
FROM %s
WHERE %s AND TRIM(region) <> ''
GROUP BY TRIM(region)
ORDER BY count DESC, region ASC`, openFlareAccessLogTable, clause)
	if limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", limit)
	}
	var rows []*OpenFlareAccessLogRegionCount
	if err := conn.Raw(sql, args...).Scan(&rows).Error; err != nil {
		if isMissingTableError(err) {
			return []*OpenFlareAccessLogRegionCount{}, nil
		}
		return nil, err
	}
	return rows, nil
}

// ListOpenFlareAccessLogBuckets lists folded access log buckets.
func ListOpenFlareAccessLogBuckets(ctx context.Context, query OpenFlareAccessLogBucketQuery) ([]*OpenFlareAccessLogBucketRow, error) {
	rows, err := buildOpenFlareAccessLogBucketRows(ctx, query)
	if err != nil {
		return nil, err
	}
	start, end := openFlareAccessLogPaginateBounds(len(rows), query.Page, query.PageSize)
	if start >= len(rows) {
		return []*OpenFlareAccessLogBucketRow{}, nil
	}
	return rows[start:end], nil
}

// CountOpenFlareAccessLogBuckets counts folded access log buckets.
func CountOpenFlareAccessLogBuckets(ctx context.Context, query OpenFlareAccessLogBucketQuery) (int64, error) {
	rows, err := buildOpenFlareAccessLogBucketRows(ctx, query)
	if err != nil {
		return 0, err
	}
	return int64(len(rows)), nil
}

// ListOpenFlareAccessLogBucketIPs lists folded IP rows for a bucket window.
func ListOpenFlareAccessLogBucketIPs(ctx context.Context, query OpenFlareAccessLogBucketIPQuery) ([]*OpenFlareAccessLogBucketIPRow, error) {
	rows, err := buildOpenFlareAccessLogBucketIPRows(ctx, query)
	if err != nil {
		return nil, err
	}
	start, end := openFlareAccessLogPaginateBounds(len(rows), query.Page, query.PageSize)
	if start >= len(rows) {
		return []*OpenFlareAccessLogBucketIPRow{}, nil
	}
	return rows[start:end], nil
}

// CountOpenFlareAccessLogBucketIPs counts folded IP rows for a bucket window.
func CountOpenFlareAccessLogBucketIPs(ctx context.Context, query OpenFlareAccessLogBucketIPQuery) (int64, error) {
	rows, err := buildOpenFlareAccessLogBucketIPRows(ctx, query)
	if err != nil {
		return 0, err
	}
	return int64(len(rows)), nil
}

// ListOpenFlareAccessLogIPSummaries lists IP summaries.
func ListOpenFlareAccessLogIPSummaries(ctx context.Context, query OpenFlareAccessLogIPSummaryQuery, recentSince time.Time) ([]*OpenFlareAccessLogIPSummaryRow, error) {
	rows, err := buildOpenFlareAccessLogIPSummaryRows(ctx, query, recentSince)
	if err != nil {
		return nil, err
	}
	start, end := openFlareAccessLogPaginateBounds(len(rows), query.Page, query.PageSize)
	if start >= len(rows) {
		return []*OpenFlareAccessLogIPSummaryRow{}, nil
	}
	return rows[start:end], nil
}

// CountOpenFlareAccessLogIPSummaries counts IP summaries.
func CountOpenFlareAccessLogIPSummaries(ctx context.Context, query OpenFlareAccessLogIPSummaryQuery) (int64, error) {
	rows, err := buildOpenFlareAccessLogIPSummaryRows(ctx, query, time.Time{})
	if err != nil {
		return 0, err
	}
	return int64(len(rows)), nil
}

// ListOpenFlareAccessLogIPTrend lists IP trend points.
func ListOpenFlareAccessLogIPTrend(ctx context.Context, query OpenFlareAccessLogIPTrendQuery) ([]*OpenFlareAccessLogIPTrendRow, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	remoteAddr := strings.TrimSpace(query.RemoteAddr)
	if remoteAddr == "" {
		return []*OpenFlareAccessLogIPTrendRow{}, nil
	}
	filter := OpenFlareAccessLogQuery{
		NodeID:     query.NodeID,
		RemoteAddr: remoteAddr,
		Host:       query.Host,
		Since:      query.Since,
	}
	clause, args := buildOpenFlareAccessLogFilterClause(filter)
	bucketSeconds := int64(query.BucketMinutes * 60)
	if bucketSeconds <= 0 {
		bucketSeconds = 1800
	}
	bucketExpr := openFlareAccessLogBucketEpochExpr(openFlareAccessLogDialect(conn), bucketSeconds)
	queryClause := combineOpenFlareAccessLogSQLClauses(clause, "TRIM(remote_addr) = ?")
	queryArgs := append(append([]any{}, args...), remoteAddr)
	sql := fmt.Sprintf(`
SELECT
	%s AS bucket_epoch,
	COUNT(*) AS request_count
FROM %s
WHERE %s
GROUP BY bucket_epoch
ORDER BY bucket_epoch ASC`, bucketExpr, openFlareAccessLogTable, queryClause)
	var rows []*OpenFlareAccessLogIPTrendRow
	if err := conn.Raw(sql, queryArgs...).Scan(&rows).Error; err != nil {
		if isMissingTableError(err) {
			return []*OpenFlareAccessLogIPTrendRow{}, nil
		}
		return nil, err
	}
	return rows, nil
}

// DeleteAllOpenFlareAccessLogs deletes all access logs.
func DeleteAllOpenFlareAccessLogs(ctx context.Context) (int64, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return 0, errors.New(errDatabaseNotInitialized)
	}
	result := conn.Where("1 = 1").Delete(&OpenFlareAccessLog{})
	if result.Error != nil {
		if isMissingTableError(result.Error) {
			return 0, nil
		}
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// DeleteOpenFlareAccessLogsBefore deletes access logs older than cutoff.
func DeleteOpenFlareAccessLogsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return 0, errors.New(errDatabaseNotInitialized)
	}
	result := conn.Where("logged_at < ?", cutoff).Delete(&OpenFlareAccessLog{})
	if result.Error != nil {
		if isMissingTableError(result.Error) {
			return 0, nil
		}
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

func buildOpenFlareAccessLogBucketRows(ctx context.Context, query OpenFlareAccessLogBucketQuery) ([]*OpenFlareAccessLogBucketRow, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	filter := openFlareAccessLogQueryFromBucket(query)
	clause, args := buildOpenFlareAccessLogFilterClause(filter)
	bucketSeconds := int64(query.FoldMinutes * 60)
	if bucketSeconds <= 0 {
		bucketSeconds = 180
	}
	bucketExpr := openFlareAccessLogBucketEpochExpr(openFlareAccessLogDialect(conn), bucketSeconds)

	type bucketAccumulator struct {
		requestCount     int64
		uniqueIPs        map[string]struct{}
		uniqueHosts      map[string]struct{}
		successCount     int64
		clientErrorCount int64
		serverErrorCount int64
	}
	accumulators := make(map[int64]*bucketAccumulator)

	var partials []openFlareAccessLogBucketAggregateRow
	sql := fmt.Sprintf(`
SELECT
	%s AS bucket_epoch,
	COUNT(*) AS request_count,
	SUM(CASE WHEN status_code < 400 THEN 1 ELSE 0 END) AS success_count,
	SUM(CASE WHEN status_code >= 400 AND status_code < 500 THEN 1 ELSE 0 END) AS client_error_count,
	SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END) AS server_error_count
FROM %s
WHERE %s
GROUP BY bucket_epoch`, bucketExpr, openFlareAccessLogTable, clause)
	if err := conn.Raw(sql, args...).Scan(&partials).Error; err != nil {
		if isMissingTableError(err) {
			return []*OpenFlareAccessLogBucketRow{}, nil
		}
		return nil, err
	}
	for _, partial := range partials {
		accumulator := accumulators[partial.BucketEpoch]
		if accumulator == nil {
			accumulator = &bucketAccumulator{
				uniqueIPs:   make(map[string]struct{}),
				uniqueHosts: make(map[string]struct{}),
			}
			accumulators[partial.BucketEpoch] = accumulator
		}
		accumulator.requestCount += partial.RequestCount
		accumulator.successCount += partial.SuccessCount
		accumulator.clientErrorCount += partial.ClientErrorCount
		accumulator.serverErrorCount += partial.ServerErrorCount
	}

	for _, column := range []string{"remote_addr", "host"} {
		dimensions, err := queryOpenFlareAccessLogBucketDimensionRows(conn, clause, args, column, bucketExpr)
		if err != nil {
			return nil, err
		}
		for _, item := range dimensions {
			accumulator := accumulators[item.BucketEpoch]
			if accumulator == nil {
				accumulator = &bucketAccumulator{
					uniqueIPs:   make(map[string]struct{}),
					uniqueHosts: make(map[string]struct{}),
				}
				accumulators[item.BucketEpoch] = accumulator
			}
			trimmed := strings.TrimSpace(item.Value)
			if trimmed == "" {
				continue
			}
			switch column {
			case "remote_addr":
				accumulator.uniqueIPs[trimmed] = struct{}{}
			case "host":
				accumulator.uniqueHosts[trimmed] = struct{}{}
			}
		}
	}

	rows := make([]*OpenFlareAccessLogBucketRow, 0, len(accumulators))
	for bucketEpoch, accumulator := range accumulators {
		rows = append(rows, &OpenFlareAccessLogBucketRow{
			BucketEpoch:      bucketEpoch,
			RequestCount:     accumulator.requestCount,
			UniqueIPCount:    int64(len(accumulator.uniqueIPs)),
			UniqueHostCount:  int64(len(accumulator.uniqueHosts)),
			SuccessCount:     accumulator.successCount,
			ClientErrorCount: accumulator.clientErrorCount,
			ServerErrorCount: accumulator.serverErrorCount,
		})
	}
	sortOpenFlareAccessLogBucketRows(rows, query.SortBy, query.SortOrder)
	return rows, nil
}

func queryOpenFlareAccessLogBucketDimensionRows(conn *gorm.DB, clause string, args []any, column string, bucketExpr string) ([]openFlareAccessLogBucketDimensionRow, error) {
	var rows []openFlareAccessLogBucketDimensionRow
	sql := fmt.Sprintf(`
SELECT
	%s AS bucket_epoch,
	TRIM(%s) AS value
FROM %s
WHERE %s AND TRIM(%s) <> ''
GROUP BY bucket_epoch, TRIM(%s)`, bucketExpr, column, openFlareAccessLogTable, clause, column, column)
	if err := conn.Raw(sql, args...).Scan(&rows).Error; err != nil {
		if isMissingTableError(err) {
			return []openFlareAccessLogBucketDimensionRow{}, nil
		}
		return nil, err
	}
	return rows, nil
}

func buildOpenFlareAccessLogBucketIPRows(ctx context.Context, query OpenFlareAccessLogBucketIPQuery) ([]*OpenFlareAccessLogBucketIPRow, error) {
	if query.BucketStartedAt.IsZero() {
		return []*OpenFlareAccessLogBucketIPRow{}, nil
	}
	foldMinutes := query.FoldMinutes
	if foldMinutes <= 0 {
		foldMinutes = 3
	}
	bucketStartedAt := query.BucketStartedAt.UTC()
	filter := OpenFlareAccessLogQuery{
		NodeID:     query.NodeID,
		RemoteAddr: query.RemoteAddr,
		Host:       query.Host,
		Path:       query.Path,
		Since:      bucketStartedAt,
		Until:      bucketStartedAt.Add(time.Duration(foldMinutes) * time.Minute),
	}
	rows, err := queryOpenFlareAccessLogIPAggregateRows(ctx, filter, false)
	if err != nil {
		return nil, err
	}
	sortOpenFlareAccessLogBucketIPRows(rows, query.SortBy, query.SortOrder)
	return rows, nil
}

func buildOpenFlareAccessLogIPSummaryRows(ctx context.Context, query OpenFlareAccessLogIPSummaryQuery, recentSince time.Time) ([]*OpenFlareAccessLogIPSummaryRow, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	filter := OpenFlareAccessLogQuery{
		NodeID:     query.NodeID,
		RemoteAddr: query.RemoteAddr,
		Host:       query.Host,
		Since:      query.Since,
	}
	clause, args := buildOpenFlareAccessLogFilterClause(filter)
	lastSeenExpr := openFlareAccessLogEpochExpr(openFlareAccessLogDialect(conn))
	recentClause := "0"
	queryArgs := make([]any, 0, len(args)+1)
	if !recentSince.IsZero() {
		recentClause = "CASE WHEN logged_at >= ? THEN 1 ELSE 0 END"
		queryArgs = append(queryArgs, recentSince)
	}
	queryArgs = append(queryArgs, args...)
	sql := fmt.Sprintf(`
SELECT
	TRIM(remote_addr) AS remote_addr,
	COUNT(*) AS total_requests,
	SUM(%s) AS recent_requests,
	MAX(%s) AS last_seen_epoch
FROM %s
WHERE %s AND TRIM(remote_addr) <> ''
GROUP BY TRIM(remote_addr)`, recentClause, lastSeenExpr, openFlareAccessLogTable, clause)
	var partials []openFlareAccessLogIPSummaryRow
	if err := conn.Raw(sql, queryArgs...).Scan(&partials).Error; err != nil {
		if isMissingTableError(err) {
			return []*OpenFlareAccessLogIPSummaryRow{}, nil
		}
		return nil, err
	}
	rows := make([]*OpenFlareAccessLogIPSummaryRow, 0, len(partials))
	for _, partial := range partials {
		remoteAddr := strings.TrimSpace(partial.RemoteAddr)
		if remoteAddr == "" {
			continue
		}
		rows = append(rows, &OpenFlareAccessLogIPSummaryRow{
			RemoteAddr:     remoteAddr,
			TotalRequests:  partial.TotalRequests,
			RecentRequests: partial.RecentRequests,
			LastSeenEpoch:  partial.LastSeenEpoch,
		})
	}
	sortOpenFlareAccessLogIPSummaryRows(rows, query.SortBy, query.SortOrder)
	return rows, nil
}

func queryOpenFlareAccessLogIPAggregateRows(ctx context.Context, filter OpenFlareAccessLogQuery, exactRemoteAddr bool) ([]*OpenFlareAccessLogBucketIPRow, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	clause, args := buildOpenFlareAccessLogFilterClause(filter)
	lastSeenExpr := openFlareAccessLogEpochExpr(openFlareAccessLogDialect(conn))
	queryClause := clause
	queryArgs := append([]any{}, args...)
	if exactRemoteAddr {
		trimmed := strings.TrimSpace(filter.RemoteAddr)
		if trimmed == "" {
			return []*OpenFlareAccessLogBucketIPRow{}, nil
		}
		queryClause = combineOpenFlareAccessLogSQLClauses(queryClause, "TRIM(remote_addr) = ?")
		queryArgs = append(queryArgs, trimmed)
	}
	sql := fmt.Sprintf(`
SELECT
	TRIM(remote_addr) AS remote_addr,
	COUNT(*) AS request_count,
	SUM(CASE WHEN status_code < 400 THEN 1 ELSE 0 END) AS success_count,
	SUM(CASE WHEN status_code >= 400 AND status_code < 500 THEN 1 ELSE 0 END) AS client_error_count,
	SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END) AS server_error_count,
	MAX(%s) AS last_seen_epoch
FROM %s
WHERE %s AND TRIM(remote_addr) <> ''
GROUP BY TRIM(remote_addr)`, lastSeenExpr, openFlareAccessLogTable, queryClause)
	var partials []openFlareAccessLogIPAggregateRow
	if err := conn.Raw(sql, queryArgs...).Scan(&partials).Error; err != nil {
		if isMissingTableError(err) {
			return []*OpenFlareAccessLogBucketIPRow{}, nil
		}
		return nil, err
	}
	rows := make([]*OpenFlareAccessLogBucketIPRow, 0, len(partials))
	for _, partial := range partials {
		remoteAddr := strings.TrimSpace(partial.RemoteAddr)
		if remoteAddr == "" {
			continue
		}
		rows = append(rows, &OpenFlareAccessLogBucketIPRow{
			RemoteAddr:       remoteAddr,
			RequestCount:     partial.RequestCount,
			SuccessCount:     partial.SuccessCount,
			ClientErrorCount: partial.ClientErrorCount,
			ServerErrorCount: partial.ServerErrorCount,
			LastSeenEpoch:    partial.LastSeenEpoch,
		})
	}
	return rows, nil
}

func openFlareAccessLogQueryFromBucket(query OpenFlareAccessLogBucketQuery) OpenFlareAccessLogQuery {
	return OpenFlareAccessLogQuery{
		NodeID:     query.NodeID,
		RemoteAddr: query.RemoteAddr,
		Host:       query.Host,
		Path:       query.Path,
		Since:      query.Since,
	}
}

func buildOpenFlareAccessLogFilterClause(query OpenFlareAccessLogQuery) (string, []any) {
	parts := make([]string, 0, 6)
	args := make([]any, 0, 6)
	if trimmed := strings.TrimSpace(query.NodeID); trimmed != "" {
		parts = append(parts, "node_id = ?")
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(query.RemoteAddr); trimmed != "" {
		parts = append(parts, "remote_addr LIKE ?")
		args = append(args, trimmed+"%")
	}
	if trimmed := strings.TrimSpace(query.Host); trimmed != "" {
		parts = append(parts, "host LIKE ?")
		args = append(args, trimmed+"%")
	}
	if trimmed := strings.TrimSpace(query.Path); trimmed != "" {
		parts = append(parts, "path LIKE ?")
		args = append(args, trimmed+"%")
	}
	if !query.Since.IsZero() {
		parts = append(parts, "logged_at >= ?")
		args = append(args, query.Since)
	}
	if !query.Until.IsZero() {
		parts = append(parts, "logged_at < ?")
		args = append(args, query.Until)
	}
	if len(parts) == 0 {
		return "TRUE", nil
	}
	return strings.Join(parts, " AND "), args
}

func applyOpenFlareAccessLogFilters(tx *gorm.DB, query OpenFlareAccessLogQuery) *gorm.DB {
	clause, args := buildOpenFlareAccessLogFilterClause(query)
	if clause == "TRUE" {
		return tx
	}
	return tx.Where(clause, args...)
}

func countOpenFlareAccessLogRecords(conn *gorm.DB, query OpenFlareAccessLogQuery) (int64, error) {
	var count int64
	if err := applyOpenFlareAccessLogFilters(conn.Model(&OpenFlareAccessLog{}), query).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func countDistinctOpenFlareAccessLogIPs(conn *gorm.DB, query OpenFlareAccessLogQuery) (int64, error) {
	clause, args := buildOpenFlareAccessLogFilterClause(query)
	sql := fmt.Sprintf(`
SELECT COUNT(*) FROM (
	SELECT TRIM(remote_addr) AS remote_addr
	FROM %s
	WHERE %s AND remote_addr <> ''
	GROUP BY TRIM(remote_addr)
) AS ips`, openFlareAccessLogTable, clause)
	var total int64
	if err := conn.Raw(sql, args...).Scan(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func openFlareAccessLogDialect(conn *gorm.DB) string {
	if conn == nil || conn.Dialector == nil {
		return "sqlite"
	}
	switch conn.Dialector.Name() {
	case "postgres":
		return "postgres"
	default:
		return "sqlite"
	}
}

func openFlareAccessLogBucketEpochExpr(dialect string, bucketSeconds int64) string {
	switch dialect {
	case "postgres":
		return fmt.Sprintf("FLOOR(EXTRACT(EPOCH FROM logged_at AT TIME ZONE 'UTC') / %d) * %d", bucketSeconds, bucketSeconds)
	default:
		return fmt.Sprintf("(CAST(strftime('%%s', logged_at) AS INTEGER) / %d) * %d", bucketSeconds, bucketSeconds)
	}
}

func openFlareAccessLogEpochExpr(dialect string) string {
	switch dialect {
	case "postgres":
		return "FLOOR(EXTRACT(EPOCH FROM logged_at AT TIME ZONE 'UTC'))::bigint"
	default:
		return "CAST((julianday(logged_at) - 2440587.5) * 86400 AS INTEGER)"
	}
}

func combineOpenFlareAccessLogSQLClauses(left string, right string) string {
	if strings.TrimSpace(left) == "" || left == "TRUE" {
		return right
	}
	return left + " AND " + right
}

func openFlareAccessLogOrderClause(sortBy string, sortOrder string) string {
	direction := "DESC"
	if openFlareAccessLogNormalizeSortOrder(sortOrder) == "asc" {
		direction = "ASC"
	}
	column := "logged_at"
	switch strings.TrimSpace(sortBy) {
	case "status_code":
		column = "status_code"
	case "remote_addr":
		column = "remote_addr"
	case "host":
		column = "host"
	case "path":
		column = "path"
	}
	if column == "logged_at" {
		return column + " " + direction + ", id " + direction
	}
	return column + " " + direction + ", logged_at " + direction + ", id " + direction
}

func sortOpenFlareAccessLogBucketIPRows(items []*OpenFlareAccessLogBucketIPRow, sortBy string, sortOrder string) {
	desc := openFlareAccessLogNormalizeSortOrder(sortOrder) != "asc"
	sort.Slice(items, func(i int, j int) bool {
		left := items[i]
		right := items[j]
		if left == nil || right == nil {
			return left != nil
		}
		var compare int
		switch strings.TrimSpace(sortBy) {
		case "last_seen_at":
			compare = openFlareAccessLogCompareInt64(left.LastSeenEpoch, right.LastSeenEpoch)
		case "remote_addr":
			compare = strings.Compare(left.RemoteAddr, right.RemoteAddr)
		default:
			compare = openFlareAccessLogCompareInt64(left.RequestCount, right.RequestCount)
		}
		if compare == 0 {
			compare = openFlareAccessLogCompareInt64(left.LastSeenEpoch, right.LastSeenEpoch)
		}
		if compare == 0 {
			compare = strings.Compare(left.RemoteAddr, right.RemoteAddr)
		}
		if desc {
			return compare > 0
		}
		return compare < 0
	})
}

func sortOpenFlareAccessLogBucketRows(items []*OpenFlareAccessLogBucketRow, sortBy string, sortOrder string) {
	desc := openFlareAccessLogNormalizeSortOrder(sortOrder) != "asc"
	sort.Slice(items, func(i int, j int) bool {
		left := items[i]
		right := items[j]
		if left == nil || right == nil {
			return left != nil
		}
		var compare int
		switch strings.TrimSpace(sortBy) {
		case "request_count":
			compare = openFlareAccessLogCompareInt64(left.RequestCount, right.RequestCount)
		default:
			compare = openFlareAccessLogCompareInt64(left.BucketEpoch, right.BucketEpoch)
		}
		if compare == 0 {
			compare = openFlareAccessLogCompareInt64(left.BucketEpoch, right.BucketEpoch)
		}
		if desc {
			return compare > 0
		}
		return compare < 0
	})
}

func sortOpenFlareAccessLogIPSummaryRows(items []*OpenFlareAccessLogIPSummaryRow, sortBy string, sortOrder string) {
	desc := openFlareAccessLogNormalizeSortOrder(sortOrder) != "asc"
	sort.Slice(items, func(i int, j int) bool {
		left := items[i]
		right := items[j]
		if left == nil || right == nil {
			return left != nil
		}
		var compare int
		switch strings.TrimSpace(sortBy) {
		case "recent_requests":
			compare = openFlareAccessLogCompareInt64(left.RecentRequests, right.RecentRequests)
		case "last_seen_at":
			compare = openFlareAccessLogCompareInt64(left.LastSeenEpoch, right.LastSeenEpoch)
		case "remote_addr":
			compare = strings.Compare(left.RemoteAddr, right.RemoteAddr)
		default:
			compare = openFlareAccessLogCompareInt64(left.TotalRequests, right.TotalRequests)
		}
		if compare == 0 {
			compare = openFlareAccessLogCompareInt64(left.LastSeenEpoch, right.LastSeenEpoch)
		}
		if compare == 0 {
			compare = strings.Compare(left.RemoteAddr, right.RemoteAddr)
		}
		if desc {
			return compare > 0
		}
		return compare < 0
	})
}

func openFlareAccessLogPaginateBounds(total int, page int, pageSize int) (int, int) {
	if page < 0 {
		page = 0
	}
	if pageSize <= 0 {
		return 0, total
	}
	start := page * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return start, end
}

func openFlareAccessLogNormalizeSortOrder(sortOrder string) string {
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		return "asc"
	}
	return "desc"
}

func openFlareAccessLogCompareInt64(left int64, right int64) int {
	switch {
	case left > right:
		return 1
	case left < right:
		return -1
	default:
		return 0
	}
}
