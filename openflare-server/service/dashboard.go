package service

import (
	"sort"
	"time"

	"github.com/rain-kl/openflare/openflare-server/model"
)

type DashboardOverviewView struct {
	GeneratedAt   time.Time             `json:"generated_at"`
	Summary       DashboardSummary      `json:"summary"`
	Traffic       DashboardTraffic      `json:"traffic"`
	Capacity      DashboardCapacity     `json:"capacity"`
	Distributions TrafficDistributions  `json:"distributions"`
	Trends        DashboardTrends       `json:"trends"`
	Nodes         []DashboardNodeHealth `json:"nodes"`
}

type DashboardSummary struct {
	TotalNodes     int `json:"total_nodes"`
	OnlineNodes    int `json:"online_nodes"`
	OfflineNodes   int `json:"offline_nodes"`
	PendingNodes   int `json:"pending_nodes"`
	UnhealthyNodes int `json:"unhealthy_nodes"`
}

type DashboardTraffic struct {
	RequestCount   int64   `json:"request_count"`
	UniqueVisitors int64   `json:"unique_visitors"`
	ErrorCount     int64   `json:"error_count"`
	EstimatedQPS   float64 `json:"estimated_qps"`
	ReportedNodes  int     `json:"reported_nodes"`
}

type DashboardCapacity struct {
	AverageCPUUsagePercent    float64 `json:"average_cpu_usage_percent"`
	AverageMemoryUsagePercent float64 `json:"average_memory_usage_percent"`
	HighCPUNodes              int     `json:"high_cpu_nodes"`
	HighMemoryNodes           int     `json:"high_memory_nodes"`
	HighStorageNodes          int     `json:"high_storage_nodes"`
}

type DashboardTrends struct {
	Traffic24h  []TrafficTrendPoint  `json:"traffic_24h"`
	Capacity24h []CapacityTrendPoint `json:"capacity_24h"`
	Network24h  []NetworkTrendPoint  `json:"network_24h"`
	DiskIO24h   []DiskIOTrendPoint   `json:"disk_io_24h"`
}

type DashboardNodeHealth struct {
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

func GetDashboardOverview() (*DashboardOverviewView, error) {
	now := time.Now()
	since := now.Add(-24 * time.Hour)

	nodes, err := model.ListNodes()
	if err != nil {
		return nil, err
	}

	snapshots, err := model.ListMetricSnapshotsSince(since)
	if err != nil {
		return nil, err
	}
	reports, err := model.ListRequestReportsSince(since)
	if err != nil {
		return nil, err
	}
	accessLogRegions, err := model.ListNodeAccessLogRegionCounts("", since, 8)
	if err != nil {
		return nil, err
	}
	activeEvents, err := model.ListActiveNodeHealthEvents()
	if err != nil {
		return nil, err
	}

	openrestySnapshots, err := model.ListNodeObservationOpenresty("", since, 0)
	if err != nil {
		return nil, err
	}

	view := &DashboardOverviewView{
		GeneratedAt:   now,
		Nodes:         make([]DashboardNodeHealth, 0, len(nodes)),
		Distributions: buildTrafficDistributions(reports, accessLogRegions, 8),
		Trends: DashboardTrends{
			Traffic24h:  buildTrafficTrendPoints(now, reports),
			Capacity24h: buildCapacityTrendPoints(now, snapshots),
			Network24h:  buildNetworkTrendPoints(now, snapshots, openrestySnapshots),
			DiskIO24h:   buildDiskIOTrendPoints(now, snapshots),
		},
	}

	var cpuNodeCount int
	var memoryNodeCount int
	latestSnapshots := latestMetricSnapshotsByNode(snapshots)
	latestTrafficReports := latestTrafficReportsByNode(reports)
	activeEventsByNode := activeHealthEventsByNode(activeEvents)

	for _, node := range nodes {
		computedStatus := computeNodeStatus(node)
		switch computedStatus {
		case NodeStatusOnline:
			view.Summary.OnlineNodes++
		case NodeStatusOffline:
			view.Summary.OfflineNodes++
		case NodeStatusPending:
			view.Summary.PendingNodes++
		}
		if node.OpenrestyStatus == OpenrestyStatusUnhealthy {
			view.Summary.UnhealthyNodes++
		}

		latestSnapshot := latestSnapshots[node.NodeID]
		latestTraffic := latestTrafficReports[node.NodeID]
		nodeActiveEvents := activeEventsByNode[node.NodeID]

		nodeHealth := DashboardNodeHealth{
			ID:               node.ID,
			NodeID:           node.NodeID,
			Name:             node.Name,
			GeoName:          node.GeoName,
			GeoLatitude:      node.GeoLatitude,
			GeoLongitude:     node.GeoLongitude,
			Status:           computedStatus,
			OpenrestyStatus:  node.OpenrestyStatus,
			CurrentVersion:   node.CurrentVersion,
			LastSeenAt:       nodeViewLastSeenAt(node),
			ActiveEventCount: len(nodeActiveEvents),
		}

		if latestSnapshot != nil {
			nodeHealth.CPUUsagePercent = latestSnapshot.CPUUsagePercent
			nodeHealth.MemoryUsagePercent = percentage(latestSnapshot.MemoryUsedBytes, latestSnapshot.MemoryTotalBytes)
			nodeHealth.StorageUsagePercent = percentage(latestSnapshot.StorageUsedBytes, latestSnapshot.StorageTotalBytes)
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

func percentage(used int64, total int64) float64 {
	if used <= 0 || total <= 0 {
		return 0
	}
	return (float64(used) / float64(total)) * 100
}

func latestMetricSnapshotsByNode(snapshots []*model.NodeMetricSnapshot) map[string]*model.NodeMetricSnapshot {
	result := make(map[string]*model.NodeMetricSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot == nil || snapshot.NodeID == "" {
			continue
		}
		if existing, ok := result[snapshot.NodeID]; ok && !snapshot.CapturedAt.After(existing.CapturedAt) {
			continue
		}
		result[snapshot.NodeID] = snapshot
	}
	return result
}

func latestTrafficReportsByNode(reports []*model.NodeRequestReport) map[string]*model.NodeRequestReport {
	result := make(map[string]*model.NodeRequestReport, len(reports))
	for _, report := range reports {
		if report == nil || report.NodeID == "" {
			continue
		}
		if existing, ok := result[report.NodeID]; ok && !report.WindowEndedAt.After(existing.WindowEndedAt) {
			continue
		}
		result[report.NodeID] = report
	}
	return result
}

func activeHealthEventsByNode(events []*model.NodeHealthEvent) map[string][]*model.NodeHealthEvent {
	result := make(map[string][]*model.NodeHealthEvent)
	for _, event := range events {
		if event == nil || event.NodeID == "" {
			continue
		}
		result[event.NodeID] = append(result[event.NodeID], event)
	}
	return result
}
