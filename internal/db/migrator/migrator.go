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
	cascadeSuffix   = " CASCADE"
)

func gooseDialect() string {
	if !config.Config.Database.Enabled {
		return dialectSqlite
	}
	return dialectPostgres
}

func tableExistsSQL(dialect string) string {
	if dialect == dialectSqlite {
		return "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?"
	}
	return "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1"
}

func tablesWithPrefixSQL(dialect string) string {
	if dialect == dialectSqlite {
		return "SELECT name FROM sqlite_master WHERE type='table' AND name LIKE ?"
	}
	return "SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_name LIKE $1"
}

func migrationDir() string {
	if !config.Config.Database.Enabled {
		return "goose/sqlite"
	}
	return "goose/postgres"
}

// Migrate 执行数据库迁移
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
	if err := goose.Up(sqlDB, migrationDir()); err != nil {
		log.Fatalf("[%s] goose migrate failed: %v\n", dbType(), err)
	}

	clearSystemConfigCache()

	log.Printf("[%s] goose migrate success\n", dbType())
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
