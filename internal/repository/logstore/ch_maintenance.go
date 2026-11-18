// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

const (
	clickHouseLimitClause       = " LIMIT ?"
	clickHouseLimitOffsetClause = " LIMIT ? OFFSET ?"
)

// DDL TTL days for ClickHouse tables (must match goose ClickHouse migrations).
const (
	chTableTTLDaysNodeAccessLogs      = 90
	chTableTTLDaysNodeMetricSnapshots = 30
	chTableTTLDaysNodeObs             = 30
	chTableTTLDaysUserAccessLogs      = 180
)

const (
	chCleanupModeTTLMaterialize = "ttl_materialize"
	chCleanupModeTruncate       = "truncate"
)

type chCleanupOutcome struct {
	EligibleCount int64
	DeletedCount  int64
	Mode          string
	TableTTLDays  int
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

// tableTTLCutoff returns the UTC instant at which rows become eligible under a fixed day TTL.
func tableTTLCutoff(tableTTLDays int, now time.Time) time.Time {
	if tableTTLDays < 1 {
		tableTTLDays = 1
	}
	return now.UTC().Add(-time.Duration(tableTTLDays) * 24 * time.Hour)
}

func materializeExpiredByTableTTL(
	ctx context.Context,
	conn driver.Conn,
	tableName string,
	tableTTLDays int,
	countSQL string,
	countArgs []any,
) (chCleanupOutcome, error) {
	outcome := chCleanupOutcome{
		Mode:         chCleanupModeTTLMaterialize,
		TableTTLDays: tableTTLDays,
	}
	count, err := countClickHouseRows(ctx, conn, countSQL, countArgs)
	if err != nil {
		return chCleanupOutcome{}, err
	}
	outcome.EligibleCount = count
	if err := materializeTableTTL(ctx, conn, tableName); err != nil {
		return chCleanupOutcome{}, err
	}
	return outcome, nil
}

func truncateClickHouseTable(ctx context.Context, conn driver.Conn, tableName string) (chCleanupOutcome, error) {
	count, err := countClickHouseRows(ctx, conn, "SELECT count() FROM "+tableName, nil)
	if err != nil {
		return chCleanupOutcome{}, err
	}
	if count == 0 {
		return chCleanupOutcome{Mode: chCleanupModeTruncate}, nil
	}
	if err := conn.Exec(ctx, "TRUNCATE TABLE "+tableName); err != nil {
		return chCleanupOutcome{}, fmt.Errorf("truncate %s: %w", tableName, err)
	}
	return chCleanupOutcome{
		EligibleCount: count,
		DeletedCount:  count,
		Mode:          chCleanupModeTruncate,
	}, nil
}
