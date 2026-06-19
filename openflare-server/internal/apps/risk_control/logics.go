// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package risk_control

import (
	"context"
	"time"

	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

var logChan chan *UserAccessLog

const (
	defaultQueueSize = 10000
	maxBatchSize     = 1000
	flushInterval    = 1 * time.Second
)

// InitLogWriter 初始化日志写入通道和后台写入协程
func InitLogWriter(ctx context.Context) {
	if !config.Config.ClickHouse.Enabled {
		return
	}

	logChan = make(chan *UserAccessLog, defaultQueueSize)
	go startBatchWorker(context.WithoutCancel(ctx))
}

// IsBufferFull 检查当前本地缓冲队列是否已满
// 如果没有启用 ClickHouse，默认返回 false，不触发限流
func IsBufferFull() bool {
	if !config.Config.ClickHouse.Enabled || logChan == nil {
		return false
	}
	return len(logChan) >= cap(logChan)
}

// QueueAccessLog 异步非阻塞地将日志推入缓冲队列
func QueueAccessLog(logItem *UserAccessLog) {
	if !config.Config.ClickHouse.Enabled || logChan == nil {
		return
	}

	select {
	case logChan <- logItem:
	default:
		logger.WarnF(context.Background(), "[RiskControl] Log queue full, dropping log item for path: %s", logItem.Path)
	}
}

func startBatchWorker(ctx context.Context) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	var batch []*UserAccessLog

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if db.ChConn == nil {
			batch = nil
			return
		}

		b, err := db.ChConn.PrepareBatch(ctx, "INSERT INTO w_user_access_logs (id, user_id, path, method, ip, user_agent, headers, status, latency, created_at)")
		if err != nil {
			logger.ErrorF(ctx, "[RiskControl] Prepare ClickHouse batch failed: %v", err)
			batch = nil
			return
		}

		for _, item := range batch {
			err = b.Append(
				item.ID,
				item.UserID,
				item.Path,
				item.Method,
				item.IP,
				item.UserAgent,
				item.Headers,
				item.Status,
				item.Latency,
				item.CreatedAt,
			)
			if err != nil {
				logger.ErrorF(ctx, "[RiskControl] Append item to ClickHouse batch failed: %v", err)
			}
		}

		if err := b.Send(); err != nil {
			logger.ErrorF(ctx, "[RiskControl] Send ClickHouse batch failed: %v", err)
		}
		batch = nil
	}

	for {
		select {
		case item, ok := <-logChan:
			if !ok {
				flush()
				return
			}
			batch = append(batch, item)
			if len(batch) >= maxBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
