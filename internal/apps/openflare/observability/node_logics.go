// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/repository"

	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

const (
	defaultObservabilityWindow      = 24 * time.Hour
	realtimeObservabilityWindow     = 30 * time.Minute
	maxObservabilityHours           = 720
	defaultObservabilityLimit       = 120
	realtimeObservabilityLimit      = 180
	maxObservabilityLimit           = 500
	maxRawMetricQueryRows           = 20000
	defaultTrafficDistributionLimit = 8
	nodeObservabilityCacheTTL       = 15 * time.Second
)

var nodeObservabilityCache struct {
	mu    sync.Mutex
	views map[string]cachedNodeObservability
}

type cachedNodeObservability struct {
	view      *NodeView
	expiresAt time.Time
}

// NodeQuery filters node observability data.
type NodeQuery struct {
	Hours    int  `json:"hours"`
	Limit    int  `json:"limit"`
	Realtime bool `json:"realtime"`
}

// NodeAnalytics groups node observability analytics.
type NodeAnalytics struct {
	Traffic       *TrafficWindowSummary `json:"traffic"`
	Distributions TrafficDistributions  `json:"distributions"`
	Health        HealthSummary         `json:"health"`
}

// NodeTrends groups node observability trend series.
type NodeTrends struct {
	Traffic24h  []TrafficTrendPoint  `json:"traffic_24h"`
	Capacity24h []CapacityTrendPoint `json:"capacity_24h"`
	Network24h  []NetworkTrendPoint  `json:"network_24h"`
	DiskIO24h   []DiskIOTrendPoint   `json:"disk_io_24h"`
}

// NodeView is the node observability API response.
type NodeView struct {
	NodeID          string                            `json:"node_id"`
	Profile         *model.OpenFlareNodeSystemProfile `json:"profile"`
	MetricSnapshots []*NodeMetricSnapshotView         `json:"metric_snapshots"`
	HealthEvents    []*model.OpenFlareHealthEvent     `json:"health_events"`
	Analytics       NodeAnalytics                     `json:"analytics"`
	Trends          NodeTrends                        `json:"trends"`
}

// HealthEventCleanupResult reports health event cleanup outcome.
type HealthEventCleanupResult struct {
	NodeID       string `json:"node_id"`
	DeletedCount int64  `json:"deleted_count"`
}

// GetNodeObservability returns observability details for a node.
func GetNodeObservability(ctx context.Context, id uint, query NodeQuery) (*NodeView, error) {
	now := time.Now()
	node, err := repository.GetOpenFlareNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	limit := normalizeObservabilityLimit(query.Limit)
	window := normalizeObservabilityWindow(query.Hours)
	if query.Realtime {
		window = realtimeObservabilityWindow
		if query.Limit <= 0 {
			limit = realtimeObservabilityLimit
		}
	}
	cacheable := !query.Realtime && limit == defaultObservabilityLimit && window == defaultObservabilityWindow
	if cacheable {
		if view, ok := getCachedNodeObservability(node.NodeID); ok {
			return view, nil
		}
	}
	since := now.Add(-window)

	profile, err := repository.GetOpenFlareNodeSystemProfile(ctx, node.NodeID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		profile = nil
	}

	rawLimit := limit
	if !query.Realtime && window >= time.Hour {
		rawLimit = maxRawMetricQueryRows
	}
	rawSnapshots, err := repository.ListOpenFlareMetricSnapshotsSince(ctx, node.NodeID, since, rawLimit)
	if err != nil {
		return nil, err
	}
	snapshots := downsampleMetricSnapshots(rawSnapshots, since, now, limit)
	edgeHealth, err := repository.ListOpenFlareEdgeHealth(ctx, node.NodeID, since, limit)
	if err != nil {
		return nil, err
	}
	accessLogRegions, err := repository.ListOpenFlareAccessLogRegionCounts(ctx, node.NodeID, since, defaultTrafficDistributionLimit)
	if err != nil {
		return nil, err
	}
	events, err := repository.ListOpenFlareHealthEvents(ctx, node.NodeID, false, limit)
	if err != nil {
		return nil, err
	}
	distributions := BuildTrafficDistributionsFromAccessLogs(
		ctx, node.NodeID, since, now, defaultTrafficDistributionLimit, accessLogRegions,
	)
	trafficSummary := buildTrafficWindowSummaryFromAccessLogs(ctx, node.NodeID, since, now)
	view := &NodeView{
		NodeID:          node.NodeID,
		Profile:         profile,
		MetricSnapshots: BuildMetricSnapshotViews(snapshots, edgeHealth),
		HealthEvents:    events,
		Analytics: NodeAnalytics{
			Traffic:       trafficSummary,
			Distributions: distributions,
			Health:        buildHealthSummary(latestMetricSnapshot(rawSnapshots), trafficSummary, events),
		},
		Trends: BuildNodeTrends(ctx, now, node.NodeID, snapshots),
	}
	if cacheable {
		setCachedNodeObservability(node.NodeID, view)
	}
	return view, nil
}

func getCachedNodeObservability(nodeID string) (*NodeView, bool) {
	nodeObservabilityCache.mu.Lock()
	defer nodeObservabilityCache.mu.Unlock()
	if nodeObservabilityCache.views == nil {
		return nil, false
	}
	entry, ok := nodeObservabilityCache.views[nodeID]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.view, true
}

func setCachedNodeObservability(nodeID string, view *NodeView) {
	nodeObservabilityCache.mu.Lock()
	defer nodeObservabilityCache.mu.Unlock()
	if nodeObservabilityCache.views == nil {
		nodeObservabilityCache.views = make(map[string]cachedNodeObservability)
	}
	nodeObservabilityCache.views[nodeID] = cachedNodeObservability{
		view:      view,
		expiresAt: time.Now().Add(nodeObservabilityCacheTTL),
	}
}

// CleanupHealthEvents removes all health events for a node.
func CleanupHealthEvents(ctx context.Context, id uint) (*HealthEventCleanupResult, error) {
	node, err := repository.GetOpenFlareNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	deletedCount, err := repository.DeleteOpenFlareHealthEventsByNodeID(ctx, node.NodeID)
	if err != nil {
		return nil, err
	}
	return &HealthEventCleanupResult{
		NodeID:       node.NodeID,
		DeletedCount: deletedCount,
	}, nil
}

func normalizeObservabilityLimit(limit int) int {
	if limit <= 0 {
		return defaultObservabilityLimit
	}
	if limit > maxObservabilityLimit {
		return maxObservabilityLimit
	}
	return limit
}

func normalizeObservabilityWindow(hours int) time.Duration {
	if hours <= 0 {
		return defaultObservabilityWindow
	}
	if hours > maxObservabilityHours {
		hours = maxObservabilityHours
	}
	return time.Duration(hours) * time.Hour
}

func downsampleMetricSnapshots(snapshots []*model.OpenFlareMetricSnapshot, since, now time.Time, maxPoints int) []*model.OpenFlareMetricSnapshot {
	if len(snapshots) == 0 {
		return snapshots
	}
	ordered := make([]*model.OpenFlareMetricSnapshot, 0, len(snapshots))
	for _, s := range snapshots {
		if s != nil {
			ordered = append(ordered, s)
		}
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].CapturedAt.Before(ordered[j].CapturedAt)
	})
	if len(ordered) <= maxPoints || maxPoints <= 0 {
		return ordered
	}

	span := now.Sub(since)
	if span <= 0 {
		span = time.Hour
	}
	bucketDur := span / time.Duration(maxPoints)
	if bucketDur <= 0 {
		bucketDur = time.Second
	}

	type bucket struct {
		latest *model.OpenFlareMetricSnapshot
		count  int
		cpuSum float64
		gpuSum float64
		ramSum int64
		swpSum int64
		l1Sum  float64
		l5Sum  float64
		l15Sum float64
		tmpSum float64
		dskSum int64
		inSum  int64
		outSum int64
		prcSum int64
		tcpSum int64
		udpSum int64
	}

	buckets := make(map[int]*bucket, maxPoints)
	keys := make([]int, 0, maxPoints)
	for _, s := range ordered {
		idx := int(s.CapturedAt.Sub(since) / bucketDur)
		if idx < 0 {
			idx = 0
		}
		if idx >= maxPoints {
			idx = maxPoints - 1
		}
		b, ok := buckets[idx]
		if !ok {
			b = &bucket{latest: s}
			buckets[idx] = b
			keys = append(keys, idx)
		}
		b.latest = s
		b.count++
		b.cpuSum += s.CPUUsagePercent
		b.gpuSum += s.GPUUsagePercent
		b.ramSum += s.MemoryUsedBytes
		b.swpSum += s.SwapUsedBytes
		b.l1Sum += s.Load1
		b.l5Sum += s.Load5
		b.l15Sum += s.Load15
		b.tmpSum += s.Temp
		b.dskSum += s.StorageUsedBytes
		b.inSum += s.NetIn
		b.outSum += s.NetOut
		b.prcSum += int64(s.ProcessCount)
		b.tcpSum += int64(s.TCPConnections)
		b.udpSum += int64(s.UDPConnections)
	}
	sort.Ints(keys)

	result := make([]*model.OpenFlareMetricSnapshot, 0, len(keys))
	for _, idx := range keys {
		b := buckets[idx]
		n := float64(b.count)
		copySnap := *b.latest
		copySnap.CPUUsagePercent = b.cpuSum / n
		copySnap.GPUUsagePercent = b.gpuSum / n
		copySnap.MemoryUsedBytes = b.ramSum / int64(b.count)
		copySnap.SwapUsedBytes = b.swpSum / int64(b.count)
		copySnap.Load1 = b.l1Sum / n
		copySnap.Load5 = b.l5Sum / n
		copySnap.Load15 = b.l15Sum / n
		copySnap.Temp = b.tmpSum / n
		copySnap.StorageUsedBytes = b.dskSum / int64(b.count)
		copySnap.NetIn = b.inSum / int64(b.count)
		copySnap.NetOut = b.outSum / int64(b.count)
		copySnap.ProcessCount = int(b.prcSum / int64(b.count))
		copySnap.TCPConnections = int(b.tcpSum / int64(b.count))
		copySnap.UDPConnections = int(b.udpSum / int64(b.count))
		result = append(result, &copySnap)
	}
	return result
}
