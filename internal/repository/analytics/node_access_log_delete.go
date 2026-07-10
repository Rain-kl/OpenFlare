// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"fmt"
	"time"
)

// DeleteAllNodeAccessLogs hard-deletes all node access logs via TRUNCATE.
func DeleteAllNodeAccessLogs(ctx context.Context) (int64, error) {
	conn, err := nodeAccessLogConn()
	if err != nil {
		return 0, err
	}
	outcome, err := truncateClickHouseTable(ctx, conn, nodeAccessLogTableName())
	if err != nil {
		return 0, err
	}
	return outcome.DeletedCount, nil
}

// DeleteNodeAccessLogsBefore force-materializes of_node_access_logs table TTL.
//
// The cutoff argument is kept for call-site compatibility and is not used to select rows:
// ClickHouse MATERIALIZE TTL only enforces the DDL policy (TableTTLDaysNodeAccessLogs).
// Returns an estimate of rows past table TTL as the int64 (not a hard-deleted count).
// Callers that need honest API fields should prefer MaterializeNodeAccessLogsTTL.
func DeleteNodeAccessLogsBefore(ctx context.Context, _ time.Time) (int64, error) {
	outcome, err := MaterializeNodeAccessLogsTTL(ctx)
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}

// MaterializeNodeAccessLogsTTL force-materializes table TTL and reports an honest outcome.
func MaterializeNodeAccessLogsTTL(ctx context.Context) (CleanupOutcome, error) {
	conn, err := nodeAccessLogConn()
	if err != nil {
		return CleanupOutcome{}, err
	}
	tableName := nodeAccessLogTableName()
	ttlDays := TableTTLDaysNodeAccessLogs
	cutoff := tableTTLCutoff(ttlDays, time.Now())
	return materializeExpiredByTableTTL(
		ctx,
		conn,
		tableName,
		ttlDays,
		fmt.Sprintf("SELECT count() FROM %s WHERE logged_at < ?", tableName),
		[]any{cutoff},
	)
}

// DeleteNodeAccessLogsByNodeBefore force-materializes table-global TTL.
//
// Node-scoped hard delete is not supported: MATERIALIZE TTL is table-global.
// The returned count is an estimate of rows for nodeID past table TTL only.
func DeleteNodeAccessLogsByNodeBefore(ctx context.Context, nodeID string, _ time.Time) (int64, error) {
	outcome, err := MaterializeNodeAccessLogsTTLByNode(ctx, nodeID)
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}

// MaterializeNodeAccessLogsTTLByNode materializes table-global TTL and estimates node-scoped rows past TTL.
func MaterializeNodeAccessLogsTTLByNode(ctx context.Context, nodeID string) (CleanupOutcome, error) {
	conn, err := nodeAccessLogConn()
	if err != nil {
		return CleanupOutcome{}, err
	}
	tableName := nodeAccessLogTableName()
	ttlDays := TableTTLDaysNodeAccessLogs
	cutoff := tableTTLCutoff(ttlDays, time.Now())
	return materializeExpiredByTableTTL(
		ctx,
		conn,
		tableName,
		ttlDays,
		fmt.Sprintf("SELECT count() FROM %s WHERE node_id = ? AND logged_at < ?", tableName),
		[]any{nodeID, cutoff},
	)
}
