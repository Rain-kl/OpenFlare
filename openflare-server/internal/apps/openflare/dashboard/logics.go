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
	view, err := buildOverviewView(ctx)
	if err != nil {
		return nil, err
	}
	return compressOverview(view), nil
}

func buildOverviewView(ctx context.Context) (*OverviewView, error) {
	now := time.Now()
	since := now.Add(-24 * time.Hour)

	nodes, err := model.ListOpenFlareNodes(ctx)
	if err != nil {
		return nil, err
	}
	snapshots, err := model.ListOpenFlareMetricSnapshotsSince(ctx, "", since, 0)
	if err != nil {
		return nil, err
	}
	reports, err := model.ListOpenFlareRequestReportsSince(ctx, "", since, 0)
	if err != nil {
		return nil, err
	}
	accessLogRegions, err := model.ListOpenFlareAccessLogRegionCounts(ctx, "", since, 8)
	if err != nil {
		return nil, err
	}
	activeEvents, err := model.ListOpenFlareActiveHealthEvents(ctx)
	if err != nil {
		return nil, err
	}
	openrestySnapshots, err := model.ListOpenFlareNodeObservationOpenresty(ctx, "", since, 0)
	if err != nil {
		return nil, err
	}

	view := &OverviewView{
		GeneratedAt:   now,
		Nodes:         make([]NodeHealth, 0, len(nodes)),
		Distributions: observability.BuildTrafficDistributions(reports, accessLogRegions, 8),
		Trends: observability.NodeTrends{
			Traffic24h:  observability.BuildTrafficTrendPoints(now, reports),
			Capacity24h: observability.BuildCapacityTrendPoints(now, snapshots),
			Network24h:  observability.BuildNetworkTrendPoints(now, snapshots, openrestySnapshots),
			DiskIO24h:   observability.BuildDiskIOTrendPoints(now, snapshots),
		},
	}

	var cpuNodeCount int
	var memoryNodeCount int
	latestSnapshots := observability.LatestMetricSnapshotsByNode(snapshots)
	latestTrafficReports := observability.LatestTrafficReportsByNode(reports)
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
		latestTraffic := latestTrafficReports[node.NodeID]
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

		if latestSnapshot != nil {
			nodeHealth.CPUUsagePercent = latestSnapshot.CPUUsagePercent
			nodeHealth.MemoryUsagePercent = observability.Percentage(latestSnapshot.MemoryUsedBytes, latestSnapshot.MemoryTotalBytes)
			nodeHealth.StorageUsagePercent = observability.Percentage(latestSnapshot.StorageUsedBytes, latestSnapshot.StorageTotalBytes)
			if latestSnapshot.CPUUsagePercent > 0 {
				view.Capacity.AverageCPUUsagePercent += latestSnapshot.CPUUsagePercent
				cpuNodeCount++
			}
			if nodeHealth.MemoryUsagePercent > 0 {
				view.Capacity.AverageMemoryUsagePercent += nodeHealth.MemoryUsagePercent
				memoryNodeCount++
			}
			if latestSnapshot.CPUUsagePercent >= 80 {
				view.Capacity.HighCPUNodes++
			}
			if nodeHealth.MemoryUsagePercent >= 85 {
				view.Capacity.HighMemoryNodes++
			}
			if nodeHealth.StorageUsagePercent >= 85 {
				view.Capacity.HighStorageNodes++
			}
		}

		if latestTraffic != nil {
			nodeHealth.RequestCount = latestTraffic.RequestCount
			nodeHealth.ErrorCount = latestTraffic.ErrorCount
			nodeHealth.UniqueVisitorCount = latestTraffic.UniqueVisitorCount
			view.Traffic.RequestCount += latestTraffic.RequestCount
			view.Traffic.UniqueVisitors += latestTraffic.UniqueVisitorCount
			view.Traffic.ErrorCount += latestTraffic.ErrorCount
			if duration := latestTraffic.WindowEndedAt.Sub(latestTraffic.WindowStartedAt).Seconds(); duration > 0 {
				view.Traffic.EstimatedQPS += float64(latestTraffic.RequestCount) / duration
			}
			view.Traffic.ReportedNodes++
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
		rows = append(rows, []any{
			point.BucketStartedAt,
			point.NetworkRxBytes,
			point.NetworkTxBytes,
			point.OpenrestyRxBytes,
			point.OpenrestyTxBytes,
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
