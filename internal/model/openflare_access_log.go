// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"

	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

const (
	columnRemoteAddr = "remote_addr"
	columnHost       = "host"
	sortOrderAsc     = "asc"
	secondsPerMinute = 60
)

type openFlareAccessLogBucketAggregateRow = analyticsmodel.NodeAccessLogBucketAggregate
type openFlareAccessLogBucketDimensionRow = analyticsmodel.NodeAccessLogBucketDimension
type openFlareAccessLogIPAggregateRow = analyticsmodel.NodeAccessLogIPAggregate
type openFlareAccessLogIPSummaryRow = analyticsmodel.NodeAccessLogIPSummary
type openFlareAccessLogIPTrendRow = analyticsmodel.NodeAccessLogIPTrend
type openFlareAccessLogWAFIPAggregateRow = analyticsmodel.NodeAccessLogWAFIPAggregate

// ListOpenFlareAccessLogWAFIPAggregates returns per-IP aggregates for WAF automatic rules.
func ListOpenFlareAccessLogWAFIPAggregates(ctx context.Context, query OpenFlareAccessLogQuery) ([]*OpenFlareAccessLogWAFIPAggregate, error) {
	rows, err := currentAccessLogStore().WAFIPAggregates(ctx, query)
	if err != nil {
		return nil, err
	}
	result := make([]*OpenFlareAccessLogWAFIPAggregate, 0, len(rows))
	for _, row := range rows {
		remoteAddr := strings.TrimSpace(row.RemoteAddr)
		if remoteAddr == "" {
			continue
		}
		statusCounts := make(map[int]int, len(row.StatusCounts))
		for code, count := range row.StatusCounts {
			statusCounts[code] = int(count)
		}
		result = append(result, &OpenFlareAccessLogWAFIPAggregate{
			RemoteAddr:       remoteAddr,
			RequestCount:     int(row.RequestCount),
			Status404Count:   int(row.Status404Count),
			ClientErrorCount: int(row.ClientErrorCount),
			ServerErrorCount: int(row.ServerErrorCount),
			IPHostCount:      int(row.IPHostCount),
			LastSeenEpoch:    row.LastSeenEpoch,
			StatusCounts:     statusCounts,
		})
	}
	return result, nil
}

// InsertOpenFlareAccessLogsBatch inserts access log rows into ClickHouse.
func InsertOpenFlareAccessLogsBatch(ctx context.Context, records []*OpenFlareAccessLog) error {
	return currentAccessLogStore().InsertBatch(ctx, records)
}

// ListOpenFlareAccessLogs lists access logs matching the query.
func ListOpenFlareAccessLogs(ctx context.Context, query OpenFlareAccessLogQuery) ([]*OpenFlareAccessLog, error) {
	return currentAccessLogStore().List(ctx, query)
}

// CountOpenFlareAccessLogs counts access logs, distinct IPs, and total bytes sent matching the query.
func CountOpenFlareAccessLogs(ctx context.Context, query OpenFlareAccessLogQuery) (int64, int64, int64, error) {
	return currentAccessLogStore().Count(ctx, query)
}

// TrafficSummaryOpenFlareAccessLogs returns window-level request/error/UV/bytes summary.
func TrafficSummaryOpenFlareAccessLogs(ctx context.Context, query OpenFlareAccessLogQuery) (OpenFlareAccessLogTrafficSummary, error) {
	return currentAccessLogStore().TrafficSummary(ctx, query)
}

// ValueCountsOpenFlareAccessLogs groups logs by status_code, host, path, remote_addr, or user_agent.
func ValueCountsOpenFlareAccessLogs(ctx context.Context, query OpenFlareAccessLogQuery, column string, limit int) ([]OpenFlareAccessLogValueCount, error) {
	return currentAccessLogStore().ValueCounts(ctx, query, column, limit)
}

// NodeAggregatesOpenFlareAccessLogs returns per-node request/error/UV for the window.
func NodeAggregatesOpenFlareAccessLogs(ctx context.Context, query OpenFlareAccessLogQuery) ([]OpenFlareAccessLogNodeAggregate, error) {
	return currentAccessLogStore().NodeAggregates(ctx, query)
}

// ListOpenFlareAccessLogRegionCounts returns region counts for access logs.
func ListOpenFlareAccessLogRegionCounts(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareAccessLogRegionCount, error) {
	return currentAccessLogStore().RegionCounts(ctx, nodeID, since, limit)
}

// ListOpenFlareAccessLogBuckets lists folded access log buckets.
func ListOpenFlareAccessLogBuckets(ctx context.Context, query OpenFlareAccessLogBucketQuery) ([]*OpenFlareAccessLogBucketRow, error) {
	return buildOpenFlareAccessLogBucketRows(ctx, query)
}

// CountOpenFlareAccessLogBuckets counts folded access log buckets.
func CountOpenFlareAccessLogBuckets(ctx context.Context, query OpenFlareAccessLogBucketQuery) (int64, error) {
	filter := openFlareAccessLogQueryFromBucket(query)
	bucketSeconds := int64(query.FoldMinutes * secondsPerMinute)
	if bucketSeconds <= 0 {
		bucketSeconds = 180
	}
	return currentAccessLogStore().CountBuckets(ctx, filter, bucketSeconds)
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
	return buildOpenFlareAccessLogIPSummaryRows(ctx, query, recentSince)
}

// CountOpenFlareAccessLogIPSummaries counts IP summaries.
func CountOpenFlareAccessLogIPSummaries(ctx context.Context, query OpenFlareAccessLogIPSummaryQuery) (int64, error) {
	filter := openFlareAccessLogQueryFromIPSummary(query)
	return currentAccessLogStore().CountIPSummaries(ctx, filter)
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
	bucketSeconds := int64(query.BucketMinutes * secondsPerMinute)
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
	bucketSeconds := int64(query.FoldMinutes * secondsPerMinute)
	if bucketSeconds <= 0 {
		bucketSeconds = 180
	}

	partials, err := currentAccessLogStore().BucketAggregates(ctx, filter, bucketSeconds)
	if err != nil {
		return nil, err
	}
	rows := make([]*OpenFlareAccessLogBucketRow, 0, len(partials))
	for _, partial := range partials {
		rows = append(rows, &OpenFlareAccessLogBucketRow{
			BucketEpoch:      partial.BucketEpoch,
			RequestCount:     partial.RequestCount,
			UniqueIPCount:    partial.UniqueIPCount,
			UniqueHostCount:  partial.UniqueHostCount,
			SuccessCount:     partial.SuccessCount,
			ClientErrorCount: partial.ClientErrorCount,
			ServerErrorCount: partial.ServerErrorCount,
			BytesSent:        partial.BytesSent,
			RequestLength:    partial.RequestLength,
		})
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
	filter := openFlareAccessLogQueryFromIPSummary(query)
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
		Hosts:      query.Hosts,
		Path:       query.Path,
		Since:      query.Since,
		Until:      query.Until,
		Page:       query.Page,
		PageSize:   query.PageSize,
		SortBy:     query.SortBy,
		SortOrder:  query.SortOrder,
	}
}

func openFlareAccessLogQueryFromIPSummary(query OpenFlareAccessLogIPSummaryQuery) OpenFlareAccessLogQuery {
	return OpenFlareAccessLogQuery{
		NodeID:     query.NodeID,
		RemoteAddr: query.RemoteAddr,
		Host:       query.Host,
		Since:      query.Since,
		Page:       query.Page,
		PageSize:   query.PageSize,
		SortBy:     query.SortBy,
		SortOrder:  query.SortOrder,
	}
}

func sortOpenFlareAccessLogBucketIPRows(items []*OpenFlareAccessLogBucketIPRow, sortBy string, sortOrder string) {
	desc := openFlareAccessLogNormalizeSortOrder(sortOrder) != sortOrderAsc
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
	desc := openFlareAccessLogNormalizeSortOrder(sortOrder) != sortOrderAsc
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
	desc := openFlareAccessLogNormalizeSortOrder(sortOrder) != sortOrderAsc
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
	if strings.EqualFold(strings.TrimSpace(sortOrder), sortOrderAsc) {
		return sortOrderAsc
	}
	return "desc"
}

func openFlareAccessLogStatusCodeToInt32(code int) int32 {
	switch {
	case code > math.MaxInt32:
		return math.MaxInt32
	case code < math.MinInt32:
		return math.MinInt32
	default:
		return int32(code)
	}
}

func openFlareAccessLogUintToInt64(value uint64) int64 {
	if value > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(value)
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
