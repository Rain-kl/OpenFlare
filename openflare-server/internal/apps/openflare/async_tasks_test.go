// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openflare

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

func TestDatabaseAutoCleanupHandlerSkipsWhenDisabled(t *testing.T) {
	previousEnabled := model.DatabaseAutoCleanupEnabled
	model.DatabaseAutoCleanupEnabled = false
	t.Cleanup(func() {
		model.DatabaseAutoCleanupEnabled = previousEnabled
	})

	result, err := (&DatabaseAutoCleanupHandler{}).Execute(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Message, "未启用")
}

func TestDatabaseAutoCleanupHandlerDeletesRowsWhenEnabled(t *testing.T) {
	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(&model.OpenFlareAccessLog{}))
	db.SetDB(sqliteDB)
	t.Cleanup(func() {
		db.SetDB(nil)
	})

	now := time.Now().UTC()
	require.NoError(t, db.DB(context.Background()).Create(&model.OpenFlareAccessLog{
		NodeID:     "node-a",
		LoggedAt:   now.Add(-48 * time.Hour),
		RemoteAddr: "203.0.113.10",
		Host:       "example.com",
		Path:       "/access",
		StatusCode: 200,
	}).Error)

	previousEnabled := model.DatabaseAutoCleanupEnabled
	previousRetentionDays := model.DatabaseAutoCleanupRetentionDays
	model.DatabaseAutoCleanupEnabled = true
	model.DatabaseAutoCleanupRetentionDays = 1
	t.Cleanup(func() {
		model.DatabaseAutoCleanupEnabled = previousEnabled
		model.DatabaseAutoCleanupRetentionDays = previousRetentionDays
	})

	result, err := (&DatabaseAutoCleanupHandler{}).Execute(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Message, "共删除")

	rows, err := model.ListOpenFlareAccessLogs(context.Background(), model.OpenFlareAccessLogQuery{Page: 0, PageSize: 10})
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestUptimeKumaSyncHandlerSkipsWhenDisabled(t *testing.T) {
	previousEnabled := model.UptimeKumaEnabled
	model.UptimeKumaEnabled = false
	t.Cleanup(func() {
		model.UptimeKumaEnabled = previousEnabled
	})

	result, err := (&UptimeKumaSyncHandler{}).Execute(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Message, "未启用")
}