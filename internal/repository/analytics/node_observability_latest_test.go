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
	"github.com/Rain-kl/Wavelet/internal/db"
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

func TestListLatestNodeRequestReports_UsesLimit1ByNodeID(t *testing.T) {
	ctx := context.Background()
	mock := &mockConn{}
	db.SetChConnForTest(mock)
	t.Cleanup(func() { db.SetChConnForTest(nil) })

	_, err := ListLatestNodeRequestReports(ctx, NodeObservabilityFilter{NodeID: "node-a"})
	require.NoError(t, err)
	require.Len(t, mock.queries, 1)
	assert.Contains(t, mock.queries[0], "LIMIT 1 BY node_id")
	assert.Contains(t, mock.queries[0], nodeRequestReportTableName())
	assert.Contains(t, mock.queries[0], "window_ended_at DESC")
	require.Len(t, mock.queryArgs, 1)
	require.Len(t, mock.queryArgs[0], 1)
	assert.Equal(t, "node-a", mock.queryArgs[0][0])
}

func TestListNodeMetricHourly_PrefersRollup(t *testing.T) {
	ctx := context.Background()
	hour := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	mock := &mockConn{
		queryFn: func(_ context.Context, query string, _ ...any) (driver.Rows, error) {
			if strings.Contains(query, nodeMetricCapacityHourlyTableName()) {
				return &mockRows{data: [][]any{{
					hour, 42.5, 60.0, int64(100), int64(200), int64(10), int64(20), uint64(2),
				}}}, nil
			}
			return nil, errors.New("raw path should not be used when rollup has rows")
		},
	}
	db.SetChConnForTest(mock)
	t.Cleanup(func() { db.SetChConnForTest(nil) })

	rows, err := ListNodeMetricHourly(ctx, NodeObservabilityFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 42.5, rows[0].AverageCPUUsagePercent)
	assert.Equal(t, 60.0, rows[0].AverageMemoryUsagePercent)
	assert.Equal(t, int64(100), rows[0].NetworkRxBytes)
	assert.Equal(t, 2, rows[0].ReportedNodes)
	require.Len(t, mock.queries, 1)
	assert.Contains(t, mock.queries[0], nodeMetricCapacityHourlyTableName())
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

func TestListNodeOpenrestyHourly_PrefersRollup(t *testing.T) {
	ctx := context.Background()
	hour := time.Date(2026, 7, 10, 14, 0, 0, 0, time.UTC)
	mock := &mockConn{
		queryFn: func(_ context.Context, query string, _ ...any) (driver.Rows, error) {
			if strings.Contains(query, nodeOpenrestyHourlyTableName()) {
				return &mockRows{data: [][]any{{
					hour, int64(50), int64(70), uint64(3),
				}}}, nil
			}
			return nil, errors.New("raw path should not be used when rollup has rows")
		},
	}
	db.SetChConnForTest(mock)
	t.Cleanup(func() { db.SetChConnForTest(nil) })

	rows, err := ListNodeOpenrestyHourly(ctx, NodeObservabilityFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(50), rows[0].OpenrestyRxBytes)
	assert.Equal(t, int64(70), rows[0].OpenrestyTxBytes)
	assert.Equal(t, 3, rows[0].ReportedNodes)
}

func TestListNodeOpenrestyHourly_FallsBackToRawOnEmptyRollup(t *testing.T) {
	ctx := context.Background()
	hour := time.Date(2026, 7, 10, 15, 0, 0, 0, time.UTC)
	mock := &mockConn{
		queryFn: func(_ context.Context, query string, _ ...any) (driver.Rows, error) {
			if strings.Contains(query, nodeOpenrestyHourlyTableName()) {
				return &mockRows{}, nil
			}
			if strings.Contains(query, nodeObsOpenrestyTableName()) {
				return &mockRows{data: [][]any{{
					hour, int64(9), int64(8), uint64(1),
				}}}, nil
			}
			return &mockRows{}, nil
		},
	}
	db.SetChConnForTest(mock)
	t.Cleanup(func() { db.SetChConnForTest(nil) })

	rows, err := ListNodeOpenrestyHourly(ctx, NodeObservabilityFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(9), rows[0].OpenrestyRxBytes)
	require.GreaterOrEqual(t, len(mock.queries), 2)
	assert.Contains(t, mock.queries[1], "lagInFrame")
}

func TestNonNegativeCounterRange(t *testing.T) {
	assert.Equal(t, int64(0), nonNegativeCounterRange(10, 10))
	assert.Equal(t, int64(0), nonNegativeCounterRange(5, 10))
	assert.Equal(t, int64(15), nonNegativeCounterRange(25, 10))
}
