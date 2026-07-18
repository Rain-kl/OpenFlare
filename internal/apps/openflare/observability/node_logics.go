// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

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

// RelayDashboardSnapshot summarizes tunnel relay status.
type RelayDashboardSnapshot struct {
	TotalProxies     int              `json:"total_proxies"`
	OnlineProxies    int              `json:"online_proxies"`
	OfflineProxies   int              `json:"offline_proxies"`
	Proxies          []RelayProxyStat `json:"proxies"`
	TotalConnections int              `json:"total_connections"`
	ClientCounts     int              `json:"client_counts"`
}

// RelayProxyStat is a single relay proxy entry.
type RelayProxyStat struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Status        string `json:"status"`
	ClientVersion string `json:"client_version"`
	LastStartTime string `json:"last_start_time"`
	LastCloseTime string `json:"last_close_time"`
	ClientAddr    string `json:"client_addr"`
}

// NodeView is the node observability API response.
type NodeView struct {
	NodeID          string                            `json:"node_id"`
	Profile         *model.OpenFlareNodeSystemProfile `json:"profile"`
	MetricSnapshots []*NodeMetricSnapshotView         `json:"metric_snapshots"`
	HealthEvents    []*model.OpenFlareHealthEvent     `json:"health_events"`
	Analytics       NodeAnalytics                     `json:"analytics"`
	Trends          NodeTrends                        `json:"trends"`
	RelayDashboard  *RelayDashboardSnapshot           `json:"relay_dashboard,omitempty"`
}

// HealthEventCleanupResult reports health event cleanup outcome.
type HealthEventCleanupResult struct {
	NodeID       string `json:"node_id"`
	DeletedCount int64  `json:"deleted_count"`
}

// GetNodeObservability returns observability details for a node.
func GetNodeObservability(ctx context.Context, id uint, query NodeQuery) (*NodeView, error) {
	now := time.Now()
	node, err := model.GetOpenFlareNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if view, ok := getCachedNodeObservability(node.NodeID); ok {
		return view, nil
	}

	limit := normalizeObservabilityLimit(query.Limit)
	since := now.Add(-normalizeObservabilityWindow(query.Hours))

	profile, err := model.GetOpenFlareNodeSystemProfile(ctx, node.NodeID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		profile = nil
	}

	snapshots, err := model.ListOpenFlareMetricSnapshotsSince(ctx, node.NodeID, since, limit)
	if err != nil {
		return nil, err
	}
	edgeHealth, err := model.ListOpenFlareEdgeHealth(ctx, node.NodeID, since, limit)
	if err != nil {
		return nil, err
	}
	accessLogRegions, err := model.ListOpenFlareAccessLogRegionCounts(ctx, node.NodeID, since, defaultTrafficDistributionLimit)
	if err != nil {
		return nil, err
	}
	events, err := model.ListOpenFlareHealthEvents(ctx, node.NodeID, false, limit)
	if err != nil {
		return nil, err
	}
	distributions := BuildTrafficDistributionsFromAccessLogs(
		ctx, since, now, defaultTrafficDistributionLimit, accessLogRegions,
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
	if node.NodeType == "tunnel_relay" {
		frpsObs, frpsErr := model.ListOpenFlareNodeObservationFrps(ctx, node.NodeID, time.Time{}, 1)
		if frpsErr != nil {
			return nil, frpsErr
		}
		var latestFrps *model.OpenFlareNodeObservationFrps
		if len(frpsObs) > 0 {
			latestFrps = frpsObs[0]
		}
		view.RelayDashboard = buildRelayDashboardSnapshot(node, latestFrps)
	}
	setCachedNodeObservability(node.NodeID, view)
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
	node, err := model.GetOpenFlareNodeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	deletedCount, err := model.DeleteOpenFlareHealthEventsByNodeID(ctx, node.NodeID)
	if err != nil {
		return nil, err
	}
	return &HealthEventCleanupResult{
		NodeID:       node.NodeID,
		DeletedCount: deletedCount,
	}, nil
}

func buildRelayDashboardSnapshot(node *model.OpenFlareNode, obs *model.OpenFlareNodeObservationFrps) *RelayDashboardSnapshot {
	if node == nil {
		return nil
	}
	totalProxies := 0
	totalConnections := 0
	clientCounts := 0
	proxies := []RelayProxyStat{}

	if obs != nil {
		totalProxies = obs.FrpsProxyCount
		totalConnections = obs.FrpsConnections
		clientCounts = obs.FrpsClientCount
		if obs.FrpsProxies != "" {
			var decoded []RelayProxyStat
			if err := json.Unmarshal([]byte(obs.FrpsProxies), &decoded); err == nil {
				proxies = decoded
			}
		}
	}
	if totalProxies < 0 {
		totalProxies = 0
	}
	onlineProxies := 0
	for _, proxy := range proxies {
		if proxy.Status == "online" {
			onlineProxies++
		}
	}
	if len(proxies) == 0 {
		onlineProxies = totalProxies
		if node.RelayStatus != "healthy" {
			onlineProxies = 0
		}
	}

	return &RelayDashboardSnapshot{
		TotalProxies:     totalProxies,
		OnlineProxies:    onlineProxies,
		OfflineProxies:   totalProxies - onlineProxies,
		Proxies:          proxies,
		TotalConnections: maxInt(totalConnections, 0),
		ClientCounts:     maxInt(clientCounts, 0),
	}
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
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
