// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/db/idgen"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

// InsertNodeMetricSnapshot writes a single metric snapshot via the batch API.
func InsertNodeMetricSnapshot(ctx context.Context, snapshot analyticsmodel.NodeMetricSnapshot) error {
	if strings.TrimSpace(snapshot.NodeID) == "" {
		return nil
	}
	return BatchInsertNodeMetricSnapshots(ctx, []analyticsmodel.NodeMetricSnapshot{snapshot})
}

// BatchInsertNodeMetricSnapshots writes metric snapshots to ClickHouse.
func BatchInsertNodeMetricSnapshots(ctx context.Context, snapshots []analyticsmodel.NodeMetricSnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}
	if db.ChConn == nil {
		return fmt.Errorf("clickhouse connection is not initialized")
	}

	batch, err := db.ChConn.PrepareBatch(ctx, analyticsmodel.NodeMetricSnapshot{}.BatchInsertSQL())
	if err != nil {
		return fmt.Errorf("prepare clickhouse batch: %w", err)
	}

	now := time.Now().UTC()
	for _, snapshot := range snapshots {
		nodeID := strings.TrimSpace(snapshot.NodeID)
		if nodeID == "" {
			continue
		}
		id := snapshot.ID
		if id == 0 {
			id = idgen.NextUint64ID()
		}
		createdAt := snapshot.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		if err := batch.Append(
			id,
			nodeID,
			snapshot.CapturedAt.UTC(),
			snapshot.CPUUsagePercent,
			snapshot.MemoryUsedBytes,
			snapshot.MemoryTotalBytes,
			snapshot.StorageUsedBytes,
			snapshot.StorageTotalBytes,
			snapshot.DiskReadBytes,
			snapshot.DiskWriteBytes,
			snapshot.NetworkRxBytes,
			snapshot.NetworkTxBytes,
			createdAt.UTC(),
		); err != nil {
			return fmt.Errorf("append node metric snapshot to batch: %w", err)
		}
	}

	if batch.Rows() == 0 {
		return nil
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("send clickhouse batch: %w", err)
	}
	return nil
}

// InsertNodeRequestReport writes a single request report via the batch API.
func InsertNodeRequestReport(ctx context.Context, report analyticsmodel.NodeRequestReport) error {
	if strings.TrimSpace(report.NodeID) == "" {
		return nil
	}
	return BatchInsertNodeRequestReports(ctx, []analyticsmodel.NodeRequestReport{report})
}

// BatchInsertNodeRequestReports writes request reports to ClickHouse.
func BatchInsertNodeRequestReports(ctx context.Context, reports []analyticsmodel.NodeRequestReport) error {
	if len(reports) == 0 {
		return nil
	}
	if db.ChConn == nil {
		return fmt.Errorf("clickhouse connection is not initialized")
	}

	batch, err := db.ChConn.PrepareBatch(ctx, analyticsmodel.NodeRequestReport{}.BatchInsertSQL())
	if err != nil {
		return fmt.Errorf("prepare clickhouse batch: %w", err)
	}

	now := time.Now().UTC()
	for _, report := range reports {
		nodeID := strings.TrimSpace(report.NodeID)
		if nodeID == "" {
			continue
		}
		id := report.ID
		if id == 0 {
			id = idgen.NextUint64ID()
		}
		createdAt := report.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		if err := batch.Append(
			id,
			nodeID,
			report.WindowStartedAt.UTC(),
			report.WindowEndedAt.UTC(),
			report.RequestCount,
			report.ErrorCount,
			report.UniqueVisitorCount,
			report.StatusCodesJSON,
			report.TopDomainsJSON,
			report.SourceCountriesJSON,
			createdAt.UTC(),
		); err != nil {
			return fmt.Errorf("append node request report to batch: %w", err)
		}
	}

	if batch.Rows() == 0 {
		return nil
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("send clickhouse batch: %w", err)
	}
	return nil
}

// InsertNodeObsOpenresty writes a single OpenResty observation via the batch API.
func InsertNodeObsOpenresty(ctx context.Context, obs analyticsmodel.NodeObsOpenresty) error {
	if strings.TrimSpace(obs.NodeID) == "" {
		return nil
	}
	return BatchInsertNodeObsOpenresty(ctx, []analyticsmodel.NodeObsOpenresty{obs})
}

// BatchInsertNodeObsOpenresty writes OpenResty observations to ClickHouse.
func BatchInsertNodeObsOpenresty(ctx context.Context, observations []analyticsmodel.NodeObsOpenresty) error {
	if len(observations) == 0 {
		return nil
	}
	if db.ChConn == nil {
		return fmt.Errorf("clickhouse connection is not initialized")
	}

	batch, err := db.ChConn.PrepareBatch(ctx, analyticsmodel.NodeObsOpenresty{}.BatchInsertSQL())
	if err != nil {
		return fmt.Errorf("prepare clickhouse batch: %w", err)
	}

	now := time.Now().UTC()
	for _, obs := range observations {
		nodeID := strings.TrimSpace(obs.NodeID)
		if nodeID == "" {
			continue
		}
		id := obs.ID
		if id == 0 {
			id = idgen.NextUint64ID()
		}
		createdAt := obs.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		capturedAt := obs.CapturedAt.UTC()
		if capturedAt.IsZero() {
			capturedAt = now
		}
		if err := batch.Append(
			id,
			nodeID,
			capturedAt,
			obs.OpenrestyRxBytes,
			obs.OpenrestyTxBytes,
			obs.OpenrestyConnections,
			createdAt.UTC(),
		); err != nil {
			return fmt.Errorf("append node openresty observation to batch: %w", err)
		}
	}

	if batch.Rows() == 0 {
		return nil
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("send clickhouse batch: %w", err)
	}
	return nil
}

// InsertNodeObsFrps writes a single FRPS observation via the batch API.
func InsertNodeObsFrps(ctx context.Context, obs analyticsmodel.NodeObsFrps) error {
	if strings.TrimSpace(obs.NodeID) == "" {
		return nil
	}
	return BatchInsertNodeObsFrps(ctx, []analyticsmodel.NodeObsFrps{obs})
}

// BatchInsertNodeObsFrps writes FRPS observations to ClickHouse.
func BatchInsertNodeObsFrps(ctx context.Context, observations []analyticsmodel.NodeObsFrps) error {
	if len(observations) == 0 {
		return nil
	}
	if db.ChConn == nil {
		return fmt.Errorf("clickhouse connection is not initialized")
	}

	batch, err := db.ChConn.PrepareBatch(ctx, analyticsmodel.NodeObsFrps{}.BatchInsertSQL())
	if err != nil {
		return fmt.Errorf("prepare clickhouse batch: %w", err)
	}

	now := time.Now().UTC()
	for _, obs := range observations {
		nodeID := strings.TrimSpace(obs.NodeID)
		if nodeID == "" {
			continue
		}
		id := obs.ID
		if id == 0 {
			id = idgen.NextUint64ID()
		}
		createdAt := obs.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		capturedAt := obs.CapturedAt.UTC()
		if capturedAt.IsZero() {
			capturedAt = now
		}
		if err := batch.Append(
			id,
			nodeID,
			capturedAt,
			obs.FrpsConnections,
			obs.FrpsProxyCount,
			obs.FrpsClientCount,
			obs.FrpsProxies,
			createdAt.UTC(),
		); err != nil {
			return fmt.Errorf("append node frps observation to batch: %w", err)
		}
	}

	if batch.Rows() == 0 {
		return nil
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("send clickhouse batch: %w", err)
	}
	return nil
}

// InsertNodeObsFrpc writes a single FRPC observation via the batch API.
func InsertNodeObsFrpc(ctx context.Context, obs analyticsmodel.NodeObsFrpc) error {
	if strings.TrimSpace(obs.NodeID) == "" {
		return nil
	}
	return BatchInsertNodeObsFrpc(ctx, []analyticsmodel.NodeObsFrpc{obs})
}

// BatchInsertNodeObsFrpc writes FRPC observations to ClickHouse.
func BatchInsertNodeObsFrpc(ctx context.Context, observations []analyticsmodel.NodeObsFrpc) error {
	if len(observations) == 0 {
		return nil
	}
	if db.ChConn == nil {
		return fmt.Errorf("clickhouse connection is not initialized")
	}

	batch, err := db.ChConn.PrepareBatch(ctx, analyticsmodel.NodeObsFrpc{}.BatchInsertSQL())
	if err != nil {
		return fmt.Errorf("prepare clickhouse batch: %w", err)
	}

	now := time.Now().UTC()
	for _, obs := range observations {
		nodeID := strings.TrimSpace(obs.NodeID)
		if nodeID == "" {
			continue
		}
		id := obs.ID
		if id == 0 {
			id = idgen.NextUint64ID()
		}
		createdAt := obs.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		capturedAt := obs.CapturedAt.UTC()
		if capturedAt.IsZero() {
			capturedAt = now
		}
		if err := batch.Append(
			id,
			nodeID,
			capturedAt,
			obs.TunnelStatus,
			obs.ConnectedRelaysCount,
			createdAt.UTC(),
		); err != nil {
			return fmt.Errorf("append node frpc observation to batch: %w", err)
		}
	}

	if batch.Rows() == 0 {
		return nil
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("send clickhouse batch: %w", err)
	}
	return nil
}
