// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db/idgen"
)

type memoryObservabilityStore struct {
	mu              sync.RWMutex
	metricSnapshots []*OpenFlareMetricSnapshot
	edgeHealth      []*OpenFlareEdgeHealth
	frpsObs         []*OpenFlareNodeObservationFrps
	frpcObs         []*OpenFlareNodeObservationFrpc
}

func (s *memoryObservabilityStore) InsertMetricSnapshot(_ context.Context, record *OpenFlareMetricSnapshot) error {
	if record == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	copyRecord := cloneOpenFlareMetricSnapshot(record)
	if memoryMetricSnapshotExists(s.metricSnapshots, copyRecord.NodeID, copyRecord.CapturedAt) {
		return nil
	}
	s.metricSnapshots = append(s.metricSnapshots, copyRecord)
	return nil
}

func (s *memoryObservabilityStore) ListMetricSnapshots(_ context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareMetricSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := memoryFilterMetricSnapshots(s.metricSnapshots, nodeID, since)
	sortOpenFlareMetricSnapshots(rows)
	return memoryLimitObservabilityRows(rows, limit), nil
}

func (s *memoryObservabilityStore) DeleteAllMetricSnapshots(_ context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := int64(len(s.metricSnapshots))
	s.metricSnapshots = nil
	return count, nil
}

func (s *memoryObservabilityStore) DeleteMetricSnapshotsBefore(_ context.Context, cutoff time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff = cutoff.UTC()
	remaining := make([]*OpenFlareMetricSnapshot, 0, len(s.metricSnapshots))
	var deleted int64
	for _, row := range s.metricSnapshots {
		if row.CapturedAt.Before(cutoff) {
			deleted++
			continue
		}
		remaining = append(remaining, row)
	}
	s.metricSnapshots = remaining
	return deleted, nil
}

func (s *memoryObservabilityStore) InsertEdgeHealth(_ context.Context, record *OpenFlareEdgeHealth) error {
	if record == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.edgeHealth = append(s.edgeHealth, cloneOpenFlareEdgeHealth(record))
	return nil
}

func (s *memoryObservabilityStore) ListEdgeHealth(_ context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareEdgeHealth, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := memoryFilterEdgeHealth(s.edgeHealth, nodeID, since)
	sortOpenFlareEdgeHealth(rows)
	return memoryLimitObservabilityRows(rows, limit), nil
}

func (s *memoryObservabilityStore) DeleteAllEdgeHealth(_ context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := int64(len(s.edgeHealth))
	s.edgeHealth = nil
	return count, nil
}

func (s *memoryObservabilityStore) DeleteEdgeHealthBefore(_ context.Context, cutoff time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff = cutoff.UTC()
	remaining := make([]*OpenFlareEdgeHealth, 0, len(s.edgeHealth))
	var deleted int64
	for _, row := range s.edgeHealth {
		if row.CapturedAt.Before(cutoff) {
			deleted++
			continue
		}
		remaining = append(remaining, row)
	}
	s.edgeHealth = remaining
	return deleted, nil
}

func (s *memoryObservabilityStore) InsertNodeObservationFrps(_ context.Context, record *OpenFlareNodeObservationFrps) error {
	if record == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.frpsObs = append(s.frpsObs, cloneOpenFlareNodeObservationFrps(record))
	return nil
}

func (s *memoryObservabilityStore) ListNodeObservationFrps(_ context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareNodeObservationFrps, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := memoryFilterFrpsObservations(s.frpsObs, nodeID, since)
	sortOpenFlareNodeObservationFrps(rows)
	return memoryLimitObservabilityRows(rows, limit), nil
}

func (s *memoryObservabilityStore) DeleteAllNodeObservationFrps(_ context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := int64(len(s.frpsObs))
	s.frpsObs = nil
	return count, nil
}

func (s *memoryObservabilityStore) DeleteNodeObservationFrpsBefore(_ context.Context, cutoff time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff = cutoff.UTC()
	remaining := make([]*OpenFlareNodeObservationFrps, 0, len(s.frpsObs))
	var deleted int64
	for _, row := range s.frpsObs {
		if row.CapturedAt.Before(cutoff) {
			deleted++
			continue
		}
		remaining = append(remaining, row)
	}
	s.frpsObs = remaining
	return deleted, nil
}

func (s *memoryObservabilityStore) InsertNodeObservationFrpc(_ context.Context, record *OpenFlareNodeObservationFrpc) error {
	if record == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.frpcObs = append(s.frpcObs, cloneOpenFlareNodeObservationFrpc(record))
	return nil
}

func (s *memoryObservabilityStore) ListNodeObservationFrpc(_ context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareNodeObservationFrpc, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := memoryFilterFrpcObservations(s.frpcObs, nodeID, since)
	sortOpenFlareNodeObservationFrpc(rows)
	return memoryLimitObservabilityRows(rows, limit), nil
}

func (s *memoryObservabilityStore) DeleteAllNodeObservationFrpc(_ context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := int64(len(s.frpcObs))
	s.frpcObs = nil
	return count, nil
}

func (s *memoryObservabilityStore) DeleteNodeObservationFrpcBefore(_ context.Context, cutoff time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff = cutoff.UTC()
	remaining := make([]*OpenFlareNodeObservationFrpc, 0, len(s.frpcObs))
	var deleted int64
	for _, row := range s.frpcObs {
		if row.CapturedAt.Before(cutoff) {
			deleted++
			continue
		}
		remaining = append(remaining, row)
	}
	s.frpcObs = remaining
	return deleted, nil
}

func memoryFilterMetricSnapshots(rows []*OpenFlareMetricSnapshot, nodeID string, since time.Time) []*OpenFlareMetricSnapshot {
	result := make([]*OpenFlareMetricSnapshot, 0, len(rows))
	for _, row := range rows {
		if !memoryObservabilityMatchesNodeID(row.NodeID, nodeID) {
			continue
		}
		if !since.IsZero() && row.CapturedAt.Before(since) {
			continue
		}
		result = append(result, row)
	}
	return result
}

func memoryFilterEdgeHealth(rows []*OpenFlareEdgeHealth, nodeID string, since time.Time) []*OpenFlareEdgeHealth {
	result := make([]*OpenFlareEdgeHealth, 0, len(rows))
	for _, row := range rows {
		if !memoryObservabilityMatchesNodeID(row.NodeID, nodeID) {
			continue
		}
		if !since.IsZero() && row.CapturedAt.Before(since) {
			continue
		}
		result = append(result, row)
	}
	return result
}

func memoryFilterFrpsObservations(rows []*OpenFlareNodeObservationFrps, nodeID string, since time.Time) []*OpenFlareNodeObservationFrps {
	result := make([]*OpenFlareNodeObservationFrps, 0, len(rows))
	for _, row := range rows {
		if !memoryObservabilityMatchesNodeID(row.NodeID, nodeID) {
			continue
		}
		if !since.IsZero() && row.CapturedAt.Before(since) {
			continue
		}
		result = append(result, row)
	}
	return result
}

func memoryFilterFrpcObservations(rows []*OpenFlareNodeObservationFrpc, nodeID string, since time.Time) []*OpenFlareNodeObservationFrpc {
	result := make([]*OpenFlareNodeObservationFrpc, 0, len(rows))
	for _, row := range rows {
		if !memoryObservabilityMatchesNodeID(row.NodeID, nodeID) {
			continue
		}
		if !since.IsZero() && row.CapturedAt.Before(since) {
			continue
		}
		result = append(result, row)
	}
	return result
}

func memoryObservabilityMatchesNodeID(rowNodeID string, nodeID string) bool {
	trimmed := strings.TrimSpace(nodeID)
	if trimmed == "" {
		return true
	}
	return rowNodeID == trimmed
}

func memoryMetricSnapshotExists(rows []*OpenFlareMetricSnapshot, nodeID string, capturedAt time.Time) bool {
	capturedAt = capturedAt.UTC()
	for _, row := range rows {
		if row.NodeID == nodeID && row.CapturedAt.UTC().Equal(capturedAt) {
			return true
		}
	}
	return false
}

func sortOpenFlareMetricSnapshots(items []*OpenFlareMetricSnapshot) {
	sort.Slice(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if left == nil || right == nil {
			return left != nil
		}
		if compare := openFlareAccessLogCompareInt64(left.CapturedAt.Unix(), right.CapturedAt.Unix()); compare != 0 {
			return compare > 0
		}
		return openFlareAccessLogCompareInt64(openFlareAccessLogUintToInt64(uint64(left.ID)), openFlareAccessLogUintToInt64(uint64(right.ID))) > 0
	})
}

func sortOpenFlareEdgeHealth(items []*OpenFlareEdgeHealth) {
	sort.Slice(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if left == nil || right == nil {
			return left != nil
		}
		if compare := openFlareAccessLogCompareInt64(left.CapturedAt.Unix(), right.CapturedAt.Unix()); compare != 0 {
			return compare > 0
		}
		return openFlareAccessLogCompareInt64(openFlareAccessLogUintToInt64(uint64(left.ID)), openFlareAccessLogUintToInt64(uint64(right.ID))) > 0
	})
}

func sortOpenFlareNodeObservationFrps(items []*OpenFlareNodeObservationFrps) {
	sort.Slice(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if left == nil || right == nil {
			return left != nil
		}
		if compare := openFlareAccessLogCompareInt64(left.CapturedAt.Unix(), right.CapturedAt.Unix()); compare != 0 {
			return compare > 0
		}
		return openFlareAccessLogCompareInt64(openFlareAccessLogUintToInt64(uint64(left.ID)), openFlareAccessLogUintToInt64(uint64(right.ID))) > 0
	})
}

func sortOpenFlareNodeObservationFrpc(items []*OpenFlareNodeObservationFrpc) {
	sort.Slice(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if left == nil || right == nil {
			return left != nil
		}
		if compare := openFlareAccessLogCompareInt64(left.CapturedAt.Unix(), right.CapturedAt.Unix()); compare != 0 {
			return compare > 0
		}
		return openFlareAccessLogCompareInt64(openFlareAccessLogUintToInt64(uint64(left.ID)), openFlareAccessLogUintToInt64(uint64(right.ID))) > 0
	})
}

func memoryLimitObservabilityRows[T any](rows []T, limit int) []T {
	if limit <= 0 || len(rows) <= limit {
		result := make([]T, len(rows))
		copy(result, rows)
		return result
	}
	result := make([]T, limit)
	copy(result, rows[:limit])
	return result
}

func cloneOpenFlareMetricSnapshot(record *OpenFlareMetricSnapshot) *OpenFlareMetricSnapshot {
	copyRecord := *record
	if copyRecord.ID == 0 {
		copyRecord.ID = uint(idgen.NextUint64ID())
	}
	now := time.Now().UTC()
	if copyRecord.CreatedAt.IsZero() {
		copyRecord.CreatedAt = now
	}
	copyRecord.CapturedAt = copyRecord.CapturedAt.UTC()
	copyRecord.CreatedAt = copyRecord.CreatedAt.UTC()
	return &copyRecord
}

func cloneOpenFlareEdgeHealth(record *OpenFlareEdgeHealth) *OpenFlareEdgeHealth {
	copyRecord := *record
	if copyRecord.ID == 0 {
		copyRecord.ID = uint(idgen.NextUint64ID())
	}
	now := time.Now().UTC()
	if copyRecord.CreatedAt.IsZero() {
		copyRecord.CreatedAt = now
	}
	if copyRecord.CapturedAt.IsZero() {
		copyRecord.CapturedAt = now
	}
	if strings.TrimSpace(copyRecord.Status) == "" {
		copyRecord.Status = edgeHealthStatusUnknown
	}
	copyRecord.CapturedAt = copyRecord.CapturedAt.UTC()
	copyRecord.CreatedAt = copyRecord.CreatedAt.UTC()
	return &copyRecord
}

func cloneOpenFlareNodeObservationFrps(record *OpenFlareNodeObservationFrps) *OpenFlareNodeObservationFrps {
	copyRecord := *record
	if copyRecord.ID == 0 {
		copyRecord.ID = uint(idgen.NextUint64ID())
	}
	now := time.Now().UTC()
	if copyRecord.CreatedAt.IsZero() {
		copyRecord.CreatedAt = now
	}
	if copyRecord.CapturedAt.IsZero() {
		copyRecord.CapturedAt = now
	}
	copyRecord.CapturedAt = copyRecord.CapturedAt.UTC()
	copyRecord.CreatedAt = copyRecord.CreatedAt.UTC()
	return &copyRecord
}

func cloneOpenFlareNodeObservationFrpc(record *OpenFlareNodeObservationFrpc) *OpenFlareNodeObservationFrpc {
	copyRecord := *record
	if copyRecord.ID == 0 {
		copyRecord.ID = uint(idgen.NextUint64ID())
	}
	now := time.Now().UTC()
	if copyRecord.CreatedAt.IsZero() {
		copyRecord.CreatedAt = now
	}
	if copyRecord.CapturedAt.IsZero() {
		copyRecord.CapturedAt = now
	}
	copyRecord.CapturedAt = copyRecord.CapturedAt.UTC()
	copyRecord.CreatedAt = copyRecord.CreatedAt.UTC()
	return &copyRecord
}
