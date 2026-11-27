// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package smartdns

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/repository/logstore"
)

// OverviewSeriesPoint represents one aggregated DNS metrics point for overview charts.
type OverviewSeriesPoint struct {
	Timestamp     time.Time `json:"timestamp"`
	QPS           float64   `json:"qps"`
	LatencyP50Ms  float64   `json:"latency_p50_ms"`
	LatencyP95Ms  float64   `json:"latency_p95_ms"`
	LatencyP99Ms  float64   `json:"latency_p99_ms"`
	CacheHitRatio float64   `json:"cache_hit_ratio"`
	QueriesTotal  uint64    `json:"queries_total"`
	QueriesError  uint64    `json:"queries_error"`
}

// OverviewResult aggregates SmartDNS cluster KPIs, snapshot state, and latency/QPS timeseries.
type OverviewResult struct {
	Enabled          bool                  `json:"enabled"`
	DelegationDomain string                `json:"delegation_domain"`
	TotalInstances   int                   `json:"total_instances"`
	OnlineInstances  int                   `json:"online_instances"`
	TotalQPS         float64               `json:"total_qps"`
	AvgP50Ms         float64               `json:"avg_p50_ms"`
	AvgP95Ms         float64               `json:"avg_p95_ms"`
	AvgP99Ms         float64               `json:"avg_p99_ms"`
	AvgCacheHitRatio float64               `json:"avg_cache_hit_ratio"`
	ActivePolicies   int                   `json:"active_policies"`
	ManagedDomains   int                   `json:"managed_domains"`
	SnapshotVersion  uint64                `json:"snapshot_version"`
	SnapshotETag     string                `json:"snapshot_etag"`
	Window           string                `json:"window"`
	Series           []OverviewSeriesPoint `json:"series"`
}

// GetOverview collects cluster-wide SmartDNS telemetry and time-series metrics.
func GetOverview(ctx context.Context, instanceID, window string) (*OverviewResult, error) {
	settings := readSmartDNSSettingsMap(ctx)
	instances, err := repository.ListDNSInstances(ctx)
	if err != nil {
		return nil, err
	}
	policies, err := repository.ListEnabledDNSPolicies(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	out := &OverviewResult{
		Enabled:          settings.Enabled,
		DelegationDomain: settings.DelegationDomain,
		TotalInstances:   len(instances),
		ActivePolicies:   len(policies),
		Window:           normalizeOverviewWindow(window),
		Series:           []OverviewSeriesPoint{},
	}

	var p50Sum, p95Sum, p99Sum, cacheSum float64
	var activeCount int
	var qpsWeightedCache float64
	for i := range instances {
		inst := &instances[i]
		if computeInstanceStatus(inst, now) == model.DNSInstanceStatusOnline {
			out.OnlineInstances++
			out.TotalQPS += inst.QPS
			p50Sum += inst.P50Ms
			p95Sum += inst.P95Ms
			p99Sum += inst.P99Ms
			cacheSum += inst.CacheHitRatio
			qpsWeightedCache += inst.CacheHitRatio * inst.QPS
			activeCount++
		}
	}
	if activeCount > 0 {
		out.AvgP50Ms = round4(p50Sum / float64(activeCount))
		out.AvgP95Ms = round4(p95Sum / float64(activeCount))
		out.AvgP99Ms = round4(p99Sum / float64(activeCount))
		if out.TotalQPS > 0 {
			out.AvgCacheHitRatio = round4(qpsWeightedCache / out.TotalQPS)
		} else {
			out.AvgCacheHitRatio = round4(cacheSum / float64(activeCount))
		}
	}
	out.TotalQPS = round4(out.TotalQPS)

	if bundle := DefaultSnapshotManager().Current(); bundle != nil {
		out.SnapshotVersion = bundle.Envelope.Version
		out.SnapshotETag = bundle.ETag
		domainSet := make(map[string]struct{})
		for _, r := range bundle.Envelope.DomainRoutes {
			if r.TargetHost != "" {
				domainSet[r.TargetHost] = struct{}{}
			}
			for _, m := range r.MemberDomains {
				if m != "" {
					domainSet[m] = struct{}{}
				}
			}
		}
		out.ManagedDomains = len(domainSet)
	}

	since := now.Add(-overviewWindowDuration(out.Window))
	if store, sErr := logstore.Active(ctx); sErr == nil && store != nil {
		if rows, lErr := store.Observability.ListDNSMetricSnapshots(ctx, strings.TrimSpace(instanceID), since, 1000); lErr == nil {
			series := aggregateOverviewSeries(rows)
			out.Series = series
		}
	}
	return out, nil
}

func aggregateOverviewSeries(rows []analyticsmodel.DNSMetricSnapshot) []OverviewSeriesPoint {
	if len(rows) == 0 {
		return []OverviewSeriesPoint{}
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].CapturedAt.Before(rows[j].CapturedAt)
	})
	points := make([]OverviewSeriesPoint, 0, len(rows))
	for _, r := range rows {
		points = append(points, OverviewSeriesPoint{
			Timestamp:     r.CapturedAt.UTC(),
			QPS:           round4(r.QPS),
			LatencyP50Ms:  round4(r.LatencyP50Ms),
			LatencyP95Ms:  round4(r.LatencyP95Ms),
			LatencyP99Ms:  round4(r.LatencyP99Ms),
			CacheHitRatio: round4(r.CacheHitRatio),
			QueriesTotal:  r.QueriesTotal,
			QueriesError:  r.QueriesNXDomain + r.QueriesServFail + r.QueriesRateLimited,
		})
	}
	return points
}

func normalizeOverviewWindow(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "6h":
		return "6h"
	case "24h":
		return "24h"
	case "7d":
		return "7d"
	default:
		return "1h"
	}
}

func overviewWindowDuration(window string) time.Duration {
	switch window {
	case "6h":
		return 6 * time.Hour
	case "24h":
		return 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	default:
		return time.Hour
	}
}
