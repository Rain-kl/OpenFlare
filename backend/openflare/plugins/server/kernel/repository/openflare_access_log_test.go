// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Wavelet/openflare/plugins/server/kernel/model"
	analyticsmodel "Wavelet/openflare/plugins/server/kernel/model/analytics"
	"Wavelet/openflare/plugins/server/kernel/repository/logstore"
	"Wavelet/pkg/idgen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// accessLogTestDBSeq 保证每个测试获得独立的 sqlite 内存库（cache=shared 下同名 DSN 复用同一库）。
var accessLogTestDBSeq int64

func setupOpenFlareAccessLogTestEnvironment(t *testing.T) (context.Context, func()) {
	t.Helper()
	dsn := fmt.Sprintf("file:repo-access-log-test-%d?mode=memory&cache=shared", atomic.AddInt64(&accessLogTestDBSeq, 1))
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&analyticsmodel.NodeAccessLog{}))
	SetDBForTest(gdb)
	require.NoError(t, idgen.Init(1))

	logstore.ResetForTest()
	logstore.SetConfigReader(func(_ context.Context, key string) (string, error) {
		if key == model.ConfigKeyLogDatabase {
			return "sqlite", nil
		}
		return "", nil
	})
	logstore.SetAccessLogHooks(logstore.AccessLogHooks{})
	logstore.SetObservabilityHooks(logstore.ObservabilityHooks{})

	ctx := context.Background()
	store, err := logstore.Active(ctx)
	require.NoError(t, err)
	// 写入入口只入队；测试环境立即 flush，保证后续查询可见。
	logstore.SetAccessLogHooks(logstore.AccessLogHooks{
		QueueNodeAccessLogs: func(logs []analyticsmodel.NodeAccessLog) {
			require.NoError(t, store.AccessLogs.BatchInsertNodeAccessLogs(context.Background(), logs))
		},
	})
	return ctx, func() {
		logstore.SetAccessLogHooks(logstore.AccessLogHooks{})
		logstore.ResetForTest()
		SetDBForTest(nil)
	}
}

func seedOpenFlareAccessLogs(t *testing.T, ctx context.Context, now time.Time) {
	t.Helper()
	records := []*model.OpenFlareAccessLog{
		{NodeID: "node-a", LoggedAt: now.Add(-5 * time.Minute), RemoteAddr: "1.1.1.1", Region: "US", Host: "a.example.com", Path: "/alpha", StatusCode: 200},
		{NodeID: "node-a", LoggedAt: now.Add(-4 * time.Minute), RemoteAddr: "2.2.2.2", Region: "US", Host: "a.example.com", Path: "/beta", StatusCode: 404},
		{NodeID: "node-b", LoggedAt: now.Add(-3 * time.Minute), RemoteAddr: "1.1.1.1", Region: "EU", Host: "b.example.com", Path: "/gamma", StatusCode: 502},
		{NodeID: "node-b", LoggedAt: now.Add(-2 * time.Minute), RemoteAddr: "3.3.3.3", Region: "EU", Host: "b.example.com", Path: "/delta", StatusCode: 200},
		{NodeID: "node-b", LoggedAt: now.Add(-1 * time.Minute), RemoteAddr: "", Region: "", Host: "b.example.com", Path: "/empty-ip", StatusCode: 200},
	}
	require.NoError(t, InsertOpenFlareAccessLogsBatch(ctx, records))
}

func TestListOpenFlareAccessLogsPaginated(t *testing.T) {
	ctx, cleanup := setupOpenFlareAccessLogTestEnvironment(t)
	defer cleanup()

	now := time.Now().UTC()
	for index := range 15 {
		record := &model.OpenFlareAccessLog{
			NodeID:     "node-page",
			LoggedAt:   now.Add(-time.Duration(index) * time.Minute),
			RemoteAddr: fmt.Sprintf("203.0.113.%d", (index%5)+1),
			Host:       "example.com",
			Path:       fmt.Sprintf("/path-%02d", index),
			StatusCode: 200,
		}
		require.NoError(t, InsertOpenFlareAccessLogsBatch(ctx, []*model.OpenFlareAccessLog{record}))
	}

	// 0-based 分页与 CH ListNodeAccessLogs 一致：page=1 size=5 → OFFSET 5 → /path-05..09。
	query := model.OpenFlareAccessLogQuery{
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

	query := model.OpenFlareAccessLogQuery{
		Since: now.Add(-10 * time.Minute),
	}
	totalRecords, totalIPs, _, err := CountOpenFlareAccessLogs(ctx, query)
	require.NoError(t, err)
	assert.Equal(t, int64(5), totalRecords)
	// GORM 与 CH 一致：distinct IP 排除空 remote_addr（CH uniqExactIf(remote_addr, remote_addr != '')）。
	assert.Equal(t, int64(3), totalIPs)
}

func TestListOpenFlareAccessLogsFiltersAndSort(t *testing.T) {
	ctx, cleanup := setupOpenFlareAccessLogTestEnvironment(t)
	defer cleanup()

	now := time.Now().UTC()
	seedOpenFlareAccessLogs(t, ctx, now)

	query := model.OpenFlareAccessLogQuery{
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

func TestDeleteOpenFlareAccessLogsBefore(t *testing.T) {
	ctx, cleanup := setupOpenFlareAccessLogTestEnvironment(t)
	defer cleanup()

	now := time.Now().UTC()
	seedOpenFlareAccessLogs(t, ctx, now)

	deleted, err := DeleteOpenFlareAccessLogsBefore(ctx, now.Add(-2*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, int64(3), deleted)

	totalRecords, _, _, err := CountOpenFlareAccessLogs(ctx, model.OpenFlareAccessLogQuery{Since: now.Add(-10 * time.Minute)})
	require.NoError(t, err)
	assert.Equal(t, int64(2), totalRecords)
}
