// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"fmt"
	"time"
)

func (s *clickhouseLogStore) chTruncateTable(ctx context.Context, tableName string) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	conn, err := chConn()
	if err != nil {
		return 0, err
	}
	outcome, err := truncateClickHouseTable(ctx, conn, tableName)
	if err != nil {
		return 0, err
	}
	return outcome.DeletedCount, nil
}

func (s *clickhouseLogStore) chMaterializeObsTTL(ctx context.Context, tableName string, ttlDays int) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	conn, err := chConn()
	if err != nil {
		return 0, err
	}
	cutoff := tableTTLCutoff(ttlDays, time.Now())
	outcome, err := materializeExpiredByTableTTL(
		ctx,
		conn,
		tableName,
		ttlDays,
		fmt.Sprintf("SELECT count() FROM %s WHERE captured_at < ?", tableName),
		[]any{cutoff},
	)
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}

func (s *clickhouseLogStore) DeleteAllMetricSnapshots(ctx context.Context) (int64, error) {
	return s.chTruncateTable(ctx, chNodeMetricSnapshotTableName())
}

func (s *clickhouseLogStore) DeleteMetricSnapshotsBefore(ctx context.Context, _ time.Time) (int64, error) {
	return s.chMaterializeObsTTL(ctx, chNodeMetricSnapshotTableName(), chTableTTLDaysNodeMetricSnapshots)
}

func (s *clickhouseLogStore) DeleteAllEdgeHealth(ctx context.Context) (int64, error) {
	return s.chTruncateTable(ctx, chNodeEdgeHealthTableName())
}

func (s *clickhouseLogStore) DeleteEdgeHealthBefore(ctx context.Context, _ time.Time) (int64, error) {
	return s.chMaterializeObsTTL(ctx, chNodeEdgeHealthTableName(), chTableTTLDaysNodeObs)
}

func (s *clickhouseLogStore) DeleteAllPingRecords(ctx context.Context) (int64, error) {
	return s.chTruncateTable(ctx, chNodePingRecordTableName())
}

func (s *clickhouseLogStore) DeletePingRecordsBefore(ctx context.Context, _ time.Time) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	conn, err := chConn()
	if err != nil {
		return 0, err
	}
	cutoff := tableTTLCutoff(chTableTTLDaysNodeMetricSnapshots, time.Now())
	outcome, err := materializeExpiredByTableTTL(
		ctx,
		conn,
		chNodePingRecordTableName(),
		chTableTTLDaysNodeMetricSnapshots,
		fmt.Sprintf("SELECT count() FROM %s WHERE recorded_at < ?", chNodePingRecordTableName()),
		[]any{cutoff},
	)
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}

func (s *clickhouseLogStore) DeletePingRecordsByTaskIDs(ctx context.Context, taskIDs []uint) (int64, error) {
	if len(taskIDs) == 0 {
		return 0, nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	conn, err := chConn()
	if err != nil {
		return 0, err
	}
	ids := make([]uint64, len(taskIDs))
	for i, id := range taskIDs {
		ids[i] = uint64(id)
	}
	count, err := countClickHouseRows(ctx, conn, fmt.Sprintf("SELECT count() FROM %s WHERE task_id IN ?", chNodePingRecordTableName()), []any{ids})
	if err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, nil
	}
	if err := conn.Exec(ctx, fmt.Sprintf("ALTER TABLE %s DELETE WHERE task_id IN ?", chNodePingRecordTableName()), ids); err != nil {
		return 0, fmt.Errorf("delete ping records by task ids: %w", err)
	}
	return count, nil
}
