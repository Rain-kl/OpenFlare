// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBatchInsertNodeAccessLogs_Empty(t *testing.T) {
	s := newClickHouseStore()
	err := s.BatchInsertNodeAccessLogs(context.Background(), nil)
	require.NoError(t, err)
}

func TestBatchInsertNodeAccessLogs_UsesModelBatchSQL(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	defer ResetForTest()

	ctx := context.Background()
	mockBatch := &chMockBatch{}
	mockConn := &chMockConn{
		batch:      mockBatch,
		batchQuery: analyticsmodel.NodeAccessLog{}.BatchInsertSQL(),
	}
	db.SetChConnForTest(mockConn)
	t.Cleanup(func() { db.SetChConnForTest(nil) })

	s := newClickHouseStore()
	loggedAt := time.Now().UTC()
	err := s.BatchInsertNodeAccessLogs(ctx, []analyticsmodel.NodeAccessLog{
		{
			NodeID:     "node-a",
			LoggedAt:   loggedAt,
			RemoteAddr: "1.1.1.1",
			Region:     "US",
			Host:       "example.com",
			Path:       "/alpha",
			StatusCode: 200,
			WAFAction:  "observed",
			WAFAudit:   `{"reason":"attack_protection"}`,
			BytesSent:  2048,
			ClientASN:  13335,
			TCPRTTUs:   4200,
			CreatedAt:  loggedAt,
		},
	})
	require.NoError(t, err)
	assert.True(t, mockConn.prepareCalled)
	assert.Equal(t, analyticsmodel.NodeAccessLog{}.BatchInsertSQL(), mockConn.preparedQuery)
	assert.True(t, mockBatch.sendCalled)
	require.Len(t, mockBatch.rows, 1)
	assert.Equal(t, "node-a", mockBatch.rows[0][1])
	require.Len(t, mockBatch.rows[0], 18)
	assert.Empty(t, mockBatch.rows[0][7])                // user_agent
	assert.Empty(t, mockBatch.rows[0][8])                // cache_status
	assert.Equal(t, uint64(2048), mockBatch.rows[0][10]) // bytes_sent
	assert.Equal(t, uint64(0), mockBatch.rows[0][11])    // request_length
	assert.Equal(t, uint32(0), mockBatch.rows[0][12])    // request_time_ms
	assert.Equal(t, "observed", mockBatch.rows[0][14])
	assert.JSONEq(t, `{"reason":"attack_protection"}`, mockBatch.rows[0][15].(string))
	assert.Equal(t, uint32(13335), mockBatch.rows[0][16])
	assert.Equal(t, uint32(4200), mockBatch.rows[0][17])
}

func TestCHRegionCounts_ScansAndSanitizesAverageLatencies(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	defer ResetForTest()

	mockConn := &chMockConn{
		queryFn: func(_ context.Context, _ string, _ ...any) (driver.Rows, error) {
			return &chMockRows{
				data: [][]any{
					{"Japan", uint64(42), 34.567, 18250.0},
					{"Germany", uint64(5), math.NaN(), 0.0},
				},
			}, nil
		},
	}
	db.SetChConnForTest(mockConn)
	t.Cleanup(func() { db.SetChConnForTest(nil) })

	s := newClickHouseStore()
	got, err := s.RegionCounts(context.Background(), "", time.Now().UTC().Add(-24*time.Hour), 250)
	require.NoError(t, err)
	require.Len(t, mockConn.queries, 1)
	assert.Contains(t, mockConn.queries[0], "avg(toFloat64(request_time_ms))")
	assert.Contains(t, mockConn.queries[0], "avgIf(toFloat64(tcp_rtt_us), tcp_rtt_us > 0)")
	require.Len(t, got, 2)
	assert.Equal(t, "Japan", got[0].Region)
	assert.Equal(t, int64(42), got[0].Count)
	assert.InDelta(t, 34.57, got[0].AvgRequestTimeMs, 1e-9)
	assert.InDelta(t, 18.25, got[0].AvgTCPRTTMs, 1e-9)
	assert.Equal(t, "Germany", got[1].Region)
	assert.Equal(t, float64(0), got[1].AvgRequestTimeMs)
	assert.Equal(t, float64(0), got[1].AvgTCPRTTMs)
}
