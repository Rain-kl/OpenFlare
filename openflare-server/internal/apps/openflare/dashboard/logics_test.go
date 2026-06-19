// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"context"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupDashboardTestDB(t *testing.T) func() {
	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(&model.OpenFlareNode{}))

	db.SetDB(sqliteDB)
	return func() {
		db.SetDB(nil)
	}
}

func TestGetOverviewStructure(t *testing.T) {
	cleanup := setupDashboardTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()
	lastSeen := now.Add(-time.Minute)

	require.NoError(t, db.DB(ctx).Create(&model.OpenFlareNode{
		NodeID:          "node-dashboard-1",
		Name:            "Edge 1",
		IP:              "10.0.0.1",
		Status:          "online",
		OpenrestyStatus: "healthy",
		CurrentVersion:  "v1.0.0",
		LastSeenAt:      &lastSeen,
	}).Error)
	require.NoError(t, db.DB(ctx).Create(&model.OpenFlareNode{
		NodeID:          "node-dashboard-2",
		Name:            "Edge 2",
		IP:              "10.0.0.2",
		Status:          "pending",
		OpenrestyStatus: "unknown",
	}).Error)

	overview, err := GetOverview(ctx)
	require.NoError(t, err)
	require.NotNil(t, overview)

	assert.False(t, overview.GeneratedAt.(time.Time).IsZero())
	assert.Equal(t, 2, overview.Summary.TotalNodes)
	assert.Equal(t, 1, overview.Summary.OnlineNodes)
	assert.Equal(t, 1, overview.Summary.PendingNodes)
	assert.Equal(t, 0, overview.Summary.OfflineNodes)
	assert.Equal(t, 0, overview.Summary.UnhealthyNodes)

	assert.Equal(t, int64(0), overview.Traffic.RequestCount)
	assert.Equal(t, int64(0), overview.Traffic.UniqueVisitors)
	assert.Equal(t, int64(0), overview.Traffic.ErrorCount)
	assert.Equal(t, float64(0), overview.Traffic.EstimatedQPS)
	assert.Equal(t, 0, overview.Traffic.ReportedNodes)

	assert.Equal(t, float64(0), overview.Capacity.AverageCPUUsagePercent)
	assert.Equal(t, float64(0), overview.Capacity.AverageMemoryUsagePercent)
	assert.Equal(t, 0, overview.Capacity.HighCPUNodes)
	assert.Equal(t, 0, overview.Capacity.HighMemoryNodes)
	assert.Equal(t, 0, overview.Capacity.HighStorageNodes)

	require.NotNil(t, overview.Distributions.StatusCodes)
	require.NotNil(t, overview.Distributions.TopDomains)
	require.NotNil(t, overview.Distributions.SourceCountries)
	assert.Empty(t, overview.Distributions.StatusCodes)
	assert.Empty(t, overview.Distributions.TopDomains)
	assert.Empty(t, overview.Distributions.SourceCountries)

	require.Len(t, overview.Trends.Traffic24h, 24)
	require.Len(t, overview.Trends.Capacity24h, 24)
	require.Len(t, overview.Trends.Network24h, 24)
	require.Len(t, overview.Trends.DiskIO24h, 24)
	for _, row := range overview.Trends.Traffic24h {
		require.Len(t, row, 4)
	}
	for _, row := range overview.Trends.Capacity24h {
		require.Len(t, row, 4)
	}
	for _, row := range overview.Trends.Network24h {
		require.Len(t, row, 6)
	}
	for _, row := range overview.Trends.DiskIO24h {
		require.Len(t, row, 4)
	}

	require.Len(t, overview.Nodes, 2)
	for _, row := range overview.Nodes {
		require.Len(t, row, 17)
	}

	nodeByID := make(map[string][]any, len(overview.Nodes))
	for _, row := range overview.Nodes {
		nodeByID[row[1].(string)] = row
	}

	onlineNode := nodeByID["node-dashboard-1"]
	require.NotNil(t, onlineNode)
	assert.Equal(t, "Edge 1", onlineNode[2])
	assert.Equal(t, "online", onlineNode[6])
	assert.Equal(t, "healthy", onlineNode[7])

	pendingNode := nodeByID["node-dashboard-2"]
	require.NotNil(t, pendingNode)
	assert.Equal(t, "Edge 2", pendingNode[2])
	assert.Equal(t, "pending", pendingNode[6])
	assert.Equal(t, "unknown", pendingNode[7])
}
