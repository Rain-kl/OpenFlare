// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package migrator 提供数据库迁移功能
package migrator

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/zone"
	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/pressly/goose/v3"
)

// migrationFS contains SQL migrations under goose/<dialect>.
//
//go:embed goose/postgres/*.sql goose/sqlite/*.sql
var migrationFS embed.FS

// dbType 返回当前数据库类型名称（用于日志输出）
func dbType() string {
	if !config.Config.Database.Enabled {
		return "SQLite"
	}
	return "PostgreSQL"
}

const (
	dialectSqlite   = "sqlite3"
	dialectPostgres = "postgres"
	// zoneImportSQLVersion is the goose SQL marker after of_zones creation and
	// before drop of legacy route domain columns. Zone data import runs here.
	zoneImportSQLVersion int64 = 202607120002
)

func gooseDialect() string {
	if !config.Config.Database.Enabled {
		return dialectSqlite
	}
	return dialectPostgres
}

func migrationDir() string {
	if !config.Config.Database.Enabled {
		return "goose/sqlite"
	}
	return "goose/postgres"
}

// Migrate 执行数据库迁移：全部结构变更走 goose SQL；Zone 历史域名导入在 SQL 之后自动执行。
func Migrate() {
	gormDB := db.DB(context.Background())
	if gormDB == nil {
		log.Fatalf("[%s] database not initialized\n", dbType())
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		log.Fatalf("[%s] load sql db failed: %v\n", dbType(), err)
	}

	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect(gooseDialect()); err != nil {
		log.Fatalf("[%s] set goose dialect failed: %v\n", dbType(), err)
	}
	if err := resyncGooseVersionSequence(sqlDB); err != nil {
		log.Fatalf("[%s] resync goose_db_version sequence failed: %v\n", dbType(), err)
	}
	// 1) SQL up to zone-import marker (includes of_zones DDL; still has legacy columns).
	if err := goose.UpTo(sqlDB, migrationDir(), zoneImportSQLVersion); err != nil {
		log.Fatalf("[%s] goose migrate (up to zone import) failed: %v\n", dbType(), err)
	}
	// 2) Auto-import legacy domains (publicsuffix; idempotent; no-op after phase-2 drop).
	if err := importZoneDomainsAfterGoose(sqlDB); err != nil {
		log.Fatalf("[%s] zone domain import failed: %v\n", dbType(), err)
	}
	// 3) Remaining SQL (drop legacy columns / managed_domains, later migrations).
	if err := goose.Up(sqlDB, migrationDir()); err != nil {
		log.Fatalf("[%s] goose migrate failed: %v\n", dbType(), err)
	}

	clearSystemConfigCache()

	log.Printf("[%s] goose migrate success\n", dbType())
}

// importZoneDomainsAfterGoose 在 SQL 迁移完成后自动导入旧路由/托管域名。
// 必须使用 publicsuffix 解析注册根域，故不能放在纯 SQL 中；幂等且在旧列删除后为空操作。
func importZoneDomainsAfterGoose(sqlDB *sql.DB) error {
	ctx := context.Background()
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin zone import transaction: %w", err)
	}
	report, err := zone.ImportLegacyTx(ctx, tx, gooseDialect() == dialectPostgres)
	if err != nil {
		_ = tx.Rollback()
		return report.LogAndReturn(err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit zone import: %w", err)
	}
	if report.Zones > 0 || report.Domains > 0 {
		log.Printf(
			"[%s] imported zone domains automatically: zones=%d domains=%d\n",
			dbType(), report.Zones, report.Domains,
		)
	}
	return nil
}

// resyncGooseVersionSequence 修复 PostgreSQL 下 goose_db_version.id 自增序列落后于
// MAX(id) 的问题（常见于从 dump 恢复或历史迁移以显式 id 复制数据后）。序列落后会
// 导致 goose 记录新版本号时 INSERT 命中 goose_db_version_pkey 唯一约束冲突。
// 仅在表已存在且为 PostgreSQL 方言时执行；SQLite 使用 AUTOINCREMENT 不受影响。
func resyncGooseVersionSequence(sqlDB *sql.DB) error {
	if gooseDialect() != dialectPostgres {
		return nil
	}

	ctx := context.Background()
	var exists bool
	if err := sqlDB.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name='goose_db_version')",
	).Scan(&exists); err != nil {
		return fmt.Errorf("check goose_db_version existence failed: %w", err)
	}
	if !exists {
		return nil
	}

	const resyncSQL = `SELECT setval(
		pg_get_serial_sequence('goose_db_version', 'id'),
		GREATEST(COALESCE((SELECT MAX(id) FROM goose_db_version), 1), 1),
		(SELECT MAX(id) IS NOT NULL FROM goose_db_version)
	)`
	if _, err := sqlDB.ExecContext(ctx, resyncSQL); err != nil {
		return fmt.Errorf("setval goose_db_version sequence failed: %w", err)
	}
	return nil
}

func clearSystemConfigCache() {
	if err := repository.InvalidateAllSystemConfigCaches(context.Background()); err != nil {
		log.Printf("[%s] clear system config cache failed: %v\n", dbType(), err)
	}
}
