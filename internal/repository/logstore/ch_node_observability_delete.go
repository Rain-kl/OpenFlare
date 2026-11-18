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

func (s *clickhouseLogStore) DeleteAllNodeObservationFrps(ctx context.Context) (int64, error) {
	return s.chTruncateTable(ctx, chNodeObsFrpsTableName())
}

func (s *clickhouseLogStore) DeleteNodeObservationFrpsBefore(ctx context.Context, _ time.Time) (int64, error) {
	return s.chMaterializeObsTTL(ctx, chNodeObsFrpsTableName(), chTableTTLDaysNodeObs)
}

func (s *clickhouseLogStore) DeleteAllNodeObservationFrpc(ctx context.Context) (int64, error) {
	return s.chTruncateTable(ctx, chNodeObsFrpcTableName())
}

func (s *clickhouseLogStore) DeleteNodeObservationFrpcBefore(ctx context.Context, _ time.Time) (int64, error) {
	return s.chMaterializeObsTTL(ctx, chNodeObsFrpcTableName(), chTableTTLDaysNodeObs)
}
