// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package task 提供任务管理接口
package task

// 任务管理相关错误消息
const (
	InvalidTaskType       = "Invalid task type"
	InvalidTimeRange      = "Invalid time range"
	TaskDispatchFailed    = "Failed to dispatch task"
	UserIDRequired        = "User ID is required"
	TaskNotFound          = "Task execution was not found"
	TaskNotRetryable      = "this task cannot be retried"
	TaskNotFailed         = "Only failed tasks can be retried"
	TaskMaxRetryExceeded  = "Maximum retry count reached"
	TaskRetryFailed       = "Failed to retry task"
	InvalidCronExpression = "Invalid Cron expression"
	ScheduleNotFound      = "Schedule was not found"
	ScheduleSaveFailed    = "Failed to save schedule"
	ScheduleDeleteFailed  = "Failed to delete schedule"
)
