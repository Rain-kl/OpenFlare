// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOpenFlareAccessLogTestEnvironment(t *testing.T) (context.Context, func()) {
	t.Helper()
	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(&OpenFlareAccessLog{}))
	db.SetDB(sqliteDB)
	return context.Background(), func() {
		db.SetDB(nil)
	}
}

func seedOpenFlareAccessLogs(t *testing.T, ctx context.Context, now time.Time) {
	t.Helper()
	records := []*OpenFlareAccessLog{
		{NodeID: "node-a", LoggedAt: now.Add(-5 * time.Minute), RemoteAddr: "1.1.1.1", Region: "US", Host: "a.example.com", Path: "/alpha", StatusCode: 200},
		{NodeID: "node-a", LoggedAt: now.Add(-4 * time.Minute), RemoteAddr: "2.2.2.2", Region: "US", Host: "a.example.com", Path: "/beta", StatusCode: 404},
		{NodeID: "node-b", LoggedAt: now.Add(-3 * time.Minute), RemoteAddr: "1.1.1.1", Region: "EU", Host: "b.example.com", Path: "/gamma", StatusCode: 502},
		{NodeID: "node-b", LoggedAt: now.Add(-2 * time.Minute), RemoteAddr: "3.3.3.3", Region: "EU", Host: "b.example.com", Path: "/delta", StatusCode: 200},
		{NodeID: "node-b", LoggedAt: now.Add(-1 * time.Minute), RemoteAddr: "", Region: "", Host: "b.example.com", Path: "/empty-ip", StatusCode: 200},
	}
	for index, record := range records {
		require.NoError(t, db.DB(ctx).Create(record).Error, "seed access log %d", index)
	}
}

func TestListOpenFlareAccessLogsPaginated(t *testing.T) {
	ctx, cleanup := setupOpenFlareAccessLogTestEnvironment(t)
	defer cleanup()

	now := time.Now().UTC()
	for index := range 15 {
		record := &OpenFlareAccessLog{
			NodeID:     "node-page",
			LoggedAt:   now.Add(-time.Duration(index) * time.Minute),
			RemoteAddr: fmt.Sprintf("203.0.113.%d", (index%5)+1),
			Host:       "example.com",
			Path:       fmt.Sprintf("/path-%02d", index),
			StatusCode: 200,
		}
		require.NoError(t, db.DB(ctx).Create(record).Error)
	}

	query := OpenFlareAccessLogQuery{
		NodeID:    "node-page",
		Since:     now.Add(-24 * time.Hour),
		Page:      1,
		PageSize:  5,
		SortBy:    "logged_at",
		SortOrder: "desc",
	}
	page, err := ListOpenFlareAccessLogs(ctx, query)
	require.NoError(t, err)
	require.Len(t, page, 5)
	assert.Equal(t, "/path-05", page[0].Path)
	assert.Equal(t, "/path-09", page[4].Path)
}

func TestCountOpenFlareAccessLogs(t *testing.T) {
	ctx, cleanup := setupOpenFlareAccessLogTestEnvironment(t)
	defer cleanup()

	now := time.Now().UTC()
	seedOpenFlareAccessLogs(t, ctx, now)

	query := OpenFlareAccessLogQuery{
		Since: now.Add(-10 * time.Minute),
	}
	totalRecords, totalIPs, err := CountOpenFlareAccessLogs(ctx, query)
	require.NoError(t, err)
	assert.Equal(t, int64(5), totalRecords)
	assert.Equal(t, int64(3), totalIPs)
}

func TestListOpenFlareAccessLogsFiltersAndSort(t *testing.T) {
	ctx, cleanup := setupOpenFlareAccessLogTestEnvironment(t)
	defer cleanup()

	now := time.Now().UTC()
	seedOpenFlareAccessLogs(t, ctx, now)

	query := OpenFlareAccessLogQuery{
		NodeID:    "node-a",
		Since:     now.Add(-10 * time.Minute),
		SortBy:    "status_code",
		SortOrder: "desc",
	}
	rows, err := ListOpenFlareAccessLogs(ctx, query)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, 404, rows[0].StatusCode)
	assert.Equal(t, 200, rows[1].StatusCode)
}

func TestListOpenFlareAccessLogsMissingTableGraceful(t *testing.T) {
	ctx, cleanup := setupOpenFlareAccessLogTestEnvironment(t)
	defer cleanup()
	require.NoError(t, db.DB(ctx).Migrator().DropTable(&OpenFlareAccessLog{}))

	query := OpenFlareAccessLogQuery{Since: time.Now().UTC().Add(-time.Hour)}
	rows, err := ListOpenFlareAccessLogs(ctx, query)
	require.NoError(t, err)
	assert.Empty(t, rows)

	totalRecords, totalIPs, err := CountOpenFlareAccessLogs(ctx, query)
	require.NoError(t, err)
	assert.Zero(t, totalRecords)
	assert.Zero(t, totalIPs)
}

func TestDeleteOpenFlareAccessLogsBefore(t *testing.T) {
	ctx, cleanup := setupOpenFlareAccessLogTestEnvironment(t)
	defer cleanup()

	now := time.Now().UTC()
	seedOpenFlareAccessLogs(t, ctx, now)

	deleted, err := DeleteOpenFlareAccessLogsBefore(ctx, now.Add(-2*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, int64(3), deleted)

	totalRecords, _, err := CountOpenFlareAccessLogs(ctx, OpenFlareAccessLogQuery{Since: now.Add(-10 * time.Minute)})
	require.NoError(t, err)
	assert.Equal(t, int64(2), totalRecords)
}
