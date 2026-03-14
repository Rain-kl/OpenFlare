package service

import (
	"atsflare/model"
	"testing"
	"time"
)

func TestBuildDiskIOTrendPointsUsesCounterDelta(t *testing.T) {
	now := time.Date(2026, 3, 14, 18, 30, 0, 0, time.UTC)
	start := trendWindowStart(now)

	points := buildDiskIOTrendPoints(now, []*model.NodeMetricSnapshot{
		{
			NodeID:         "node-a",
			CapturedAt:     start.Add(22 * time.Hour),
			DiskReadBytes:  100,
			DiskWriteBytes: 200,
		},
		{
			NodeID:         "node-a",
			CapturedAt:     start.Add(23 * time.Hour),
			DiskReadBytes:  250,
			DiskWriteBytes: 260,
		},
	})

	last := points[len(points)-1]
	if last.DiskReadBytes != 150 || last.DiskWriteBytes != 60 {
		t.Fatalf("expected disk io trend to use counter delta, got %+v", last)
	}
}
