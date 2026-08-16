// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"errors"
	"fmt"

	"github.com/Rain-kl/Wavelet/internal/infra/config"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

// ClickHouseOperationalStats summarizes ClickHouse merge/mutation pressure
// and in-process batch writer queue health.
type ClickHouseOperationalStats = analyticsmodel.ClickHouseOperationalStats

// GetClickHouseOperationalStats returns operational metrics for the configured database.
func GetClickHouseOperationalStats(ctx context.Context) (*ClickHouseOperationalStats, error) {
	if db.ChConn == nil {
		return nil, errors.New("clickhouse native connection is not initialized")
	}
	database := config.Config.ClickHouse.Database
	stats := &ClickHouseOperationalStats{Database: database}

	partsSQL := `
SELECT
	count() AS active_parts,
	ifNull(sum(rows), 0) AS total_rows
FROM system.parts
WHERE active AND database = ?`
	var activeParts, totalRows uint64
	if err := db.ChConn.QueryRow(ctx, partsSQL, database).Scan(&activeParts, &totalRows); err != nil {
		return nil, fmt.Errorf("query system.parts: %w", err)
	}
	stats.ActiveParts = safeInt64Count(activeParts)
	stats.TotalRows = safeInt64Count(totalRows)

	mutationsSQL := `
SELECT count()
FROM system.mutations
WHERE is_done = 0 AND database = ?`
	if err := db.ChConn.QueryRow(ctx, mutationsSQL, database).Scan(&stats.PendingMutations); err != nil {
		return nil, fmt.Errorf("query system.mutations: %w", err)
	}

	asyncSQL := `
SELECT
	count() AS queue_entries,
	ifNull(sum(bytes), 0) AS queue_bytes
FROM system.asynchronous_inserts
WHERE database = ?`
	var queueEntries, queueBytes uint64
	if err := db.ChConn.QueryRow(ctx, asyncSQL, database).Scan(&queueEntries, &queueBytes); err != nil {
		// Older ClickHouse versions may not expose asynchronous_inserts; treat as optional.
		stats.AsyncInsertQueue = 0
		stats.AsyncInsertBytes = 0
	} else {
		stats.AsyncInsertQueue = safeInt64Count(queueEntries)
		stats.AsyncInsertBytes = safeInt64Count(queueBytes)
	}

	return stats, nil
}
