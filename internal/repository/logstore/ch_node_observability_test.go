// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"testing"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCHBatchInsertNodeEdgeHealth_EmptyNodeID(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	defer ResetForTest()

	mockBatch := &chMockBatch{}
	mockConn := &chMockConn{
		batch:      mockBatch,
		batchQuery: analyticsmodel.NodeEdgeHealth{}.BatchInsertSQL(),
	}
	db.SetChConnForTest(mockConn)
	t.Cleanup(func() { db.SetChConnForTest(nil) })

	s := newClickHouseStore()
	err := s.BatchInsertNodeEdgeHealth(context.Background(), []analyticsmodel.NodeEdgeHealth{{}})
	require.NoError(t, err)
	assert.False(t, mockBatch.sendCalled)
	assert.Empty(t, mockBatch.rows)
}

func TestCHBatchInsertNodeEdgeHealth_UsesEdgeHealthBatchSQL(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	defer ResetForTest()

	ctx := context.Background()
	mockBatch := &chMockBatch{}
	mockConn := &chMockConn{
		batch:      mockBatch,
		batchQuery: analyticsmodel.NodeEdgeHealth{}.BatchInsertSQL(),
	}
	db.SetChConnForTest(mockConn)
	t.Cleanup(func() { db.SetChConnForTest(nil) })

	s := newClickHouseStore()
	capturedAt := time.Now().UTC()
	err := s.BatchInsertNodeEdgeHealth(ctx, []analyticsmodel.NodeEdgeHealth{{
		NodeID:      "node-a",
		CapturedAt:  capturedAt,
		Status:      "",
		Connections: 3,
		CreatedAt:   capturedAt,
	}})
	require.NoError(t, err)
	assert.True(t, mockConn.prepareCalled)
	assert.Equal(t, analyticsmodel.NodeEdgeHealth{}.BatchInsertSQL(), mockConn.preparedQuery)
	assert.True(t, mockBatch.sendCalled)
	require.Len(t, mockBatch.rows, 1)
	assert.Equal(t, "node-a", mockBatch.rows[0][1])
	assert.Equal(t, "unknown", mockBatch.rows[0][3]) // status default
	assert.Equal(t, int64(3), mockBatch.rows[0][4])  // connections
}
