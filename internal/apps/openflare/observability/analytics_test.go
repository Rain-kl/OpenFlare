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
}

func TestBuildTrafficTrendPointsBucketsByHour(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 19, 17, 30, 0, 0, time.UTC)
	reports := []*model.OpenFlareRequestReport{
		{
			NodeID:          "node-a",
			WindowStartedAt: now.Add(-3 * time.Hour),
			WindowEndedAt:   now.Add(-3*time.Hour + time.Minute),
			RequestCount:    10,
			ErrorCount:      1,
		},
		{
			NodeID:          "node-a",
			WindowStartedAt: now.Add(-30 * time.Minute),
			WindowEndedAt:   now.Add(-29 * time.Minute),
			RequestCount:    6,
			ErrorCount:      0,
		},
	}

	points := BuildTrafficTrendPoints(now, reports)
	if len(points) != observabilityTrendBuckets {
		t.Fatalf("BuildTrafficTrendPoints() len = %d, want %d", len(points), observabilityTrendBuckets)
	}

	var totalRequests int64
	for _, point := range points {
		totalRequests += point.RequestCount
	}
	if totalRequests != 16 {
		t.Fatalf("total request_count = %d, want 16", totalRequests)
	}

	currentHour := points[len(points)-1]
	if currentHour.RequestCount != 6 {
		t.Fatalf("current hour request_count = %d, want 6", currentHour.RequestCount)
	}
	if currentHour.ErrorCount != 0 {
		t.Fatalf("current hour error_count = %d, want 0", currentHour.ErrorCount)
	}
}

func TestBuildMetricSnapshotViewsMergesOpenrestyObservation(t *testing.T) {
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
	openrestyObs := []*model.OpenFlareNodeObservationOpenresty{
		{
			NodeID:               "node-a",
			CapturedAt:           capturedAt.Add(5 * time.Second),
			OpenrestyRxBytes:     4096,
			OpenrestyTxBytes:     8192,
			OpenrestyConnections: 7,
		},
	}

	views := BuildMetricSnapshotViews(snapshots, openrestyObs)
	if len(views) != 1 {
		t.Fatalf("BuildMetricSnapshotViews() len = %d, want 1", len(views))
	}
	if views[0].OpenrestyRxBytes != 4096 {
		t.Fatalf("OpenrestyRxBytes = %d, want 4096", views[0].OpenrestyRxBytes)
	}
	if views[0].OpenrestyTxBytes != 8192 {
		t.Fatalf("OpenrestyTxBytes = %d, want 8192", views[0].OpenrestyTxBytes)
	}
	if views[0].OpenrestyConnections != 7 {
		t.Fatalf("OpenrestyConnections = %d, want 7", views[0].OpenrestyConnections)
	}
}

func TestLatestTrafficReportUsesLatestWindowEndedAt(t *testing.T) {
	t.Parallel()

	older := &model.OpenFlareRequestReport{
		WindowEndedAt: time.Date(2026, 6, 19, 10, 0, 0, 0, time.UTC),
		RequestCount:  3,
	}
	newer := &model.OpenFlareRequestReport{
		WindowEndedAt: time.Date(2026, 6, 19, 11, 0, 0, 0, time.UTC),
		RequestCount:  9,
	}

	latest := latestTrafficReport([]*model.OpenFlareRequestReport{older, newer})
	if latest == nil || latest.RequestCount != 9 {
		t.Fatalf("latestTrafficReport() = %#v, want newer report with request_count 9", latest)
	}
}

func TestBuildTrafficWindowSummaryNilWithoutReport(t *testing.T) {
	t.Parallel()

	if summary := buildTrafficWindowSummary(nil); summary != nil {
		t.Fatalf("buildTrafficWindowSummary(nil) = %#v, want nil", summary)
	}
}
