// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"Wavelet/openflare/plugins/server/kernel/runtimeconfig"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

var (
	chMu   sync.RWMutex
	chConn driver.Conn
)

// SetChConnForTest sets a mock or test ClickHouse connection.
func SetChConnForTest(conn driver.Conn) {
	chMu.Lock()
	defer chMu.Unlock()
	chConn = conn
}

// ChConn returns the active ClickHouse driver connection, initializing lazily if needed.
func ChConn(ctx context.Context) (driver.Conn, error) {
	chMu.RLock()
	c := chConn
	chMu.RUnlock()
	if c != nil {
		return c, nil
	}

	chMu.Lock()
	defer chMu.Unlock()
	if chConn != nil {
		return chConn, nil
	}

	if !runtimeconfig.ClickHouseEnabled() {
		return nil, fmt.Errorf("clickhouse is not enabled")
	}

	cfg := runtimeconfig.Get().ClickHouse
	opts := &clickhouse.Options{
		Addr: cfg.Hosts,
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		DialTimeout:     time.Duration(cfg.DialTimeout) * time.Second,
		MaxOpenConns:    cfg.MaxOpenConn,
		MaxIdleConns:    cfg.MaxIdleConn,
		ConnMaxLifetime: time.Duration(cfg.ConnMaxLifetime) * time.Second,
		BlockBufferSize: cfg.BlockBufferSize,
	}
	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open clickhouse connection: %w", err)
	}
	chConn = conn
	return chConn, nil
}
