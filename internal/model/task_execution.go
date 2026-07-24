// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"time"
)

// TaskExecutionStatus 任务执行状态
type TaskExecutionStatus string

// 任务执行状态
const (
	TaskExecutionStatusPending   TaskExecutionStatus = "pending"
	TaskExecutionStatusRunning   TaskExecutionStatus = "running"
	TaskExecutionStatusSucceeded TaskExecutionStatus = "succeeded"
	TaskExecutionStatusFailed    TaskExecutionStatus = "failed"
)

// TaskExecution 任务执行记录
type TaskExecution struct {
	ID           uint64              `json:"id,string" gorm:"primaryKey"`
	TaskID       string              `json:"task_id" gorm:"size:128;uniqueIndex;not null"`
	TaskType     string              `json:"task_type" gorm:"size:64;index;not null"`
	TaskName     string              `json:"task_name" gorm:"size:128"`
	Status       TaskExecutionStatus `json:"status" gorm:"size:32;index;not null"`
	Retryable    bool                `json:"retryable" gorm:"not null;default:false"`
	MaxRetry     int                 `json:"max_retry" gorm:"not null;default:0"`
	RetryCount   int                 `json:"retry_count" gorm:"not null;default:0"`
	Log          string              `json:"log" gorm:"type:text"`
	ErrorMessage string              `json:"error_message" gorm:"type:text"`
	Result       string              `json:"result" gorm:"type:text"`
	StartedAt    *time.Time          `json:"started_at" gorm:"index"`
	FinishedAt   *time.Time          `json:"finished_at"`
	Duration     int64               `json:"duration" gorm:"comment:耗时毫秒"`
	Payload      string              `json:"payload" gorm:"type:text"`
	TriggeredBy  string              `json:"triggered_by" gorm:"size:32;not null;default:system"`
	CreatedAt    time.Time           `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt    time.Time           `json:"updated_at" gorm:"autoUpdateTime"`
}

// TaskExecutionCleanupStats describes task execution log cleanup results.
type TaskExecutionCleanupStats struct {
	HighFrequencyDeleted int64
	LowFrequencyDeleted  int64
}

// TableName 表名
func (TaskExecution) TableName() string {
	return "w_task_executions"
}

// ListTaskExecutionsRequest 查询任务执行记录列表请求
type ListTaskExecutionsRequest struct {
	Status   string `form:"status"`
	TaskType string `form:"task_type"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}
