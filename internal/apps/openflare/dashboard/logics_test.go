// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"context"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/repository"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
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
	resetAccessLogStore := repository.SetAccessLogStoreForTest(repository.NewMemoryAccessLogStore())
	resetObservabilityStore := repository.SetObservabilityStoreForTest(repository.NewMemoryObservabilityStore())
	return func() {
		resetObservabilityStore()
		resetAccessLogStore()
		db.SetDB(nil)
	}
}

func TestGetOverviewStructure(t *testing.T) {
	cleanup := setupDashboardTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()
	lastSeen := now.Add(-15 * time.Second) // within default 60s offline threshold

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

	// Seed older + newer snapshots per node; health must use latest-per-node, not a global raw limit.
	require.NoError(t, repository.InsertOpenFlareMetricSnapshot(ctx, &model.OpenFlareMetricSnapshot{
		NodeID:           "node-dashboard-1",
		CapturedAt:       now.Add(-2 * time.Hour),
		CPUUsagePercent:  10,
		MemoryUsedBytes:  1,
		MemoryTotalBytes: 10,
	}))
	require.NoError(t, repository.InsertOpenFlareMetricSnapshot(ctx, &model.OpenFlareMetricSnapshot{
		NodeID:            "node-dashboard-1",
		CapturedAt:        now.Add(-time.Minute),
		CPUUsagePercent:   55,
		MemoryUsedBytes:   5,
		MemoryTotalBytes:  10,
		StorageUsedBytes:  2,
		StorageTotalBytes: 10,
	}))
	// Business traffic from access logs (L1 authority): 12 requests, 1 server error, 4 unique IPs.
	logs := make([]*model.OpenFlareAccessLog, 0, 12)
	for i := 0; i < 11; i++ {
		logs = append(logs, &model.OpenFlareAccessLog{
			NodeID:     "node-dashboard-1",
			LoggedAt:   now.Add(-time.Minute),
			RemoteAddr: "10.0.0." + string(rune('1'+i%4)), // rough; fixed below
			Host:       "app.example.com",
			Path:       "/",
			StatusCode: 200,
			BytesSent:  100,
		})
	}
	ips := []string{"10.0.0.10", "10.0.0.11", "10.0.0.12", "10.0.0.13"}
	for i := 0; i < 11; i++ {
		logs[i].RemoteAddr = ips[i%4]
	}
	logs = append(logs, &model.OpenFlareAccessLog{
		NodeID:     "node-dashboard-1",
		LoggedAt:   now.Add(-time.Minute),
		RemoteAddr: ips[0],
		Host:       "app.example.com",
		Path:       "/err",
		StatusCode: 502,
		BytesSent:  10,
	})
	require.NoError(t, repository.InsertOpenFlareAccessLogsBatch(ctx, logs))

	overview, err := GetOverview(ctx)
	require.NoError(t, err)
	require.NotNil(t, overview)

	assert.False(t, overview.GeneratedAt.(time.Time).IsZero())
	assert.Equal(t, 2, overview.Summary.TotalNodes)
	assert.Equal(t, 1, overview.Summary.OnlineNodes)
	assert.Equal(t, 1, overview.Summary.PendingNodes)
	assert.Equal(t, 0, overview.Summary.OfflineNodes)
	assert.Equal(t, 0, overview.Summary.UnhealthyNodes)

	assert.Equal(t, int64(12), overview.Traffic.RequestCount)
	assert.Equal(t, int64(4), overview.Traffic.UniqueVisitors)
	assert.Equal(t, int64(1), overview.Traffic.ErrorCount)
	// QPS over 24h window
	assert.InDelta(t, 12.0/(24*3600.0), overview.Traffic.EstimatedQPS, 0.0001)
	assert.Equal(t, 1, overview.Traffic.ReportedNodes)
	// Node-level traffic from access log aggregates
	onlineNodeCheck := overview.Nodes
	require.NotEmpty(t, onlineNodeCheck)

	assert.Equal(t, 55.0, overview.Capacity.AverageCPUUsagePercent)
	assert.Equal(t, 50.0, overview.Capacity.AverageMemoryUsagePercent)
	assert.Equal(t, 0, overview.Capacity.HighCPUNodes)
	assert.Equal(t, 0, overview.Capacity.HighMemoryNodes)
	assert.Equal(t, 0, overview.Capacity.HighStorageNodes)

	require.NotNil(t, overview.Distributions.StatusCodes)
	require.NotNil(t, overview.Distributions.TopDomains)
	require.NotNil(t, overview.Distributions.SourceCountries)
	// Status/top domains come from access logs.
	assert.NotEmpty(t, overview.Distributions.StatusCodes)
	assert.NotEmpty(t, overview.Distributions.TopDomains)
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
		require.Len(t, row, 4)
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
	// Latest-per-node health fields (indexes match compressDashboardNodes).
	assert.Equal(t, 55.0, onlineNode[11])      // cpu_usage_percent from latest snapshot
	assert.Equal(t, 50.0, onlineNode[12])      // memory_usage_percent
	assert.Equal(t, int64(12), onlineNode[14]) // request_count from access logs
	assert.Equal(t, int64(1), onlineNode[15])  // error_count
	assert.Equal(t, int64(4), onlineNode[16])  // unique visitors

	pendingNode := nodeByID["node-dashboard-2"]
	require.NotNil(t, pendingNode)
	assert.Equal(t, "Edge 2", pendingNode[2])
	assert.Equal(t, "pending", pendingNode[6])
	assert.Equal(t, "unknown", pendingNode[7])

	assert.Equal(t, 55.0, overview.Capacity.AverageCPUUsagePercent)
	assert.Equal(t, 1, overview.Traffic.ReportedNodes)
}
