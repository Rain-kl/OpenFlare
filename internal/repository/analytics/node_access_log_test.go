// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBatchInsertNodeAccessLogs_Empty(t *testing.T) {
	err := BatchInsertNodeAccessLogs(context.Background(), nil)
	require.NoError(t, err)
}

func TestBatchInsertNodeAccessLogs_UsesModelBatchSQL(t *testing.T) {
	ctx := context.Background()
	mockBatch := &mockBatch{}
	mockConn := &mockConn{
		batch:      mockBatch,
		batchQuery: analyticsmodel.NodeAccessLog{}.BatchInsertSQL(),
	}
	db.SetChConnForTest(mockConn)
	t.Cleanup(func() { db.SetChConnForTest(nil) })

	loggedAt := time.Now().UTC()
	err := BatchInsertNodeAccessLogs(ctx, []analyticsmodel.NodeAccessLog{
		{
			NodeID:     "node-a",
			LoggedAt:   loggedAt,
			RemoteAddr: "1.1.1.1",
			Region:     "US",
			Host:       "example.com",
			Path:       "/alpha",
			StatusCode: 200,
			BytesSent:  2048,
			CreatedAt:  loggedAt,
		},
	})
	require.NoError(t, err)
	assert.True(t, mockConn.prepareCalled)
	assert.Equal(t, analyticsmodel.NodeAccessLog{}.BatchInsertSQL(), mockConn.preparedQuery)
	assert.True(t, mockBatch.sendCalled)
	require.Len(t, mockBatch.rows, 1)
	assert.Equal(t, "node-a", mockBatch.rows[0][1])
	require.Len(t, mockBatch.rows[0], 14)
	assert.Equal(t, "", mockBatch.rows[0][7])            // user_agent
	assert.Equal(t, "", mockBatch.rows[0][8])            // cache_status
	assert.Equal(t, uint64(2048), mockBatch.rows[0][10]) // bytes_sent
	assert.Equal(t, uint64(0), mockBatch.rows[0][11])    // request_length
	assert.Equal(t, uint32(0), mockBatch.rows[0][12])    // request_time_ms
}
