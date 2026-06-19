// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"context"
	"fmt"
	"log"
)

const openFlareNodeAccessLogsDDL = `
CREATE TABLE IF NOT EXISTS of_node_access_logs
(
    id          UInt64,
    node_id     String,
    logged_at   DateTime64(3, 'UTC'),
    remote_addr String,
    region      String,
    host        String,
    path        String,
    status_code Int32,
    created_at  DateTime64(3, 'UTC')
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(logged_at)
ORDER BY (node_id, logged_at, remote_addr, host, path, status_code)
SETTINGS index_granularity = 8192`

// EnsureClickHouseSchema creates the openflare database and required tables.
func EnsureClickHouseSchema(ctx context.Context) error {
	if ChConn == nil {
		return fmt.Errorf("clickhouse connection is not initialized")
	}

	if err := ChConn.Exec(ctx, "CREATE DATABASE IF NOT EXISTS openflare"); err != nil {
		return fmt.Errorf("create database openflare: %w", err)
	}
	if err := ChConn.Exec(ctx, openFlareNodeAccessLogsDDL); err != nil {
		return fmt.Errorf("create table of_node_access_logs: %w", err)
	}
	return nil
}

func ensureClickHouseSchemaOnStartup() {
	ctx := context.Background()
	if err := EnsureClickHouseSchema(ctx); err != nil {
		log.Fatalf("[ClickHouse] ensure schema failed: %v\n", err)
	}
	log.Println("[ClickHouse] schema ready (database: openflare)")
}