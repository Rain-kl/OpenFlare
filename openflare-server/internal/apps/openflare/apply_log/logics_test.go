// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package apply_log

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

func setupApplyLogTestDB(t *testing.T) func() {
	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)

	err = sqliteDB.AutoMigrate(&model.OpenFlareApplyLog{})
	require.NoError(t, err)

	db.SetDB(sqliteDB)

	return func() {
		db.SetDB(nil)
	}
}

func TestListPageAndCleanup(t *testing.T) {
	cleanup := setupApplyLogTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()

	logs := []model.OpenFlareApplyLog{
		{NodeID: "node-logs", Version: "v1", Result: "success", Message: "1", CreatedAt: now.Add(-10 * 24 * time.Hour)},
		{NodeID: "node-logs", Version: "v2", Result: "success", Message: "2", CreatedAt: now.Add(-5 * 24 * time.Hour)},
		{NodeID: "node-logs", Version: "v3", Result: "success", Message: "3", CreatedAt: now},
	}
	for i := range logs {
		require.NoError(t, db.DB(ctx).Create(&logs[i]).Error)
	}

	pageResult, err := ListPage(ctx, ListQuery{
		NodeID:   "node-logs",
		PageNo:   1,
		PageSize: 2,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, pageResult.Total)
	assert.Len(t, pageResult.Rows, 2)
	assert.Equal(t, 2, pageResult.TotalPage)
	assert.Equal(t, 1, pageResult.Current)

	cleanupResult, err := Cleanup(ctx, CleanupInput{
		DeleteAll:     false,
		RetentionDays: 7,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), cleanupResult.DeletedCount)
	assert.NotNil(t, cleanupResult.Cutoff)

	remaining, err := model.ListOpenFlareApplyLogs(ctx, model.OpenFlareApplyLogQuery{
		NodeID:   "node-logs",
		PageNo:   1,
		PageSize: 10,
	})
	require.NoError(t, err)
	assert.Len(t, remaining, 2)

	cleanupAll, err := Cleanup(ctx, CleanupInput{DeleteAll: true})
	require.NoError(t, err)
	assert.Equal(t, int64(2), cleanupAll.DeletedCount)
	assert.True(t, cleanupAll.DeleteAll)

	finalLogs, err := model.ListOpenFlareApplyLogs(ctx, model.OpenFlareApplyLogQuery{
		NodeID:   "node-logs",
		PageNo:   1,
		PageSize: 10,
	})
	require.NoError(t, err)
	assert.Empty(t, finalLogs)
}

func TestCleanupInvalidRetentionDays(t *testing.T) {
	cleanup := setupApplyLogTestDB(t)
	defer cleanup()

	ctx := context.Background()

	_, err := Cleanup(ctx, CleanupInput{RetentionDays: 0})
	require.Error(t, err)
	assert.Equal(t, errRetentionDaysOutOfRange, err.Error())

	_, err = Cleanup(ctx, CleanupInput{RetentionDays: 4000})
	require.Error(t, err)
	assert.Equal(t, errRetentionDaysOutOfRange, err.Error())
}
