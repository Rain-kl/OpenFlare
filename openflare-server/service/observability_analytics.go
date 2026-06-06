package service

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/rain-kl/openflare/openflare-server/model"
)

type DistributionItem struct {
	Key   string `json:"key"`
	Value int64  `json:"value"`
}

type TrafficDistributions struct {
	StatusCodes     []DistributionItem `json:"status_codes"`
	TopDomains      []DistributionItem `json:"top_domains"`
	SourceCountries []DistributionItem `json:"source_countries"`
}

type TrafficWindowSummary struct {
	WindowStartedAt    time.Time `json:"window_started_at"`
	WindowEndedAt      time.Time `json:"window_ended_at"`
	RequestCount       int64     `json:"request_count"`
	UniqueVisitorCount int64     `json:"unique_visitor_count"`
	ErrorCount         int64     `json:"error_count"`
	EstimatedQPS       float64   `json:"estimated_qps"`
	ErrorRatePercent   float64   `json:"error_rate_percent"`
}

type ObservabilityHealthSummary struct {
	ActiveAlerts    int  `json:"active_alerts"`
	CriticalAlerts  int  `json:"critical_alerts"`
	WarningAlerts   int  `json:"warning_alerts"`
	InfoAlerts      int  `json:"info_alerts"`
	ResolvedAlerts  int  `json:"resolved_alerts"`
	HasCapacityRisk bool `json:"has_capacity_risk"`
	HasTrafficRisk  bool `json:"has_traffic_risk"`
	HasRuntimeRisk  bool `json:"has_runtime_risk"`
}

type distributionAccumulator map[string]int64

func buildTrafficWindowSummary(report *model.NodeRequestReport) TrafficWindowSummary {
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

func buildTrafficDistributions(
	reports []*model.NodeRequestReport,
	accessLogRegions []*model.NodeAccessLogRegionCount,
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

func buildObservabilityHealthSummary(snapshot *model.NodeMetricSnapshot, report *model.NodeRequestReport, events []*model.NodeHealthEvent) ObservabilityHealthSummary {
	summary := ObservabilityHealthSummary{}
	for _, event := range events {
		if event == nil {
			continue
		}
		if event.Status == NodeHealthEventStatusResolved {
			summary.ResolvedAlerts++
			continue
		}
		summary.ActiveAlerts++
		switch event.Severity {
		case NodeHealthSeverityCritical:
			summary.CriticalAlerts++
		case NodeHealthSeverityWarning:
			summary.WarningAlerts++
		default:
			summary.InfoAlerts++
		}
	}
	if snapshot != nil {
		memoryUsage := percentage(snapshot.MemoryUsedBytes, snapshot.MemoryTotalBytes)
		storageUsage := percentage(snapshot.StorageUsedBytes, snapshot.StorageTotalBytes)
		summary.HasCapacityRisk = snapshot.CPUUsagePercent >= 80 || memoryUsage >= 85 || storageUsage >= 85
	}
	if report != nil && report.RequestCount >= 100 {
		summary.HasTrafficRisk = (float64(report.ErrorCount) / float64(report.RequestCount)) >= 0.05
	}
	summary.HasRuntimeRisk = summary.ActiveAlerts > 0 || summary.HasCapacityRisk || summary.HasTrafficRisk
	return summary
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
