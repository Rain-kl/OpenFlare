// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// DDL TTL days for analytics tables (must match goose ClickHouse migrations).
const (
	// TableTTLDaysNodeAccessLogs is the of_node_access_logs TTL (90 days).
	TableTTLDaysNodeAccessLogs = 90
	// TableTTLDaysNodeMetricSnapshots is the of_node_metric_snapshots TTL (30 days).
	TableTTLDaysNodeMetricSnapshots = 30
	// TableTTLDaysNodeRequestReports is the of_node_request_reports TTL (30 days).
	TableTTLDaysNodeRequestReports = 30
	// TableTTLDaysNodeObs is the of_node_obs_* TTL (30 days).
	TableTTLDaysNodeObs = 30
	// TableTTLDaysUserAccessLogs is the w_user_access_logs TTL (180 days).
	TableTTLDaysUserAccessLogs = 180
)

const (
	// CleanupModeTTLMaterialize expires rows via table TTL instead of ALTER DELETE mutations.
	// This is not a hard delete: deleted_count must stay 0; use EligibleCount as an estimate.
	CleanupModeTTLMaterialize = "ttl_materialize"
	// CleanupModeTruncate removes all rows via TRUNCATE TABLE (hard delete).
	CleanupModeTruncate = "truncate"
)

// CleanupOutcome describes a non-mutation ClickHouse cleanup operation.
//
// For CleanupModeTruncate:
//   - DeletedCount and EligibleCount are the rows removed by TRUNCATE.
//
// For CleanupModeTTLMaterialize:
//   - DeletedCount is always 0 (MATERIALIZE TTL is async / not a counted hard delete).
//   - EligibleCount is an estimate of rows already past the table TTL policy (not an
//     arbitrary user cutoff younger than the DDL TTL).
//   - TableTTLDays is the DDL TTL used for the estimate and materialize.
type CleanupOutcome struct {
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

// materializeExpiredByTableTTL force-materializes table TTL and estimates rows past that policy.
//
// countSQL must count only rows older than the table TTL (callers pass tableTTLCutoff args).
// Node-scoped filters may be used for the estimate only; MATERIALIZE is always table-global.
func materializeExpiredByTableTTL(
	ctx context.Context,
	conn driver.Conn,
	tableName string,
	tableTTLDays int,
	countSQL string,
	countArgs []any,
) (CleanupOutcome, error) {
	outcome := CleanupOutcome{
		Mode:         CleanupModeTTLMaterialize,
		TableTTLDays: tableTTLDays,
	}
	count, err := countClickHouseRows(ctx, conn, countSQL, countArgs)
	if err != nil {
		return CleanupOutcome{}, err
	}
	outcome.EligibleCount = count
	// Always force materialize so ClickHouse applies the DDL TTL policy promptly.
	// EligibleCount is informational only; MATERIALIZE does not return a deleted row count.
	if err := materializeTableTTL(ctx, conn, tableName); err != nil {
		return CleanupOutcome{}, err
	}
	return outcome, nil
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
		DeletedCount:  count,
		Mode:          CleanupModeTruncate,
	}, nil
}
