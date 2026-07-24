// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package risk_control

import (
	"context"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/infra/config"
	"github.com/Rain-kl/Wavelet/internal/infra/persistence/batchwriter"
	"github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/Rain-kl/Wavelet/internal/platform/lifecycle"
	analyticsrepo "github.com/Rain-kl/Wavelet/internal/repository/analytics"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

const (
	// Bound visibility lag for sparse access-log traffic when MinBatchSize is not met.
	accessLogMaxFlushWait = 3 * time.Second
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
	cfg.Name = "user_access_logs"
	cfg.MaxFlushWait = accessLogMaxFlushWait
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
		batchwriter.WithFlushErrorHandler[*analytics.UserAccessLog](func(ctx context.Context, items []*analytics.UserAccessLog, err error) {
			logger.ErrorF(ctx, "[RiskControl] Send ClickHouse batch failed (batch=%d): %v", len(items), err)
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

// LogWriterStats returns queue depth and failure counters for the access-log writer.
// When the writer is not initialized, it returns a zero-value Stats with the expected name.
func LogWriterStats() batchwriter.Stats {
	writer := currentLogWriter()
	if writer == nil {
		return batchwriter.Stats{Name: "user_access_logs"}
	}
	return writer.Stats()
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
