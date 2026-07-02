// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

const (
	// CleanupModeTTLMaterialize expires rows via table TTL instead of ALTER DELETE mutations.
	CleanupModeTTLMaterialize = "ttl_materialize"
	// CleanupModeTruncate removes all rows via TRUNCATE TABLE.
	CleanupModeTruncate = "truncate"
)

// CleanupOutcome describes a non-mutation ClickHouse cleanup operation.
type CleanupOutcome struct {
	EligibleCount int64
	Mode          string
}

func countClickHouseRows(ctx context.Context, conn driver.Conn, countSQL string, countArgs []any) (int64, error) {
	var count uint64
	if err := conn.QueryRow(ctx, countSQL, countArgs...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count clickhouse rows: %w", err)
	}
	return safeInt64Count(count), nil
}

func materializeTableTTL(ctx context.Context, conn driver.Conn, tableName string) error {
	sql := fmt.Sprintf("ALTER TABLE %s MATERIALIZE TTL", tableName)
	if err := conn.Exec(ctx, sql); err != nil {
		return fmt.Errorf("materialize ttl on %s: %w", tableName, err)
	}
	return nil
}

func expireRowsViaTTL(ctx context.Context, conn driver.Conn, tableName string, countSQL string, countArgs []any) (CleanupOutcome, error) {
	count, err := countClickHouseRows(ctx, conn, countSQL, countArgs)
	if err != nil {
		return CleanupOutcome{}, err
	}
	if count == 0 {
		return CleanupOutcome{Mode: CleanupModeTTLMaterialize}, nil
	}
	if err := materializeTableTTL(ctx, conn, tableName); err != nil {
		return CleanupOutcome{}, err
	}
	return CleanupOutcome{
		EligibleCount: count,
		Mode:          CleanupModeTTLMaterialize,
	}, nil
}

func truncateClickHouseTable(ctx context.Context, conn driver.Conn, tableName string) (CleanupOutcome, error) {
	count, err := countClickHouseRows(ctx, conn, "SELECT count() FROM "+tableName, nil)
	if err != nil {
		return CleanupOutcome{}, err
	}
	if count == 0 {
		return CleanupOutcome{Mode: CleanupModeTruncate}, nil
	}
	if err := conn.Exec(ctx, "TRUNCATE TABLE "+tableName); err != nil {
		return CleanupOutcome{}, fmt.Errorf("truncate %s: %w", tableName, err)
	}
	return CleanupOutcome{
		EligibleCount: count,
		Mode:          CleanupModeTruncate,
	}, nil
}