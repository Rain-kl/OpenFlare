package service

import (
	"errors"
	"openflare/model"
	"time"

	"gorm.io/gorm"
)

const (
	defaultObservabilityWindow = 24 * time.Hour
	defaultObservabilityLimit  = 120
	maxObservabilityLimit      = 500
)

type NodeObservabilityQuery struct {
	Hours int `json:"hours"`
	Limit int `json:"limit"`
}

type NodeObservabilityView struct {
	NodeID          string                      `json:"node_id"`
	Profile         *model.NodeSystemProfile    `json:"profile"`
	MetricSnapshots []*model.NodeMetricSnapshot `json:"metric_snapshots"`
	TrafficReports  []*model.NodeRequestReport  `json:"traffic_reports"`
	HealthEvents    []*model.NodeHealthEvent    `json:"health_events"`
	Analytics       NodeObservabilityAnalytics  `json:"analytics"`
	Trends          NodeObservabilityTrends     `json:"trends"`
}

type NodeObservabilityAnalytics struct {
	Traffic       TrafficWindowSummary       `json:"traffic"`
	Distributions TrafficDistributions       `json:"distributions"`
	Health        ObservabilityHealthSummary `json:"health"`
}

type NodeObservabilityTrends struct {
	Traffic24h  []TrafficTrendPoint  `json:"traffic_24h"`
	Capacity24h []CapacityTrendPoint `json:"capacity_24h"`
	Network24h  []NetworkTrendPoint  `json:"network_24h"`
	DiskIO24h   []DiskIOTrendPoint   `json:"disk_io_24h"`
}

type NodeHealthEventCleanupResult struct {
	NodeID       string `json:"node_id"`
	DeletedCount int64  `json:"deleted_count"`
}

func GetNodeObservability(id uint, query NodeObservabilityQuery) (*NodeObservabilityView, error) {
	now := time.Now()
	node, err := model.GetNodeByID(id)
	if err != nil {
		return nil, err
	}

	limit := normalizeObservabilityLimit(query.Limit)
	since := now.Add(-normalizeObservabilityWindow(query.Hours))

	profile, err := model.GetNodeSystemProfile(node.NodeID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		profile = nil
	}

	snapshots, err := model.ListNodeMetricSnapshots(node.NodeID, since, limit)
	if err != nil {
		return nil, err
	}
	reports, err := model.ListNodeRequestReports(node.NodeID, since, limit)
	if err != nil {
		return nil, err
	}
	accessLogRegions, err := model.ListNodeAccessLogRegionCounts(node.NodeID, since, 8)
	if err != nil {
		return nil, err
	}
	trendSnapshots, err := model.ListNodeMetricSnapshots(node.NodeID, now.Add(-24*time.Hour), 0)
	if err != nil {
		return nil, err
	}
	trendReports, err := model.ListNodeRequestReports(node.NodeID, now.Add(-24*time.Hour), 0)
	if err != nil {
		return nil, err
	}
	events, err := model.ListNodeHealthEvents(node.NodeID, false, limit)
	if err != nil {
		return nil, err
	}

	return &NodeObservabilityView{
		NodeID:          node.NodeID,
		Profile:         profile,
		MetricSnapshots: snapshots,
		TrafficReports:  reports,
		HealthEvents:    events,
		Analytics: NodeObservabilityAnalytics{
			Traffic:       buildTrafficWindowSummary(latestTrafficReport(reports)),
			Distributions: buildTrafficDistributions(reports, accessLogRegions, 8),
			Health:        buildObservabilityHealthSummary(latestMetricSnapshot(snapshots), latestTrafficReport(reports), events),
		},
		Trends: NodeObservabilityTrends{
			Traffic24h:  buildTrafficTrendPoints(now, trendReports),
			Capacity24h: buildCapacityTrendPoints(now, trendSnapshots),
			Network24h:  buildNetworkTrendPoints(now, trendSnapshots),
			DiskIO24h:   buildDiskIOTrendPoints(now, trendSnapshots),
		},
	}, nil
}

func CleanupNodeHealthEvents(id uint) (*NodeHealthEventCleanupResult, error) {
	node, err := model.GetNodeByID(id)
	if err != nil {
		return nil, err
	}
	deletedCount, err := model.DeleteNodeHealthEvents(node.NodeID)
	if err != nil {
		return nil, err
	}
	return &NodeHealthEventCleanupResult{
		NodeID:       node.NodeID,
		DeletedCount: deletedCount,
	}, nil
}

func latestMetricSnapshot(snapshots []*model.NodeMetricSnapshot) *model.NodeMetricSnapshot {
	for _, snapshot := range snapshots {
		if snapshot != nil {
			return snapshot
		}
	}
	return nil
}

func latestTrafficReport(reports []*model.NodeRequestReport) *model.NodeRequestReport {
	for _, report := range reports {
		if report != nil {
			return report
		}
	}
	return nil
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
