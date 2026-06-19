// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
)

const observabilityTrendBuckets = 24

const (
	healthEventStatusActive   = "active"
	healthEventStatusResolved = "resolved"
	healthSeverityCritical    = "critical"
	healthSeverityWarning     = "warning"
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

func buildTrafficWindowSummary(report *model.OpenFlareRequestReport) TrafficWindowSummary {
	if report == nil {
		return TrafficWindowSummary{}
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
	return summary
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

// BuildNetworkTrendPoints builds 24h network trend buckets.
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
	for _, snapshot := range snapshots {
		index, ok := trendBucketIndex(snapshot.CapturedAt, start)
		if !ok {
			continue
		}
		points[index].NetworkRxBytes += snapshot.NetworkRxBytes
		points[index].NetworkTxBytes += snapshot.NetworkTxBytes
		if snapshot.NodeID != "" {
			accumulators[index].nodes[snapshot.NodeID] = struct{}{}
		}
	}
	for _, obs := range openrestyObs {
		index, ok := trendBucketIndex(obs.CapturedAt, start)
		if !ok {
			continue
		}
		points[index].OpenrestyRxBytes += obs.OpenrestyRxBytes
		points[index].OpenrestyTxBytes += obs.OpenrestyTxBytes
		if obs.NodeID != "" {
			accumulators[index].nodes[obs.NodeID] = struct{}{}
		}
	}
	for index := range points {
		points[index].ReportedNodes = len(accumulators[index].nodes)
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
			nodeKey = "__unknown__"
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
		readDelta := snapshot.DiskReadBytes - previous.read
		writeDelta := snapshot.DiskWriteBytes - previous.write
		if readDelta < 0 {
			readDelta = 0
		}
		if writeDelta < 0 {
			writeDelta = 0
		}
		points[index].DiskReadBytes += readDelta
		points[index].DiskWriteBytes += writeDelta
		if snapshot.NodeID != "" {
			accumulators[index].nodes[snapshot.NodeID] = struct{}{}
		}
	}
	for index := range points {
		points[index].ReportedNodes = len(accumulators[index].nodes)
	}
	return points
}

func latestMetricSnapshot(snapshots []*model.OpenFlareMetricSnapshot) *model.OpenFlareMetricSnapshot {
	for _, snapshot := range snapshots {
		if snapshot != nil {
			return snapshot
		}
	}
	return nil
}

func latestTrafficReport(reports []*model.OpenFlareRequestReport) *model.OpenFlareRequestReport {
	for _, report := range reports {
		if report != nil {
			return report
		}
	}
	return nil
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
	return (float64(used) / float64(total)) * 100
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
