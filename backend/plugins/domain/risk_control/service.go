// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package risk_control

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/batchwriter"
	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/risk_control/logstore"
	"context"
	"sync"
	"time"
)

const (
	// fallbackLogEngine 是日志库状态不可得时对外暴露的引擎标识。
	fallbackLogEngine = "sqlite"
	// accessLogMaxFlushWait 强制把不足 MinBatchSize 的访问日志刷盘，避免管理台低频访问永远看不到记录。
	accessLogMaxFlushWait = 2 * time.Second
)

var (
	logWriterMu sync.RWMutex
	logWriter   *batchwriter.Writer[*logstore.UserAccessLog]
)

// InitLogWriter initializes the access-log batch writer for the active log database.
func InitLogWriter(ctx context.Context) {
	logWriterMu.Lock()
	defer logWriterMu.Unlock()
	if logWriter != nil {
		return
	}

	cfg := batchwriter.DefaultConfig()
	cfg.MaxFlushWait = accessLogMaxFlushWait
	writer, err := batchwriter.New[*logstore.UserAccessLog](cfg, writeAccessLogBatch,
		batchwriter.WithDropHandler[*logstore.UserAccessLog](func(item *logstore.UserAccessLog) {
			path := ""
			if item != nil {
				path = item.Path
			}
			logger.WarnF(context.Background(), "[RiskControl] Log queue full, dropping log item for path: %s", path)
		}),
		batchwriter.WithFlushErrorHandler[*logstore.UserAccessLog](func(ctx context.Context, items []*logstore.UserAccessLog, err error) {
			logger.ErrorF(ctx, "[RiskControl] flush access-log batch failed (batch=%d): %v", len(items), err)
		}),
	)
	if err != nil {
		logger.ErrorF(ctx, "[RiskControl] init log writer failed: %v", err)
		return
	}

	writer.Start(ctx)
	logWriter = writer
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
func QueueAccessLog(logItem *logstore.UserAccessLog) {
	writer := currentLogWriter()
	if writer == nil || logItem == nil {
		return
	}
	writer.TryEnqueue(logItem)
}

// SetLogWriterForTest swaps the access-log writer for unit tests.
func SetLogWriterForTest(writer *batchwriter.Writer[*logstore.UserAccessLog]) func() {
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

func currentLogWriter() *batchwriter.Writer[*logstore.UserAccessLog] {
	logWriterMu.RLock()
	defer logWriterMu.RUnlock()
	return logWriter
}

const drainPollInterval = 50 * time.Millisecond

// Drain waits until the in-memory access-log queue has been empty for one flush interval.
func Drain(ctx context.Context) error {
	writer := currentLogWriter()
	if writer == nil {
		return nil
	}
	quietPeriod := batchwriter.DefaultConfig().FlushInterval
	if quietPeriod <= 0 {
		quietPeriod = time.Second
	}
	ticker := time.NewTicker(drainPollInterval)
	defer ticker.Stop()
	var quietSince time.Time
	for {
		if writer.Len() == 0 {
			if quietSince.IsZero() {
				quietSince = time.Now()
			} else if time.Since(quietSince) >= quietPeriod {
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

// MigrateAndSwitchEngine migrates access logs to target database and switches the active store.
func MigrateAndSwitchEngine(ctx context.Context, targetEngine string, reportProgress func(copied int)) error {
	if err := Drain(ctx); err != nil {
		return err
	}
	src, dst, err := loadMigrationStores(ctx, targetEngine)
	if err != nil {
		return err
	}
	if err := copyAccessLogs(ctx, src, dst, reportProgress); err != nil {
		return err
	}
	resetLogStoreCache()
	return nil
}

// riskControlServiceImpl implements contracts.RiskControlService by orchestrating
// the repository layer and mapping persistence rows into contract DTOs.
type riskControlServiceImpl struct{}

func (s *riskControlServiceImpl) QueryAccessLogs(ctx context.Context, filter contracts.AccessLogFilterDTO, page, pageSize int) ([]contracts.AccessLogDTO, uint64, error) {
	list, total, err := listAccessLogs(ctx, logstore.AccessLogFilter{
		UserIDs:   filter.UserIDs,
		Path:      filter.Path,
		StartTime: filter.StartTime,
		EndTime:   filter.EndTime,
	}, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	items := make([]contracts.AccessLogDTO, len(list))
	for i, item := range list {
		items[i] = contracts.AccessLogDTO{
			ID:        item.ID,
			UserID:    item.UserID,
			IP:        item.IP,
			UserAgent: item.UserAgent,
			Method:    item.Method,
			Path:      item.Path,
			Status:    item.Status,
			Latency:   item.Latency,
			CreatedAt: item.CreatedAt,
		}
	}
	return items, total, nil
}

func (s *riskControlServiceImpl) QueryAccessLogStats(ctx context.Context, days int) ([]contracts.AccessLogDailyStatsDTO, error) {
	trend, err := accessLogDailyTrend(ctx, days)
	if err != nil {
		return nil, err
	}
	res := make([]contracts.AccessLogDailyStatsDTO, len(trend))
	for i, t := range trend {
		res[i] = contracts.AccessLogDailyStatsDTO{
			Date: t.Date,
			PV:   t.Count,
		}
	}
	return res, nil
}

func (s *riskControlServiceImpl) ActiveLogEngine(ctx context.Context) string {
	engine, err := activeLogDatabase(ctx)
	if err != nil {
		return fallbackLogEngine
	}
	return engine
}

func (s *riskControlServiceImpl) IsLogEngineMigrating(ctx context.Context) bool {
	return logStoreMigrating(ctx)
}

func (s *riskControlServiceImpl) Drain(ctx context.Context) error {
	return Drain(ctx)
}

func (s *riskControlServiceImpl) SwitchLogEngine(ctx context.Context, targetEngine string) error {
	return MigrateAndSwitchEngine(ctx, targetEngine, nil)
}
