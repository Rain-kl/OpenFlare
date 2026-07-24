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
	"github.com/Rain-kl/Wavelet/internal/infra/config"
)

const (
	clickhouseMaxExecTime       = 60 // ClickHouse 最大执行时间（秒）
	clickhouseReadTimeoutFactor = 2  // ReadTimeout 为 DialTimeout 的倍数

	// async_insert 仅挂在运行时 ChConn（写路径）上，不进入 migrator OpenDB：
	// 迁移/DDL 需要同步可见结果，且不应走异步 insert 缓冲。
	//
	// 为何启用：batchwriter 仍可能在短间隔内写出相对小的块；服务端 async_insert
	// 把多次 INSERT 合并成更大 part，减轻 3c6g 上 background merge 的 CPU 压力。
	// wait_for_async_insert=1：调用方在 flush 返回前等待落盘，避免进程崩溃丢批。
	// max_data_size / busy_timeout：约 10MB 或 ~2s 触发刷出，在延迟与 part 数之间折中。
	clickhouseAsyncInsertMaxDataSize   = 10_000_000
	clickhouseAsyncInsertBusyTimeoutMs = 2000
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

// buildClickHouseOptions builds the runtime native client options (queries + batch inserts).
// Migrator uses a separate clickhouse.OpenDB path without async_insert settings.
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
