// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"fmt"
	"time"
)

// DeleteAllNodeAccessLogs deletes all node access logs.
func DeleteAllNodeAccessLogs(ctx context.Context) (int64, error) {
	conn, err := nodeAccessLogConn()
	if err != nil {
		return 0, err
	}
	outcome, err := truncateClickHouseTable(ctx, conn, nodeAccessLogTableName())
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}

// DeleteNodeAccessLogsBefore expires logs older than cutoff via table TTL.
func DeleteNodeAccessLogsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	conn, err := nodeAccessLogConn()
	if err != nil {
		return 0, err
	}
	tableName := nodeAccessLogTableName()
	cutoff = cutoff.UTC()
	outcome, err := expireRowsViaTTL(
		ctx,
		conn,
		tableName,
		fmt.Sprintf("SELECT count() FROM %s WHERE logged_at < ?", tableName),
		[]any{cutoff},
	)
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}

// DeleteNodeAccessLogsByNodeBefore expires logs for a node older than cutoff via table TTL.
func DeleteNodeAccessLogsByNodeBefore(ctx context.Context, nodeID string, before time.Time) (int64, error) {
	conn, err := nodeAccessLogConn()
	if err != nil {
		return 0, err
	}
	tableName := nodeAccessLogTableName()
	before = before.UTC()
	outcome, err := expireRowsViaTTL(
		ctx,
		conn,
		tableName,
		fmt.Sprintf("SELECT count() FROM %s WHERE node_id = ? AND logged_at < ?", tableName),
		[]any{nodeID, before},
	)
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}