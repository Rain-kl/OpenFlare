// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package migrator 提供数据库迁移功能
package migrator

import (
	"context"
	"embed"
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

func gooseDialect() string {
	if !config.Config.Database.Enabled {
		return "sqlite3"
	}
	return "postgres"
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
	if err := goose.Up(sqlDB, migrationDir()); err != nil {
		log.Fatalf("[%s] goose migrate failed: %v\n", dbType(), err)
	}

	clearSystemConfigCache()

	log.Printf("[%s] goose migrate success\n", dbType())
}

func clearSystemConfigCache() {
	if err := repository.InvalidateAllSystemConfigCaches(context.Background()); err != nil {
		log.Printf("[%s] clear system config cache failed: %v\n", dbType(), err)
	}
}
