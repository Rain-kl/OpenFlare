// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package risk_control

import (
	"context"
	"sync"

	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/db/batchwriter"
	"github.com/Rain-kl/Wavelet/internal/lifecycle"
	"github.com/Rain-kl/Wavelet/internal/model/analytics"
	analyticsrepo "github.com/Rain-kl/Wavelet/internal/repository/analytics"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

var (
	logWriterMu sync.RWMutex
	logWriter   *batchwriter.Writer[*analytics.UserAccessLog]
)

// InitLogWriter initializes the ClickHouse access-log batch writer.
func InitLogWriter(ctx context.Context) {
	if !config.Config.ClickHouse.Enabled {
		return
	}

	logWriterMu.Lock()
	defer logWriterMu.Unlock()
	if logWriter != nil {
		return
	}

	cfg := batchwriter.DefaultConfig()
	writer, err := batchwriter.New[*analytics.UserAccessLog](cfg, func(ctx context.Context, items []*analytics.UserAccessLog) error {
		rows := make([]analytics.UserAccessLog, 0, len(items))
		for _, item := range items {
			if item == nil {
				continue
			}
			rows = append(rows, *item)
		}
		return analyticsrepo.BatchInsert(ctx, rows)
	},
		batchwriter.WithDropHandler[*analytics.UserAccessLog](func(item *analytics.UserAccessLog) {
			path := ""
			if item != nil {
				path = item.Path
			}
			logger.WarnF(context.Background(), "[RiskControl] Log queue full, dropping log item for path: %s", path)
		}),
		batchwriter.WithFlushErrorHandler[*analytics.UserAccessLog](func(ctx context.Context, batchSize int, err error) {
			logger.ErrorF(ctx, "[RiskControl] Send ClickHouse batch failed (batch=%d): %v", batchSize, err)
		}),
	)
	if err != nil {
		logger.ErrorF(ctx, "[RiskControl] init log writer failed: %v", err)
		return
	}

	writer.Start(ctx)
	logWriter = writer
	lifecycle.OnShutdown("risk_control_log_writer", StopLogWriter)
}

// StopLogWriter stops the ClickHouse access-log batch writer and drains pending logs.
func StopLogWriter(ctx context.Context) error {
	writer := currentLogWriter()
	if writer == nil {
		return nil
	}
	return writer.Stop(ctx)
}

// IsBufferFull reports whether the access-log queue has no remaining capacity.
func IsBufferFull() bool {
	writer := currentLogWriter()
	if writer == nil {
		return false
	}
	return writer.IsFull()
}

// QueueAccessLog enqueues an access log without blocking.
func QueueAccessLog(logItem *analytics.UserAccessLog) {
	writer := currentLogWriter()
	if writer == nil || logItem == nil {
		return
	}
	writer.TryEnqueue(logItem)
}

// SetLogWriterForTest swaps the access-log writer for unit tests.
func SetLogWriterForTest(writer *batchwriter.Writer[*analytics.UserAccessLog]) func() {
	logWriterMu.Lock()
	previous := logWriter
	logWriter = writer
	logWriterMu.Unlock()
	return func() {
		logWriterMu.Lock()
		logWriter = previous
		logWriterMu.Unlock()
	}
}

func currentLogWriter() *batchwriter.Writer[*analytics.UserAccessLog] {
	logWriterMu.RLock()
	defer logWriterMu.RUnlock()
	return logWriter
}