// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

const (
	defaultObservabilityWindow = 24 * time.Hour
	defaultObservabilityLimit  = 120
	maxObservabilityLimit      = 500
)

// NodeQuery filters node observability data.
type NodeQuery struct {
	Hours int `json:"hours"`
	Limit int `json:"limit"`
}

// NodeAnalytics groups node observability analytics.
type NodeAnalytics struct {
	Traffic       TrafficWindowSummary `json:"traffic"`
	Distributions TrafficDistributions `json:"distributions"`
	Health        HealthSummary        `json:"health"`
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
	MetricSnapshots []*model.OpenFlareMetricSnapshot  `json:"metric_snapshots"`
	TrafficReports  []*model.OpenFlareRequestReport   `json:"traffic_reports"`
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
	reports, err := model.ListOpenFlareRequestReportsSince(ctx, node.NodeID, since, limit)
	if err != nil {
		return nil, err
	}
	accessLogRegions, err := model.ListOpenFlareAccessLogRegionCounts(ctx, node.NodeID, since, 8)
	if err != nil {
		return nil, err
	}
	trendSnapshots, err := model.ListOpenFlareMetricSnapshotsSince(ctx, node.NodeID, now.Add(-24*time.Hour), 0)
	if err != nil {
		return nil, err
	}
	trendOpenresty, err := model.ListOpenFlareNodeObservationOpenresty(ctx, node.NodeID, now.Add(-24*time.Hour), 0)
	if err != nil {
		return nil, err
	}
	trendReports, err := model.ListOpenFlareRequestReportsSince(ctx, node.NodeID, now.Add(-24*time.Hour), 0)
	if err != nil {
		return nil, err
	}
	events, err := model.ListOpenFlareHealthEvents(ctx, node.NodeID, false, limit)
	if err != nil {
		return nil, err
	}

	view := &NodeView{
		NodeID:          node.NodeID,
		Profile:         profile,
		MetricSnapshots: snapshots,
		TrafficReports:  reports,
		HealthEvents:    events,
		Analytics: NodeAnalytics{
			Traffic:       buildTrafficWindowSummary(latestTrafficReport(reports)),
			Distributions: BuildTrafficDistributions(reports, accessLogRegions, 8),
			Health:        buildHealthSummary(latestMetricSnapshot(snapshots), latestTrafficReport(reports), events),
		},
		Trends: NodeTrends{
			Traffic24h:  BuildTrafficTrendPoints(now, trendReports),
			Capacity24h: BuildCapacityTrendPoints(now, trendSnapshots),
			Network24h:  BuildNetworkTrendPoints(now, trendSnapshots, trendOpenresty),
			DiskIO24h:   BuildDiskIOTrendPoints(now, trendSnapshots),
		},
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
	return view, nil
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
