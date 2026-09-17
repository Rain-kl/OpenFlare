// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"fmt"

	analyticsmodel "Wavelet/openflare/plugins/server/kernel/model/analytics"
	"Wavelet/openflare/plugins/server/kernel/runtimeconfig"
)

// ClickHouseOperationalStats summarizes ClickHouse merge/mutation pressure
// and in-process batch writer queue health.
type ClickHouseOperationalStats = analyticsmodel.ClickHouseOperationalStats

// GetClickHouseOperationalStats returns operational metrics for the configured database.
func GetClickHouseOperationalStats(ctx context.Context) (*ClickHouseOperationalStats, error) {
	conn, err := ChConn(ctx)
	if err != nil {
		return nil, fmt.Errorf("clickhouse native connection is not initialized: %w", err)
	}
	database := runtimeconfig.Get().ClickHouse.Database
	stats := &ClickHouseOperationalStats{Database: database}

	partsSQL := `
SELECT
	count() AS active_parts,
	ifNull(sum(rows), 0) AS total_rows
FROM system.parts
WHERE active AND database = ?`
	var activeParts, totalRows uint64
	if err := conn.QueryRow(ctx, partsSQL, database).Scan(&activeParts, &totalRows); err != nil {
		return nil, fmt.Errorf("query system.parts: %w", err)
	}
	stats.ActiveParts = safeInt64Count(activeParts)
	stats.TotalRows = safeInt64Count(totalRows)

	mutationsSQL := `
SELECT
	count() AS pending_mutations
FROM system.mutations
WHERE NOT is_done AND database = ?`
	if err := conn.QueryRow(ctx, mutationsSQL, database).Scan(&stats.PendingMutations); err != nil {
		return nil, fmt.Errorf("query system.mutations: %w", err)
	}

	asyncSQL := `
SELECT
	ifNull(sum(entries), 0) AS queue_entries,
	ifNull(sum(bytes), 0) AS queue_bytes
FROM system.asynchronous_inserts
WHERE database = ?`
	var queueEntries, queueBytes uint64
	if err := conn.QueryRow(ctx, asyncSQL, database).Scan(&queueEntries, &queueBytes); err != nil {
		return nil, fmt.Errorf("query system.asynchronous_inserts: %w", err)
	}
	stats.AsyncInsertQueue = safeInt64Count(queueEntries)
	stats.AsyncInsertBytes = safeInt64Count(queueBytes)

	return stats, nil
}
