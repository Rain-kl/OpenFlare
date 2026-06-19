// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package migrator

import (
	"context"
	"database/sql"
	"embed"
	"io/fs"
	"log"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/pressly/goose/v3"
)

const (
	clickhouseMigrationDir      = "goose/clickhouse"
	clickhouseGooseVersionTable = "goose_clickhouse_version"
	clickhouseMaxExecTime       = 60
	clickhouseReadTimeoutFactor = 2
)

// clickhouseMigrationFS contains SQL migrations under goose/clickhouse.
//
//go:embed goose/clickhouse/*.sql
var clickhouseMigrationFS embed.FS

// MigrateClickHouse runs goose migrations against ClickHouse when enabled.
func MigrateClickHouse() {
	if !config.Config.ClickHouse.Enabled {
		return
	}

	cfg := config.Config.ClickHouse
	sqlDB := clickhouse.OpenDB(&clickhouse.Options{
		Addr: cfg.Hosts,
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": clickhouseMaxExecTime,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		DialTimeout:     time.Duration(cfg.DialTimeout) * time.Second,
		MaxOpenConns:    cfg.MaxOpenConn,
		MaxIdleConns:    cfg.MaxIdleConn,
		ConnMaxLifetime: time.Duration(cfg.ConnMaxLifetime) * time.Second,
		ReadTimeout:     time.Duration(cfg.DialTimeout*clickhouseReadTimeoutFactor) * time.Second,
		BlockBufferSize: cfg.BlockBufferSize,
	})

	subFS, err := fs.Sub(clickhouseMigrationFS, "goose/clickhouse")
	if err != nil {
		closeClickHouseDB(sqlDB)
		log.Fatalf("[ClickHouse] get sub fs failed: %v\n", err)
	}

	provider, err := goose.NewProvider(
		"clickhouse",
		sqlDB,
		subFS,
		goose.WithTableName(clickhouseGooseVersionTable),
		goose.WithDisableGlobalRegistry(true),
	)
	if err != nil {
		closeClickHouseDB(sqlDB)
		log.Fatalf("[ClickHouse] create goose provider failed: %v\n", err)
	}

	if _, err := provider.Up(context.Background()); err != nil {
		closeClickHouseDB(sqlDB)
		log.Fatalf("[ClickHouse] goose migrate failed: %v\n", err)
	}
	closeClickHouseDB(sqlDB)

	log.Println("[ClickHouse] goose migrate success")
}

func closeClickHouseDB(sqlDB *sql.DB) {
	if err := sqlDB.Close(); err != nil {
		log.Printf("[ClickHouse] close sql db failed: %v\n", err)
	}
}
