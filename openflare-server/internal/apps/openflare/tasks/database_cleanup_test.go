// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tasks

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

func setupDatabaseCleanupTestDB(t *testing.T) context.Context {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(
		&model.OpenFlareAccessLog{},
		&model.OpenFlareMetricSnapshot{},
		&model.OpenFlareRequestReport{},
	))
	db.SetDB(sqliteDB)
	t.Cleanup(func() {
		db.SetDB(nil)
	})
	return context.Background()
}

func TestCleanupDatabaseObservabilityDeletesTargetedRows(t *testing.T) {
	ctx := setupDatabaseCleanupTestDB(t)
	now := time.Now().UTC()

	require.NoError(t, db.DB(ctx).Create(&model.OpenFlareMetricSnapshot{
		NodeID:          "node-a",
		CapturedAt:      now.Add(-10 * 24 * time.Hour),
		CPUUsagePercent: 10,
	}).Error)
	require.NoError(t, db.DB(ctx).Create(&model.OpenFlareMetricSnapshot{
		NodeID:          "node-a",
		CapturedAt:      now.Add(-12 * time.Hour),
		CPUUsagePercent: 20,
	}).Error)

	retentionDays := 7
	result, err := CleanupDatabaseObservability(ctx, DatabaseCleanupInput{
		Target:        DatabaseCleanupTargetMetricSnapshots,
		RetentionDays: &retentionDays,
	})
	require.NoError(t, err)
	assert.False(t, result.DeleteAll)
	assert.Equal(t, int64(1), result.DeletedCount)

	rows, err := model.ListOpenFlareMetricSnapshotsSince(ctx, "", time.Time{}, 0)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, float64(20), rows[0].CPUUsagePercent)
}

func TestCleanupDatabaseObservabilityDeletesAllRowsWhenRetentionMissing(t *testing.T) {
	ctx := setupDatabaseCleanupTestDB(t)
	now := time.Now().UTC()

	require.NoError(t, db.DB(ctx).Create(&model.OpenFlareAccessLog{
		NodeID:     "node-a",
		LoggedAt:   now.Add(-3 * time.Hour),
		RemoteAddr: "203.0.113.1",
		Host:       "example.com",
		Path:       "/one",
		StatusCode: 200,
	}).Error)
	require.NoError(t, db.DB(ctx).Create(&model.OpenFlareAccessLog{
		NodeID:     "node-a",
		LoggedAt:   now.Add(-2 * time.Hour),
		RemoteAddr: "203.0.113.2",
		Host:       "example.com",
		Path:       "/two",
		StatusCode: 502,
	}).Error)

	result, err := CleanupDatabaseObservability(ctx, DatabaseCleanupInput{
		Target: DatabaseCleanupTargetAccessLogs,
	})
	require.NoError(t, err)
	assert.True(t, result.DeleteAll)
	assert.Equal(t, int64(2), result.DeletedCount)

	rows, err := model.ListOpenFlareAccessLogs(ctx, model.OpenFlareAccessLogQuery{Page: 0, PageSize: 10})
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestRunDatabaseAutoCleanupOnceDeletesAllObservabilityTargets(t *testing.T) {
	ctx := setupDatabaseCleanupTestDB(t)
	now := time.Now().UTC()

	require.NoError(t, db.DB(ctx).Create(&model.OpenFlareAccessLog{
		NodeID:     "node-a",
		LoggedAt:   now.Add(-48 * time.Hour),
		RemoteAddr: "203.0.113.10",
		Host:       "example.com",
		Path:       "/access",
		StatusCode: 200,
	}).Error)
	require.NoError(t, db.DB(ctx).Create(&model.OpenFlareMetricSnapshot{
		NodeID:          "node-a",
		CapturedAt:      now.Add(-48 * time.Hour),
		CPUUsagePercent: 10,
	}).Error)
	require.NoError(t, db.DB(ctx).Create(&model.OpenFlareRequestReport{
		NodeID:          "node-a",
		WindowStartedAt: now.Add(-49 * time.Hour),
		WindowEndedAt:   now.Add(-48 * time.Hour),
		RequestCount:    15,
	}).Error)

	previousEnabled := model.DatabaseAutoCleanupEnabled
	previousRetentionDays := model.DatabaseAutoCleanupRetentionDays
	model.DatabaseAutoCleanupEnabled = true
	model.DatabaseAutoCleanupRetentionDays = 1
	t.Cleanup(func() {
		model.DatabaseAutoCleanupEnabled = previousEnabled
		model.DatabaseAutoCleanupRetentionDays = previousRetentionDays
	})

	summary, err := RunDatabaseAutoCleanupOnce(now)
	require.NoError(t, err)
	require.NotNil(t, summary)
	require.Len(t, summary.Results, 3)

	accessLogs, err := model.ListOpenFlareAccessLogs(ctx, model.OpenFlareAccessLogQuery{Page: 0, PageSize: 10})
	require.NoError(t, err)
	assert.Empty(t, accessLogs)

	metricSnapshots, err := model.ListOpenFlareMetricSnapshotsSince(ctx, "", time.Time{}, 0)
	require.NoError(t, err)
	assert.Empty(t, metricSnapshots)

	requestReports, err := model.ListOpenFlareRequestReportsSince(ctx, "", time.Time{}, 0)
	require.NoError(t, err)
	assert.Empty(t, requestReports)
}
