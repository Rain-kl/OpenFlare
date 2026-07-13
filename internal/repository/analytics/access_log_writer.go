// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"fmt"

	"github.com/Rain-kl/Wavelet/internal/db"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

// BatchInsert writes access logs to ClickHouse using the native batch API.
func BatchInsert(ctx context.Context, logs []analyticsmodel.UserAccessLog) error {
	if len(logs) == 0 {
		return nil
	}
	if db.ChConn == nil {
		return fmt.Errorf("clickhouse connection is not initialized")
	}

	batch, err := db.ChConn.PrepareBatch(ctx, analyticsmodel.UserAccessLog{}.BatchInsertSQL())
	if err != nil {
		return fmt.Errorf("prepare clickhouse batch: %w", err)
	}

	for _, logItem := range logs {
		if err := batch.Append(
			logItem.ID,
			logItem.UserID,
			logItem.Path,
			logItem.Method,
			logItem.IP,
			logItem.UserAgent,
			logItem.Headers,
			logItem.Status,
			logItem.Latency,
			logItem.CreatedAt,
		); err != nil {
			return fmt.Errorf("append access log to batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("send clickhouse batch: %w", err)
	}
	return nil
}
