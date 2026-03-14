package service

import (
	"atsflare/model"
	"errors"
	"sort"
	"time"

	"gorm.io/gorm"
)

type DashboardOverviewView struct {
	GeneratedAt   time.Time             `json:"generated_at"`
	Summary       DashboardSummary      `json:"summary"`
	Traffic       DashboardTraffic      `json:"traffic"`
	Capacity      DashboardCapacity     `json:"capacity"`
	Config        DashboardConfig       `json:"config"`
	Risk          DashboardRiskSummary  `json:"risk"`
	Peaks         DashboardPeakSummary  `json:"peaks"`
	Distributions TrafficDistributions  `json:"distributions"`
	Trends        DashboardTrends       `json:"trends"`
	Nodes         []DashboardNodeHealth `json:"nodes"`
	ActiveAlerts  []DashboardAlert      `json:"active_alerts"`
}

type DashboardSummary struct {
	TotalNodes     int `json:"total_nodes"`
	OnlineNodes    int `json:"online_nodes"`
	OfflineNodes   int `json:"offline_nodes"`
	PendingNodes   int `json:"pending_nodes"`
	UnhealthyNodes int `json:"unhealthy_nodes"`
	ActiveAlerts   int `json:"active_alerts"`
	LaggingNodes   int `json:"lagging_nodes"`
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

type DashboardConfig struct {
	ActiveVersion string `json:"active_version"`
	LaggingNodes  int    `json:"lagging_nodes"`
	PendingNodes  int    `json:"pending_nodes"`
}

type DashboardRiskSummary struct {
	CriticalAlerts   int `json:"critical_alerts"`
	WarningAlerts    int `json:"warning_alerts"`
	InfoAlerts       int `json:"info_alerts"`
	OfflineNodes     int `json:"offline_nodes"`
	UnhealthyNodes   int `json:"unhealthy_nodes"`
	LaggingNodes     int `json:"lagging_nodes"`
	HighCPUNodes     int `json:"high_cpu_nodes"`
	HighMemoryNodes  int `json:"high_memory_nodes"`
	HighStorageNodes int `json:"high_storage_nodes"`
}

type DashboardPeakSummary struct {
	PeakRequestHour DashboardPeakHour  `json:"peak_request_hour"`
	PeakErrorHour   DashboardPeakHour  `json:"peak_error_hour"`
	BusiestNode     *DashboardPeakNode `json:"busiest_node"`
	RiskiestNode    *DashboardPeakNode `json:"riskiest_node"`
}

type DashboardPeakHour struct {
	BucketStartedAt time.Time `json:"bucket_started_at"`
	RequestCount    int64     `json:"request_count"`
	ErrorCount      int64     `json:"error_count"`
}

type DashboardPeakNode struct {
	NodeID              string  `json:"node_id"`
	NodeName            string  `json:"node_name"`
	RequestCount        int64   `json:"request_count"`
	ErrorCount          int64   `json:"error_count"`
	CPUUsagePercent     float64 `json:"cpu_usage_percent"`
	ActiveEventCount    int     `json:"active_event_count"`
	OpenrestyStatus     string  `json:"openresty_status"`
	StorageUsagePercent float64 `json:"storage_usage_percent"`
}

type DashboardTrends struct {
	Traffic24h  []TrafficTrendPoint  `json:"traffic_24h"`
	Capacity24h []CapacityTrendPoint `json:"capacity_24h"`
	Network24h  []NetworkTrendPoint  `json:"network_24h"`
	DiskIO24h   []DiskIOTrendPoint   `json:"disk_io_24h"`
}

type DashboardNodeHealth struct {
	ID                  uint      `json:"id"`
	NodeID              string    `json:"node_id"`
	Name                string    `json:"name"`
	GeoName             string    `json:"geo_name"`
	GeoLatitude         *float64  `json:"geo_latitude"`
	GeoLongitude        *float64  `json:"geo_longitude"`
	Status              string    `json:"status"`
	OpenrestyStatus     string    `json:"openresty_status"`
	CurrentVersion      string    `json:"current_version"`
	LastSeenAt          time.Time `json:"last_seen_at"`
	ActiveEventCount    int       `json:"active_event_count"`
	CPUUsagePercent     float64   `json:"cpu_usage_percent"`
	MemoryUsagePercent  float64   `json:"memory_usage_percent"`
	StorageUsagePercent float64   `json:"storage_usage_percent"`
	RequestCount        int64     `json:"request_count"`
	ErrorCount          int64     `json:"error_count"`
	UniqueVisitorCount  int64     `json:"unique_visitor_count"`
}

type DashboardAlert struct {
	NodeID          string    `json:"node_id"`
	NodeName        string    `json:"node_name"`
	EventType       string    `json:"event_type"`
	Severity        string    `json:"severity"`
	Message         string    `json:"message"`
	LastTriggeredAt time.Time `json:"last_triggered_at"`
	Status          string    `json:"status"`
}

func GetDashboardOverview() (*DashboardOverviewView, error) {
	now := time.Now()
	since := now.Add(-24 * time.Hour)

	nodes, err := model.ListNodes()
	if err != nil {
		return nil, err
	}

	activeVersion := ""
	if version, versionErr := model.GetActiveConfigVersion(); versionErr == nil {
		activeVersion = version.Version
	} else if !errors.Is(versionErr, gorm.ErrRecordNotFound) {
		return nil, versionErr
	}

	snapshots, err := model.ListMetricSnapshotsSince(since)
	if err != nil {
		return nil, err
	}
	reports, err := model.ListRequestReportsSince(since)
	if err != nil {
		return nil, err
	}
	activeEvents, err := model.ListActiveNodeHealthEvents()
	if err != nil {
		return nil, err
	}

	view := &DashboardOverviewView{
		GeneratedAt:   now,
		Nodes:         make([]DashboardNodeHealth, 0, len(nodes)),
		ActiveAlerts:  make([]DashboardAlert, 0),
		Distributions: buildTrafficDistributions(reports, 8),
		Trends: DashboardTrends{
			Traffic24h:  buildTrafficTrendPoints(now, reports),
			Capacity24h: buildCapacityTrendPoints(now, snapshots),
			Network24h:  buildNetworkTrendPoints(now, snapshots),
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
			view.Risk.OfflineNodes++
		case NodeStatusPending:
			view.Summary.PendingNodes++
		}
		if node.OpenrestyStatus == OpenrestyStatusUnhealthy {
			view.Summary.UnhealthyNodes++
			view.Risk.UnhealthyNodes++
		}
		if activeVersion != "" && node.CurrentVersion != "" && node.CurrentVersion != activeVersion {
			view.Summary.LaggingNodes++
			view.Risk.LaggingNodes++
		}
		if activeVersion != "" && node.CurrentVersion == "" && computedStatus != NodeStatusPending {
			view.Summary.LaggingNodes++
			view.Risk.LaggingNodes++
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
			LastSeenAt:       node.LastSeenAt,
			ActiveEventCount: len(nodeActiveEvents),
		}

		for _, event := range nodeActiveEvents {
			view.ActiveAlerts = append(view.ActiveAlerts, DashboardAlert{
				NodeID:          node.NodeID,
				NodeName:        node.Name,
				EventType:       event.EventType,
				Severity:        event.Severity,
				Message:         event.Message,
				LastTriggeredAt: event.LastTriggeredAt,
				Status:          event.Status,
			})
			switch event.Severity {
			case NodeHealthSeverityCritical:
				view.Risk.CriticalAlerts++
			case NodeHealthSeverityWarning:
				view.Risk.WarningAlerts++
			default:
				view.Risk.InfoAlerts++
			}
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
				view.Risk.HighCPUNodes++
			}
			if nodeHealth.MemoryUsagePercent >= 85 {
				view.Capacity.HighMemoryNodes++
				view.Risk.HighMemoryNodes++
			}
			if nodeHealth.StorageUsagePercent >= 85 {
				view.Capacity.HighStorageNodes++
				view.Risk.HighStorageNodes++
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

		view.Summary.ActiveAlerts += len(nodeActiveEvents)
		view.Nodes = append(view.Nodes, nodeHealth)
	}

	view.Summary.TotalNodes = len(nodes)
	view.Config.ActiveVersion = activeVersion
	view.Config.LaggingNodes = view.Summary.LaggingNodes
	view.Config.PendingNodes = view.Summary.PendingNodes

	if cpuNodeCount > 0 {
		view.Capacity.AverageCPUUsagePercent /= float64(cpuNodeCount)
	}
	if memoryNodeCount > 0 {
		view.Capacity.AverageMemoryUsagePercent /= float64(memoryNodeCount)
	}

	sort.Slice(view.ActiveAlerts, func(i int, j int) bool {
		if severityWeight(view.ActiveAlerts[i].Severity) == severityWeight(view.ActiveAlerts[j].Severity) {
			return view.ActiveAlerts[i].LastTriggeredAt.After(view.ActiveAlerts[j].LastTriggeredAt)
		}
		return severityWeight(view.ActiveAlerts[i].Severity) > severityWeight(view.ActiveAlerts[j].Severity)
	})

	sort.Slice(view.Nodes, func(i int, j int) bool {
		if view.Nodes[i].ActiveEventCount == view.Nodes[j].ActiveEventCount {
			return view.Nodes[i].CPUUsagePercent > view.Nodes[j].CPUUsagePercent
		}
		return view.Nodes[i].ActiveEventCount > view.Nodes[j].ActiveEventCount
	})

	if len(view.ActiveAlerts) > 8 {
		view.ActiveAlerts = view.ActiveAlerts[:8]
	}

	view.Peaks.PeakRequestHour = peakTrafficHour(view.Trends.Traffic24h, func(point TrafficTrendPoint) int64 {
		return point.RequestCount
	})
	view.Peaks.PeakErrorHour = peakTrafficHour(view.Trends.Traffic24h, func(point TrafficTrendPoint) int64 {
		return point.ErrorCount
	})
	view.Peaks.BusiestNode = busiestDashboardNode(view.Nodes)
	view.Peaks.RiskiestNode = riskiestDashboardNode(view.Nodes)

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

func severityWeight(severity string) int {
	switch severity {
	case NodeHealthSeverityCritical:
		return 3
	case NodeHealthSeverityWarning:
		return 2
	default:
		return 1
	}
}

func peakTrafficHour(points []TrafficTrendPoint, selector func(point TrafficTrendPoint) int64) DashboardPeakHour {
	var result DashboardPeakHour
	var maxValue int64 = -1
	for _, point := range points {
		value := selector(point)
		if value <= maxValue {
			continue
		}
		maxValue = value
		result = DashboardPeakHour{
			BucketStartedAt: point.BucketStartedAt,
			RequestCount:    point.RequestCount,
			ErrorCount:      point.ErrorCount,
		}
	}
	return result
}

func busiestDashboardNode(nodes []DashboardNodeHealth) *DashboardPeakNode {
	var selected *DashboardPeakNode
	for _, node := range nodes {
		candidate := &DashboardPeakNode{
			NodeID:              node.NodeID,
			NodeName:            node.Name,
			RequestCount:        node.RequestCount,
			ErrorCount:          node.ErrorCount,
			CPUUsagePercent:     node.CPUUsagePercent,
			ActiveEventCount:    node.ActiveEventCount,
			OpenrestyStatus:     node.OpenrestyStatus,
			StorageUsagePercent: node.StorageUsagePercent,
		}
		if selected == nil || candidate.RequestCount > selected.RequestCount || (candidate.RequestCount == selected.RequestCount && candidate.ErrorCount > selected.ErrorCount) {
			selected = candidate
		}
	}
	return selected
}

func riskiestDashboardNode(nodes []DashboardNodeHealth) *DashboardPeakNode {
	if len(nodes) == 0 {
		return nil
	}
	node := nodes[0]
	return &DashboardPeakNode{
		NodeID:              node.NodeID,
		NodeName:            node.Name,
		RequestCount:        node.RequestCount,
		ErrorCount:          node.ErrorCount,
		CPUUsagePercent:     node.CPUUsagePercent,
		ActiveEventCount:    node.ActiveEventCount,
		OpenrestyStatus:     node.OpenrestyStatus,
		StorageUsagePercent: node.StorageUsagePercent,
	}
}
