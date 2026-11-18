// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"fmt"
	"time"
)

func (s *clickhouseLogStore) DeleteAll(ctx context.Context) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	conn, err := chConn()
	if err != nil {
		return 0, err
	}
	outcome, err := truncateClickHouseTable(ctx, conn, chNodeAccessLogTableName())
	if err != nil {
		return 0, err
	}
	return outcome.DeletedCount, nil
}

func (s *clickhouseLogStore) DeleteBefore(ctx context.Context, _ time.Time) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	outcome, err := materializeNodeAccessLogsTTL(ctx)
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}

func materializeNodeAccessLogsTTL(ctx context.Context) (chCleanupOutcome, error) {
	conn, err := chConn()
	if err != nil {
		return chCleanupOutcome{}, err
	}
	tableName := chNodeAccessLogTableName()
	ttlDays := chTableTTLDaysNodeAccessLogs
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

func (s *clickhouseLogStore) DeleteByNodeBefore(ctx context.Context, nodeID string, _ time.Time) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	outcome, err := materializeNodeAccessLogsTTLByNode(ctx, nodeID)
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}

func materializeNodeAccessLogsTTLByNode(ctx context.Context, nodeID string) (chCleanupOutcome, error) {
	conn, err := chConn()
	if err != nil {
		return chCleanupOutcome{}, err
	}
	tableName := chNodeAccessLogTableName()
	ttlDays := chTableTTLDaysNodeAccessLogs
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
