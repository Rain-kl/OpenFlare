// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_inproc_worker

import "time"

type taskExecutionStatus string

const (
	taskExecutionStatusPending   taskExecutionStatus = "pending"
	taskExecutionStatusRunning   taskExecutionStatus = "running"
	taskExecutionStatusSucceeded taskExecutionStatus = "succeeded"
	taskExecutionStatusFailed    taskExecutionStatus = "failed"

	taskExecutionColStatus = "status"
)

// taskExecution maps to the admin-owned w_task_executions table so the
// console can list in-process runs the same way it lists Asynq runs.
type taskExecution struct {
	ID           uint64              `gorm:"primaryKey"`
	TaskID       string              `gorm:"size:128;uniqueIndex;not null"`
	TaskType     string              `gorm:"size:64;index;not null"`
	TaskName     string              `gorm:"size:128"`
	Status       taskExecutionStatus `gorm:"size:32;index;not null"`
	Retryable    bool                `gorm:"not null;default:false"`
	MaxRetry     int                 `gorm:"not null;default:0"`
	RetryCount   int                 `gorm:"not null;default:0"`
	Log          string              `gorm:"type:text"`
	ErrorMessage string              `gorm:"type:text"`
	Result       string              `gorm:"type:text"`
	StartedAt    *time.Time          `gorm:"index"`
	FinishedAt   *time.Time
	Duration     int64  `gorm:"comment:耗时毫秒"`
	Payload      string `gorm:"type:text"`
	TriggeredBy  string `gorm:"size:32;not null;default:system"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (taskExecution) TableName() string {
	return "w_task_executions"
}
