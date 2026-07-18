// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"context"
	"sort"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/observability"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// Summary is the dashboard node summary section.
type Summary struct {
	TotalNodes     int `json:"total_nodes"`
	OnlineNodes    int `json:"online_nodes"`
	OfflineNodes   int `json:"offline_nodes"`
	PendingNodes   int `json:"pending_nodes"`
	UnhealthyNodes int `json:"unhealthy_nodes"`
}

// Traffic is the dashboard traffic section.
type Traffic struct {
	RequestCount   int64   `json:"request_count"`
	UniqueVisitors int64   `json:"unique_visitors"`
	ErrorCount     int64   `json:"error_count"`
	EstimatedQPS   float64 `json:"estimated_qps"`
	ReportedNodes  int     `json:"reported_nodes"`
}

// Capacity is the dashboard capacity section.
type Capacity struct {
	AverageCPUUsagePercent    float64 `json:"average_cpu_usage_percent"`
	AverageMemoryUsagePercent float64 `json:"average_memory_usage_percent"`
	HighCPUNodes              int     `json:"high_cpu_nodes"`
	HighMemoryNodes           int     `json:"high_memory_nodes"`
	HighStorageNodes          int     `json:"high_storage_nodes"`
}

// NodeHealth is a dashboard node health row.
type NodeHealth struct {
	ID                  uint     `json:"id"`
	NodeID              string   `json:"node_id"`
	Name                string   `json:"name"`
	GeoName             string   `json:"geo_name"`
	GeoLatitude         *float64 `json:"geo_latitude"`
	GeoLongitude        *float64 `json:"geo_longitude"`
	Status              string   `json:"status"`
	OpenrestyStatus     string   `json:"openresty_status"`
	CurrentVersion      string   `json:"current_version"`
	LastSeenAt          any      `json:"last_seen_at"`
	ActiveEventCount    int      `json:"active_event_count"`
	CPUUsagePercent     float64  `json:"cpu_usage_percent"`
	MemoryUsagePercent  float64  `json:"memory_usage_percent"`
	StorageUsagePercent float64  `json:"storage_usage_percent"`
	RequestCount        int64    `json:"request_count"`
	ErrorCount          int64    `json:"error_count"`
	UniqueVisitorCount  int64    `json:"unique_visitor_count"`
}

// OverviewView is the expanded dashboard overview payload.
type OverviewView struct {
	GeneratedAt   time.Time                          `json:"generated_at"`
	Summary       Summary                            `json:"summary"`
	Traffic       Traffic                            `json:"traffic"`
	Capacity      Capacity                           `json:"capacity"`
	Distributions observability.TrafficDistributions `json:"distributions"`
	Trends        observability.NodeTrends           `json:"trends"`
	Nodes         []NodeHealth                       `json:"nodes"`
}

// OverviewPayload is the compact legacy dashboard overview response.
type OverviewPayload struct {
	GeneratedAt   any                  `json:"generated_at"`
	Summary       Summary              `json:"summary"`
	Traffic       Traffic              `json:"traffic"`
	Capacity      Capacity             `json:"capacity"`
	Distributions distributionsPayload `json:"distributions"`
	Trends        trendsPayload        `json:"trends"`
	Nodes         [][]any              `json:"nodes"`
}

type distributionsPayload struct {
	StatusCodes     [][]any `json:"status_codes"`
	TopDomains      [][]any `json:"top_domains"`
	SourceCountries [][]any `json:"source_countries"`
}

type trendsPayload struct {
	Traffic24h  [][]any `json:"traffic_24h"`
	Capacity24h [][]any `json:"capacity_24h"`
	Network24h  [][]any `json:"network_24h"`
	DiskIO24h   [][]any `json:"disk_io_24h"`
}

// GetOverview aggregates dashboard overview data from nodes and observability tables.
func GetOverview(ctx context.Context) (*OverviewPayload, error) {
	if payload, ok := getCachedOverview(); ok {
		return payload, nil
	}
	view, err := buildOverviewView(ctx)
	if err != nil {
		return nil, err
	}
	payload := compressOverview(view)
	setCachedOverview(payload)
	return payload, nil
}

func buildOverviewView(ctx context.Context) (*OverviewView, error) {
	now := time.Now()
	since := now.Add(-24 * time.Hour)

	nodes, err := model.ListOpenFlareNodes(ctx)
	if err != nil {
		return nil, err
	}
	// Latest-per-node health: dedicated LIMIT 1 BY queries (not a global raw LIMIT).
	latestSnapshotRows, err := model.ListOpenFlareLatestMetricSnapshotsSince(ctx, "", since)
	if err != nil {
		return nil, err
	}
	snapshots, err := model.ListOpenFlareMetricSnapshotsSince(ctx, "", since, dashboardOverviewSnapshotLimit)
	if err != nil {
		return nil, err
	}
	accessLogRegions, err := model.ListOpenFlareAccessLogRegionCounts(ctx, "", since, dashboardDistributionLimit)
	if err != nil {
		return nil, err
	}
	activeEvents, err := model.ListOpenFlareActiveHealthEvents(ctx)
	if err != nil {
		return nil, err
	}

	// L1 business: trends + distributions + totals from access logs only.
	view := &OverviewView{
		GeneratedAt: now,
		Nodes:       make([]NodeHealth, 0, len(nodes)),
		Distributions: observability.BuildTrafficDistributionsFromAccessLogs(
			ctx, since, now, dashboardDistributionLimit, accessLogRegions,
		),
		Trends: observability.BuildNodeTrends(ctx, now, "", snapshots),
	}

	// Global traffic summary uses true window uniqExact for UV (not sum of hourly uniques).
	if summary, sumErr := model.TrafficSummaryOpenFlareAccessLogs(ctx, model.OpenFlareAccessLogQuery{
		Since: since,
		Until: now,
	}); sumErr == nil {
		view.Traffic.RequestCount = summary.RequestCount
		view.Traffic.ErrorCount = summary.ErrorCount
		view.Traffic.UniqueVisitors = summary.UniqueIPCount
		view.Traffic.ReportedNodes = int(summary.NodeCount)
		if summary.RequestCount > 0 {
			// Average QPS over the 24h window.
			view.Traffic.EstimatedQPS = float64(summary.RequestCount) / (24 * 3600)
		}
	} else {
		// Fallback: sum hourly request/error buckets only (UV left from summary path).
		applyTrafficTotalsFromTrend(&view.Traffic, view.Trends.Traffic24h)
	}

	nodeTraffic := map[string]model.OpenFlareAccessLogNodeAggregate{}
	if aggregates, aggErr := model.NodeAggregatesOpenFlareAccessLogs(ctx, model.OpenFlareAccessLogQuery{
		Since: since,
		Until: now,
	}); aggErr == nil {
		for _, row := range aggregates {
			nodeTraffic[row.NodeID] = row
		}
	}

	var cpuNodeCount int
	var memoryNodeCount int
	latestSnapshots := observability.LatestMetricSnapshotsByNode(latestSnapshotRows)
	activeEventsByNode := observability.ActiveHealthEventsByNode(activeEvents)

	for _, node := range nodes {
		computedStatus := computeNodeStatus(&node)
		switch computedStatus {
		case nodeStatusOnline:
			view.Summary.OnlineNodes++
		case nodeStatusOffline:
			view.Summary.OfflineNodes++
		case nodeStatusPending:
			view.Summary.PendingNodes++
		}
		if node.OpenrestyStatus == "unhealthy" {
			view.Summary.UnhealthyNodes++
		}

		latestSnapshot := latestSnapshots[node.NodeID]
		nodeActiveEvents := activeEventsByNode[node.NodeID]

		nodeHealth := NodeHealth{
			ID:               node.ID,
			NodeID:           node.NodeID,
			Name:             node.Name,
			GeoName:          node.GeoName,
			GeoLatitude:      node.GeoLatitude,
			GeoLongitude:     node.GeoLongitude,
			Status:           computedStatus,
			OpenrestyStatus:  node.OpenrestyStatus,
			CurrentVersion:   node.CurrentVersion,
			LastSeenAt:       nodeViewLastSeenAt(&node),
			ActiveEventCount: len(nodeActiveEvents),
		}

		cpuNodeCount, memoryNodeCount = applyNodeSnapshotMetrics(&nodeHealth, latestSnapshot, view, cpuNodeCount, memoryNodeCount)
		if agg, ok := nodeTraffic[node.NodeID]; ok {
			nodeHealth.RequestCount = agg.RequestCount
			nodeHealth.ErrorCount = agg.ErrorCount
			nodeHealth.UniqueVisitorCount = agg.UniqueIPCount
		}

		view.Nodes = append(view.Nodes, nodeHealth)
	}

	view.Summary.TotalNodes = len(nodes)
	if cpuNodeCount > 0 {
		view.Capacity.AverageCPUUsagePercent /= float64(cpuNodeCount)
	}
	if memoryNodeCount > 0 {
		view.Capacity.AverageMemoryUsagePercent /= float64(memoryNodeCount)
	}

	sort.Slice(view.Nodes, func(i int, j int) bool {
		if view.Nodes[i].ActiveEventCount == view.Nodes[j].ActiveEventCount {
			return view.Nodes[i].CPUUsagePercent > view.Nodes[j].CPUUsagePercent
		}
		return view.Nodes[i].ActiveEventCount > view.Nodes[j].ActiveEventCount
	})

	return view, nil
}

func applyNodeSnapshotMetrics(nodeHealth *NodeHealth, snapshot *model.OpenFlareMetricSnapshot, view *OverviewView, cpuNodeCount, memoryNodeCount int) (int, int) {
	if snapshot == nil {
		return cpuNodeCount, memoryNodeCount
	}
	nodeHealth.CPUUsagePercent = snapshot.CPUUsagePercent
	nodeHealth.MemoryUsagePercent = observability.Percentage(snapshot.MemoryUsedBytes, snapshot.MemoryTotalBytes)
	nodeHealth.StorageUsagePercent = observability.Percentage(snapshot.StorageUsedBytes, snapshot.StorageTotalBytes)
	if snapshot.CPUUsagePercent > 0 {
		view.Capacity.AverageCPUUsagePercent += snapshot.CPUUsagePercent
		cpuNodeCount++
	}
	if nodeHealth.MemoryUsagePercent > 0 {
		view.Capacity.AverageMemoryUsagePercent += nodeHealth.MemoryUsagePercent
		memoryNodeCount++
	}
	if snapshot.CPUUsagePercent >= highCPUUsagePercentThreshold {
		view.Capacity.HighCPUNodes++
	}
	if nodeHealth.MemoryUsagePercent >= highMemoryUsagePercentThreshold {
		view.Capacity.HighMemoryNodes++
	}
	if nodeHealth.StorageUsagePercent >= highStorageUsagePercentThreshold {
		view.Capacity.HighStorageNodes++
	}
	return cpuNodeCount, memoryNodeCount
}

func applyTrafficTotalsFromTrend(traffic *Traffic, points []observability.TrafficTrendPoint) {
	if traffic == nil {
		return
	}
	traffic.RequestCount = 0
	traffic.ErrorCount = 0
	// Do not sum hourly unique visitors — that overcounts. UV must come from TrafficSummary.
	for _, point := range points {
		traffic.RequestCount += point.RequestCount
		traffic.ErrorCount += point.ErrorCount
	}
	if traffic.RequestCount > 0 && traffic.ReportedNodes == 0 {
		traffic.ReportedNodes = 1
	}
}

func compressOverview(view *OverviewView) *OverviewPayload {
	if view == nil {
		return &OverviewPayload{
			Distributions: distributionsPayload{
				StatusCodes:     [][]any{},
				TopDomains:      [][]any{},
				SourceCountries: [][]any{},
			},
			Trends: trendsPayload{
				Traffic24h:  [][]any{},
				Capacity24h: [][]any{},
				Network24h:  [][]any{},
				DiskIO24h:   [][]any{},
			},
			Nodes: [][]any{},
		}
	}
	return &OverviewPayload{
		GeneratedAt: view.GeneratedAt,
		Summary:     view.Summary,
		Traffic:     view.Traffic,
		Capacity:    view.Capacity,
		Distributions: distributionsPayload{
			StatusCodes:     compressDistributionItems(view.Distributions.StatusCodes),
			TopDomains:      compressDistributionItems(view.Distributions.TopDomains),
			SourceCountries: compressDistributionItems(view.Distributions.SourceCountries),
		},
		Trends: trendsPayload{
			Traffic24h:  compressTrafficTrendPoints(view.Trends.Traffic24h),
			Capacity24h: compressCapacityTrendPoints(view.Trends.Capacity24h),
			Network24h:  compressNetworkTrendPoints(view.Trends.Network24h),
			DiskIO24h:   compressDiskIOTrendPoints(view.Trends.DiskIO24h),
		},
		Nodes: compressDashboardNodes(view.Nodes),
	}
}

func compressDistributionItems(items []observability.DistributionItem) [][]any {
	rows := make([][]any, 0, len(items))
	for _, item := range items {
		rows = append(rows, []any{item.Key, item.Value})
	}
	return rows
}

func compressTrafficTrendPoints(points []observability.TrafficTrendPoint) [][]any {
	rows := make([][]any, 0, len(points))
	for _, point := range points {
		rows = append(rows, []any{
			point.BucketStartedAt,
			point.RequestCount,
			point.ErrorCount,
			point.UniqueVisitorCount,
		})
	}
	return rows
}

func compressCapacityTrendPoints(points []observability.CapacityTrendPoint) [][]any {
	rows := make([][]any, 0, len(points))
	for _, point := range points {
		rows = append(rows, []any{
			point.BucketStartedAt,
			point.AverageCPUUsagePercent,
			point.AverageMemoryUsagePercent,
			point.ReportedNodes,
		})
	}
	return rows
}

func compressNetworkTrendPoints(points []observability.NetworkTrendPoint) [][]any {
	rows := make([][]any, 0, len(points))
	for _, point := range points {
		// Compact layout (stable positions):
		// [0] bucket, [1] host_rx, [2] host_tx, [3] bytes_received, [4] bytes_provided, [5] reported_nodes
		rows = append(rows, []any{
			point.BucketStartedAt,
			point.NetworkRxBytes,
			point.NetworkTxBytes,
			point.BytesReceived,
			point.BytesProvided,
			point.ReportedNodes,
		})
	}
	return rows
}

func compressDiskIOTrendPoints(points []observability.DiskIOTrendPoint) [][]any {
	rows := make([][]any, 0, len(points))
	for _, point := range points {
		rows = append(rows, []any{
			point.BucketStartedAt,
			point.DiskReadBytes,
			point.DiskWriteBytes,
			point.ReportedNodes,
		})
	}
	return rows
}

func compressDashboardNodes(nodes []NodeHealth) [][]any {
	rows := make([][]any, 0, len(nodes))
	for _, node := range nodes {
		rows = append(rows, []any{
			node.ID,
			node.NodeID,
			node.Name,
			node.GeoName,
			node.GeoLatitude,
			node.GeoLongitude,
			node.Status,
			node.OpenrestyStatus,
			node.CurrentVersion,
			node.LastSeenAt,
			node.ActiveEventCount,
			node.CPUUsagePercent,
			node.MemoryUsagePercent,
			node.StorageUsagePercent,
			node.RequestCount,
			node.ErrorCount,
			node.UniqueVisitorCount,
		})
	}
	return rows
}
