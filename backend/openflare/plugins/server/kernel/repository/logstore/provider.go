// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"Wavelet/openflare/plugins/server/kernel/model"
	"Wavelet/openflare/plugins/server/kernel/runtimeconfig"
	"Wavelet/pkg/logger"

	"gorm.io/gorm"
)

// logDatabaseKey / logMigrationKey 对应 model.ConfigKeyLogDatabase / ConfigKeyLogDBMigration。
const (
	logDatabaseKey  = model.ConfigKeyLogDatabase
	logMigrationKey = model.ConfigKeyLogDBMigration
)

// 日志库名常量（与 model 配置值一致，集中避免散落字符串字面量）。
const (
	dbNamePostgres   = "postgres"
	dbNameSQLite     = "sqlite"
	dbNameClickHouse = "clickhouse"
)

// errConfigReaderNotWired 表示 config reader 尚未注入（首启/测试场景按 seed 规则兜底）。
var errConfigReaderNotWired = errors.New("logstore: config reader not wired")

// ConfigReader 读取系统配置字符串值，由 bootstrap 注入（避免 logstore ↔ repository 循环依赖）。
type ConfigReader func(ctx context.Context, key string) (string, error)

const resolveCacheTTL = 1 * time.Second

var (
	configReader ConfigReader
	dbResolver   func(ctx context.Context) *gorm.DB

	storeMu         sync.RWMutex
	active          *Store
	activeDB        string
	lastResolveDB   string
	lastResolveTime time.Time
)

// SetConfigReader 注入系统配置读取函数（bootstrap 调用，测试可注入内存实现）。
func SetConfigReader(fn ConfigReader) { configReader = fn }

// SetDBResolver 注入数据库解析函数。
func SetDBResolver(fn func(ctx context.Context) *gorm.DB) { dbResolver = fn }

func getGormDB(ctx context.Context) *gorm.DB {
	if dbResolver != nil {
		return dbResolver(ctx)
	}
	return nil
}

func getConfig(ctx context.Context, key string) (string, error) {
	if configReader == nil {
		return "", errConfigReaderNotWired
	}
	return configReader(ctx, key)
}

// Active 返回当前生效的日志库 Store。按 log_database 系统配置惰性解析并缓存，
// 配置更新（含迁移任务翻转）后自动重建。
func Active(ctx context.Context) (*Store, error) {
	current, err := resolveDatabase(ctx)
	if err != nil {
		return nil, err
	}
	storeMu.RLock()
	if active != nil && activeDB == current {
		s := active
		storeMu.RUnlock()
		return s, nil
	}
	storeMu.RUnlock()

	storeMu.Lock()
	defer storeMu.Unlock()
	if active != nil && activeDB == current {
		return active, nil
	}
	s, err := buildStore(ctx, current, false)
	if err != nil {
		return nil, err
	}
	active = s
	activeDB = current
	return s, nil
}

// Build 直接按目标构造 store（不经 Active 缓存）。
func Build(ctx context.Context, database string) (*Store, error) {
	return buildStore(ctx, database, false)
}

// BuildForMigration 构造迁移目标 store：与 Build 相同但不做冻结检查
// （迁移期间 log_db_migration=migrating 已冻结源库写入，目标库的清空/复制写入必须放行）。
func BuildForMigration(ctx context.Context, database string) (*Store, error) {
	return buildStore(ctx, database, true)
}

// buildStore 按目标构造实现。skipFreeze 为 true 时该 store 跳过冻结检查
// （仅迁移任务的目标 store 使用）。gorm 分支 UserAccessLogs 用独立包装类型
// （gormLogStore 已占用 List/Count 方法名，无法再实现 UserAccessLogStore）。
func buildStore(ctx context.Context, database string, skipFreeze bool) (*Store, error) {
	switch database {
	case dbNameClickHouse:
		ch := newClickHouseStore()
		ch.skipFreeze = skipFreeze
		ual := newClickHouseUserAccessLogStore()
		ual.skipFreeze = skipFreeze
		return &Store{
			AccessLogs:     ch,
			Observability:  ch,
			UserAccessLogs: ual,
			Status:         ch,
		}, nil
	case dbNamePostgres, dbNameSQLite:
		gdb := getGormDB(ctx)
		g := newGormStore(gdb)
		g.skipFreeze = skipFreeze
		ual := newUserAccessLogGormStore(gdb)
		ual.skipFreeze = skipFreeze
		return &Store{
			AccessLogs:     g,
			Observability:  g,
			UserAccessLogs: ual,
			Status:         g,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported log database: %s", database)
	}
}

// Migrating 返回日志库是否处于迁移冻结状态。
func Migrating(ctx context.Context) bool {
	v, err := getConfig(ctx, logMigrationKey)
	if err != nil {
		if !errors.Is(err, errConfigReaderNotWired) {
			logger.ErrorF(ctx, "read log migration config failed: %v", err)
		}
		return false
	}
	return v == "migrating"
}

// Init 在 bootstrap 阶段预热一次激活 store（幂等，失败不致命——首次使用时再解析），
// 并兜底预建「当前月 + 未来 2 个月」分区：进程停机跨月边界、重启后每日 cleanup 之前
// 首次写入不会报 "no partition of relation found"（CH/SQLite 分支 EnsurePartitions 为 no-op）。
func Init(ctx context.Context) {
	s, err := Active(ctx)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	if err := s.AccessLogs.EnsurePartitions(ctx, now, now.AddDate(0, partitionLeadMonths, 0)); err != nil {
		logger.WarnF(ctx, "logstore: ensure startup partitions failed: %v", err)
	}
}

// InvalidateCache 清空日志库解析缓存（在修改 log_database 配置后显式调用）。
func InvalidateCache() {
	storeMu.Lock()
	defer storeMu.Unlock()
	lastResolveTime = time.Time{}
	lastResolveDB = ""
}

// ResetForTest 清空缓存的激活 store 与 config reader，便于测试注入。
func ResetForTest() {
	storeMu.Lock()
	active = nil
	activeDB = ""
	lastResolveDB = ""
	lastResolveTime = time.Time{}
	storeMu.Unlock()
	configReader = nil
}

// ActiveDatabase 返回当前日志主库名（postgres|sqlite|clickhouse）。
func ActiveDatabase(ctx context.Context) (string, error) {
	return resolveDatabase(ctx)
}

// resolveDatabase 读取 log_database：值缺失或 reader 未装配（首启）时按启动规则 seed；
// 已装配 reader 的真实读取错误直接透出，避免把读失败当首次启动。
func resolveDatabase(ctx context.Context) (string, error) {
	storeMu.RLock()
	if active != nil && time.Since(lastResolveTime) < resolveCacheTTL {
		db := lastResolveDB
		storeMu.RUnlock()
		return db, nil
	}
	storeMu.RUnlock()

	v, err := getConfig(ctx, logDatabaseKey)
	if err != nil && !errors.Is(err, errConfigReaderNotWired) {
		return "", err
	}

	resolved := v
	if resolved == "" {
		// 首次启动 seed：CH 启用 → clickhouse；否则随主库。
		resolved = dbNameSQLite
		if runtimeconfig.DatabaseEnabled() {
			resolved = dbNamePostgres
		}
		if runtimeconfig.ClickHouseEnabled() {
			resolved = dbNameClickHouse
		}
	}

	storeMu.Lock()
	lastResolveDB = resolved
	lastResolveTime = time.Now()
	storeMu.Unlock()

	return resolved, nil
}
