// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"fmt"
	"strings"
	"time"

	analyticsmodel "Wavelet/openflare/plugins/server/kernel/model/analytics"
	"Wavelet/pkg/idgen"
)

// BatchInsertNodeAccessLogs writes node access logs to ClickHouse using the native batch API.
func BatchInsertNodeAccessLogs(ctx context.Context, logs []analyticsmodel.NodeAccessLog) error {
	if len(logs) == 0 {
		return nil
	}
	conn, err := ChConn(ctx)
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
