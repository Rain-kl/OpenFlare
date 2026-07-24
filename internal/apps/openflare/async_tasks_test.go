// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openflare

import (
	"context"
	"testing"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDatabaseAutoCleanupHandlerSkipsWhenDisabled(t *testing.T) {
	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(&model.SystemConfig{}))
	db.SetDB(sqliteDB)
	t.Cleanup(func() { db.SetDB(nil) })

	ctx := context.Background()
	require.NoError(t, repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyDatabaseAutoCleanupEnabled, "false"))

	result, err := (&DatabaseAutoCleanupHandler{}).Execute(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Message, "未启用")
}

func TestDatabaseAutoCleanupHandlerDeletesRowsWhenEnabled(t *testing.T) {
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

	ctx := context.Background()
	now := time.Now().UTC()
	require.NoError(t, repository.InsertOpenFlareAccessLogsBatch(ctx, []*model.OpenFlareAccessLog{{
		NodeID:     "node-a",
		LoggedAt:   now.Add(-95 * 24 * time.Hour),
		RemoteAddr: "203.0.113.10",
		Host:       "example.com",
		Path:       "/access",
		StatusCode: 200,
	}}))

	require.NoError(t, repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyDatabaseAutoCleanupEnabled, "true"))
	require.NoError(t, repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyDatabaseAutoCleanupRetentionDays, "1"))

	result, err := (&DatabaseAutoCleanupHandler{}).Execute(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Message, "共删除")

	rows, err := repository.ListOpenFlareAccessLogs(ctx, model.OpenFlareAccessLogQuery{Page: 0, PageSize: 10})
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestUptimeKumaSyncHandlerSkipsWhenDisabled(t *testing.T) {
	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(&model.SystemConfig{}))
	db.SetDB(sqliteDB)
	t.Cleanup(func() { db.SetDB(nil) })

	ctx := context.Background()
	require.NoError(t, repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyUptimeKumaEnabled, "false"))

	result, err := (&UptimeKumaSyncHandler{}).Execute(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Message, "未启用")
}
