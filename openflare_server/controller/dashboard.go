package controller

import (
	"openflare/service"

	"github.com/gin-gonic/gin"
)

type dashboardOverviewPayload struct {
	GeneratedAt   any                           `json:"generated_at"`
	Summary       service.DashboardSummary      `json:"summary"`
	Traffic       service.DashboardTraffic      `json:"traffic"`
	Capacity      service.DashboardCapacity     `json:"capacity"`
	Distributions dashboardDistributionsPayload `json:"distributions"`
	Trends        dashboardTrendsPayload        `json:"trends"`
	Nodes         [][]any                       `json:"nodes"`
}

type dashboardDistributionsPayload struct {
	StatusCodes     [][]any `json:"status_codes"`
	TopDomains      [][]any `json:"top_domains"`
	SourceCountries [][]any `json:"source_countries"`
}

type dashboardTrendsPayload struct {
	Traffic24h  [][]any `json:"traffic_24h"`
	Capacity24h [][]any `json:"capacity_24h"`
	Network24h  [][]any `json:"network_24h"`
	DiskIO24h   [][]any `json:"disk_io_24h"`
}

// GetDashboardOverview godoc
// @Summary Get dashboard overview
// @Tags Dashboard
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/dashboard/overview [get]
func GetDashboardOverview(c *gin.Context) {
	view, err := service.GetDashboardOverview()
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccess(c, compressDashboardOverview(view))
}

func compressDashboardOverview(view *service.DashboardOverviewView) *dashboardOverviewPayload {
	if view == nil {
		return &dashboardOverviewPayload{
			Distributions: dashboardDistributionsPayload{
				StatusCodes:     [][]any{},
				TopDomains:      [][]any{},
				SourceCountries: [][]any{},
			},
			Trends: dashboardTrendsPayload{
				Traffic24h:  [][]any{},
				Capacity24h: [][]any{},
				Network24h:  [][]any{},
				DiskIO24h:   [][]any{},
			},
			Nodes: [][]any{},
		}
	}
	return &dashboardOverviewPayload{
		GeneratedAt: view.GeneratedAt,
		Summary:     view.Summary,
		Traffic:     view.Traffic,
		Capacity:    view.Capacity,
		Distributions: dashboardDistributionsPayload{
			StatusCodes:     compressDistributionItems(view.Distributions.StatusCodes),
			TopDomains:      compressDistributionItems(view.Distributions.TopDomains),
			SourceCountries: compressDistributionItems(view.Distributions.SourceCountries),
		},
		Trends: dashboardTrendsPayload{
			Traffic24h:  compressTrafficTrendPoints(view.Trends.Traffic24h),
			Capacity24h: compressCapacityTrendPoints(view.Trends.Capacity24h),
			Network24h:  compressNetworkTrendPoints(view.Trends.Network24h),
			DiskIO24h:   compressDiskIOTrendPoints(view.Trends.DiskIO24h),
		},
		Nodes: compressDashboardNodes(view.Nodes),
	}
}

func compressDistributionItems(items []service.DistributionItem) [][]any {
	rows := make([][]any, 0, len(items))
	for _, item := range items {
		rows = append(rows, []any{item.Key, item.Value})
	}
	return rows
}

func compressTrafficTrendPoints(points []service.TrafficTrendPoint) [][]any {
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

func compressCapacityTrendPoints(points []service.CapacityTrendPoint) [][]any {
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

func compressNetworkTrendPoints(points []service.NetworkTrendPoint) [][]any {
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

func compressDiskIOTrendPoints(points []service.DiskIOTrendPoint) [][]any {
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

func compressDashboardNodes(nodes []service.DashboardNodeHealth) [][]any {
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
