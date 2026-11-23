// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/Rain-kl/Wavelet/internal/infra/persistence/idgen"
	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

// InsertMetricSnapshot 写入入口：冻结检查 + 经 hook 入队（异步），不直接落库。
func (s *clickhouseLogStore) InsertMetricSnapshot(ctx context.Context, record *model.OpenFlareMetricSnapshot) error {
	if record == nil {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	if h := currentObservabilityHooks().QueueMetricSnapshot; h != nil {
		h(toAnalyticsNodeMetricSnapshot(record))
	}
	return nil
}

// BatchInsertNodeMetricSnapshots 是 batchwriter flush 目标：CH 原生批量写入。
func (s *clickhouseLogStore) BatchInsertNodeMetricSnapshots(ctx context.Context, snapshots []analyticsmodel.NodeMetricSnapshot) error {
	return chBatchInsert(ctx, s, analyticsmodel.NodeMetricSnapshot{}.BatchInsertSQL(), len(snapshots), func(batch driver.Batch, now time.Time) error {
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
		return nil
	})
}

// InsertEdgeHealth 写入入口：冻结检查 + 经 hook 入队（异步），不直接落库。
func (s *clickhouseLogStore) InsertEdgeHealth(ctx context.Context, record *model.OpenFlareEdgeHealth) error {
	if record == nil {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	if h := currentObservabilityHooks().QueueEdgeHealth; h != nil {
		h(toAnalyticsNodeEdgeHealth(record))
	}
	return nil
}

// BatchInsertNodeEdgeHealth 是 batchwriter flush 目标：CH 原生批量写入。
func (s *clickhouseLogStore) BatchInsertNodeEdgeHealth(ctx context.Context, rows []analyticsmodel.NodeEdgeHealth) error {
	return chBatchInsert(ctx, s, analyticsmodel.NodeEdgeHealth{}.BatchInsertSQL(), len(rows), func(batch driver.Batch, now time.Time) error {
		for _, row := range rows {
			nodeID := strings.TrimSpace(row.NodeID)
			if nodeID == "" {
				continue
			}
			id := row.ID
			if id == 0 {
				id = idgen.NextUint64ID()
			}
			createdAt := row.CreatedAt
			if createdAt.IsZero() {
				createdAt = now
			}
			capturedAt := row.CapturedAt.UTC()
			if capturedAt.IsZero() {
				capturedAt = now
			}
			if err := batch.Append(
				id,
				nodeID,
				capturedAt,
				normalizeEdgeHealthStatus(row.Status),
				row.Connections,
				createdAt.UTC(),
			); err != nil {
				return fmt.Errorf("append node edge health to batch: %w", err)
			}
		}
		return nil
	})
}

func chBatchInsert(ctx context.Context, s *clickhouseLogStore, batchSQL string, count int, appendFn func(batch driver.Batch, now time.Time) error) error {
	if count == 0 {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	conn, err := chConn()
	if err != nil {
		return err
	}
	batch, err := conn.PrepareBatch(ctx, batchSQL)
	if err != nil {
		return fmt.Errorf("prepare clickhouse batch: %w", err)
	}
	if err := appendFn(batch, time.Now().UTC()); err != nil {
		return err
	}
	if batch.Rows() == 0 {
		return nil
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("send clickhouse batch: %w", err)
	}
	return nil
}
