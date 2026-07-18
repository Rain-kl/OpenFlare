// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
)

func TestBuildTrafficTrendPointsFromHourlyBucketsByHour(t *testing.T) {
	now := time.Date(2026, 7, 2, 15, 30, 0, 0, time.UTC)
	hourly := []*model.OpenFlareTrafficHourly{
		{
			NodeID:             "node-a",
			Hour:               now.Add(-2 * time.Hour).Truncate(time.Hour),
			RequestCount:       12,
			ErrorCount:         1,
			UniqueVisitorCount: 4,
		},
	}
	points := BuildTrafficTrendPointsFromHourly(now, hourly)
	if len(points) != observabilityTrendBuckets {
		t.Fatalf("BuildTrafficTrendPointsFromHourly() len = %d, want %d", len(points), observabilityTrendBuckets)
	}
	// Hourly UV must not be summed into trend points.
	for _, point := range points {
		if point.UniqueVisitorCount != 0 {
			t.Fatalf("UniqueVisitorCount = %d, want 0 on hourly path", point.UniqueVisitorCount)
		}
	}
	index, ok := trendBucketIndex(now.Add(-2*time.Hour).Truncate(time.Hour), trendWindowStart(now))
	if !ok {
		t.Fatal("expected valid bucket index")
	}
	if points[index].RequestCount != 12 {
		t.Fatalf("request_count = %d, want 12", points[index].RequestCount)
	}
	if points[index].ErrorCount != 1 {
		t.Fatalf("error_count = %d, want 1", points[index].ErrorCount)
	}
}

func TestBuildMetricSnapshotViewsMergesEdgeHealthConnections(t *testing.T) {
	t.Parallel()

	capturedAt := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	snapshots := []*model.OpenFlareMetricSnapshot{
		{
			ID:              1,
			NodeID:          "node-a",
			CapturedAt:      capturedAt,
			CPUUsagePercent: 12.5,
		},
	}
	edgeHealth := []*model.OpenFlareEdgeHealth{
		{
			NodeID:      "node-a",
			CapturedAt:  capturedAt.Add(5 * time.Second),
			Status:      "healthy",
			Connections: 7,
		},
	}

	views := BuildMetricSnapshotViews(snapshots, edgeHealth)
	if len(views) != 1 {
		t.Fatalf("BuildMetricSnapshotViews() len = %d, want 1", len(views))
	}
	if views[0].OpenrestyConnections != 7 {
		t.Fatalf("OpenrestyConnections = %d, want 7", views[0].OpenrestyConnections)
	}
}

func TestBuildTrafficWindowSummaryFromAccessLogsNilWithoutData(t *testing.T) {
	t.Parallel()

	// Without an access-log store / data, summary is nil.
	if summary := buildTrafficWindowSummaryFromAccessLogs(t.Context(), "missing", time.Now().Add(-time.Hour), time.Now()); summary != nil {
		t.Fatalf("buildTrafficWindowSummaryFromAccessLogs() = %#v, want nil", summary)
	}
}

func TestBuildCapacityTrendPointsFromHourlyFillsBuckets(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 10, 9, 30, 0, 0, time.UTC)
	hourly := []*model.OpenFlareMetricHourly{
		{
			Hour:                      now.Add(-3 * time.Hour).Truncate(time.Hour),
			AverageCPUUsagePercent:    42.5,
			AverageMemoryUsagePercent: 61.2,
			ReportedNodes:             1,
		},
		{
			Hour:                      now.Truncate(time.Hour),
			AverageCPUUsagePercent:    12.0,
			AverageMemoryUsagePercent: 50.0,
			ReportedNodes:             2,
		},
	}

	points := BuildCapacityTrendPointsFromHourly(now, hourly)
	if len(points) != observabilityTrendBuckets {
		t.Fatalf("len = %d, want %d", len(points), observabilityTrendBuckets)
	}
	if points[len(points)-4].AverageCPUUsagePercent != 42.5 {
		t.Fatalf("hour-3 cpu = %v, want 42.5", points[len(points)-4].AverageCPUUsagePercent)
	}
	if points[len(points)-1].ReportedNodes != 2 {
		t.Fatalf("current hour reported_nodes = %d, want 2", points[len(points)-1].ReportedNodes)
	}
}

func TestEmptyNetworkTrendPointsHas24Buckets(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 10, 9, 30, 0, 0, time.UTC)
	points := emptyNetworkTrendPoints(now)
	if len(points) != observabilityTrendBuckets {
		t.Fatalf("len(points) = %d, want %d", len(points), observabilityTrendBuckets)
	}
	if !points[0].BucketStartedAt.Before(points[len(points)-1].BucketStartedAt) {
		t.Fatalf("bucket order invalid: first=%v last=%v", points[0].BucketStartedAt, points[len(points)-1].BucketStartedAt)
	}
}

func TestBuildHealthSummaryUsesTrafficSummary(t *testing.T) {
	t.Parallel()

	snapshot := &model.OpenFlareMetricSnapshot{
		CPUUsagePercent:  10,
		MemoryUsedBytes:  1,
		MemoryTotalBytes: 10,
	}
	traffic := &TrafficWindowSummary{
		RequestCount: 200,
		ErrorCount:   20, // 10% error rate
	}
	summary := buildHealthSummary(snapshot, traffic, nil)
	if !summary.HasTrafficRisk {
		t.Fatal("HasTrafficRisk = false, want true for 10% error rate with >=100 requests")
	}
	if summary.HasCapacityRisk {
		t.Fatal("HasCapacityRisk = true, want false")
	}
}

func TestBuildDiskIOTrendPointsFromHourlyFillsBuckets(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 10, 9, 30, 0, 0, time.UTC)
	hourly := []*model.OpenFlareMetricHourly{
		{
			Hour:           now.Add(-1 * time.Hour).Truncate(time.Hour),
			DiskReadBytes:  1024,
			DiskWriteBytes: 2048,
			ReportedNodes:  1,
		},
	}

	points := BuildDiskIOTrendPointsFromHourly(now, hourly)
	prev := points[len(points)-2]
	if prev.DiskReadBytes != 1024 || prev.DiskWriteBytes != 2048 {
		t.Fatalf("previous hour disk io = %#v, want read=1024 write=2048", prev)
	}
}
