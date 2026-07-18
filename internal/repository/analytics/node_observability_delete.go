// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"fmt"
	"time"
)

// DeleteAllNodeMetricSnapshots hard-deletes all node metric snapshots via TRUNCATE.
func DeleteAllNodeMetricSnapshots(ctx context.Context) (int64, error) {
	conn, err := observabilityConn()
	if err != nil {
		return 0, err
	}
	outcome, err := truncateClickHouseTable(ctx, conn, nodeMetricSnapshotTableName())
	if err != nil {
		return 0, err
	}
	return outcome.DeletedCount, nil
}

// DeleteNodeMetricSnapshotsBefore force-materializes of_node_metric_snapshots table TTL.
// cutoff is ignored; see MaterializeNodeMetricSnapshotsTTL.
func DeleteNodeMetricSnapshotsBefore(ctx context.Context, _ time.Time) (int64, error) {
	outcome, err := MaterializeNodeMetricSnapshotsTTL(ctx)
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}

// MaterializeNodeMetricSnapshotsTTL force-materializes table TTL and reports an honest outcome.
func MaterializeNodeMetricSnapshotsTTL(ctx context.Context) (CleanupOutcome, error) {
	conn, err := observabilityConn()
	if err != nil {
		return CleanupOutcome{}, err
	}
	tableName := nodeMetricSnapshotTableName()
	ttlDays := TableTTLDaysNodeMetricSnapshots
	cutoff := tableTTLCutoff(ttlDays, time.Now())
	return materializeExpiredByTableTTL(
		ctx,
		conn,
		tableName,
		ttlDays,
		fmt.Sprintf("SELECT count() FROM %s WHERE captured_at < ?", tableName),
		[]any{cutoff},
	)
}

// DeleteAllNodeEdgeHealth truncates of_node_edge_health.
func DeleteAllNodeEdgeHealth(ctx context.Context) (int64, error) {
	conn, err := observabilityConn()
	if err != nil {
		return 0, err
	}
	outcome, err := truncateClickHouseTable(ctx, conn, nodeEdgeHealthTableName())
	if err != nil {
		return 0, err
	}
	return outcome.DeletedCount, nil
}

// DeleteNodeEdgeHealthBefore force-materializes of_node_edge_health TTL.
func DeleteNodeEdgeHealthBefore(ctx context.Context, _ time.Time) (int64, error) {
	outcome, err := MaterializeNodeEdgeHealthTTL(ctx)
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}

// MaterializeNodeEdgeHealthTTL force-materializes of_node_edge_health table TTL.
func MaterializeNodeEdgeHealthTTL(ctx context.Context) (CleanupOutcome, error) {
	conn, err := observabilityConn()
	if err != nil {
		return CleanupOutcome{}, err
	}
	tableName := nodeEdgeHealthTableName()
	ttlDays := TableTTLDaysNodeObs
	cutoff := tableTTLCutoff(ttlDays, time.Now())
	return materializeExpiredByTableTTL(
		ctx,
		conn,
		tableName,
		ttlDays,
		fmt.Sprintf("SELECT count() FROM %s WHERE captured_at < ?", tableName),
		[]any{cutoff},
	)
}

// DeleteAllNodeObsFrps hard-deletes all FRPS observations via TRUNCATE.
func DeleteAllNodeObsFrps(ctx context.Context) (int64, error) {
	conn, err := observabilityConn()
	if err != nil {
		return 0, err
	}
	outcome, err := truncateClickHouseTable(ctx, conn, nodeObsFrpsTableName())
	if err != nil {
		return 0, err
	}
	return outcome.DeletedCount, nil
}

// DeleteNodeObsFrpsBefore force-materializes of_node_obs_frps table TTL.
// cutoff is ignored; see MaterializeNodeObsFrpsTTL.
func DeleteNodeObsFrpsBefore(ctx context.Context, _ time.Time) (int64, error) {
	outcome, err := MaterializeNodeObsFrpsTTL(ctx)
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}

// MaterializeNodeObsFrpsTTL force-materializes table TTL and reports an honest outcome.
func MaterializeNodeObsFrpsTTL(ctx context.Context) (CleanupOutcome, error) {
	conn, err := observabilityConn()
	if err != nil {
		return CleanupOutcome{}, err
	}
	tableName := nodeObsFrpsTableName()
	ttlDays := TableTTLDaysNodeObs
	cutoff := tableTTLCutoff(ttlDays, time.Now())
	return materializeExpiredByTableTTL(
		ctx,
		conn,
		tableName,
		ttlDays,
		fmt.Sprintf("SELECT count() FROM %s WHERE captured_at < ?", tableName),
		[]any{cutoff},
	)
}

// DeleteAllNodeObsFrpc hard-deletes all FRPC observations via TRUNCATE.
func DeleteAllNodeObsFrpc(ctx context.Context) (int64, error) {
	conn, err := observabilityConn()
	if err != nil {
		return 0, err
	}
	outcome, err := truncateClickHouseTable(ctx, conn, nodeObsFrpcTableName())
	if err != nil {
		return 0, err
	}
	return outcome.DeletedCount, nil
}

// DeleteNodeObsFrpcBefore force-materializes of_node_obs_frpc table TTL.
// cutoff is ignored; see MaterializeNodeObsFrpcTTL.
func DeleteNodeObsFrpcBefore(ctx context.Context, _ time.Time) (int64, error) {
	outcome, err := MaterializeNodeObsFrpcTTL(ctx)
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}

// MaterializeNodeObsFrpcTTL force-materializes table TTL and reports an honest outcome.
func MaterializeNodeObsFrpcTTL(ctx context.Context) (CleanupOutcome, error) {
	conn, err := observabilityConn()
	if err != nil {
		return CleanupOutcome{}, err
	}
	tableName := nodeObsFrpcTableName()
	ttlDays := TableTTLDaysNodeObs
	cutoff := tableTTLCutoff(ttlDays, time.Now())
	return materializeExpiredByTableTTL(
		ctx,
		conn,
		tableName,
		ttlDays,
		fmt.Sprintf("SELECT count() FROM %s WHERE captured_at < ?", tableName),
		[]any{cutoff},
	)
}
