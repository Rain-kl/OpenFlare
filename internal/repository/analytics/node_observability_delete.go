// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"fmt"
	"time"
)

// DeleteAllNodeMetricSnapshots deletes all node metric snapshots.
func DeleteAllNodeMetricSnapshots(ctx context.Context) (int64, error) {
	conn, err := observabilityConn()
	if err != nil {
		return 0, err
	}
	outcome, err := truncateClickHouseTable(ctx, conn, nodeMetricSnapshotTableName())
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}

// DeleteNodeMetricSnapshotsBefore expires metric snapshots captured before cutoff via table TTL.
func DeleteNodeMetricSnapshotsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	conn, err := observabilityConn()
	if err != nil {
		return 0, err
	}
	tableName := nodeMetricSnapshotTableName()
	cutoff = cutoff.UTC()
	outcome, err := expireRowsViaTTL(
		ctx,
		conn,
		tableName,
		fmt.Sprintf("SELECT count() FROM %s WHERE captured_at < ?", tableName),
		[]any{cutoff},
	)
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}

// DeleteAllNodeRequestReports deletes all node request reports.
func DeleteAllNodeRequestReports(ctx context.Context) (int64, error) {
	conn, err := observabilityConn()
	if err != nil {
		return 0, err
	}
	outcome, err := truncateClickHouseTable(ctx, conn, nodeRequestReportTableName())
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}

// DeleteNodeRequestReportsBefore expires request reports ending before cutoff via table TTL.
func DeleteNodeRequestReportsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	conn, err := observabilityConn()
	if err != nil {
		return 0, err
	}
	tableName := nodeRequestReportTableName()
	cutoff = cutoff.UTC()
	outcome, err := expireRowsViaTTL(
		ctx,
		conn,
		tableName,
		fmt.Sprintf("SELECT count() FROM %s WHERE window_ended_at < ?", tableName),
		[]any{cutoff},
	)
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}

// DeleteAllNodeObsOpenresty deletes all OpenResty observations.
func DeleteAllNodeObsOpenresty(ctx context.Context) (int64, error) {
	conn, err := observabilityConn()
	if err != nil {
		return 0, err
	}
	outcome, err := truncateClickHouseTable(ctx, conn, nodeObsOpenrestyTableName())
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}

// DeleteNodeObsOpenrestyBefore expires OpenResty observations captured before cutoff via table TTL.
func DeleteNodeObsOpenrestyBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	conn, err := observabilityConn()
	if err != nil {
		return 0, err
	}
	tableName := nodeObsOpenrestyTableName()
	cutoff = cutoff.UTC()
	outcome, err := expireRowsViaTTL(
		ctx,
		conn,
		tableName,
		fmt.Sprintf("SELECT count() FROM %s WHERE captured_at < ?", tableName),
		[]any{cutoff},
	)
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}

// DeleteAllNodeObsFrps deletes all FRPS observations.
func DeleteAllNodeObsFrps(ctx context.Context) (int64, error) {
	conn, err := observabilityConn()
	if err != nil {
		return 0, err
	}
	outcome, err := truncateClickHouseTable(ctx, conn, nodeObsFrpsTableName())
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}

// DeleteNodeObsFrpsBefore expires FRPS observations captured before cutoff via table TTL.
func DeleteNodeObsFrpsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	conn, err := observabilityConn()
	if err != nil {
		return 0, err
	}
	tableName := nodeObsFrpsTableName()
	cutoff = cutoff.UTC()
	outcome, err := expireRowsViaTTL(
		ctx,
		conn,
		tableName,
		fmt.Sprintf("SELECT count() FROM %s WHERE captured_at < ?", tableName),
		[]any{cutoff},
	)
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}

// DeleteAllNodeObsFrpc deletes all FRPC observations.
func DeleteAllNodeObsFrpc(ctx context.Context) (int64, error) {
	conn, err := observabilityConn()
	if err != nil {
		return 0, err
	}
	outcome, err := truncateClickHouseTable(ctx, conn, nodeObsFrpcTableName())
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}

// DeleteNodeObsFrpcBefore expires FRPC observations captured before cutoff via table TTL.
func DeleteNodeObsFrpcBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	conn, err := observabilityConn()
	if err != nil {
		return 0, err
	}
	tableName := nodeObsFrpcTableName()
	cutoff = cutoff.UTC()
	outcome, err := expireRowsViaTTL(
		ctx,
		conn,
		tableName,
		fmt.Sprintf("SELECT count() FROM %s WHERE captured_at < ?", tableName),
		[]any{cutoff},
	)
	if err != nil {
		return 0, err
	}
	return outcome.EligibleCount, nil
}