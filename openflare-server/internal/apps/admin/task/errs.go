// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package task 提供任务管理接口
package task

// 任务管理相关错误消息
const (
	InvalidTaskType       = "无效的任务类型"
	InvalidTimeRange      = "无效的时间范围"
	TaskDispatchFailed    = "任务下发失败"
	UserIDRequired        = "用户ID必填"
	TaskNotFound          = "任务执行记录不存在"
	TaskNotRetryable      = "该任务不支持重试"
	TaskNotFailed         = "只有失败的任务才能重试"
	TaskMaxRetryExceeded  = "已达到最大重试次数"
	TaskRetryFailed       = "任务重试失败"
	InvalidCronExpression = "无效的 Cron 表达式"
	ScheduleNotFound      = "定时任务不存在"
	ScheduleSaveFailed    = "保存定时任务失败"
	ScheduleDeleteFailed  = "删除定时任务失败"
)
