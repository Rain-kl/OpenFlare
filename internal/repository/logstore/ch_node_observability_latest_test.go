// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCHListLatestMetricSnapshots_UsesLimit1ByNodeID(t *testing.T) {
	ctx := context.Background()
	mock := &chMockConn{}
	db.SetChConnForTest(mock)
	t.Cleanup(func() { db.SetChConnForTest(nil) })

	s := newClickHouseStore()
	since := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	_, err := s.ListLatestMetricSnapshots(ctx, "", since)
	require.NoError(t, err)
	require.Len(t, mock.queries, 1)
	assert.Contains(t, mock.queries[0], "LIMIT 1 BY node_id")
	assert.Contains(t, mock.queries[0], chNodeMetricSnapshotTableName())
	assert.Contains(t, mock.queries[0], "captured_at DESC")
	assert.NotContains(t, mock.queries[0], "LIMIT ?")
	require.Len(t, mock.queryArgs, 1)
	require.Len(t, mock.queryArgs[0], 1)
	assert.Equal(t, since, mock.queryArgs[0][0])
}

func TestCHNodePageSamplingIsScopedPerNodeAndTask(t *testing.T) {
	mock := &chMockConn{}
	db.SetChConnForTest(mock)
	t.Cleanup(func() { db.SetChConnForTest(nil) })
	ctx := context.Background()
	store := newClickHouseStore()
	since := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	_, err := store.ListLatestMetricSnapshotsByNodeIDs(ctx, []string{"edge-a", "edge-b"}, since)
	require.NoError(t, err)
	_, err = store.ListRecentPingRecordsByTask(ctx, map[uint]analyticsmodel.PingTaskWindow{
		1: {NodeIDs: []string{"edge-a", "edge-b"}, Since: since},
		2: {NodeIDs: []string{"edge-b"}, Since: since.Add(-time.Hour)},
	}, 12)
	require.NoError(t, err)

	require.Len(t, mock.queries, 2)
	assert.Contains(t, mock.queries[0], "node_id IN (?, ?) AND captured_at >= ?")
	assert.Contains(t, mock.queries[0], "LIMIT 1 BY node_id")
	assert.Equal(t, []any{"edge-a", "edge-b", since}, mock.queryArgs[0])
	assert.Contains(t, mock.queries[1], "(task_id = ? AND recorded_at >= ? AND node_id IN (?, ?)) OR (task_id = ? AND recorded_at >= ? AND node_id IN (?))")
	assert.Contains(t, mock.queries[1], "LIMIT ? BY node_id, task_id")
	assert.Equal(t, []any{uint64(1), since, "edge-a", "edge-b", uint64(2), since.Add(-time.Hour), "edge-b", 12}, mock.queryArgs[1])
}

func TestCHListMetricHourly_PrefersRollup(t *testing.T) {
	ctx := context.Background()
	hour := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	since := hour.Add(-1 * time.Hour)
	mock := &chMockConn{
		queryFn: func(_ context.Context, query string, _ ...any) (driver.Rows, error) {
			if strings.Contains(query, chNodeMetricCapacityHourlyTableName()) {
				return &chMockRows{data: [][]any{{
					hour, 42.5, 60.0, int64(100), int64(200), int64(10), int64(20), uint64(2),
				}}}, nil
			}
			return nil, errors.New("raw path should not be used when rollup covers the window")
		},
	}
	db.SetChConnForTest(mock)
	t.Cleanup(func() { db.SetChConnForTest(nil) })

	s := newClickHouseStore()
	rows, err := s.ListMetricHourly(ctx, "", since)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.InDelta(t, 42.5, rows[0].AverageCPUUsagePercent, 1e-9)
	assert.InDelta(t, 60.0, rows[0].AverageMemoryUsagePercent, 1e-9)
	assert.Equal(t, int64(100), rows[0].NetworkRxBytes)
	assert.Equal(t, 2, rows[0].ReportedNodes)
	require.Len(t, mock.queries, 1)
	assert.Contains(t, mock.queries[0], chNodeMetricCapacityHourlyTableName())
}

func TestCHListMetricHourly_MergesRawGapsWithPartialRollup(t *testing.T) {
	ctx := context.Background()
	since := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	rollupHour := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	rawHour := time.Date(2026, 7, 9, 15, 0, 0, 0, time.UTC)
	mock := &chMockConn{
		queryFn: func(_ context.Context, query string, _ ...any) (driver.Rows, error) {
			if strings.Contains(query, chNodeMetricCapacityHourlyTableName()) {
				return &chMockRows{data: [][]any{{
					rollupHour, 99.0, 99.0, int64(1), int64(1), int64(1), int64(1), uint64(1),
				}}}, nil
			}
			if strings.Contains(query, chNodeMetricSnapshotTableName()) {
				return &chMockRows{data: [][]any{
					{rawHour, 12.0, 34.0, int64(5), int64(6), int64(7), int64(8), uint64(1)},
					{rollupHour, 50.0, 50.0, int64(9), int64(9), int64(9), int64(9), uint64(1)},
				}}, nil
			}
			return &chMockRows{}, nil
		},
	}
	db.SetChConnForTest(mock)
	t.Cleanup(func() { db.SetChConnForTest(nil) })

	s := newClickHouseStore()
	rows, err := s.ListMetricHourly(ctx, "", since)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, rawHour, rows[0].Hour)
	assert.InDelta(t, 12.0, rows[0].AverageCPUUsagePercent, 1e-9)
	assert.Equal(t, rollupHour, rows[1].Hour)
	assert.InDelta(t, 99.0, rows[1].AverageCPUUsagePercent, 1e-9)
	require.GreaterOrEqual(t, len(mock.queries), 2)
	assert.Contains(t, mock.queries[1], "lagInFrame")
}

func TestMergeNodeMetricHourlyPreferRollup(t *testing.T) {
	h1 := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	h2 := time.Date(2026, 7, 10, 11, 0, 0, 0, time.UTC)
	merged := mergeNodeMetricHourlyPreferRollup(
		[]analyticsmodel.NodeMetricHourly{{Hour: h2, AverageCPUUsagePercent: 80}},
		[]analyticsmodel.NodeMetricHourly{
			{Hour: h1, AverageCPUUsagePercent: 10},
			{Hour: h2, AverageCPUUsagePercent: 20},
		},
	)
	require.Len(t, merged, 2)
	assert.Equal(t, h1, merged[0].Hour)
	assert.InDelta(t, 10.0, merged[0].AverageCPUUsagePercent, 1e-9)
	assert.Equal(t, h2, merged[1].Hour)
	assert.InDelta(t, 80.0, merged[1].AverageCPUUsagePercent, 1e-9)
}

func TestHourlyRollupCoversWindow(t *testing.T) {
	since := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	assert.True(t, hourlyRollupCoversWindow(since, since))
	assert.True(t, hourlyRollupCoversWindow(since.Add(2*time.Hour), since))
	assert.False(t, hourlyRollupCoversWindow(since.Add(3*time.Hour), since))
	assert.True(t, hourlyRollupCoversWindow(time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC), time.Time{}))
}

func TestCHListMetricHourly_FallsBackToRawOnRollupError(t *testing.T) {
	ctx := context.Background()
	hour := time.Date(2026, 7, 10, 13, 0, 0, 0, time.UTC)
	mock := &chMockConn{
		queryFn: func(_ context.Context, query string, _ ...any) (driver.Rows, error) {
			if strings.Contains(query, chNodeMetricCapacityHourlyTableName()) {
				return nil, errors.New("rollup missing")
			}
			if strings.Contains(query, chNodeMetricSnapshotTableName()) {
				return &chMockRows{data: [][]any{{
					hour, 10.0, 20.0, int64(1), int64(2), int64(3), int64(4), uint64(1),
				}}}, nil
			}
			return &chMockRows{}, nil
		},
	}
	db.SetChConnForTest(mock)
	t.Cleanup(func() { db.SetChConnForTest(nil) })

	s := newClickHouseStore()
	rows, err := s.ListMetricHourly(ctx, "", time.Time{})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.InDelta(t, 10.0, rows[0].AverageCPUUsagePercent, 1e-9)
	assert.Equal(t, int64(3), rows[0].DiskReadBytes)
	require.GreaterOrEqual(t, len(mock.queries), 2)
	assert.Contains(t, mock.queries[0], chNodeMetricCapacityHourlyTableName())
	assert.Contains(t, mock.queries[1], "lagInFrame")
}
