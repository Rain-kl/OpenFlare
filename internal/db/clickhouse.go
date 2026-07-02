// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package db 提供数据库连接与基础设施
package db

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/Rain-kl/Wavelet/internal/config"
	"go.opentelemetry.io/otel/attribute"
	clickhouseDriver "gorm.io/driver/clickhouse"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"
)

const (
	clickhouseMaxExecTime              = 60 // ClickHouse 最大执行时间（秒）
	clickhouseReadTimeoutFactor        = 2  // ReadTimeout 为 DialTimeout 的倍数
	clickhouseAsyncInsertMaxDataSize   = 10_000_000
	clickhouseAsyncInsertBusyTimeoutMs = 1000
)

var (
	// ChConn ClickHouse 原生连接实例，用于批量写入
	ChConn driver.Conn

	chDB *gorm.DB
)

func init() {
	if !config.Config.ClickHouse.Enabled {
		return
	}

	cfg := config.Config.ClickHouse
	if cfg.Database == "" {
		log.Fatalf("[ClickHouse] database name is required (expected: openflare)\n")
	}

	opts := buildClickHouseOptions()

	var err error
	ChConn, err = clickhouse.Open(opts)
	if err != nil {
		log.Fatalf("[ClickHouse] init connection failed: %v\n", err)
	}

	if err = ChConn.Ping(context.Background()); err != nil {
		log.Fatalf("[ClickHouse] ping failed: %v\n", err)
	}

	chDB, err = gorm.Open(clickhouseDriver.New(clickhouseDriver.Config{
		DSN: buildClickHouseDSN(),
	}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		log.Fatalf("[ClickHouse] init gorm connection failed: %v\n", err)
	}

	if err = chDB.Use(
		tracing.NewPlugin(
			tracing.WithoutMetrics(),
			tracing.WithAttributes(
				attribute.String("db.instance", cfg.Database),
				attribute.String("db.system", "ClickHouse"),
			),
		),
	); err != nil {
		log.Fatalf("[ClickHouse] init trace failed: %v\n", err)
	}

	sqlDB, err := chDB.DB()
	if err != nil {
		log.Fatalf("[ClickHouse] load sql db failed: %v\n", err)
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdleConn)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConn)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	log.Println("[ClickHouse] connection established successfully")
}

func buildClickHouseOptions() *clickhouse.Options {
	cfg := config.Config.ClickHouse

	return &clickhouse.Options{
		Addr: cfg.Hosts,
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time":           clickhouseMaxExecTime,
			"async_insert":                 1,
			"wait_for_async_insert":        0,
			"async_insert_max_data_size":   clickhouseAsyncInsertMaxDataSize,
			"async_insert_busy_timeout_ms": clickhouseAsyncInsertBusyTimeoutMs,
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
	}
}

func buildClickHouseDSN() string {
	cfg := config.Config.ClickHouse

	chURL := &url.URL{
		Scheme: "clickhouse",
		Host:   strings.Join(cfg.Hosts, ","),
		Path:   "/" + cfg.Database,
	}
	if cfg.Username != "" || cfg.Password != "" {
		chURL.User = url.UserPassword(cfg.Username, cfg.Password)
	}

	query := chURL.Query()
	query.Set("dial_timeout", fmt.Sprintf("%ds", cfg.DialTimeout))
	query.Set("read_timeout", fmt.Sprintf("%ds", cfg.DialTimeout*clickhouseReadTimeoutFactor))
	query.Set("max_execution_time", strconv.Itoa(clickhouseMaxExecTime))
	// GORM uses clickhouse-go ParseDSN; unknown params are passed as server Settings.
	query.Set("async_insert", "1")
	query.Set("wait_for_async_insert", "0")
	query.Set("async_insert_max_data_size", strconv.Itoa(clickhouseAsyncInsertMaxDataSize))
	query.Set("async_insert_busy_timeout_ms", strconv.Itoa(clickhouseAsyncInsertBusyTimeoutMs))
	query.Set("max_insert_block_size", strconv.Itoa(clickhouseAsyncInsertMaxDataSize))
	chURL.RawQuery = query.Encode()

	return chURL.String()
}

// ChDB returns a context-aware GORM ClickHouse instance.
func ChDB(ctx context.Context) *gorm.DB {
	if chDB == nil {
		return nil
	}
	return chDB.WithContext(ctx)
}

// SetChDBForTest sets the package-level ClickHouse GORM instance for testing.
func SetChDBForTest(d *gorm.DB) {
	chDB = d
}

// SetChConnForTest sets the package-level native ClickHouse connection for testing.
func SetChConnForTest(c driver.Conn) {
	ChConn = c
}