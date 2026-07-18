// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"context"
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
	sortOrderAsc              = "asc"
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

const metricSnapshotEdgeHealthMatchWindow = 2 * time.Minute

// NodeMetricSnapshotView is a metric snapshot enriched with edge health connections.
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

// NetworkTrendPoint is a business-byte trend bucket from access logs (L1).
// Host NIC trends are intentionally not exposed.
type NetworkTrendPoint struct {
	BucketStartedAt time.Time `json:"bucket_started_at"`
	BytesReceived   int64     `json:"bytes_received"` // sum(request_length)
	BytesProvided   int64     `json:"bytes_provided"` // sum(bytes_sent)
	ReportedNodes   int       `json:"reported_nodes"`
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

func buildTrafficWindowSummaryFromAccessLogs(
	ctx context.Context,
	nodeID string,
	since, until time.Time,
) *TrafficWindowSummary {
	row, err := model.TrafficSummaryOpenFlareAccessLogs(ctx, model.OpenFlareAccessLogQuery{
		NodeID: nodeID,
		Since:  since,
		Until:  until,
	})
	if err != nil || row.RequestCount <= 0 {
		return nil
	}
	summary := &TrafficWindowSummary{
		WindowStartedAt:    since.UTC(),
		WindowEndedAt:      until.UTC(),
		RequestCount:       row.RequestCount,
		UniqueVisitorCount: row.UniqueIPCount,
		ErrorCount:         row.ErrorCount,
	}
	if duration := until.Sub(since).Seconds(); duration > 0 {
		summary.EstimatedQPS = float64(row.RequestCount) / duration
	}
	if row.RequestCount > 0 {
		summary.ErrorRatePercent = (float64(row.ErrorCount) / float64(row.RequestCount)) * 100
	}
	return summary
}

// BuildMetricSnapshotViews merges metric snapshots with edge health connections for API responses.
func BuildMetricSnapshotViews(
	snapshots []*model.OpenFlareMetricSnapshot,
	edgeHealth []*model.OpenFlareEdgeHealth,
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
		}
		if matched := matchEdgeHealth(snapshot.CapturedAt, edgeHealth); matched != nil {
			view.OpenrestyConnections = matched.Connections
		}
		views = append(views, view)
	}
	return views
}

// BuildTrafficDistributionsFromAccessLogs builds distributions from access logs (L1).
func BuildTrafficDistributionsFromAccessLogs(
	ctx context.Context,
	since, until time.Time,
	limit int,
	accessLogRegions []*model.OpenFlareAccessLogRegionCount,
) TrafficDistributions {
	statusCodes := make(distributionAccumulator)
	topDomains := make(distributionAccumulator)

	query := model.OpenFlareAccessLogQuery{Since: since, Until: until}
	if statusRows, err := model.ValueCountsOpenFlareAccessLogs(ctx, query, "status_code", limit); err == nil {
		for _, row := range statusRows {
			if strings.TrimSpace(row.Value) == "" || row.Count <= 0 {
				continue
			}
			statusCodes[row.Value] = row.Count
		}
	}
	if hostRows, err := model.ValueCountsOpenFlareAccessLogs(ctx, query, "host", limit); err == nil {
		for _, row := range hostRows {
			if strings.TrimSpace(row.Value) == "" || row.Count <= 0 {
				continue
			}
			topDomains[row.Value] = row.Count
		}
	}

	sourceCountries := make(distributionAccumulator)
	for _, item := range accessLogRegions {
		if item == nil || strings.TrimSpace(item.Region) == "" || item.Count <= 0 {
			continue
		}
		sourceCountries[item.Region] = item.Count
	}
	return TrafficDistributions{
		StatusCodes:     toDistributionItems(statusCodes, limit),
		TopDomains:      toDistributionItems(topDomains, limit),
		SourceCountries: toDistributionItems(sourceCountries, limit),
	}
}

func buildHealthSummary(
	snapshot *model.OpenFlareMetricSnapshot,
	traffic *TrafficWindowSummary,
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
	if traffic != nil && traffic.RequestCount >= 100 {
		summary.HasTrafficRisk = (float64(traffic.ErrorCount) / float64(traffic.RequestCount)) >= 0.05
	}
	summary.HasRuntimeRisk = summary.ActiveAlerts > 0 || summary.HasCapacityRisk || summary.HasTrafficRisk
	return summary
}

// BuildNodeTrends builds 24h trend series.
// Business traffic (requests/errors and provided/received bytes) comes from access logs.
// Host capacity/disk come from metric snapshots (hourly when available). Host NIC is not tracked.
func BuildNodeTrends(
	ctx context.Context,
	now time.Time,
	nodeID string,
	snapshots []*model.OpenFlareMetricSnapshot,
) NodeTrends {
	trendSince := now.Add(-24 * time.Hour)

	trafficTrend := BuildTrafficTrendPointsFromAccessLogs(ctx, now, nodeID, trendSince)
	capacityTrend := BuildCapacityTrendPoints(now, snapshots)
	networkTrend := emptyNetworkTrendPoints(now)
	applyAccessLogBytesToNetworkTrend(ctx, now, nodeID, trendSince, networkTrend)
	diskIOTrend := BuildDiskIOTrendPoints(now, snapshots)

	metricHourly, metricErr := model.ListOpenFlareMetricHourlySince(ctx, nodeID, trendSince)
	if metricErr == nil && len(metricHourly) > 0 {
		capacityTrend = BuildCapacityTrendPointsFromHourly(now, metricHourly)
		diskIOTrend = BuildDiskIOTrendPointsFromHourly(now, metricHourly)
	}

	return NodeTrends{
		Traffic24h:  trafficTrend,
		Capacity24h: capacityTrend,
		Network24h:  networkTrend,
		DiskIO24h:   diskIOTrend,
	}
}

// BuildTrafficTrendPointsFromAccessLogs builds 24h request/error buckets from access logs.
// Prefers of_access_log_hourly when available; falls back to raw bucket aggregates.
// UniqueVisitorCount on hourly path is 0 (use TrafficSummary for exact UV).
func BuildTrafficTrendPointsFromAccessLogs(ctx context.Context, now time.Time, nodeID string, since time.Time) []TrafficTrendPoint {
	start := trendWindowStart(now)
	points := make([]TrafficTrendPoint, observabilityTrendBuckets)
	for index := range points {
		points[index].BucketStartedAt = start.Add(time.Duration(index) * time.Hour)
	}

	if hourly, err := model.ListOpenFlareTrafficHourlySince(ctx, nodeID, since); err == nil && len(hourly) > 0 {
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
			// UniqueVisitorCount intentionally not summed from hourly rollup (always 0 / overcounts).
		}
		return points
	}

	buckets, err := model.ListOpenFlareAccessLogBuckets(ctx, model.OpenFlareAccessLogBucketQuery{
		NodeID:      nodeID,
		Since:       since,
		Until:       now,
		FoldMinutes: 60,
		SortBy:      defaultAccessLogSortBy,
		SortOrder:   sortOrderAsc,
	})
	if err != nil || len(buckets) == 0 {
		return points
	}
	byEpoch := make(map[int64]*model.OpenFlareAccessLogBucketRow, len(buckets))
	for _, row := range buckets {
		if row == nil {
			continue
		}
		byEpoch[row.BucketEpoch] = row
	}
	for index := range points {
		epoch := points[index].BucketStartedAt.Unix()
		if row, ok := byEpoch[epoch]; ok {
			points[index].RequestCount = row.RequestCount
			points[index].ErrorCount = row.ServerErrorCount
			points[index].UniqueVisitorCount = row.UniqueIPCount
		}
	}
	return points
}

func applyAccessLogBytesToNetworkTrend(ctx context.Context, now time.Time, nodeID string, since time.Time, points []NetworkTrendPoint) {
	if len(points) == 0 {
		return
	}
	// Prefer of_access_log_hourly (summed across hosts).
	if hourly, err := analyticsListAccessLogHourlyBytes(ctx, nodeID, since); err == nil && len(hourly) > 0 {
		for hourUnix, totals := range hourly {
			for index := range points {
				if points[index].BucketStartedAt.Unix() == hourUnix {
					points[index].BytesProvided = totals.provided
					points[index].BytesReceived = totals.received
				}
			}
		}
		return
	}

	buckets, err := model.ListOpenFlareAccessLogBuckets(ctx, model.OpenFlareAccessLogBucketQuery{
		NodeID:      nodeID,
		Since:       since,
		Until:       now,
		FoldMinutes: 60,
		SortBy:      defaultAccessLogSortBy,
		SortOrder:   sortOrderAsc,
	})
	if err != nil || len(buckets) == 0 {
		return
	}
	byEpoch := make(map[int64]*model.OpenFlareAccessLogBucketRow, len(buckets))
	for _, row := range buckets {
		if row == nil {
			continue
		}
		byEpoch[row.BucketEpoch] = row
	}
	for index := range points {
		epoch := points[index].BucketStartedAt.Unix()
		if row, ok := byEpoch[epoch]; ok {
			points[index].BytesProvided = row.BytesSent
			points[index].BytesReceived = row.RequestLength
		}
	}
}

type accessLogHourBytes struct {
	provided int64
	received int64
}

func analyticsListAccessLogHourlyBytes(ctx context.Context, nodeID string, since time.Time) (map[int64]accessLogHourBytes, error) {
	rows, err := model.ListOpenFlareAccessLogHourlySince(ctx, nodeID, since)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]accessLogHourBytes)
	for _, row := range rows {
		if row == nil {
			continue
		}
		key := row.Hour.UTC().Truncate(time.Hour).Unix()
		cur := out[key]
		cur.provided += row.BytesSent
		cur.received += row.RequestLength
		out[key] = cur
	}
	return out, nil
}

// BuildTrafficTrendPointsFromHourly builds 24h traffic trend buckets from hourly rollups.
// UniqueVisitorCount is left at 0: hourly UV is not summed (use TrafficSummary for exact UV).
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

func emptyNetworkTrendPoints(now time.Time) []NetworkTrendPoint {
	start := trendWindowStart(now)
	points := make([]NetworkTrendPoint, observabilityTrendBuckets)
	for index := range points {
		points[index].BucketStartedAt = start.Add(time.Duration(index) * time.Hour)
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

func matchEdgeHealth(
	capturedAt time.Time,
	health []*model.OpenFlareEdgeHealth,
) *model.OpenFlareEdgeHealth {
	var matched *model.OpenFlareEdgeHealth
	bestDelta := metricSnapshotEdgeHealthMatchWindow + time.Second
	for _, row := range health {
		if row == nil {
			continue
		}
		delta := capturedAt.Sub(row.CapturedAt)
		if delta < 0 {
			delta = -delta
		}
		if delta > metricSnapshotEdgeHealthMatchWindow {
			continue
		}
		if matched == nil || delta < bestDelta {
			matched = row
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
