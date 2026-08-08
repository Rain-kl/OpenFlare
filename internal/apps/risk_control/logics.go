// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package risk_control

import (
	"context"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/infra/persistence/batchwriter"
	"github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/Rain-kl/Wavelet/internal/platform/lifecycle"
	"github.com/Rain-kl/Wavelet/internal/repository/logstore"
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

// InitLogWriter initializes the user access-log batch writer.
// The active log store is resolved via logstore at flush time, so the writer
// runs for PG/SQLite as well as ClickHouse.
func InitLogWriter(ctx context.Context) {
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
		s, err := logstore.Active(ctx)
		if err != nil {
			return err
		}
		return s.UserAccessLogs.BatchInsert(ctx, rows)
	},
		batchwriter.WithDropHandler[*analytics.UserAccessLog](func(item *analytics.UserAccessLog) {
			path := ""
			if item != nil {
				path = item.Path
			}
			logger.WarnF(context.Background(), "[RiskControl] Log queue full, dropping log item for path: %s", path)
		}),
		batchwriter.WithFlushErrorHandler[*analytics.UserAccessLog](func(ctx context.Context, items []*analytics.UserAccessLog, err error) {
			logger.ErrorF(ctx, "[RiskControl] Send log batch failed (batch=%d): %v", len(items), err)
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

// StopLogWriter stops the user access-log batch writer and drains pending logs.
func StopLogWriter(ctx context.Context) error {
	writer := currentLogWriter()
	if writer == nil {
		return nil
	}
	return writer.Stop(ctx)
}

// DrainLogWriter 等待用户访问日志 writer 的在途批次落库：队列 Depth 归零后
// 再保持一个 flush 周期（1s）持续为空才返回；不停止 writer（迁移冻结后由
// ensureWritable 拒绝新写入）。writer 未初始化时直接返回 nil。
func DrainLogWriter(ctx context.Context) error {
	writer := currentLogWriter()
	if writer == nil {
		return nil
	}
	ticker := time.NewTicker(drainPollInterval)
	defer ticker.Stop()
	var quietSince time.Time
	for {
		if writer.Stats().Depth == 0 {
			if quietSince.IsZero() {
				quietSince = time.Now()
			} else if time.Since(quietSince) >= batchwriter.DefaultConfig().FlushInterval {
				return nil
			}
		} else {
			quietSince = time.Time{}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// drainPollInterval 用户访问日志队列轮询间隔。
const drainPollInterval = 50 * time.Millisecond

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
