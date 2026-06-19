// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"sort"
	"strings"
	"time"
)

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

// InsertOpenFlareAccessLogsBatch inserts access log rows into ClickHouse.
func InsertOpenFlareAccessLogsBatch(ctx context.Context, records []*OpenFlareAccessLog) error {
	return currentAccessLogStore().InsertBatch(ctx, records)
}

// ListOpenFlareAccessLogs lists access logs matching the query.
func ListOpenFlareAccessLogs(ctx context.Context, query OpenFlareAccessLogQuery) ([]*OpenFlareAccessLog, error) {
	return currentAccessLogStore().List(ctx, query)
}

// CountOpenFlareAccessLogs counts access logs and distinct IPs matching the query.
func CountOpenFlareAccessLogs(ctx context.Context, query OpenFlareAccessLogQuery) (int64, int64, error) {
	return currentAccessLogStore().Count(ctx, query)
}

// ListOpenFlareAccessLogRegionCounts returns region counts for access logs.
func ListOpenFlareAccessLogRegionCounts(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareAccessLogRegionCount, error) {
	return currentAccessLogStore().RegionCounts(ctx, nodeID, since, limit)
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
	bucketSeconds := int64(query.BucketMinutes * 60)
	if bucketSeconds <= 0 {
		bucketSeconds = 1800
	}
	rows, err := currentAccessLogStore().IPTrend(ctx, filter, bucketSeconds)
	if err != nil {
		return nil, err
	}
	result := make([]*OpenFlareAccessLogIPTrendRow, len(rows))
	for index, row := range rows {
		result[index] = &OpenFlareAccessLogIPTrendRow{
			BucketEpoch:  row.BucketEpoch,
			RequestCount: row.RequestCount,
		}
	}
	return result, nil
}

// DeleteAllOpenFlareAccessLogs deletes all access logs.
func DeleteAllOpenFlareAccessLogs(ctx context.Context) (int64, error) {
	return currentAccessLogStore().DeleteAll(ctx)
}

// DeleteOpenFlareAccessLogsBefore deletes access logs older than cutoff.
func DeleteOpenFlareAccessLogsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return currentAccessLogStore().DeleteBefore(ctx, cutoff)
}

// DeleteOpenFlareAccessLogsByNodeBefore deletes access logs for a node older than cutoff.
func DeleteOpenFlareAccessLogsByNodeBefore(ctx context.Context, nodeID string, cutoff time.Time) (int64, error) {
	return currentAccessLogStore().DeleteByNodeBefore(ctx, nodeID, cutoff)
}

func buildOpenFlareAccessLogBucketRows(ctx context.Context, query OpenFlareAccessLogBucketQuery) ([]*OpenFlareAccessLogBucketRow, error) {
	filter := openFlareAccessLogQueryFromBucket(query)
	bucketSeconds := int64(query.FoldMinutes * 60)
	if bucketSeconds <= 0 {
		bucketSeconds = 180
	}

	type bucketAccumulator struct {
		requestCount     int64
		uniqueIPs        map[string]struct{}
		uniqueHosts      map[string]struct{}
		successCount     int64
		clientErrorCount int64
		serverErrorCount int64
	}
	accumulators := make(map[int64]*bucketAccumulator)

	partials, err := currentAccessLogStore().BucketAggregates(ctx, filter, bucketSeconds)
	if err != nil {
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
		dimensions, err := currentAccessLogStore().BucketDimensions(ctx, filter, column, bucketSeconds)
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
	filter := OpenFlareAccessLogQuery{
		NodeID:     query.NodeID,
		RemoteAddr: query.RemoteAddr,
		Host:       query.Host,
		Since:      query.Since,
	}
	partials, err := currentAccessLogStore().IPSummaries(ctx, filter, recentSince)
	if err != nil {
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
	partials, err := currentAccessLogStore().IPAggregates(ctx, filter, exactRemoteAddr)
	if err != nil {
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
		args = append(args, query.Since.UTC())
	}
	if !query.Until.IsZero() {
		parts = append(parts, "logged_at < ?")
		args = append(args, query.Until.UTC())
	}
	if len(parts) == 0 {
		return "1", nil
	}
	return strings.Join(parts, " AND "), args
}

func combineOpenFlareAccessLogSQLClauses(left string, right string) string {
	if strings.TrimSpace(left) == "" || left == "TRUE" || left == "1" {
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
	sort.Slice(items, func(i, j int) bool {
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
	sort.Slice(items, func(i, j int) bool {
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
	sort.Slice(items, func(i, j int) bool {
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

