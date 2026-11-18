// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/infra/persistence/idgen"
	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

// InsertBatch 节点访问日志写入入口：冻结检查后经 hook 入队（异步），不直接落库。
func (s *clickhouseLogStore) InsertBatch(ctx context.Context, records []*model.OpenFlareAccessLog) error {
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	rows := make([]analyticsmodel.NodeAccessLog, 0, len(records))
	for _, r := range records {
		if r == nil {
			continue
		}
		rows = append(rows, toAnalyticsNodeAccessLog(r))
	}
	if h := currentAccessLogHooks().QueueNodeAccessLogs; h != nil {
		h(rows)
	}
	return nil
}

// BatchInsertNodeAccessLogs 是 batchwriter flush 目标：CH 原生批量写入。
func (s *clickhouseLogStore) BatchInsertNodeAccessLogs(ctx context.Context, logs []analyticsmodel.NodeAccessLog) error {
	if len(logs) == 0 {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	conn, err := chConn()
	if err != nil {
		return err
	}

	batch, err := conn.PrepareBatch(ctx, analyticsmodel.NodeAccessLog{}.BatchInsertSQL())
	if err != nil {
		return fmt.Errorf("prepare clickhouse batch: %w", err)
	}

	now := time.Now().UTC()
	for _, logItem := range logs {
		id := logItem.ID
		if id == 0 {
			id = idgen.NextUint64ID()
		}
		createdAt := logItem.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		if err := batch.Append(
			id,
			logItem.NodeID,
			logItem.LoggedAt.UTC(),
			strings.TrimSpace(logItem.RemoteAddr),
			logItem.Region,
			logItem.Host,
			logItem.Path,
			strings.TrimSpace(logItem.UserAgent),
			strings.TrimSpace(logItem.CacheStatus),
			logItem.StatusCode,
			logItem.BytesSent,
			logItem.RequestLength,
			logItem.RequestTimeMs,
			createdAt.UTC(),
		); err != nil {
			return fmt.Errorf("append node access log to batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("send clickhouse batch: %w", err)
	}
	return nil
}
