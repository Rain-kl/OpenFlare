// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"math"
	"sync"
	"time"

	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	analyticsrepo "github.com/Rain-kl/Wavelet/internal/repository/analytics"
)

// ObservabilityInsertHooks queues observability rows for async ClickHouse write.
// Wired from openflare/chwriter.Init so model never imports the apps layer.
type ObservabilityInsertHooks struct {
	QueueMetricSnapshot       func(analyticsmodel.NodeMetricSnapshot)
	QueueRequestReport        func(analyticsmodel.NodeRequestReport)
	QueueOpenrestyObservation func(analyticsmodel.NodeObsOpenresty)
	QueueFrpsObservation      func(analyticsmodel.NodeObsFrps)
	QueueFrpcObservation      func(analyticsmodel.NodeObsFrpc)
}

var (
	observabilityInsertHooksMu sync.RWMutex
	observabilityInsertHooks   ObservabilityInsertHooks
)

// SetObservabilityInsertHooks registers async queue callbacks for observability inserts.
func SetObservabilityInsertHooks(hooks ObservabilityInsertHooks) {
	observabilityInsertHooksMu.Lock()
	observabilityInsertHooks = hooks
	observabilityInsertHooksMu.Unlock()
}

func currentObservabilityInsertHooks() ObservabilityInsertHooks {
	observabilityInsertHooksMu.RLock()
	defer observabilityInsertHooksMu.RUnlock()
	return observabilityInsertHooks
}

type observabilityStore interface {
	InsertMetricSnapshot(ctx context.Context, record *OpenFlareMetricSnapshot) error
	ListMetricSnapshots(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareMetricSnapshot, error)
	DeleteAllMetricSnapshots(ctx context.Context) (int64, error)
	DeleteMetricSnapshotsBefore(ctx context.Context, cutoff time.Time) (int64, error)

	InsertRequestReport(ctx context.Context, record *OpenFlareRequestReport) error
	ListRequestReports(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareRequestReport, error)
	DeleteAllRequestReports(ctx context.Context) (int64, error)
	DeleteRequestReportsBefore(ctx context.Context, cutoff time.Time) (int64, error)

	InsertNodeObservationOpenresty(ctx context.Context, record *OpenFlareNodeObservationOpenresty) error
	ListNodeObservationOpenresty(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareNodeObservationOpenresty, error)
	DeleteAllNodeObservationOpenresty(ctx context.Context) (int64, error)
	DeleteNodeObservationOpenrestyBefore(ctx context.Context, cutoff time.Time) (int64, error)

	InsertNodeObservationFrps(ctx context.Context, record *OpenFlareNodeObservationFrps) error
	ListNodeObservationFrps(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareNodeObservationFrps, error)
	DeleteAllNodeObservationFrps(ctx context.Context) (int64, error)
	DeleteNodeObservationFrpsBefore(ctx context.Context, cutoff time.Time) (int64, error)

	InsertNodeObservationFrpc(ctx context.Context, record *OpenFlareNodeObservationFrpc) error
	ListNodeObservationFrpc(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareNodeObservationFrpc, error)
	DeleteAllNodeObservationFrpc(ctx context.Context) (int64, error)
	DeleteNodeObservationFrpcBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

var (
	observabilityStoreMu     sync.RWMutex
	observabilityStoreHolder observabilityStore
)

func currentObservabilityStore() observabilityStore {
	observabilityStoreMu.RLock()
	defer observabilityStoreMu.RUnlock()
	if observabilityStoreHolder != nil {
		return observabilityStoreHolder
	}
	return clickhouseObservabilityStore{}
}

// SetObservabilityStoreForTest swaps the observability store implementation for unit tests.
func SetObservabilityStoreForTest(store observabilityStore) func() {
	observabilityStoreMu.Lock()
	previous := observabilityStoreHolder
	observabilityStoreHolder = store
	observabilityStoreMu.Unlock()
	return func() {
		observabilityStoreMu.Lock()
		observabilityStoreHolder = previous
		observabilityStoreMu.Unlock()
	}
}

// NewMemoryObservabilityStore returns an in-memory observability store for unit tests.
func NewMemoryObservabilityStore() observabilityStore {
	return &memoryObservabilityStore{}
}

type clickhouseObservabilityStore struct{}

func (clickhouseObservabilityStore) InsertMetricSnapshot(_ context.Context, record *OpenFlareMetricSnapshot) error {
	if record == nil {
		return nil
	}
	if hook := currentObservabilityInsertHooks().QueueMetricSnapshot; hook != nil {
		hook(toAnalyticsNodeMetricSnapshot(record))
	}
	return nil
}

func (clickhouseObservabilityStore) ListMetricSnapshots(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareMetricSnapshot, error) {
	rows, err := analyticsrepo.ListNodeMetricSnapshots(ctx, toNodeObservabilityFilter(nodeID, since, limit))
	if err != nil {
		return nil, err
	}
	return fromAnalyticsNodeMetricSnapshots(rows), nil
}

func (clickhouseObservabilityStore) DeleteAllMetricSnapshots(ctx context.Context) (int64, error) {
	return analyticsrepo.DeleteAllNodeMetricSnapshots(ctx)
}

func (clickhouseObservabilityStore) DeleteMetricSnapshotsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return analyticsrepo.DeleteNodeMetricSnapshotsBefore(ctx, cutoff)
}

func (clickhouseObservabilityStore) InsertRequestReport(_ context.Context, record *OpenFlareRequestReport) error {
	if record == nil {
		return nil
	}
	if hook := currentObservabilityInsertHooks().QueueRequestReport; hook != nil {
		hook(toAnalyticsNodeRequestReport(record))
	}
	return nil
}

func (clickhouseObservabilityStore) ListRequestReports(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareRequestReport, error) {
	rows, err := analyticsrepo.ListNodeRequestReports(ctx, toNodeObservabilityFilter(nodeID, since, limit))
	if err != nil {
		return nil, err
	}
	return fromAnalyticsNodeRequestReports(rows), nil
}

func (clickhouseObservabilityStore) DeleteAllRequestReports(ctx context.Context) (int64, error) {
	return analyticsrepo.DeleteAllNodeRequestReports(ctx)
}

func (clickhouseObservabilityStore) DeleteRequestReportsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return analyticsrepo.DeleteNodeRequestReportsBefore(ctx, cutoff)
}

func (clickhouseObservabilityStore) InsertNodeObservationOpenresty(_ context.Context, record *OpenFlareNodeObservationOpenresty) error {
	if record == nil {
		return nil
	}
	if hook := currentObservabilityInsertHooks().QueueOpenrestyObservation; hook != nil {
		hook(toAnalyticsNodeObsOpenresty(record))
	}
	return nil
}

func (clickhouseObservabilityStore) ListNodeObservationOpenresty(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareNodeObservationOpenresty, error) {
	rows, err := analyticsrepo.ListNodeObsOpenresty(ctx, toNodeObservabilityFilter(nodeID, since, limit))
	if err != nil {
		return nil, err
	}
	return fromAnalyticsNodeObsOpenresty(rows), nil
}

func (clickhouseObservabilityStore) DeleteAllNodeObservationOpenresty(ctx context.Context) (int64, error) {
	return analyticsrepo.DeleteAllNodeObsOpenresty(ctx)
}

func (clickhouseObservabilityStore) DeleteNodeObservationOpenrestyBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return analyticsrepo.DeleteNodeObsOpenrestyBefore(ctx, cutoff)
}

func (clickhouseObservabilityStore) InsertNodeObservationFrps(_ context.Context, record *OpenFlareNodeObservationFrps) error {
	if record == nil {
		return nil
	}
	if hook := currentObservabilityInsertHooks().QueueFrpsObservation; hook != nil {
		hook(toAnalyticsNodeObsFrps(record))
	}
	return nil
}

func (clickhouseObservabilityStore) ListNodeObservationFrps(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareNodeObservationFrps, error) {
	rows, err := analyticsrepo.ListNodeObsFrps(ctx, toNodeObservabilityFilter(nodeID, since, limit))
	if err != nil {
		return nil, err
	}
	return fromAnalyticsNodeObsFrps(rows), nil
}

func (clickhouseObservabilityStore) DeleteAllNodeObservationFrps(ctx context.Context) (int64, error) {
	return analyticsrepo.DeleteAllNodeObsFrps(ctx)
}

func (clickhouseObservabilityStore) DeleteNodeObservationFrpsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return analyticsrepo.DeleteNodeObsFrpsBefore(ctx, cutoff)
}

func (clickhouseObservabilityStore) InsertNodeObservationFrpc(_ context.Context, record *OpenFlareNodeObservationFrpc) error {
	if record == nil {
		return nil
	}
	if hook := currentObservabilityInsertHooks().QueueFrpcObservation; hook != nil {
		hook(toAnalyticsNodeObsFrpc(record))
	}
	return nil
}

func (clickhouseObservabilityStore) ListNodeObservationFrpc(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareNodeObservationFrpc, error) {
	rows, err := analyticsrepo.ListNodeObsFrpc(ctx, toNodeObservabilityFilter(nodeID, since, limit))
	if err != nil {
		return nil, err
	}
	return fromAnalyticsNodeObsFrpc(rows), nil
}

func (clickhouseObservabilityStore) DeleteAllNodeObservationFrpc(ctx context.Context) (int64, error) {
	return analyticsrepo.DeleteAllNodeObsFrpc(ctx)
}

func (clickhouseObservabilityStore) DeleteNodeObservationFrpcBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return analyticsrepo.DeleteNodeObsFrpcBefore(ctx, cutoff)
}

func toNodeObservabilityFilter(nodeID string, since time.Time, limit int) analyticsrepo.NodeObservabilityFilter {
	return analyticsrepo.NodeObservabilityFilter{
		NodeID: nodeID,
		Since:  since,
		Limit:  limit,
	}
}

func toAnalyticsNodeMetricSnapshot(record *OpenFlareMetricSnapshot) analyticsmodel.NodeMetricSnapshot {
	return analyticsmodel.NodeMetricSnapshot{
		ID:                uint64(record.ID),
		NodeID:            record.NodeID,
		CapturedAt:        record.CapturedAt,
		CPUUsagePercent:   record.CPUUsagePercent,
		MemoryUsedBytes:   record.MemoryUsedBytes,
		MemoryTotalBytes:  record.MemoryTotalBytes,
		StorageUsedBytes:  record.StorageUsedBytes,
		StorageTotalBytes: record.StorageTotalBytes,
		DiskReadBytes:     record.DiskReadBytes,
		DiskWriteBytes:    record.DiskWriteBytes,
		NetworkRxBytes:    record.NetworkRxBytes,
		NetworkTxBytes:    record.NetworkTxBytes,
		CreatedAt:         record.CreatedAt,
	}
}

func fromAnalyticsNodeMetricSnapshots(rows []analyticsmodel.NodeMetricSnapshot) []*OpenFlareMetricSnapshot {
	result := make([]*OpenFlareMetricSnapshot, len(rows))
	for index, row := range rows {
		result[index] = &OpenFlareMetricSnapshot{
			ID:                uint(row.ID),
			NodeID:            row.NodeID,
			CapturedAt:        row.CapturedAt,
			CPUUsagePercent:   row.CPUUsagePercent,
			MemoryUsedBytes:   row.MemoryUsedBytes,
			MemoryTotalBytes:  row.MemoryTotalBytes,
			StorageUsedBytes:  row.StorageUsedBytes,
			StorageTotalBytes: row.StorageTotalBytes,
			DiskReadBytes:     row.DiskReadBytes,
			DiskWriteBytes:    row.DiskWriteBytes,
			NetworkRxBytes:    row.NetworkRxBytes,
			NetworkTxBytes:    row.NetworkTxBytes,
			CreatedAt:         row.CreatedAt,
		}
	}
	return result
}

func toAnalyticsNodeRequestReport(record *OpenFlareRequestReport) analyticsmodel.NodeRequestReport {
	return analyticsmodel.NodeRequestReport{
		ID:                  uint64(record.ID),
		NodeID:              record.NodeID,
		WindowStartedAt:     record.WindowStartedAt,
		WindowEndedAt:       record.WindowEndedAt,
		RequestCount:        record.RequestCount,
		ErrorCount:          record.ErrorCount,
		UniqueVisitorCount:  record.UniqueVisitorCount,
		StatusCodesJSON:     record.StatusCodesJSON,
		TopDomainsJSON:      record.TopDomainsJSON,
		SourceCountriesJSON: record.SourceCountriesJSON,
		CreatedAt:           record.CreatedAt,
	}
}

func fromAnalyticsNodeRequestReports(rows []analyticsmodel.NodeRequestReport) []*OpenFlareRequestReport {
	result := make([]*OpenFlareRequestReport, len(rows))
	for index, row := range rows {
		result[index] = &OpenFlareRequestReport{
			ID:                  uint(row.ID),
			NodeID:              row.NodeID,
			WindowStartedAt:     row.WindowStartedAt,
			WindowEndedAt:       row.WindowEndedAt,
			RequestCount:        row.RequestCount,
			ErrorCount:          row.ErrorCount,
			UniqueVisitorCount:  row.UniqueVisitorCount,
			StatusCodesJSON:     row.StatusCodesJSON,
			TopDomainsJSON:      row.TopDomainsJSON,
			SourceCountriesJSON: row.SourceCountriesJSON,
			CreatedAt:           row.CreatedAt,
		}
	}
	return result
}

func toAnalyticsNodeObsOpenresty(record *OpenFlareNodeObservationOpenresty) analyticsmodel.NodeObsOpenresty {
	return analyticsmodel.NodeObsOpenresty{
		ID:                   uint64(record.ID),
		NodeID:               record.NodeID,
		CapturedAt:           record.CapturedAt,
		OpenrestyRxBytes:     record.OpenrestyRxBytes,
		OpenrestyTxBytes:     record.OpenrestyTxBytes,
		OpenrestyConnections: record.OpenrestyConnections,
		CreatedAt:            record.CreatedAt,
	}
}

func fromAnalyticsNodeObsOpenresty(rows []analyticsmodel.NodeObsOpenresty) []*OpenFlareNodeObservationOpenresty {
	result := make([]*OpenFlareNodeObservationOpenresty, len(rows))
	for index, row := range rows {
		result[index] = &OpenFlareNodeObservationOpenresty{
			ID:                   uint(row.ID),
			NodeID:               row.NodeID,
			CapturedAt:           row.CapturedAt,
			OpenrestyRxBytes:     row.OpenrestyRxBytes,
			OpenrestyTxBytes:     row.OpenrestyTxBytes,
			OpenrestyConnections: row.OpenrestyConnections,
			CreatedAt:            row.CreatedAt,
		}
	}
	return result
}

func toAnalyticsNodeObsFrps(record *OpenFlareNodeObservationFrps) analyticsmodel.NodeObsFrps {
	return analyticsmodel.NodeObsFrps{
		ID:              uint64(record.ID),
		NodeID:          record.NodeID,
		CapturedAt:      record.CapturedAt,
		FrpsConnections: openFlareObservabilityIntToInt32(record.FrpsConnections),
		FrpsProxyCount:  openFlareObservabilityIntToInt32(record.FrpsProxyCount),
		FrpsClientCount: openFlareObservabilityIntToInt32(record.FrpsClientCount),
		FrpsProxies:     record.FrpsProxies,
		CreatedAt:       record.CreatedAt,
	}
}

func fromAnalyticsNodeObsFrps(rows []analyticsmodel.NodeObsFrps) []*OpenFlareNodeObservationFrps {
	result := make([]*OpenFlareNodeObservationFrps, len(rows))
	for index, row := range rows {
		result[index] = &OpenFlareNodeObservationFrps{
			ID:              uint(row.ID),
			NodeID:          row.NodeID,
			CapturedAt:      row.CapturedAt,
			FrpsConnections: int(row.FrpsConnections),
			FrpsProxyCount:  int(row.FrpsProxyCount),
			FrpsClientCount: int(row.FrpsClientCount),
			FrpsProxies:     row.FrpsProxies,
			CreatedAt:       row.CreatedAt,
		}
	}
	return result
}

func toAnalyticsNodeObsFrpc(record *OpenFlareNodeObservationFrpc) analyticsmodel.NodeObsFrpc {
	return analyticsmodel.NodeObsFrpc{
		ID:                   uint64(record.ID),
		NodeID:               record.NodeID,
		CapturedAt:           record.CapturedAt,
		TunnelStatus:         record.TunnelStatus,
		ConnectedRelaysCount: openFlareObservabilityIntToInt32(record.ConnectedRelaysCount),
		CreatedAt:            record.CreatedAt,
	}
}

func openFlareObservabilityIntToInt32(value int) int32 {
	switch {
	case value > math.MaxInt32:
		return math.MaxInt32
	case value < math.MinInt32:
		return math.MinInt32
	default:
		return int32(value)
	}
}

func fromAnalyticsNodeObsFrpc(rows []analyticsmodel.NodeObsFrpc) []*OpenFlareNodeObservationFrpc {
	result := make([]*OpenFlareNodeObservationFrpc, len(rows))
	for index, row := range rows {
		result[index] = &OpenFlareNodeObservationFrpc{
			ID:                   uint(row.ID),
			NodeID:               row.NodeID,
			CapturedAt:           row.CapturedAt,
			TunnelStatus:         row.TunnelStatus,
			ConnectedRelaysCount: int(row.ConnectedRelaysCount),
			CreatedAt:            row.CreatedAt,
		}
	}
	return result
}
