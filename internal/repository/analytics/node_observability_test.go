// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"testing"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsertNodeEdgeHealth_EmptyNodeID(t *testing.T) {
	err := InsertNodeEdgeHealth(context.Background(), analyticsmodel.NodeEdgeHealth{})
	require.NoError(t, err)
}

func TestInsertNodeEdgeHealth_UsesEdgeHealthBatchSQL(t *testing.T) {
	ctx := context.Background()
	mockBatch := &mockBatch{}
	mockConn := &mockConn{
		batch:      mockBatch,
		batchQuery: analyticsmodel.NodeEdgeHealth{}.BatchInsertSQL(),
	}
	db.SetChConnForTest(mockConn)
	t.Cleanup(func() { db.SetChConnForTest(nil) })

	capturedAt := time.Now().UTC()
	err := InsertNodeEdgeHealth(ctx, analyticsmodel.NodeEdgeHealth{
		NodeID:      "node-a",
		CapturedAt:  capturedAt,
		Status:      "",
		Connections: 3,
		CreatedAt:   capturedAt,
	})
	require.NoError(t, err)
	assert.True(t, mockConn.prepareCalled)
	assert.Equal(t, analyticsmodel.NodeEdgeHealth{}.BatchInsertSQL(), mockConn.preparedQuery)
	assert.True(t, mockBatch.sendCalled)
	require.Len(t, mockBatch.rows, 1)
	assert.Equal(t, "node-a", mockBatch.rows[0][1])
	assert.Equal(t, "unknown", mockBatch.rows[0][3]) // status default
	assert.Equal(t, int64(3), mockBatch.rows[0][4])  // connections
}
