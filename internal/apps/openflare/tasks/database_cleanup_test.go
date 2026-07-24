// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tasks

import (
	"context"
	"testing"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	analyticsrepo "github.com/Rain-kl/Wavelet/internal/repository/analytics"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupDatabaseCleanupTestDB(t *testing.T) context.Context {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(&model.SystemConfig{}))
	db.SetDB(sqliteDB)
	resetAccessLogStore := repository.SetAccessLogStoreForTest(repository.NewMemoryAccessLogStore())
	resetObservabilityStore := repository.SetObservabilityStoreForTest(repository.NewMemoryObservabilityStore())
	t.Cleanup(func() {
		resetObservabilityStore()
		resetAccessLogStore()
		db.SetDB(nil)
	})
	return context.Background()
}

func TestCleanupDatabaseObservabilityRejectsRetentionShorterThanTableTTL(t *testing.T) {
	ctx := setupDatabaseCleanupTestDB(t)

	retentionDays := 7 // metric snapshots DDL TTL is 30 days
	result, err := CleanupDatabaseObservability(ctx, DatabaseCleanupInput{
		Target:        DatabaseCleanupTargetMetricSnapshots,
		RetentionDays: &retentionDays,
	})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "不能小于表 TTL")
	assert.Contains(t, err.Error(), "30")
}

func TestCleanupDatabaseObservabilityRejectsAccessLogRetentionShorterThanTableTTL(t *testing.T) {
	ctx := setupDatabaseCleanupTestDB(t)

	retentionDays := 30 // access logs DDL TTL is 90 days
	result, err := CleanupDatabaseObservability(ctx, DatabaseCleanupInput{
		Target:        DatabaseCleanupTargetAccessLogs,
		RetentionDays: &retentionDays,
	})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "90")
}

func TestCleanupDatabaseObservabilityMaterializeDoesNotClaimHardDelete(t *testing.T) {
	ctx := setupDatabaseCleanupTestDB(t)
	now := time.Now().UTC()

	// One row past metric table TTL (30d), one still inside the window.
	require.NoError(t, repository.InsertOpenFlareMetricSnapshot(ctx, &model.OpenFlareMetricSnapshot{
		NodeID:          "node-a",
		CapturedAt:      now.Add(-40 * 24 * time.Hour),
		CPUUsagePercent: 10,
	}))
	require.NoError(t, repository.InsertOpenFlareMetricSnapshot(ctx, &model.OpenFlareMetricSnapshot{
		NodeID:          "node-a",
		CapturedAt:      now.Add(-12 * time.Hour),
		CPUUsagePercent: 20,
	}))

	retentionDays := analyticsrepo.TableTTLDaysNodeMetricSnapshots
	result, err := CleanupDatabaseObservability(ctx, DatabaseCleanupInput{
		Target:        DatabaseCleanupTargetMetricSnapshots,
		RetentionDays: &retentionDays,
	})
	require.NoError(t, err)
	assert.False(t, result.DeleteAll)
	assert.Equal(t, analyticsrepo.CleanupModeTTLMaterialize, result.CleanupMode)
	assert.Equal(t, analyticsrepo.TableTTLDaysNodeMetricSnapshots, result.TableTTLDays)
	// MATERIALIZE is not a counted hard delete.
	assert.Equal(t, int64(0), result.DeletedCount)
	assert.Equal(t, int64(1), result.EligibleCount)
	require.NotNil(t, result.Cutoff)
	assert.True(t, result.Cutoff.Before(now.Add(-29*24*time.Hour)))

	// Memory store applies the table-TTL cutoff for tests; only the recent row remains.
	rows, err := repository.ListOpenFlareMetricSnapshotsSince(ctx, "", time.Time{}, 0)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, float64(20), rows[0].CPUUsagePercent)
}

func TestCleanupDatabaseObservabilityDeletesAllRowsWhenRetentionMissing(t *testing.T) {
	ctx := setupDatabaseCleanupTestDB(t)
	now := time.Now().UTC()

	require.NoError(t, repository.InsertOpenFlareAccessLogsBatch(ctx, []*model.OpenFlareAccessLog{
		{
			NodeID:     "node-a",
			LoggedAt:   now.Add(-3 * time.Hour),
			RemoteAddr: "203.0.113.1",
			Host:       "example.com",
			Path:       "/one",
			StatusCode: 200,
		},
		{
			NodeID:     "node-a",
			LoggedAt:   now.Add(-2 * time.Hour),
			RemoteAddr: "203.0.113.2",
			Host:       "example.com",
			Path:       "/two",
			StatusCode: 502,
		},
	}))

	result, err := CleanupDatabaseObservability(ctx, DatabaseCleanupInput{
		Target: DatabaseCleanupTargetAccessLogs,
	})
	require.NoError(t, err)
	assert.True(t, result.DeleteAll)
	assert.Equal(t, analyticsrepo.CleanupModeTruncate, result.CleanupMode)
	assert.Equal(t, int64(2), result.DeletedCount)
	assert.Equal(t, int64(2), result.EligibleCount)

	rows, err := repository.ListOpenFlareAccessLogs(ctx, model.OpenFlareAccessLogQuery{Page: 0, PageSize: 10})
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestRunDatabaseAutoCleanupOnceClampsRetentionToTableTTL(t *testing.T) {
	ctx := setupDatabaseCleanupTestDB(t)
	now := time.Now().UTC()

	// Access logs TTL=90d, metrics TTL=30d. Config retention=1 must clamp, not reject.
	require.NoError(t, repository.InsertOpenFlareAccessLogsBatch(ctx, []*model.OpenFlareAccessLog{{
		NodeID:     "node-a",
		LoggedAt:   now.Add(-100 * 24 * time.Hour),
		RemoteAddr: "203.0.113.10",
		Host:       "example.com",
		Path:       "/access",
		StatusCode: 200,
	}}))
	require.NoError(t, repository.InsertOpenFlareMetricSnapshot(ctx, &model.OpenFlareMetricSnapshot{
		NodeID:          "node-a",
		CapturedAt:      now.Add(-40 * 24 * time.Hour),
		CPUUsagePercent: 10,
	}))
	require.NoError(t, repository.InsertOpenFlareEdgeHealth(ctx, &model.OpenFlareEdgeHealth{
		NodeID:      "node-a",
		CapturedAt:  now.Add(-40 * 24 * time.Hour),
		Status:      "healthy",
		Connections: 2,
	}))

	require.NoError(t, repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyDatabaseAutoCleanupEnabled, "true"))
	require.NoError(t, repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyDatabaseAutoCleanupRetentionDays, "1"))

	summary, err := RunDatabaseAutoCleanupOnce(ctx, now)
	require.NoError(t, err)
	require.NotNil(t, summary)
	require.Len(t, summary.Results, 5)
	assert.Equal(t, 1, summary.RetentionDays)

	for _, result := range summary.Results {
		assert.Equal(t, analyticsrepo.CleanupModeTTLMaterialize, result.CleanupMode)
		assert.Equal(t, int64(0), result.DeletedCount, "target %s must not claim hard delete", result.Target)
		assert.GreaterOrEqual(t, result.TableTTLDays, 30)
		require.NotNil(t, result.RetentionDays)
		assert.GreaterOrEqual(t, *result.RetentionDays, result.TableTTLDays)
	}

	accessLogs, err := repository.ListOpenFlareAccessLogs(ctx, model.OpenFlareAccessLogQuery{Page: 0, PageSize: 10})
	require.NoError(t, err)
	assert.Empty(t, accessLogs)

	metricSnapshots, err := repository.ListOpenFlareMetricSnapshotsSince(ctx, "", time.Time{}, 0)
	require.NoError(t, err)
	assert.Empty(t, metricSnapshots)

	edgeHealth, err := repository.ListOpenFlareEdgeHealth(ctx, "", time.Time{}, 0)
	require.NoError(t, err)
	assert.Empty(t, edgeHealth)
}

func TestTableTTLDaysForCleanupTarget(t *testing.T) {
	days, ok := TableTTLDaysForCleanupTarget(DatabaseCleanupTargetAccessLogs)
	require.True(t, ok)
	assert.Equal(t, 90, days)

	days, ok = TableTTLDaysForCleanupTarget(DatabaseCleanupTargetMetricSnapshots)
	require.True(t, ok)
	assert.Equal(t, 30, days)

	_, ok = TableTTLDaysForCleanupTarget("unknown")
	assert.False(t, ok)
}
