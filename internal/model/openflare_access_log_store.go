// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"sync"
	"time"

	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	analyticsrepo "github.com/Rain-kl/Wavelet/internal/repository/analytics"
)

// AccessLogInsertHooks queues node access logs for async ClickHouse write.
// Wired from openflare/chwriter.Init so model never imports the apps layer.
type AccessLogInsertHooks struct {
	QueueNodeAccessLogs func(logs []analyticsmodel.NodeAccessLog)
}

var (
	accessLogInsertHooksMu sync.RWMutex
	accessLogInsertHooks   AccessLogInsertHooks
)

// SetAccessLogInsertHooks registers async queue callbacks for access log inserts.
func SetAccessLogInsertHooks(hooks AccessLogInsertHooks) {
	accessLogInsertHooksMu.Lock()
	accessLogInsertHooks = hooks
	accessLogInsertHooksMu.Unlock()
}

func currentAccessLogInsertHooks() AccessLogInsertHooks {
	accessLogInsertHooksMu.RLock()
	defer accessLogInsertHooksMu.RUnlock()
	return accessLogInsertHooks
}

type accessLogStore interface {
	InsertBatch(ctx context.Context, records []*OpenFlareAccessLog) error
	List(ctx context.Context, query OpenFlareAccessLogQuery) ([]*OpenFlareAccessLog, error)
	Count(ctx context.Context, query OpenFlareAccessLogQuery) (int64, int64, error)
	RegionCounts(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareAccessLogRegionCount, error)
	BucketAggregates(ctx context.Context, filter OpenFlareAccessLogQuery, bucketSeconds int64) ([]openFlareAccessLogBucketAggregateRow, error)
	CountBuckets(ctx context.Context, filter OpenFlareAccessLogQuery, bucketSeconds int64) (int64, error)
	BucketDimensions(ctx context.Context, filter OpenFlareAccessLogQuery, column string, bucketSeconds int64) ([]openFlareAccessLogBucketDimensionRow, error)
	IPAggregates(ctx context.Context, filter OpenFlareAccessLogQuery, exactRemoteAddr bool) ([]openFlareAccessLogIPAggregateRow, error)
	WAFIPAggregates(ctx context.Context, filter OpenFlareAccessLogQuery) ([]openFlareAccessLogWAFIPAggregateRow, error)
	IPSummaries(ctx context.Context, filter OpenFlareAccessLogQuery, recentSince time.Time) ([]openFlareAccessLogIPSummaryRow, error)
	CountIPSummaries(ctx context.Context, filter OpenFlareAccessLogQuery) (int64, error)
	IPTrend(ctx context.Context, filter OpenFlareAccessLogQuery, bucketSeconds int64) ([]openFlareAccessLogIPTrendRow, error)
	DeleteAll(ctx context.Context) (int64, error)
	DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error)
	DeleteByNodeBefore(ctx context.Context, nodeID string, before time.Time) (int64, error)
}

var (
	accessLogStoreMu     sync.RWMutex
	accessLogStoreHolder accessLogStore
)

func currentAccessLogStore() accessLogStore {
	accessLogStoreMu.RLock()
	defer accessLogStoreMu.RUnlock()
	if accessLogStoreHolder != nil {
		return accessLogStoreHolder
	}
	return clickhouseAccessLogStore{}
}

// SetAccessLogStoreForTest swaps the access log store implementation for unit tests.
func SetAccessLogStoreForTest(store accessLogStore) func() {
	accessLogStoreMu.Lock()
	previous := accessLogStoreHolder
	accessLogStoreHolder = store
	accessLogStoreMu.Unlock()
	return func() {
		accessLogStoreMu.Lock()
		accessLogStoreHolder = previous
		accessLogStoreMu.Unlock()
	}
}

// NewMemoryAccessLogStore returns an in-memory access log store for unit tests.
func NewMemoryAccessLogStore() accessLogStore {
	return &memoryAccessLogStore{
		records: make([]*OpenFlareAccessLog, 0),
	}
}

type clickhouseAccessLogStore struct{}

func (clickhouseAccessLogStore) InsertBatch(_ context.Context, records []*OpenFlareAccessLog) error {
	logs := make([]analyticsmodel.NodeAccessLog, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		logs = append(logs, toAnalyticsNodeAccessLog(record))
	}
	if hook := currentAccessLogInsertHooks().QueueNodeAccessLogs; hook != nil {
		hook(logs)
	}
	return nil
}

func (clickhouseAccessLogStore) List(ctx context.Context, query OpenFlareAccessLogQuery) ([]*OpenFlareAccessLog, error) {
	rows, err := analyticsrepo.ListNodeAccessLogs(ctx, toNodeAccessLogFilter(query))
	if err != nil {
		return nil, err
	}
	return fromAnalyticsNodeAccessLogs(rows), nil
}

func (clickhouseAccessLogStore) Count(ctx context.Context, query OpenFlareAccessLogQuery) (int64, int64, error) {
	return analyticsrepo.CountNodeAccessLogs(ctx, toNodeAccessLogFilter(query))
}

func (clickhouseAccessLogStore) RegionCounts(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareAccessLogRegionCount, error) {
	rows, err := analyticsrepo.RegionCountsNodeAccessLogs(ctx, nodeID, since, limit)
	if err != nil {
		return nil, err
	}
	result := make([]*OpenFlareAccessLogRegionCount, len(rows))
	for index, row := range rows {
		result[index] = &OpenFlareAccessLogRegionCount{
			Region: row.Region,
			Count:  row.Count,
		}
	}
	return result, nil
}

func (clickhouseAccessLogStore) BucketAggregates(ctx context.Context, filter OpenFlareAccessLogQuery, bucketSeconds int64) ([]openFlareAccessLogBucketAggregateRow, error) {
	rows, err := analyticsrepo.BucketAggregatesNodeAccessLogs(ctx, toNodeAccessLogFilter(filter), bucketSeconds)
	if err != nil {
		return nil, err
	}
	result := make([]openFlareAccessLogBucketAggregateRow, len(rows))
	for index, row := range rows {
		result[index] = openFlareAccessLogBucketAggregateRow{
			BucketEpoch:      row.BucketEpoch,
			RequestCount:     row.RequestCount,
			SuccessCount:     row.SuccessCount,
			ClientErrorCount: row.ClientErrorCount,
			ServerErrorCount: row.ServerErrorCount,
			UniqueIPCount:    row.UniqueIPCount,
			UniqueHostCount:  row.UniqueHostCount,
		}
	}
	return result, nil
}

func (clickhouseAccessLogStore) CountBuckets(ctx context.Context, filter OpenFlareAccessLogQuery, bucketSeconds int64) (int64, error) {
	return analyticsrepo.CountBucketAggregatesNodeAccessLogs(ctx, toNodeAccessLogFilter(filter), bucketSeconds)
}

func (clickhouseAccessLogStore) BucketDimensions(ctx context.Context, filter OpenFlareAccessLogQuery, column string, bucketSeconds int64) ([]openFlareAccessLogBucketDimensionRow, error) {
	rows, err := analyticsrepo.BucketDimensionsNodeAccessLogs(ctx, toNodeAccessLogFilter(filter), column, bucketSeconds)
	if err != nil {
		return nil, err
	}
	result := make([]openFlareAccessLogBucketDimensionRow, len(rows))
	for index, row := range rows {
		result[index] = openFlareAccessLogBucketDimensionRow{
			BucketEpoch: row.BucketEpoch,
			Value:       row.Value,
		}
	}
	return result, nil
}

func (clickhouseAccessLogStore) IPAggregates(ctx context.Context, filter OpenFlareAccessLogQuery, exactRemoteAddr bool) ([]openFlareAccessLogIPAggregateRow, error) {
	rows, err := analyticsrepo.IPAggregatesNodeAccessLogs(ctx, toNodeAccessLogFilter(filter), exactRemoteAddr)
	if err != nil {
		return nil, err
	}
	result := make([]openFlareAccessLogIPAggregateRow, len(rows))
	for index, row := range rows {
		result[index] = openFlareAccessLogIPAggregateRow{
			RemoteAddr:       row.RemoteAddr,
			RequestCount:     row.RequestCount,
			SuccessCount:     row.SuccessCount,
			ClientErrorCount: row.ClientErrorCount,
			ServerErrorCount: row.ServerErrorCount,
			LastSeenEpoch:    row.LastSeenEpoch,
		}
	}
	return result, nil
}

func (clickhouseAccessLogStore) IPSummaries(ctx context.Context, filter OpenFlareAccessLogQuery, recentSince time.Time) ([]openFlareAccessLogIPSummaryRow, error) {
	rows, err := analyticsrepo.IPSummariesNodeAccessLogs(ctx, toNodeAccessLogFilter(filter), recentSince)
	if err != nil {
		return nil, err
	}
	result := make([]openFlareAccessLogIPSummaryRow, len(rows))
	for index, row := range rows {
		result[index] = openFlareAccessLogIPSummaryRow{
			RemoteAddr:     row.RemoteAddr,
			TotalRequests:  row.TotalRequests,
			RecentRequests: row.RecentRequests,
			LastSeenEpoch:  row.LastSeenEpoch,
		}
	}
	return result, nil
}

func (clickhouseAccessLogStore) CountIPSummaries(ctx context.Context, filter OpenFlareAccessLogQuery) (int64, error) {
	return analyticsrepo.CountIPSummaryNodeAccessLogs(ctx, toNodeAccessLogFilter(filter))
}

func (clickhouseAccessLogStore) WAFIPAggregates(ctx context.Context, filter OpenFlareAccessLogQuery) ([]openFlareAccessLogWAFIPAggregateRow, error) {
	rows, err := analyticsrepo.IPAggregatesForWAFNodeAccessLogs(ctx, toNodeAccessLogFilter(filter))
	if err != nil {
		return nil, err
	}
	result := make([]openFlareAccessLogWAFIPAggregateRow, len(rows))
	for index, row := range rows {
		result[index] = openFlareAccessLogWAFIPAggregateRow{
			RemoteAddr:       row.RemoteAddr,
			RequestCount:     row.RequestCount,
			Status404Count:   row.Status404Count,
			ClientErrorCount: row.ClientErrorCount,
			ServerErrorCount: row.ServerErrorCount,
			IPHostCount:      row.IPHostCount,
			LastSeenEpoch:    row.LastSeenEpoch,
			StatusCounts:     row.StatusCounts,
		}
	}
	return result, nil
}

func (clickhouseAccessLogStore) IPTrend(ctx context.Context, filter OpenFlareAccessLogQuery, bucketSeconds int64) ([]openFlareAccessLogIPTrendRow, error) {
	rows, err := analyticsrepo.IPTrendNodeAccessLogs(ctx, toNodeAccessLogFilter(filter), bucketSeconds)
	if err != nil {
		return nil, err
	}
	result := make([]openFlareAccessLogIPTrendRow, len(rows))
	for index, row := range rows {
		result[index] = openFlareAccessLogIPTrendRow{
			BucketEpoch:  row.BucketEpoch,
			RequestCount: row.RequestCount,
		}
	}
	return result, nil
}

func (clickhouseAccessLogStore) DeleteAll(ctx context.Context) (int64, error) {
	return analyticsrepo.DeleteAllNodeAccessLogs(ctx)
}

func (clickhouseAccessLogStore) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return analyticsrepo.DeleteNodeAccessLogsBefore(ctx, cutoff)
}

func (clickhouseAccessLogStore) DeleteByNodeBefore(ctx context.Context, nodeID string, before time.Time) (int64, error) {
	return analyticsrepo.DeleteNodeAccessLogsByNodeBefore(ctx, nodeID, before)
}

func toNodeAccessLogFilter(query OpenFlareAccessLogQuery) analyticsrepo.NodeAccessLogFilter {
	return analyticsrepo.NodeAccessLogFilter{
		NodeID:     query.NodeID,
		RemoteAddr: query.RemoteAddr,
		Host:       query.Host,
		Path:       query.Path,
		Since:      query.Since,
		Until:      query.Until,
		Page:       query.Page,
		PageSize:   query.PageSize,
		SortBy:     query.SortBy,
		SortOrder:  query.SortOrder,
	}
}

func toAnalyticsNodeAccessLog(record *OpenFlareAccessLog) analyticsmodel.NodeAccessLog {
	return analyticsmodel.NodeAccessLog{
		ID:         record.ID,
		NodeID:     record.NodeID,
		LoggedAt:   record.LoggedAt,
		RemoteAddr: record.RemoteAddr,
		Region:     record.Region,
		Host:       record.Host,
		Path:       record.Path,
		StatusCode: openFlareAccessLogStatusCodeToInt32(record.StatusCode),
		CreatedAt:  record.CreatedAt,
	}
}

func fromAnalyticsNodeAccessLogs(rows []analyticsmodel.NodeAccessLog) []*OpenFlareAccessLog {
	result := make([]*OpenFlareAccessLog, len(rows))
	for index, row := range rows {
		result[index] = &OpenFlareAccessLog{
			ID:         row.ID,
			NodeID:     row.NodeID,
			LoggedAt:   row.LoggedAt,
			RemoteAddr: row.RemoteAddr,
			Region:     row.Region,
			Host:       row.Host,
			Path:       row.Path,
			StatusCode: int(row.StatusCode),
			CreatedAt:  row.CreatedAt,
		}
	}
	return result
}
