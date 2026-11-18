// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"fmt"

	"github.com/Rain-kl/Wavelet/internal/infra/config"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

// ClickHouseOperationalStats 汇总 ClickHouse 运行状态与异步队列压力。
func (s *clickhouseLogStore) ClickHouseOperationalStats(ctx context.Context) (*analyticsmodel.ClickHouseOperationalStats, error) {
	conn, err := chConn()
	if err != nil {
		return nil, err
	}
	database := config.Config.ClickHouse.Database
	stats := &analyticsmodel.ClickHouseOperationalStats{Database: database}

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
SELECT count()
FROM system.mutations
WHERE is_done = 0 AND database = ?`
	if err := conn.QueryRow(ctx, mutationsSQL, database).Scan(&stats.PendingMutations); err != nil {
		return nil, fmt.Errorf("query system.mutations: %w", err)
	}

	asyncSQL := `
SELECT
	count() AS queue_entries,
	ifNull(sum(bytes), 0) AS queue_bytes
FROM system.asynchronous_inserts
WHERE database = ?`
	var queueEntries, queueBytes uint64
	if err := conn.QueryRow(ctx, asyncSQL, database).Scan(&queueEntries, &queueBytes); err != nil {
		// Older ClickHouse versions may not expose asynchronous_inserts; treat as optional.
		stats.AsyncInsertQueue = 0
		stats.AsyncInsertBytes = 0
	} else {
		stats.AsyncInsertQueue = safeInt64Count(queueEntries)
		stats.AsyncInsertBytes = safeInt64Count(queueBytes)
	}

	return stats, nil
}
