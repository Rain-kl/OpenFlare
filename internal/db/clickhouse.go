// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package db 提供数据库连接与基础设施
package db

import (
	"context"
	"log"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/Rain-kl/Wavelet/internal/config"
)

const (
	clickhouseMaxExecTime              = 60 // ClickHouse 最大执行时间（秒）
	clickhouseReadTimeoutFactor        = 2  // ReadTimeout 为 DialTimeout 的倍数
	clickhouseAsyncInsertMaxDataSize   = 10_000_000
	clickhouseAsyncInsertBusyTimeoutMs = 1000
)

var (
	// ChConn ClickHouse 原生连接实例，用于批量写入与查询
	ChConn driver.Conn
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
			"wait_for_async_insert":        1,
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

// ChConnReady reports whether the native ClickHouse connection is initialized.
func ChConnReady() bool {
	return ChConn != nil
}

// SetChConnForTest sets the package-level native ClickHouse connection for testing.
func SetChConnForTest(c driver.Conn) {
	ChConn = c
}