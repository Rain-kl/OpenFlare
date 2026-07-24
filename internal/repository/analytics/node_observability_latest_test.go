// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListLatestNodeMetricSnapshots_UsesLimit1ByNodeID(t *testing.T) {
	ctx := context.Background()
	mock := &mockConn{}
	db.SetChConnForTest(mock)
	t.Cleanup(func() { db.SetChConnForTest(nil) })

	since := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	_, err := ListLatestNodeMetricSnapshots(ctx, NodeObservabilityFilter{Since: since})
	require.NoError(t, err)
	require.Len(t, mock.queries, 1)
	assert.Contains(t, mock.queries[0], "LIMIT 1 BY node_id")
	assert.Contains(t, mock.queries[0], nodeMetricSnapshotTableName())
	assert.Contains(t, mock.queries[0], "captured_at DESC")
	assert.NotContains(t, mock.queries[0], "LIMIT ?")
	require.Len(t, mock.queryArgs, 1)
	require.Len(t, mock.queryArgs[0], 1)
	assert.Equal(t, since, mock.queryArgs[0][0])
}

func TestListNodeMetricHourly_PrefersRollup(t *testing.T) {
	ctx := context.Background()
	hour := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	since := hour.Add(-1 * time.Hour)
	mock := &mockConn{
		queryFn: func(_ context.Context, query string, _ ...any) (driver.Rows, error) {
			if strings.Contains(query, nodeMetricCapacityHourlyTableName()) {
				return &mockRows{data: [][]any{{
					hour, 42.5, 60.0, int64(100), int64(200), int64(10), int64(20), uint64(2),
				}}}, nil
			}
			return nil, errors.New("raw path should not be used when rollup covers the window")
		},
	}
	db.SetChConnForTest(mock)
	t.Cleanup(func() { db.SetChConnForTest(nil) })

	rows, err := ListNodeMetricHourly(ctx, NodeObservabilityFilter{Since: since})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 42.5, rows[0].AverageCPUUsagePercent)
	assert.Equal(t, 60.0, rows[0].AverageMemoryUsagePercent)
	assert.Equal(t, int64(100), rows[0].NetworkRxBytes)
	assert.Equal(t, 2, rows[0].ReportedNodes)
	require.Len(t, mock.queries, 1)
	assert.Contains(t, mock.queries[0], nodeMetricCapacityHourlyTableName())
}

func TestListNodeMetricHourly_MergesRawGapsWithPartialRollup(t *testing.T) {
	ctx := context.Background()
	// 24h window starts far before the only rollup bucket (last hour).
	since := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	rollupHour := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	rawHour := time.Date(2026, 7, 9, 15, 0, 0, 0, time.UTC)
	mock := &mockConn{
		queryFn: func(_ context.Context, query string, _ ...any) (driver.Rows, error) {
			if strings.Contains(query, nodeMetricCapacityHourlyTableName()) {
				return &mockRows{data: [][]any{{
					rollupHour, 99.0, 99.0, int64(1), int64(1), int64(1), int64(1), uint64(1),
				}}}, nil
			}
			if strings.Contains(query, nodeMetricSnapshotTableName()) {
				return &mockRows{data: [][]any{
					{rawHour, 12.0, 34.0, int64(5), int64(6), int64(7), int64(8), uint64(1)},
					{rollupHour, 50.0, 50.0, int64(9), int64(9), int64(9), int64(9), uint64(1)},
				}}, nil
			}
			return &mockRows{}, nil
		},
	}
	db.SetChConnForTest(mock)
	t.Cleanup(func() { db.SetChConnForTest(nil) })

	rows, err := ListNodeMetricHourly(ctx, NodeObservabilityFilter{Since: since})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, rawHour, rows[0].Hour)
	assert.Equal(t, 12.0, rows[0].AverageCPUUsagePercent)
	// Overlapping hour prefers rollup (99) over raw (50).
	assert.Equal(t, rollupHour, rows[1].Hour)
	assert.Equal(t, 99.0, rows[1].AverageCPUUsagePercent)
	require.GreaterOrEqual(t, len(mock.queries), 2)
	assert.Contains(t, mock.queries[1], "lagInFrame")
}

func TestMergeNodeMetricHourlyPreferRollup(t *testing.T) {
	h1 := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	h2 := time.Date(2026, 7, 10, 11, 0, 0, 0, time.UTC)
	merged := mergeNodeMetricHourlyPreferRollup(
		[]NodeMetricHourly{{Hour: h2, AverageCPUUsagePercent: 80}},
		[]NodeMetricHourly{
			{Hour: h1, AverageCPUUsagePercent: 10},
			{Hour: h2, AverageCPUUsagePercent: 20},
		},
	)
	require.Len(t, merged, 2)
	assert.Equal(t, h1, merged[0].Hour)
	assert.Equal(t, 10.0, merged[0].AverageCPUUsagePercent)
	assert.Equal(t, h2, merged[1].Hour)
	assert.Equal(t, 80.0, merged[1].AverageCPUUsagePercent)
}

func TestHourlyRollupCoversWindow(t *testing.T) {
	since := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	assert.True(t, hourlyRollupCoversWindow(since, since))
	assert.True(t, hourlyRollupCoversWindow(since.Add(2*time.Hour), since))
	assert.False(t, hourlyRollupCoversWindow(since.Add(3*time.Hour), since))
	assert.True(t, hourlyRollupCoversWindow(time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC), time.Time{}))
}

func TestListNodeMetricHourly_FallsBackToRawOnRollupError(t *testing.T) {
	ctx := context.Background()
	hour := time.Date(2026, 7, 10, 13, 0, 0, 0, time.UTC)
	mock := &mockConn{
		queryFn: func(_ context.Context, query string, _ ...any) (driver.Rows, error) {
			if strings.Contains(query, nodeMetricCapacityHourlyTableName()) {
				return nil, errors.New("rollup missing")
			}
			if strings.Contains(query, nodeMetricSnapshotTableName()) {
				return &mockRows{data: [][]any{{
					hour, 10.0, 20.0, int64(1), int64(2), int64(3), int64(4), uint64(1),
				}}}, nil
			}
			return &mockRows{}, nil
		},
	}
	db.SetChConnForTest(mock)
	t.Cleanup(func() { db.SetChConnForTest(nil) })

	rows, err := ListNodeMetricHourly(ctx, NodeObservabilityFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 10.0, rows[0].AverageCPUUsagePercent)
	assert.Equal(t, int64(3), rows[0].DiskReadBytes)
	require.GreaterOrEqual(t, len(mock.queries), 2)
	assert.Contains(t, mock.queries[0], nodeMetricCapacityHourlyTableName())
	assert.Contains(t, mock.queries[1], "lagInFrame")
}
