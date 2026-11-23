// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/repository"

	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

const (
	defaultObservabilityWindow      = 24 * time.Hour
	defaultObservabilityLimit       = 120
	maxObservabilityLimit           = 500
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
	Hours int `json:"hours"`
	Limit int `json:"limit"`
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
	cacheable := limit == defaultObservabilityLimit && window == defaultObservabilityWindow
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

	snapshots, err := repository.ListOpenFlareMetricSnapshotsSince(ctx, node.NodeID, since, limit)
	if err != nil {
		return nil, err
	}
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
			Health:        buildHealthSummary(latestMetricSnapshot(snapshots), trafficSummary, events),
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
	return time.Duration(hours) * time.Hour
}
