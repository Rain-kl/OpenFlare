// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

const (
	errUnknownTaskType            = "未知的任务类型: %s"
	errCreateTaskExecutionFailed  = "创建任务执行记录失败: %w"
	errTaskEnqueueFailed          = "任务入队失败: %w"
	errTaskExecutionNotFound      = "任务执行记录不存在: %w"
	errRetryOnlyFailedTask        = "只有失败的任务才能重试，当前状态: %s"
	errTaskNotRetryable           = "该任务不支持重试"
	errTaskMaxRetryExceeded       = "已达到最大重试次数 %d"
	errCreateRetryExecutionFailed = "创建重试任务执行记录失败: %w"
	errRetryTaskEnqueueFailed     = "重试任务入队失败: %w"
	errUnregisteredTaskHandler    = "未注册的任务处理器: %s"
)
