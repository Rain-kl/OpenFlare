// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"

	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	analyticsrepo "github.com/Rain-kl/Wavelet/internal/repository/analytics"
)

// ObservabilityInsertHooks queues observability rows for async ClickHouse write.
// Wired from openflare/chwriter.Init so model never imports the apps layer.
type ObservabilityInsertHooks struct {
	QueueMetricSnapshot  func(analyticsmodel.NodeMetricSnapshot)
	QueueEdgeHealth      func(analyticsmodel.NodeEdgeHealth)
	QueueFrpsObservation func(analyticsmodel.NodeObsFrps)
	QueueFrpcObservation func(analyticsmodel.NodeObsFrpc)
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
	InsertMetricSnapshot(ctx context.Context, record *model.OpenFlareMetricSnapshot) error
	ListMetricSnapshots(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareMetricSnapshot, error)
	DeleteAllMetricSnapshots(ctx context.Context) (int64, error)
	DeleteMetricSnapshotsBefore(ctx context.Context, cutoff time.Time) (int64, error)

	InsertEdgeHealth(ctx context.Context, record *model.OpenFlareEdgeHealth) error
	ListEdgeHealth(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareEdgeHealth, error)
	DeleteAllEdgeHealth(ctx context.Context) (int64, error)
	DeleteEdgeHealthBefore(ctx context.Context, cutoff time.Time) (int64, error)

	InsertNodeObservationFrps(ctx context.Context, record *model.OpenFlareNodeObservationFrps) error
	ListNodeObservationFrps(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareNodeObservationFrps, error)
	DeleteAllNodeObservationFrps(ctx context.Context) (int64, error)
	DeleteNodeObservationFrpsBefore(ctx context.Context, cutoff time.Time) (int64, error)

	InsertNodeObservationFrpc(ctx context.Context, record *model.OpenFlareNodeObservationFrpc) error
	ListNodeObservationFrpc(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareNodeObservationFrpc, error)
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

func (clickhouseObservabilityStore) InsertMetricSnapshot(_ context.Context, record *model.OpenFlareMetricSnapshot) error {
	if record == nil {
		return nil
	}
	if hook := currentObservabilityInsertHooks().QueueMetricSnapshot; hook != nil {
		hook(toAnalyticsNodeMetricSnapshot(record))
	}
	return nil
}

func (clickhouseObservabilityStore) ListMetricSnapshots(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareMetricSnapshot, error) {
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

const edgeHealthStatusUnknown = "unknown"

func normalizeEdgeHealthStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return edgeHealthStatusUnknown
	}
	return status
}

func (clickhouseObservabilityStore) InsertEdgeHealth(_ context.Context, record *model.OpenFlareEdgeHealth) error {
	if record == nil {
		return nil
	}
	if hook := currentObservabilityInsertHooks().QueueEdgeHealth; hook != nil {
		hook(toAnalyticsNodeEdgeHealth(record))
	}
	return nil
}

func (clickhouseObservabilityStore) ListEdgeHealth(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareEdgeHealth, error) {
	rows, err := analyticsrepo.ListNodeEdgeHealth(ctx, toNodeObservabilityFilter(nodeID, since, limit))
	if err != nil {
		return nil, err
	}
	return fromAnalyticsNodeEdgeHealth(rows), nil
}

func (clickhouseObservabilityStore) DeleteAllEdgeHealth(ctx context.Context) (int64, error) {
	return analyticsrepo.DeleteAllNodeEdgeHealth(ctx)
}

func (clickhouseObservabilityStore) DeleteEdgeHealthBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return analyticsrepo.DeleteNodeEdgeHealthBefore(ctx, cutoff)
}

func (clickhouseObservabilityStore) InsertNodeObservationFrps(_ context.Context, record *model.OpenFlareNodeObservationFrps) error {
	if record == nil {
		return nil
	}
	if hook := currentObservabilityInsertHooks().QueueFrpsObservation; hook != nil {
		hook(toAnalyticsNodeObsFrps(record))
	}
	return nil
}

func (clickhouseObservabilityStore) ListNodeObservationFrps(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareNodeObservationFrps, error) {
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

func (clickhouseObservabilityStore) InsertNodeObservationFrpc(_ context.Context, record *model.OpenFlareNodeObservationFrpc) error {
	if record == nil {
		return nil
	}
	if hook := currentObservabilityInsertHooks().QueueFrpcObservation; hook != nil {
		hook(toAnalyticsNodeObsFrpc(record))
	}
	return nil
}

func (clickhouseObservabilityStore) ListNodeObservationFrpc(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareNodeObservationFrpc, error) {
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

func toAnalyticsNodeMetricSnapshot(record *model.OpenFlareMetricSnapshot) analyticsmodel.NodeMetricSnapshot {
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

func fromAnalyticsNodeMetricSnapshots(rows []analyticsmodel.NodeMetricSnapshot) []*model.OpenFlareMetricSnapshot {
	result := make([]*model.OpenFlareMetricSnapshot, len(rows))
	for index, row := range rows {
		result[index] = &model.OpenFlareMetricSnapshot{
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

func toAnalyticsNodeEdgeHealth(record *model.OpenFlareEdgeHealth) analyticsmodel.NodeEdgeHealth {
	return analyticsmodel.NodeEdgeHealth{
		ID:          uint64(record.ID),
		NodeID:      record.NodeID,
		CapturedAt:  record.CapturedAt,
		Status:      normalizeEdgeHealthStatus(record.Status),
		Connections: record.Connections,
		CreatedAt:   record.CreatedAt,
	}
}

func fromAnalyticsNodeEdgeHealth(rows []analyticsmodel.NodeEdgeHealth) []*model.OpenFlareEdgeHealth {
	result := make([]*model.OpenFlareEdgeHealth, len(rows))
	for index, row := range rows {
		result[index] = &model.OpenFlareEdgeHealth{
			ID:          uint(row.ID),
			NodeID:      row.NodeID,
			CapturedAt:  row.CapturedAt,
			Status:      normalizeEdgeHealthStatus(row.Status),
			Connections: row.Connections,
			CreatedAt:   row.CreatedAt,
		}
	}
	return result
}

func toAnalyticsNodeObsFrps(record *model.OpenFlareNodeObservationFrps) analyticsmodel.NodeObsFrps {
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

func fromAnalyticsNodeObsFrps(rows []analyticsmodel.NodeObsFrps) []*model.OpenFlareNodeObservationFrps {
	result := make([]*model.OpenFlareNodeObservationFrps, len(rows))
	for index, row := range rows {
		result[index] = &model.OpenFlareNodeObservationFrps{
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

func toAnalyticsNodeObsFrpc(record *model.OpenFlareNodeObservationFrpc) analyticsmodel.NodeObsFrpc {
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

func fromAnalyticsNodeObsFrpc(rows []analyticsmodel.NodeObsFrpc) []*model.OpenFlareNodeObservationFrpc {
	result := make([]*model.OpenFlareNodeObservationFrpc, len(rows))
	for index, row := range rows {
		result[index] = &model.OpenFlareNodeObservationFrpc{
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
