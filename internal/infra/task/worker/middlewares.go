// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package worker 提供 Asynq 任务处理服务器与中间件
package worker

import (
	"context"

	"github.com/hibiken/asynq"
)

// taskLoggingMiddleware 任务处理中间件
// 注意：OTel Span 创建、日志记录、TaskExecution 状态管理
// 已由 task.ProcessTask 统一处理，此中间件保留用于未来扩展（如限流、监控等）
func taskLoggingMiddleware(h asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		return h.ProcessTask(ctx, t)
	})
}
