// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
)

const observabilityTrendBuckets = 24
const unknownTrendNodeKey = "__unknown__"

const (
	healthEventStatusActive   = "active"
	healthEventStatusResolved = "resolved"
	healthSeverityCritical    = "critical"
	healthSeverityWarning     = "warning"
	percentageMultiplier      = 100
)

// DistributionItem is a key/value distribution entry.
type DistributionItem struct {
	Key   string `json:"key"`
	Value int64  `json:"value"`
}

// TrafficDistributions groups traffic distribution charts.
type TrafficDistributions struct {
	StatusCodes     []DistributionItem `json:"status_codes"`
	TopDomains      []DistributionItem `json:"top_domains"`
	SourceCountries []DistributionItem `json:"source_countries"`
}

const metricSnapshotOpenrestyMatchWindow = 2 * time.Minute

// NodeMetricSnapshotView is a metric snapshot enriched with OpenResty observations.
type NodeMetricSnapshotView struct {
	ID                   uint      `json:"id,omitempty"`
	NodeID               string    `json:"node_id,omitempty"`
	CapturedAt           time.Time `json:"captured_at"`
	CPUUsagePercent      float64   `json:"cpu_usage_percent"`
	MemoryUsedBytes      int64     `json:"memory_used_bytes"`
	MemoryTotalBytes     int64     `json:"memory_total_bytes"`
	StorageUsedBytes     int64     `json:"storage_used_bytes"`
	StorageTotalBytes    int64     `json:"storage_total_bytes"`
	DiskReadBytes        int64     `json:"disk_read_bytes"`
	DiskWriteBytes       int64     `json:"disk_write_bytes"`
	NetworkRxBytes       int64     `json:"network_rx_bytes"`
	NetworkTxBytes       int64     `json:"network_tx_bytes"`
	OpenrestyRxBytes     int64     `json:"openresty_rx_bytes"`
	OpenrestyTxBytes     int64     `json:"openresty_tx_bytes"`
	OpenrestyConnections int64     `json:"openresty_connections"`
}

// TrafficWindowSummary summarizes a traffic reporting window.
type TrafficWindowSummary struct {
	WindowStartedAt    time.Time `json:"window_started_at"`
	WindowEndedAt      time.Time `json:"window_ended_at"`
	RequestCount       int64     `json:"request_count"`
	UniqueVisitorCount int64     `json:"unique_visitor_count"`
	ErrorCount         int64     `json:"error_count"`
	EstimatedQPS       float64   `json:"estimated_qps"`
	ErrorRatePercent   float64   `json:"error_rate_percent"`
}

// HealthSummary summarizes node health alerts and risks.
type HealthSummary struct {
	ActiveAlerts    int  `json:"active_alerts"`
	CriticalAlerts  int  `json:"critical_alerts"`
	WarningAlerts   int  `json:"warning_alerts"`
	InfoAlerts      int  `json:"info_alerts"`
	ResolvedAlerts  int  `json:"resolved_alerts"`
	HasCapacityRisk bool `json:"has_capacity_risk"`
	HasTrafficRisk  bool `json:"has_traffic_risk"`
	HasRuntimeRisk  bool `json:"has_runtime_risk"`
}

// TrafficTrendPoint is a traffic trend bucket.
type TrafficTrendPoint struct {
	BucketStartedAt    time.Time `json:"bucket_started_at"`
	RequestCount       int64     `json:"request_count"`
	ErrorCount         int64     `json:"error_count"`
	UniqueVisitorCount int64     `json:"unique_visitor_count"`
}

// CapacityTrendPoint is a capacity trend bucket.
type CapacityTrendPoint struct {
	BucketStartedAt           time.Time `json:"bucket_started_at"`
	AverageCPUUsagePercent    float64   `json:"average_cpu_usage_percent"`
	AverageMemoryUsagePercent float64   `json:"average_memory_usage_percent"`
	ReportedNodes             int       `json:"reported_nodes"`
}

// NetworkTrendPoint is a network trend bucket.
type NetworkTrendPoint struct {
	BucketStartedAt  time.Time `json:"bucket_started_at"`
	NetworkRxBytes   int64     `json:"network_rx_bytes"`
	NetworkTxBytes   int64     `json:"network_tx_bytes"`
	OpenrestyRxBytes int64     `json:"openresty_rx_bytes"`
	OpenrestyTxBytes int64     `json:"openresty_tx_bytes"`
	ReportedNodes    int       `json:"reported_nodes"`
}

// DiskIOTrendPoint is a disk IO trend bucket.
type DiskIOTrendPoint struct {
	BucketStartedAt time.Time `json:"bucket_started_at"`
	DiskReadBytes   int64     `json:"disk_read_bytes"`
	DiskWriteBytes  int64     `json:"disk_write_bytes"`
	ReportedNodes   int       `json:"reported_nodes"`
}

type distributionAccumulator map[string]int64

type capacityTrendAccumulator struct {
	cpuSum   float64
	cpuCount int
	memSum   float64
	memCount int
	nodes    map[string]struct{}
}

type snapshotTrendAccumulator struct {
	nodes map[string]struct{}
}

type diskCounterState struct {
	read  int64
	write int64
	seen  bool
}

type networkCounterState struct {
	rx   int64
	tx   int64
	seen bool
}

func buildTrafficWindowSummary(report *model.OpenFlareRequestReport) *TrafficWindowSummary {
	if report == nil {
		return nil
	}
	summary := TrafficWindowSummary{
		WindowStartedAt:    report.WindowStartedAt,
		WindowEndedAt:      report.WindowEndedAt,
		RequestCount:       report.RequestCount,
		UniqueVisitorCount: report.UniqueVisitorCount,
		ErrorCount:         report.ErrorCount,
	}
	if duration := report.WindowEndedAt.Sub(report.WindowStartedAt).Seconds(); duration > 0 {
		summary.EstimatedQPS = float64(report.RequestCount) / duration
	}
	if report.RequestCount > 0 {
		summary.ErrorRatePercent = (float64(report.ErrorCount) / float64(report.RequestCount)) * 100
	}
	return &summary
}

// BuildMetricSnapshotViews merges metric snapshots with OpenResty observations for API responses.
func BuildMetricSnapshotViews(
	snapshots []*model.OpenFlareMetricSnapshot,
	openrestyObs []*model.OpenFlareNodeObservationOpenresty,
) []*NodeMetricSnapshotView {
	if len(snapshots) == 0 {
		return []*NodeMetricSnapshotView{}
	}
	views := make([]*NodeMetricSnapshotView, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot == nil {
			continue
		}
		view := &NodeMetricSnapshotView{
			ID:                snapshot.ID,
			NodeID:            snapshot.NodeID,
			CapturedAt:        snapshot.CapturedAt,
			CPUUsagePercent:   snapshot.CPUUsagePercent,
			MemoryUsedBytes:   snapshot.MemoryUsedBytes,
			MemoryTotalBytes:  snapshot.MemoryTotalBytes,
			StorageUsedBytes:  snapshot.StorageUsedBytes,
			StorageTotalBytes: snapshot.StorageTotalBytes,
			DiskReadBytes:     snapshot.DiskReadBytes,
			DiskWriteBytes:    snapshot.DiskWriteBytes,
			NetworkRxBytes:    snapshot.NetworkRxBytes,
			NetworkTxBytes:    snapshot.NetworkTxBytes,
		}
		if matched := matchOpenrestyObservation(snapshot.CapturedAt, openrestyObs); matched != nil {
			view.OpenrestyRxBytes = matched.OpenrestyRxBytes
			view.OpenrestyTxBytes = matched.OpenrestyTxBytes
			view.OpenrestyConnections = matched.OpenrestyConnections
		}
		views = append(views, view)
	}
	return views
}

// BuildTrafficDistributions aggregates traffic distribution charts.
func BuildTrafficDistributions(
	reports []*model.OpenFlareRequestReport,
	accessLogRegions []*model.OpenFlareAccessLogRegionCount,
	limit int,
) TrafficDistributions {
	statusCodes := make(distributionAccumulator)
	topDomains := make(distributionAccumulator)
	reportSourceCountries := make(distributionAccumulator)
	for _, report := range reports {
		mergeJSONCounts(statusCodes, report.StatusCodesJSON)
		mergeJSONCounts(topDomains, report.TopDomainsJSON)
		mergeJSONCounts(reportSourceCountries, report.SourceCountriesJSON)
	}
	sourceCountries := reportSourceCountries
	if len(accessLogRegions) > 0 {
		sourceCountries = make(distributionAccumulator, len(accessLogRegions))
		for _, item := range accessLogRegions {
			if item == nil || strings.TrimSpace(item.Region) == "" || item.Count <= 0 {
				continue
			}
			sourceCountries[item.Region] = item.Count
		}
	}
	return TrafficDistributions{
		StatusCodes:     toDistributionItems(statusCodes, limit),
		TopDomains:      toDistributionItems(topDomains, limit),
		SourceCountries: toDistributionItems(sourceCountries, limit),
	}
}

func buildHealthSummary(
	snapshot *model.OpenFlareMetricSnapshot,
	report *model.OpenFlareRequestReport,
	events []*model.OpenFlareHealthEvent,
) HealthSummary {
	summary := HealthSummary{}
	for _, event := range events {
		if event == nil {
			continue
		}
		if event.Status == healthEventStatusResolved {
			summary.ResolvedAlerts++
			continue
		}
		summary.ActiveAlerts++
		switch event.Severity {
		case healthSeverityCritical:
			summary.CriticalAlerts++
		case healthSeverityWarning:
			summary.WarningAlerts++
		default:
			summary.InfoAlerts++
		}
	}
	if snapshot != nil {
		memoryUsage := Percentage(snapshot.MemoryUsedBytes, snapshot.MemoryTotalBytes)
		storageUsage := Percentage(snapshot.StorageUsedBytes, snapshot.StorageTotalBytes)
		summary.HasCapacityRisk = snapshot.CPUUsagePercent >= 80 || memoryUsage >= 85 || storageUsage >= 85
	}
	if report != nil && report.RequestCount >= 100 {
		summary.HasTrafficRisk = (float64(report.ErrorCount) / float64(report.RequestCount)) >= 0.05
	}
	summary.HasRuntimeRisk = summary.ActiveAlerts > 0 || summary.HasCapacityRisk || summary.HasTrafficRisk
	return summary
}

// BuildNodeTrends builds 24h trend series, preferring ClickHouse hourly aggregates
// over limited raw snapshot windows so capacity/network/disk charts stay complete.
func BuildNodeTrends(
	ctx context.Context,
	now time.Time,
	nodeID string,
	snapshots []*model.OpenFlareMetricSnapshot,
	openrestyObs []*model.OpenFlareNodeObservationOpenresty,
	reports []*model.OpenFlareRequestReport,
) NodeTrends {
	trendSince := now.Add(-24 * time.Hour)
	trafficTrend := BuildTrafficTrendPoints(now, reports)
	if trafficHourly, err := model.ListOpenFlareTrafficHourlySince(ctx, nodeID, trendSince); err == nil && len(trafficHourly) > 0 {
		trafficTrend = BuildTrafficTrendPointsFromHourly(now, trafficHourly)
	}

	capacityTrend := BuildCapacityTrendPoints(now, snapshots)
	networkTrend := BuildNetworkTrendPoints(now, snapshots, openrestyObs)
	diskIOTrend := BuildDiskIOTrendPoints(now, snapshots)

	metricHourly, metricErr := model.ListOpenFlareMetricHourlySince(ctx, nodeID, trendSince)
	if metricErr == nil && len(metricHourly) > 0 {
		capacityTrend = BuildCapacityTrendPointsFromHourly(now, metricHourly)
		diskIOTrend = BuildDiskIOTrendPointsFromHourly(now, metricHourly)
	}
	openrestyHourly, openrestyErr := model.ListOpenFlareOpenrestyHourlySince(ctx, nodeID, trendSince)
	if metricErr == nil && openrestyErr == nil && (len(metricHourly) > 0 || len(openrestyHourly) > 0) {
		networkTrend = BuildNetworkTrendPointsFromHourly(now, metricHourly, openrestyHourly)
	}

	return NodeTrends{
		Traffic24h:  trafficTrend,
		Capacity24h: capacityTrend,
		Network24h:  networkTrend,
		DiskIO24h:   diskIOTrend,
	}
}

// BuildTrafficTrendPointsFromHourly builds 24h traffic trend buckets from hourly rollups.
func BuildTrafficTrendPointsFromHourly(now time.Time, hourly []*model.OpenFlareTrafficHourly) []TrafficTrendPoint {
	start := trendWindowStart(now)
	points := make([]TrafficTrendPoint, observabilityTrendBuckets)
	for index := range points {
		points[index].BucketStartedAt = start.Add(time.Duration(index) * time.Hour)
	}
	for _, row := range hourly {
		if row == nil {
			continue
		}
		index, ok := trendBucketIndex(row.Hour, start)
		if !ok {
			continue
		}
		points[index].RequestCount += row.RequestCount
		points[index].ErrorCount += row.ErrorCount
		points[index].UniqueVisitorCount += row.UniqueVisitorCount
	}
	return points
}

// BuildTrafficTrendPoints builds 24h traffic trend buckets.
func BuildTrafficTrendPoints(now time.Time, reports []*model.OpenFlareRequestReport) []TrafficTrendPoint {
	start := trendWindowStart(now)
	points := make([]TrafficTrendPoint, observabilityTrendBuckets)
	for index := range points {
		points[index].BucketStartedAt = start.Add(time.Duration(index) * time.Hour)
	}
	for _, report := range reports {
		index, ok := trendBucketIndex(report.WindowEndedAt, start)
		if !ok {
			continue
		}
		points[index].RequestCount += report.RequestCount
		points[index].ErrorCount += report.ErrorCount
		points[index].UniqueVisitorCount += report.UniqueVisitorCount
	}
	return points
}

// BuildCapacityTrendPoints builds 24h capacity trend buckets.
func BuildCapacityTrendPoints(now time.Time, snapshots []*model.OpenFlareMetricSnapshot) []CapacityTrendPoint {
	start := trendWindowStart(now)
	points := make([]CapacityTrendPoint, observabilityTrendBuckets)
	accumulators := make([]capacityTrendAccumulator, observabilityTrendBuckets)
	for index := range points {
		points[index].BucketStartedAt = start.Add(time.Duration(index) * time.Hour)
		accumulators[index].nodes = make(map[string]struct{})
	}
	for _, snapshot := range snapshots {
		index, ok := trendBucketIndex(snapshot.CapturedAt, start)
		if !ok {
			continue
		}
		if snapshot.CPUUsagePercent > 0 {
			accumulators[index].cpuSum += snapshot.CPUUsagePercent
			accumulators[index].cpuCount++
		}
		if memoryUsage := Percentage(snapshot.MemoryUsedBytes, snapshot.MemoryTotalBytes); memoryUsage > 0 {
			accumulators[index].memSum += memoryUsage
			accumulators[index].memCount++
		}
		if snapshot.NodeID != "" {
			accumulators[index].nodes[snapshot.NodeID] = struct{}{}
		}
	}
	for index := range points {
		if accumulators[index].cpuCount > 0 {
			points[index].AverageCPUUsagePercent = accumulators[index].cpuSum / float64(accumulators[index].cpuCount)
		}
		if accumulators[index].memCount > 0 {
			points[index].AverageMemoryUsagePercent = accumulators[index].memSum / float64(accumulators[index].memCount)
		}
		points[index].ReportedNodes = len(accumulators[index].nodes)
	}
	return points
}

// BuildCapacityTrendPointsFromHourly builds 24h capacity trend buckets from hourly aggregates.
func BuildCapacityTrendPointsFromHourly(now time.Time, hourly []*model.OpenFlareMetricHourly) []CapacityTrendPoint {
	start := trendWindowStart(now)
	points := make([]CapacityTrendPoint, observabilityTrendBuckets)
	for index := range points {
		points[index].BucketStartedAt = start.Add(time.Duration(index) * time.Hour)
	}
	for _, row := range hourly {
		if row == nil {
			continue
		}
		index, ok := trendBucketIndex(row.Hour, start)
		if !ok {
			continue
		}
		points[index].AverageCPUUsagePercent = row.AverageCPUUsagePercent
		points[index].AverageMemoryUsagePercent = row.AverageMemoryUsagePercent
		points[index].ReportedNodes = row.ReportedNodes
	}
	return points
}

// BuildNetworkTrendPoints builds 24h network trend buckets.
// Host and OpenResty counters are cumulative; values are consecutive deltas.
func BuildNetworkTrendPoints(
	now time.Time,
	snapshots []*model.OpenFlareMetricSnapshot,
	openrestyObs []*model.OpenFlareNodeObservationOpenresty,
) []NetworkTrendPoint {
	start := trendWindowStart(now)
	points := make([]NetworkTrendPoint, observabilityTrendBuckets)
	accumulators := make([]snapshotTrendAccumulator, observabilityTrendBuckets)
	for index := range points {
		points[index].BucketStartedAt = start.Add(time.Duration(index) * time.Hour)
		accumulators[index].nodes = make(map[string]struct{})
	}
	sort.Slice(snapshots, func(i int, j int) bool {
		if snapshots[i].CapturedAt.Equal(snapshots[j].CapturedAt) {
			return snapshots[i].NodeID < snapshots[j].NodeID
		}
		return snapshots[i].CapturedAt.Before(snapshots[j].CapturedAt)
	})
	previousHostByNode := make(map[string]networkCounterState, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot == nil {
			continue
		}
		nodeKey := snapshot.NodeID
		if nodeKey == "" {
			nodeKey = unknownTrendNodeKey
		}
		previous := previousHostByNode[nodeKey]
		previousHostByNode[nodeKey] = networkCounterState{
			rx:   snapshot.NetworkRxBytes,
			tx:   snapshot.NetworkTxBytes,
			seen: true,
		}
		if !previous.seen {
			continue
		}
		index, ok := trendBucketIndex(snapshot.CapturedAt, start)
		if !ok {
			continue
		}
		points[index].NetworkRxBytes += nonNegativeDelta(snapshot.NetworkRxBytes, previous.rx)
		points[index].NetworkTxBytes += nonNegativeDelta(snapshot.NetworkTxBytes, previous.tx)
		if snapshot.NodeID != "" {
			accumulators[index].nodes[snapshot.NodeID] = struct{}{}
		}
	}
	sort.Slice(openrestyObs, func(i int, j int) bool {
		if openrestyObs[i].CapturedAt.Equal(openrestyObs[j].CapturedAt) {
			return openrestyObs[i].NodeID < openrestyObs[j].NodeID
		}
		return openrestyObs[i].CapturedAt.Before(openrestyObs[j].CapturedAt)
	})
	previousOpenrestyByNode := make(map[string]networkCounterState, len(openrestyObs))
	for _, obs := range openrestyObs {
		if obs == nil {
			continue
		}
		nodeKey := obs.NodeID
		if nodeKey == "" {
			nodeKey = unknownTrendNodeKey
		}
		previous := previousOpenrestyByNode[nodeKey]
		previousOpenrestyByNode[nodeKey] = networkCounterState{
			rx:   obs.OpenrestyRxBytes,
			tx:   obs.OpenrestyTxBytes,
			seen: true,
		}
		if !previous.seen {
			continue
		}
		index, ok := trendBucketIndex(obs.CapturedAt, start)
		if !ok {
			continue
		}
		points[index].OpenrestyRxBytes += nonNegativeDelta(obs.OpenrestyRxBytes, previous.rx)
		points[index].OpenrestyTxBytes += nonNegativeDelta(obs.OpenrestyTxBytes, previous.tx)
		if obs.NodeID != "" {
			accumulators[index].nodes[obs.NodeID] = struct{}{}
		}
	}
	for index := range points {
		points[index].ReportedNodes = len(accumulators[index].nodes)
	}
	return points
}

// BuildNetworkTrendPointsFromHourly builds 24h network trend buckets from hourly aggregates.
func BuildNetworkTrendPointsFromHourly(
	now time.Time,
	metricHourly []*model.OpenFlareMetricHourly,
	openrestyHourly []*model.OpenFlareOpenrestyHourly,
) []NetworkTrendPoint {
	start := trendWindowStart(now)
	points := make([]NetworkTrendPoint, observabilityTrendBuckets)
	for index := range points {
		points[index].BucketStartedAt = start.Add(time.Duration(index) * time.Hour)
	}
	for _, row := range metricHourly {
		if row == nil {
			continue
		}
		index, ok := trendBucketIndex(row.Hour, start)
		if !ok {
			continue
		}
		points[index].NetworkRxBytes += row.NetworkRxBytes
		points[index].NetworkTxBytes += row.NetworkTxBytes
		if row.ReportedNodes > points[index].ReportedNodes {
			points[index].ReportedNodes = row.ReportedNodes
		}
	}
	for _, row := range openrestyHourly {
		if row == nil {
			continue
		}
		index, ok := trendBucketIndex(row.Hour, start)
		if !ok {
			continue
		}
		points[index].OpenrestyRxBytes += row.OpenrestyRxBytes
		points[index].OpenrestyTxBytes += row.OpenrestyTxBytes
		if row.ReportedNodes > points[index].ReportedNodes {
			points[index].ReportedNodes = row.ReportedNodes
		}
	}
	return points
}

// BuildDiskIOTrendPoints builds 24h disk IO trend buckets.
func BuildDiskIOTrendPoints(now time.Time, snapshots []*model.OpenFlareMetricSnapshot) []DiskIOTrendPoint {
	start := trendWindowStart(now)
	points := make([]DiskIOTrendPoint, observabilityTrendBuckets)
	accumulators := make([]snapshotTrendAccumulator, observabilityTrendBuckets)
	for index := range points {
		points[index].BucketStartedAt = start.Add(time.Duration(index) * time.Hour)
		accumulators[index].nodes = make(map[string]struct{})
	}
	sort.Slice(snapshots, func(i int, j int) bool {
		if snapshots[i].CapturedAt.Equal(snapshots[j].CapturedAt) {
			return snapshots[i].NodeID < snapshots[j].NodeID
		}
		return snapshots[i].CapturedAt.Before(snapshots[j].CapturedAt)
	})
	previousByNode := make(map[string]diskCounterState, len(snapshots))
	for _, snapshot := range snapshots {
		nodeKey := snapshot.NodeID
		if nodeKey == "" {
			nodeKey = unknownTrendNodeKey
		}
		previous := previousByNode[nodeKey]
		previousByNode[nodeKey] = diskCounterState{
			read:  snapshot.DiskReadBytes,
			write: snapshot.DiskWriteBytes,
			seen:  true,
		}
		if !previous.seen {
			continue
		}
		index, ok := trendBucketIndex(snapshot.CapturedAt, start)
		if !ok {
			continue
		}
		points[index].DiskReadBytes += nonNegativeDelta(snapshot.DiskReadBytes, previous.read)
		points[index].DiskWriteBytes += nonNegativeDelta(snapshot.DiskWriteBytes, previous.write)
		if snapshot.NodeID != "" {
			accumulators[index].nodes[snapshot.NodeID] = struct{}{}
		}
	}
	for index := range points {
		points[index].ReportedNodes = len(accumulators[index].nodes)
	}
	return points
}

// BuildDiskIOTrendPointsFromHourly builds 24h disk IO trend buckets from hourly aggregates.
func BuildDiskIOTrendPointsFromHourly(now time.Time, hourly []*model.OpenFlareMetricHourly) []DiskIOTrendPoint {
	start := trendWindowStart(now)
	points := make([]DiskIOTrendPoint, observabilityTrendBuckets)
	for index := range points {
		points[index].BucketStartedAt = start.Add(time.Duration(index) * time.Hour)
	}
	for _, row := range hourly {
		if row == nil {
			continue
		}
		index, ok := trendBucketIndex(row.Hour, start)
		if !ok {
			continue
		}
		points[index].DiskReadBytes += row.DiskReadBytes
		points[index].DiskWriteBytes += row.DiskWriteBytes
		points[index].ReportedNodes = row.ReportedNodes
	}
	return points
}

func nonNegativeDelta(current int64, previous int64) int64 {
	delta := current - previous
	if delta < 0 {
		return 0
	}
	return delta
}

func latestMetricSnapshot(snapshots []*model.OpenFlareMetricSnapshot) *model.OpenFlareMetricSnapshot {
	var latest *model.OpenFlareMetricSnapshot
	for _, snapshot := range snapshots {
		if snapshot == nil {
			continue
		}
		if latest == nil || snapshot.CapturedAt.After(latest.CapturedAt) {
			latest = snapshot
		}
	}
	return latest
}

func latestTrafficReport(reports []*model.OpenFlareRequestReport) *model.OpenFlareRequestReport {
	var latest *model.OpenFlareRequestReport
	for _, report := range reports {
		if report == nil {
			continue
		}
		if latest == nil || report.WindowEndedAt.After(latest.WindowEndedAt) {
			latest = report
		}
	}
	return latest
}

func matchOpenrestyObservation(
	capturedAt time.Time,
	observations []*model.OpenFlareNodeObservationOpenresty,
) *model.OpenFlareNodeObservationOpenresty {
	var matched *model.OpenFlareNodeObservationOpenresty
	bestDelta := metricSnapshotOpenrestyMatchWindow + time.Second
	for _, observation := range observations {
		if observation == nil {
			continue
		}
		delta := capturedAt.Sub(observation.CapturedAt)
		if delta < 0 {
			delta = -delta
		}
		if delta > metricSnapshotOpenrestyMatchWindow {
			continue
		}
		if matched == nil || delta < bestDelta {
			matched = observation
			bestDelta = delta
		}
	}
	return matched
}

// LatestMetricSnapshotsByNode returns the latest snapshot per node.
func LatestMetricSnapshotsByNode(snapshots []*model.OpenFlareMetricSnapshot) map[string]*model.OpenFlareMetricSnapshot {
	result := make(map[string]*model.OpenFlareMetricSnapshot, len(snapshots))
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

// LatestTrafficReportsByNode returns the latest traffic report per node.
func LatestTrafficReportsByNode(reports []*model.OpenFlareRequestReport) map[string]*model.OpenFlareRequestReport {
	result := make(map[string]*model.OpenFlareRequestReport, len(reports))
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

// ActiveHealthEventsByNode groups active health events by node id.
func ActiveHealthEventsByNode(events []*model.OpenFlareHealthEvent) map[string][]*model.OpenFlareHealthEvent {
	result := make(map[string][]*model.OpenFlareHealthEvent)
	for _, event := range events {
		if event == nil || event.NodeID == "" {
			continue
		}
		result[event.NodeID] = append(result[event.NodeID], event)
	}
	return result
}

// Percentage returns used/total as a percentage.
func Percentage(used int64, total int64) float64 {
	if used <= 0 || total <= 0 {
		return 0
	}
	return (float64(used) / float64(total)) * percentageMultiplier
}

func mergeJSONCounts(target distributionAccumulator, raw string) {
	if len(target) == 0 && strings.TrimSpace(raw) == "" {
		return
	}
	values := parseJSONCounts(raw)
	for key, value := range values {
		if strings.TrimSpace(key) == "" || value <= 0 {
			continue
		}
		target[key] += value
	}
}

func parseJSONCounts(raw string) map[string]int64 {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	values := make(map[string]int64)
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	return values
}

func toDistributionItems(values distributionAccumulator, limit int) []DistributionItem {
	if len(values) == 0 {
		return []DistributionItem{}
	}
	items := make([]DistributionItem, 0, len(values))
	for key, value := range values {
		if strings.TrimSpace(key) == "" || value <= 0 {
			continue
		}
		items = append(items, DistributionItem{Key: key, Value: value})
	}
	sort.Slice(items, func(i int, j int) bool {
		if items[i].Value == items[j].Value {
			return items[i].Key < items[j].Key
		}
		return items[i].Value > items[j].Value
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func trendWindowStart(now time.Time) time.Time {
	return now.Truncate(time.Hour).Add(-(observabilityTrendBuckets - 1) * time.Hour)
}

func trendBucketIndex(timestamp time.Time, start time.Time) (int, bool) {
	if timestamp.Before(start) {
		return 0, false
	}
	delta := timestamp.Sub(start)
	index := int(delta / time.Hour)
	if index < 0 || index >= observabilityTrendBuckets {
		return 0, false
	}
	return index, true
}
