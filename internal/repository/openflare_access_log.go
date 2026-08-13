// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"sort"
	"strings"
	"time"

	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository/logstore"
)

const (
	sortOrderAsc     = "asc"
	secondsPerMinute = 60
)

// ListOpenFlareAccessLogWAFIPAggregates returns per-IP aggregates for WAF automatic rules.
func ListOpenFlareAccessLogWAFIPAggregates(ctx context.Context, query model.OpenFlareAccessLogQuery) ([]*model.OpenFlareAccessLogWAFIPAggregate, error) {
	s, err := logstore.Active(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.AccessLogs.WAFIPAggregates(ctx, query)
	if err != nil {
		return nil, err
	}
	result := make([]*model.OpenFlareAccessLogWAFIPAggregate, 0, len(rows))
	for _, row := range rows {
		remoteAddr := strings.TrimSpace(row.RemoteAddr)
		if remoteAddr == "" {
			continue
		}
		statusCounts := make(map[int]int, len(row.StatusCounts))
		for code, count := range row.StatusCounts {
			statusCounts[code] = int(count)
		}
		result = append(result, &model.OpenFlareAccessLogWAFIPAggregate{
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

// InsertOpenFlareAccessLogsBatch inserts access log rows into the active log store.
func InsertOpenFlareAccessLogsBatch(ctx context.Context, records []*model.OpenFlareAccessLog) error {
	s, err := logstore.Active(ctx)
	if err != nil {
		return err
	}
	return s.AccessLogs.InsertBatch(ctx, records)
}

// ListOpenFlareAccessLogs lists access logs matching the query.
func ListOpenFlareAccessLogs(ctx context.Context, query model.OpenFlareAccessLogQuery) ([]*model.OpenFlareAccessLog, error) {
	s, err := logstore.Active(ctx)
	if err != nil {
		return nil, err
	}
	return s.AccessLogs.List(ctx, query)
}

// CountOpenFlareAccessLogs counts access logs, distinct IPs, and total bytes sent matching the query.
func CountOpenFlareAccessLogs(ctx context.Context, query model.OpenFlareAccessLogQuery) (int64, int64, int64, error) {
	s, err := logstore.Active(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	return s.AccessLogs.Count(ctx, query)
}

// TrafficSummaryOpenFlareAccessLogs returns window-level request/error/UV/bytes summary.
func TrafficSummaryOpenFlareAccessLogs(ctx context.Context, query model.OpenFlareAccessLogQuery) (model.OpenFlareAccessLogTrafficSummary, error) {
	s, err := logstore.Active(ctx)
	if err != nil {
		return model.OpenFlareAccessLogTrafficSummary{}, err
	}
	return s.AccessLogs.TrafficSummary(ctx, query)
}

// ValueCountsOpenFlareAccessLogs groups logs by status_code, host, path, remote_addr, or user_agent.
func ValueCountsOpenFlareAccessLogs(ctx context.Context, query model.OpenFlareAccessLogQuery, column string, limit int) ([]model.OpenFlareAccessLogValueCount, error) {
	s, err := logstore.Active(ctx)
	if err != nil {
		return nil, err
	}
	return s.AccessLogs.ValueCounts(ctx, query, column, limit)
}

// NodeAggregatesOpenFlareAccessLogs returns per-node request/error/UV for the window.
func NodeAggregatesOpenFlareAccessLogs(ctx context.Context, query model.OpenFlareAccessLogQuery) ([]model.OpenFlareAccessLogNodeAggregate, error) {
	s, err := logstore.Active(ctx)
	if err != nil {
		return nil, err
	}
	return s.AccessLogs.NodeAggregates(ctx, query)
}

// ListOpenFlareAccessLogRegionCounts returns region counts for access logs.
func ListOpenFlareAccessLogRegionCounts(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareAccessLogRegionCount, error) {
	s, err := logstore.Active(ctx)
	if err != nil {
		return nil, err
	}
	return s.AccessLogs.RegionCounts(ctx, nodeID, since, limit)
}

// ListOpenFlareAccessLogBuckets lists folded access log buckets.
func ListOpenFlareAccessLogBuckets(ctx context.Context, query model.OpenFlareAccessLogBucketQuery) ([]*model.OpenFlareAccessLogBucketRow, error) {
	return buildOpenFlareAccessLogBucketRows(ctx, query)
}

// CountOpenFlareAccessLogBuckets counts folded access log buckets.
func CountOpenFlareAccessLogBuckets(ctx context.Context, query model.OpenFlareAccessLogBucketQuery) (int64, error) {
	filter := openFlareAccessLogQueryFromBucket(query)
	bucketSeconds := int64(query.FoldMinutes * secondsPerMinute)
	if bucketSeconds <= 0 {
		bucketSeconds = 180
	}
	s, err := logstore.Active(ctx)
	if err != nil {
		return 0, err
	}
	return s.AccessLogs.CountBuckets(ctx, filter, bucketSeconds)
}

// ListOpenFlareAccessLogBucketIPs lists folded IP rows for a bucket window.
func ListOpenFlareAccessLogBucketIPs(ctx context.Context, query model.OpenFlareAccessLogBucketIPQuery) ([]*model.OpenFlareAccessLogBucketIPRow, error) {
	rows, err := buildOpenFlareAccessLogBucketIPRows(ctx, query)
	if err != nil {
		return nil, err
	}
	start, end := openFlareAccessLogPaginateBounds(len(rows), query.Page, query.PageSize)
	if start >= len(rows) {
		return []*model.OpenFlareAccessLogBucketIPRow{}, nil
	}
	return rows[start:end], nil
}

// CountOpenFlareAccessLogBucketIPs counts folded IP rows for a bucket window.
func CountOpenFlareAccessLogBucketIPs(ctx context.Context, query model.OpenFlareAccessLogBucketIPQuery) (int64, error) {
	rows, err := buildOpenFlareAccessLogBucketIPRows(ctx, query)
	if err != nil {
		return 0, err
	}
	return int64(len(rows)), nil
}

// ListOpenFlareAccessLogIPSummaries lists IP summaries.
func ListOpenFlareAccessLogIPSummaries(ctx context.Context, query model.OpenFlareAccessLogIPSummaryQuery, recentSince time.Time) ([]*analyticsmodel.NodeAccessLogIPSummary, error) {
	return buildOpenFlareAccessLogIPSummaryRows(ctx, query, recentSince)
}

// CountOpenFlareAccessLogIPSummaries counts IP summaries.
func CountOpenFlareAccessLogIPSummaries(ctx context.Context, query model.OpenFlareAccessLogIPSummaryQuery) (int64, error) {
	filter := openFlareAccessLogQueryFromIPSummary(query)
	s, err := logstore.Active(ctx)
	if err != nil {
		return 0, err
	}
	return s.AccessLogs.CountIPSummaries(ctx, filter)
}

// ListOpenFlareAccessLogIPTrend lists IP trend points.
func ListOpenFlareAccessLogIPTrend(ctx context.Context, query model.OpenFlareAccessLogIPTrendQuery) ([]*analyticsmodel.NodeAccessLogIPTrend, error) {
	remoteAddr := strings.TrimSpace(query.RemoteAddr)
	if remoteAddr == "" {
		return []*analyticsmodel.NodeAccessLogIPTrend{}, nil
	}
	filter := model.OpenFlareAccessLogQuery{
		NodeID:     query.NodeID,
		RemoteAddr: remoteAddr,
		Host:       query.Host,
		Since:      query.Since,
	}
	bucketSeconds := int64(query.BucketMinutes * secondsPerMinute)
	if bucketSeconds <= 0 {
		bucketSeconds = 1800
	}
	s, err := logstore.Active(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.AccessLogs.IPTrend(ctx, filter, bucketSeconds)
	if err != nil {
		return nil, err
	}
	result := make([]*analyticsmodel.NodeAccessLogIPTrend, len(rows))
	for index, row := range rows {
		result[index] = &analyticsmodel.NodeAccessLogIPTrend{
			BucketEpoch:  row.BucketEpoch,
			RequestCount: row.RequestCount,
		}
	}
	return result, nil
}

// DeleteAllOpenFlareAccessLogs deletes all access logs.
func DeleteAllOpenFlareAccessLogs(ctx context.Context) (int64, error) {
	s, err := logstore.Active(ctx)
	if err != nil {
		return 0, err
	}
	return s.AccessLogs.DeleteAll(ctx)
}

// DeleteOpenFlareAccessLogsBefore deletes access logs older than cutoff.
func DeleteOpenFlareAccessLogsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	s, err := logstore.Active(ctx)
	if err != nil {
		return 0, err
	}
	return s.AccessLogs.DeleteBefore(ctx, cutoff)
}

// DeleteOpenFlareAccessLogsByNodeBefore deletes access logs for a node older than cutoff.
func DeleteOpenFlareAccessLogsByNodeBefore(ctx context.Context, nodeID string, cutoff time.Time) (int64, error) {
	s, err := logstore.Active(ctx)
	if err != nil {
		return 0, err
	}
	return s.AccessLogs.DeleteByNodeBefore(ctx, nodeID, cutoff)
}

func buildOpenFlareAccessLogBucketRows(ctx context.Context, query model.OpenFlareAccessLogBucketQuery) ([]*model.OpenFlareAccessLogBucketRow, error) {
	filter := openFlareAccessLogQueryFromBucket(query)
	bucketSeconds := int64(query.FoldMinutes * secondsPerMinute)
	if bucketSeconds <= 0 {
		bucketSeconds = 180
	}
	s, err := logstore.Active(ctx)
	if err != nil {
		return nil, err
	}
	partials, err := s.AccessLogs.BucketAggregates(ctx, filter, bucketSeconds)
	if err != nil {
		return nil, err
	}
	rows := make([]*model.OpenFlareAccessLogBucketRow, 0, len(partials))
	for _, partial := range partials {
		rows = append(rows, &model.OpenFlareAccessLogBucketRow{
			BucketEpoch:      partial.BucketEpoch,
			RequestCount:     partial.RequestCount,
			UniqueIPCount:    partial.UniqueIPCount,
			UniqueHostCount:  partial.UniqueHostCount,
			SuccessCount:     partial.SuccessCount,
			ClientErrorCount: partial.ClientErrorCount,
			ServerErrorCount: partial.ServerErrorCount,
			Status2xxCount:   partial.Status2xxCount,
			Status4xxCount:   partial.Status4xxCount,
			Status5xxCount:   partial.Status5xxCount,
			BytesSent:        partial.BytesSent,
			RequestLength:    partial.RequestLength,
		})
	}
	return rows, nil
}

func buildOpenFlareAccessLogBucketIPRows(ctx context.Context, query model.OpenFlareAccessLogBucketIPQuery) ([]*model.OpenFlareAccessLogBucketIPRow, error) {
	if query.BucketStartedAt.IsZero() {
		return []*model.OpenFlareAccessLogBucketIPRow{}, nil
	}
	foldMinutes := query.FoldMinutes
	if foldMinutes <= 0 {
		foldMinutes = 3
	}
	bucketStartedAt := query.BucketStartedAt.UTC()
	filter := model.OpenFlareAccessLogQuery{
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

func buildOpenFlareAccessLogIPSummaryRows(ctx context.Context, query model.OpenFlareAccessLogIPSummaryQuery, recentSince time.Time) ([]*analyticsmodel.NodeAccessLogIPSummary, error) {
	filter := openFlareAccessLogQueryFromIPSummary(query)
	s, err := logstore.Active(ctx)
	if err != nil {
		return nil, err
	}
	partials, err := s.AccessLogs.IPSummaries(ctx, filter, recentSince)
	if err != nil {
		return nil, err
	}
	rows := make([]*analyticsmodel.NodeAccessLogIPSummary, 0, len(partials))
	for _, partial := range partials {
		remoteAddr := strings.TrimSpace(partial.RemoteAddr)
		if remoteAddr == "" {
			continue
		}
		rows = append(rows, &analyticsmodel.NodeAccessLogIPSummary{
			RemoteAddr:      remoteAddr,
			Region:          strings.TrimSpace(partial.Region),
			TotalRequests:   partial.TotalRequests,
			Success2xxCount: partial.Success2xxCount,
			SuccessRatio:    partial.SuccessRatio,
			BytesReceived:   partial.BytesReceived,
			BytesSent:       partial.BytesSent,
			RecentRequests:  0,
			LastSeenEpoch:   partial.LastSeenEpoch,
		})
	}
	return rows, nil
}

func queryOpenFlareAccessLogIPAggregateRows(ctx context.Context, filter model.OpenFlareAccessLogQuery, exactRemoteAddr bool) ([]*model.OpenFlareAccessLogBucketIPRow, error) {
	s, err := logstore.Active(ctx)
	if err != nil {
		return nil, err
	}
	partials, err := s.AccessLogs.IPAggregates(ctx, filter, exactRemoteAddr)
	if err != nil {
		return nil, err
	}
	rows := make([]*model.OpenFlareAccessLogBucketIPRow, 0, len(partials))
	for _, partial := range partials {
		remoteAddr := strings.TrimSpace(partial.RemoteAddr)
		if remoteAddr == "" {
			continue
		}
		rows = append(rows, &model.OpenFlareAccessLogBucketIPRow{
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

func openFlareAccessLogQueryFromBucket(query model.OpenFlareAccessLogBucketQuery) model.OpenFlareAccessLogQuery {
	return model.OpenFlareAccessLogQuery{
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

func openFlareAccessLogQueryFromIPSummary(query model.OpenFlareAccessLogIPSummaryQuery) model.OpenFlareAccessLogQuery {
	return model.OpenFlareAccessLogQuery{
		NodeID:     query.NodeID,
		RemoteAddr: query.RemoteAddr,
		Host:       query.Host,
		Since:      query.Since,
		Until:      query.Until,
		Page:       query.Page,
		PageSize:   query.PageSize,
		SortBy:     query.SortBy,
		SortOrder:  query.SortOrder,
	}
}

func sortOpenFlareAccessLogBucketIPRows(items []*model.OpenFlareAccessLogBucketIPRow, sortBy string, sortOrder string) {
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
