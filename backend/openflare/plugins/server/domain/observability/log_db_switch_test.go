// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"Wavelet/openflare/plugins/server/kernel/model"
	analyticsmodel "Wavelet/openflare/plugins/server/kernel/model/analytics"
	"Wavelet/openflare/plugins/server/kernel/repository"
	"Wavelet/openflare/plugins/server/kernel/repository/logstore"
	"Wavelet/openflare/plugins/server/kernel/runtimeconfig"
)

var logDBSwitchDBSeq int64

// newLogDBSwitchDB 构造内存 sqlite 库（含日志 5 表 + 系统配置表）。
func newLogDBSwitchDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:log-db-switch-%d?mode=memory&cache=shared", atomic.AddInt64(&logDBSwitchDBSeq, 1))
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(
		&model.SystemConfig{},
		&analyticsmodel.NodeAccessLog{},
		&analyticsmodel.NodeMetricSnapshot{},
		&analyticsmodel.NodeEdgeHealth{},
		&analyticsmodel.NodeObsFrps{},
		&analyticsmodel.NodeObsFrpc{},
		&analyticsmodel.UserAccessLog{},
	))
	return gdb
}

// TestCopyAccessLogsPreservesIDs sqlite→sqlite 模拟：源 store 3 条，目标空库，
// copyAccessLogs 后 ID 保留、数量一致。
func TestCopyAccessLogsPreservesIDs(t *testing.T) {
	t.Cleanup(runtimeconfig.Override(false, false))
	logstore.ResetForTest()
	defer logstore.ResetForTest()

	ctx := context.Background()
	srcDB := newLogDBSwitchDB(t)
	dstDB := newLogDBSwitchDB(t)

	repository.SetDBForTest(srcDB)
	src, err := logstore.Active(ctx) // 无 reader 时按 seed 规则解析为 sqlite
	require.NoError(t, err)
	repository.SetDBForTest(dstDB)
	dst, err := logstore.BuildForMigration(ctx, "sqlite")
	require.NoError(t, err)
	t.Cleanup(func() { repository.SetDBForTest(nil) })

	now := time.Now().UTC()
	rows := []analyticsmodel.NodeAccessLog{
		{ID: 101, NodeID: "n1", LoggedAt: now, RemoteAddr: "1.1.1.1", Host: "a.example.com", Path: "/"},
		{ID: 202, NodeID: "n2", LoggedAt: now, RemoteAddr: "2.2.2.2", Host: "b.example.com", Path: "/x"},
		{ID: 303, NodeID: "n1", LoggedAt: now, RemoteAddr: "3.3.3.3", Host: "c.example.com", Path: "/y"},
	}
	require.NoError(t, src.AccessLogs.BatchInsertNodeAccessLogs(ctx, rows))

	require.NoError(t, copyAccessLogs(ctx, src, dst))

	var got []analyticsmodel.NodeAccessLog
	require.NoError(t, dstDB.Order("id ASC").Find(&got).Error)
	require.Len(t, got, 3)
	for i, wantID := range []uint64{101, 202, 303} {
		assert.Equal(t, wantID, got[i].ID, "row %d id preserved", i)
	}
	assert.Equal(t, "n1", got[0].NodeID)
	assert.Equal(t, "n2", got[1].NodeID)
	assert.Equal(t, "n1", got[2].NodeID)
	assert.Equal(t, "1.1.1.1", got[0].RemoteAddr)

	// 源库保持不变。
	var srcCount int64
	require.NoError(t, srcDB.Model(&analyticsmodel.NodeAccessLog{}).Count(&srcCount).Error)
	assert.Equal(t, int64(3), srcCount)
}

// TestCopyUserAccessLogsPreservesIDs sqlite→sqlite 模拟：源库用户访问日志按 id 升序
// 复制到目标库，ID 保留、数量一致，且源库保持不变。
func TestCopyUserAccessLogsPreservesIDs(t *testing.T) {
	t.Cleanup(runtimeconfig.Override(false, false))
	logstore.ResetForTest()
	defer logstore.ResetForTest()

	ctx := context.Background()
	srcDB := newLogDBSwitchDB(t)
	dstDB := newLogDBSwitchDB(t)

	repository.SetDBForTest(srcDB)
	src, err := logstore.Active(ctx)
	require.NoError(t, err)
	repository.SetDBForTest(dstDB)
	dst, err := logstore.BuildForMigration(ctx, "sqlite")
	require.NoError(t, err)
	t.Cleanup(func() { repository.SetDBForTest(nil) })

	now := time.Now().UTC()
	rows := []analyticsmodel.UserAccessLog{
		{ID: 11, UserID: 1, Path: "/a", CreatedAt: now},
		{ID: 22, UserID: 2, Path: "/b", CreatedAt: now.Add(time.Second)},
		{ID: 33, UserID: 1, Path: "/c", CreatedAt: now.Add(2 * time.Second)},
	}
	require.NoError(t, src.UserAccessLogs.BatchInsert(ctx, rows))

	require.NoError(t, copyUserAccessLogs(ctx, src, dst))

	var got []analyticsmodel.UserAccessLog
	require.NoError(t, dstDB.Order("id ASC").Find(&got).Error)
	require.Len(t, got, 3)
	for i, wantID := range []uint64{11, 22, 33} {
		assert.Equal(t, wantID, got[i].ID, "row %d id preserved", i)
	}

	var srcCount int64
	require.NoError(t, srcDB.Model(&analyticsmodel.UserAccessLog{}).Count(&srcCount).Error)
	assert.Equal(t, int64(3), srcCount)
}

// TestClearTargetLogTablesClearsUserAccessLogs 验证清空目标包含用户访问日志表
// （6 张日志表之一），迁移「覆盖目标库已有日志」幂等前提成立。
func TestClearTargetLogTablesClearsUserAccessLogs(t *testing.T) {
	t.Cleanup(runtimeconfig.Override(false, false))
	logstore.ResetForTest()
	defer logstore.ResetForTest()

	ctx := context.Background()
	dstDB := newLogDBSwitchDB(t)
	repository.SetDBForTest(dstDB)
	t.Cleanup(func() { repository.SetDBForTest(nil) })
	dst, err := logstore.BuildForMigration(ctx, "sqlite")
	require.NoError(t, err)

	now := time.Now().UTC()
	require.NoError(t, dst.UserAccessLogs.BatchInsert(ctx, []analyticsmodel.UserAccessLog{
		{ID: 1, UserID: 1, Path: "/a", CreatedAt: now},
		{ID: 2, UserID: 2, Path: "/b", CreatedAt: now},
	}))

	require.NoError(t, clearTargetLogTables(ctx, dst))

	var count int64
	require.NoError(t, dstDB.Model(&analyticsmodel.UserAccessLog{}).Count(&count).Error)
	assert.Zero(t, count, "用户访问日志应被清空")
}

// TestClearTargetLogTablesDuringMigration 回归：冻结标记置位后，BuildForMigration 构造的
// 目标 store 必须放行用户访问日志清空/写入。skipFreeze 未传播到 UserAccessLogs store 时
// DeleteAll 会误报 ErrMigrating，导致真实切换任务在清空目标库阶段失败。
func TestClearTargetLogTablesDuringMigration(t *testing.T) {
	logstore.ResetForTest()
	defer logstore.ResetForTest()

	gdb := newLogDBSwitchDB(t)
	repository.SetDBForTest(gdb)
	t.Cleanup(func() { repository.SetDBForTest(nil) })
	ctx := context.Background()

	logstore.SetConfigReader(func(ctx context.Context, key string) (string, error) {
		cfg, err := repository.GetSystemConfigByKey(ctx, key)
		if err != nil {
			return "", err
		}
		return cfg.Value, nil
	})

	// 预置目标库已有日志（迁移「覆盖目标库已有日志」幂等前提）。
	now := time.Now().UTC()
	require.NoError(t, gdb.Create(&analyticsmodel.UserAccessLog{ID: 1, UserID: 1, Path: "/a", CreatedAt: now}).Error)

	// 冻结标记置位（与真实任务 Execute 流程一致）。
	require.NoError(t, setMigrationFlag(ctx, "migrating"))
	t.Cleanup(func() { _ = setMigrationFlag(ctx, "") })
	require.True(t, logstore.Migrating(ctx))

	dst, err := logstore.BuildForMigration(ctx, "sqlite")
	require.NoError(t, err)

	require.NoError(t, clearTargetLogTables(ctx, dst), "迁移冻结期间目标库清空必须放行")

	var count int64
	require.NoError(t, gdb.Model(&analyticsmodel.UserAccessLog{}).Count(&count).Error)
	assert.Zero(t, count, "用户访问日志应被清空")
}

// TestValidateSwitch 各非法组合报错。
func TestValidateSwitch(t *testing.T) {
	t.Cleanup(func() {
	})

	gdb := newLogDBSwitchDB(t)
	repository.SetDBForTest(gdb)
	t.Cleanup(func() { repository.SetDBForTest(nil) })
	ctx := context.Background()
	setLogDB := func(v string) {
		require.NoError(t, repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyLogDatabase, v))
	}

	t.Run("same target rejected", func(t *testing.T) {
		setLogDB("sqlite")
		t.Cleanup(runtimeconfig.Override(false, false))
		err := validateSwitch(ctx, "sqlite")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "相同")
	})
	t.Run("clickhouse disabled rejected", func(t *testing.T) {
		setLogDB("sqlite")
		t.Cleanup(runtimeconfig.Override(false, false))
		err := validateSwitch(ctx, "clickhouse")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ClickHouse 未启用")
	})
	t.Run("postgres requires main db enabled", func(t *testing.T) {
		setLogDB("sqlite")
		t.Cleanup(runtimeconfig.Override(false, false))
		err := validateSwitch(ctx, "postgres")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "PostgreSQL 未启用")
	})
	t.Run("sqlite rejected when main db is postgres", func(t *testing.T) {
		setLogDB("postgres")
		t.Cleanup(runtimeconfig.Override(true, false))
		err := validateSwitch(ctx, "sqlite")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "SQLite")
	})
	t.Run("valid postgres migration", func(t *testing.T) {
		setLogDB("sqlite")
		t.Cleanup(runtimeconfig.Override(true, false))
		require.NoError(t, validateSwitch(ctx, "postgres"))
	})
}

// TestLogDBSwitchValidatePayload 参数归一化与非法值拒绝。
func TestLogDBSwitchValidatePayload(t *testing.T) {
	h := &LogDBSwitchHandler{}
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{name: "postgresql normalized", in: `{"target":"postgresql"}`, want: "postgres", ok: true},
		{name: "sqlite3 normalized", in: `{"target":"sqlite3"}`, want: "sqlite", ok: true},
		{name: "ch normalized", in: `{"target":"ch"}`, want: "clickhouse", ok: true},
		{name: "postgres passthrough", in: `{"target":"postgres"}`, want: "postgres", ok: true},
		{name: "invalid target", in: `{"target":"mysql"}`, ok: false},
		{name: "malformed json", in: `not-json`, ok: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := h.ValidatePayload([]byte(c.in))
			if !c.ok {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			var p logDBSwitchPayload
			require.NoError(t, json.Unmarshal(out, &p))
			assert.Equal(t, c.want, p.Target)
		})
	}
}

// TestExecuteFailureClearsMigrationFlag 迁移失败后 log_db_migration 冻结标记被清除。
// 在 FRESH DB（不预置 log_db_migration 行）上验证：setMigrationFlag 必须 upsert 建行，
// 且失败后经缓存路径（GetSystemConfigByKey）可观察为空。
func TestExecuteFailureClearsMigrationFlag(t *testing.T) {
	t.Cleanup(runtimeconfig.Override(true, runtimeconfig.ClickHouseEnabled()))

	logstore.ResetForTest()
	defer logstore.ResetForTest()

	gdb := newLogDBSwitchDB(t)
	repository.SetDBForTest(gdb)
	t.Cleanup(func() { repository.SetDBForTest(nil) })
	ctx := context.Background()

	// FRESH DB：log_db_migration 行不存在（不预置），log_database 预置为 sqlite。
	require.NoError(t, repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyLogDatabase, "sqlite"))
	_, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyLogDBMigration)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	// configReader 对 log_database 报错，使 logstore.Active 在冻结标记置位后失败。
	logstore.SetConfigReader(func(_ context.Context, key string) (string, error) {
		if key == model.ConfigKeyLogDatabase {
			return "", errors.New("reader error")
		}
		return "", nil
	})

	_, err = (&LogDBSwitchHandler{}).Execute(ctx, []byte(`{"target":"postgres"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reader error")

	// 冻结标记必须被 upsert 持久化（行存在）并经缓存路径可观察为空，源库恢复可写。
	cfg, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyLogDBMigration)
	require.NoError(t, err, "setMigrationFlag 应 upsert 创建 log_db_migration 行")
	assert.Empty(t, cfg.Value, "失败后冻结标记必须清除，源库保持可写")
	assert.False(t, logstore.Migrating(ctx))
}

// TestSetMigrationFlagObservableThroughCache 在 FRESH DB 上验证 setMigrationFlag 写入
// 经缓存路径（logstore.Migrating → repository 读取）实时反映：置位 true、清除 false。
func TestSetMigrationFlagObservableThroughCache(t *testing.T) {
	logstore.ResetForTest()
	defer logstore.ResetForTest()

	gdb := newLogDBSwitchDB(t)
	repository.SetDBForTest(gdb)
	t.Cleanup(func() { repository.SetDBForTest(nil) })
	ctx := context.Background()

	// 按 bootstrap 同款注入 repository 读取，走 RAM 缓存路径。
	logstore.SetConfigReader(func(ctx context.Context, key string) (string, error) {
		cfg, err := repository.GetSystemConfigByKey(ctx, key)
		if err != nil {
			return "", err
		}
		return cfg.Value, nil
	})

	// FRESH DB：行缺失 → fail-open false。
	assert.False(t, logstore.Migrating(ctx))

	require.NoError(t, setMigrationFlag(ctx, "migrating"))
	assert.True(t, logstore.Migrating(ctx), "置位后缓存路径必须立即观察到 migrating")

	require.NoError(t, setMigrationFlag(ctx, ""))
	assert.False(t, logstore.Migrating(ctx), "清除后缓存路径必须立即观察到非 migrating")
}

// TestFlipLogDatabaseRefreshesCachedConfig 验证翻转日志主库后缓存路径立即反映新库
// （logstore.ActiveDatabase / GetSystemConfigByKey），防止各进程继续写旧库（split-brain）。
func TestFlipLogDatabaseRefreshesCachedConfig(t *testing.T) {
	logstore.ResetForTest()
	defer logstore.ResetForTest()

	gdb := newLogDBSwitchDB(t)
	repository.SetDBForTest(gdb)
	t.Cleanup(func() { repository.SetDBForTest(nil) })
	ctx := context.Background()

	logstore.SetConfigReader(func(ctx context.Context, key string) (string, error) {
		cfg, err := repository.GetSystemConfigByKey(ctx, key)
		if err != nil {
			return "", err
		}
		return cfg.Value, nil
	})

	require.NoError(t, repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyLogDatabase, "sqlite"))
	active, err := logstore.ActiveDatabase(ctx)
	require.NoError(t, err)
	assert.Equal(t, "sqlite", active) // 预热缓存

	require.NoError(t, flipLogDatabase(ctx, "postgres"))

	active, err = logstore.ActiveDatabase(ctx)
	require.NoError(t, err)
	assert.Equal(t, "postgres", active, "翻转后缓存路径必须立即反映新库")
	cfg, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyLogDatabase)
	require.NoError(t, err)
	assert.Equal(t, "postgres", cfg.Value)
}
