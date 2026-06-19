// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package option

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

func setupOptionTestDB(t *testing.T) func() {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(&model.OpenFlareOption{}))

	db.SetDB(sqliteDB)
	ResetInitializationForTest()

	return func() {
		db.SetDB(nil)
		ResetInitializationForTest()
	}
}

func TestListOptionsFiltersSecretKeys(t *testing.T) {
	cleanup := setupOptionTestDB(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, model.UpdateOpenFlareOptions(ctx, []model.OpenFlareOption{
		{Key: "SystemName", Value: "TestFlare"},
		{Key: "SMTPToken", Value: "secret-token"},
		{Key: "GitHubClientSecret", Value: "secret-id"},
	}))

	options, err := listOptions(ctx)
	require.NoError(t, err)

	keys := make(map[string]string, len(options))
	for _, option := range options {
		keys[option.Key] = option.Value
	}

	assert.Equal(t, "TestFlare", keys["SystemName"])
	assert.NotContains(t, keys, "SMTPToken")
	assert.NotContains(t, keys, "GitHubClientSecret")
}

func TestUpdateOptionHotReloadsOptionMap(t *testing.T) {
	cleanup := setupOptionTestDB(t)
	defer cleanup()
	ctx := context.Background()

	err := updateOption(ctx, model.OpenFlareOption{
		Key:   "SystemName",
		Value: "HotReloaded",
	})
	require.NoError(t, err)

	assert.Equal(t, "HotReloaded", model.OptionValue("SystemName"))
	assert.Equal(t, "HotReloaded", model.SystemName)
}

func TestGetNotice(t *testing.T) {
	cleanup := setupOptionTestDB(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, updateOption(ctx, model.OpenFlareOption{Key: "Notice", Value: "hello"}))

	notice, err := getNotice(ctx)
	require.NoError(t, err)
	assert.Equal(t, "hello", notice)
}

func TestLookupGeoIPDisabledProvider(t *testing.T) {
	cleanup := setupOptionTestDB(t)
	defer cleanup()
	ctx := context.Background()

	view, err := lookupGeoIP(ctx, "disabled", "8.8.8.8")
	require.NoError(t, err)
	assert.Equal(t, "disabled", view.Provider)
	assert.Equal(t, "8.8.8.8", view.IP)
}

func TestCleanupDatabaseObservabilityDeletesRows(t *testing.T) {
	cleanup := setupOptionTestDB(t)
	defer cleanup()
	ctx := context.Background()

	sqliteDB := db.DB(ctx)
	require.NoError(t, sqliteDB.AutoMigrate(&model.OpenFlareAccessLog{}))

	now := time.Now().UTC()
	require.NoError(t, sqliteDB.Create(&model.OpenFlareAccessLog{
		NodeID:     "node-a",
		LoggedAt:   now.Add(-10 * 24 * time.Hour),
		RemoteAddr: "203.0.113.1",
		Host:       "example.com",
		Path:       "/old",
		StatusCode: 200,
	}).Error)
	require.NoError(t, sqliteDB.Create(&model.OpenFlareAccessLog{
		NodeID:     "node-a",
		LoggedAt:   now.Add(-2 * time.Hour),
		RemoteAddr: "203.0.113.2",
		Host:       "example.com",
		Path:       "/recent",
		StatusCode: 200,
	}).Error)

	retention := 7
	result, err := cleanupDatabaseObservability(ctx, databaseCleanupInput{
		Target:        "node_access_logs",
		RetentionDays: &retention,
	})
	require.NoError(t, err)
	assert.Equal(t, "node_access_logs", result.Target)
	assert.Equal(t, "访问日志", result.TargetLabel)
	assert.Equal(t, int64(1), result.DeletedCount)
	assert.False(t, result.DeleteAll)
	require.NotNil(t, result.RetentionDays)
	assert.Equal(t, 7, *result.RetentionDays)

	rows, err := model.ListOpenFlareAccessLogs(ctx, model.OpenFlareAccessLogQuery{Page: 0, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "/recent", rows[0].Path)
}
