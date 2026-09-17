// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package contracts defines unified service interfaces and DTOs for cross-plugin communication.
package contracts

import (
	"context"
	"time"
)

// TaskParamDTO describes a parameter accepted by a background task.
type TaskParamDTO struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Placeholder string `json:"placeholder,omitempty"`
	Description string `json:"description,omitempty"`
	Default     any    `json:"default,omitempty"`
}

// TaskMetaDTO describes the metadata and configuration of a registered background task.
type TaskMetaDTO struct {
	Type         string         `json:"type"`
	AsynqTask    string         `json:"asynq_task"`
	Name         string         `json:"name"`
	DisplayName  string         `json:"display_name,omitempty"`
	Description  string         `json:"description"`
	Category     string         `json:"category,omitempty"`
	SupportsTime bool           `json:"supports_time"`
	Params       []TaskParamDTO `json:"params,omitempty"`
	MaxRetry     int            `json:"max_retry"`
	Timeout      time.Duration  `json:"timeout,omitempty"`
	Queue        string         `json:"queue"`
	Retryable    bool           `json:"retryable"`
	Schedule     string         `json:"schedule,omitempty"`
}

// TaskResultDTO represents the outcome of a background task execution.
type TaskResultDTO struct {
	Message string `json:"message"`
	Detail  any    `json:"detail,omitempty"`
}

// TaskHandler is the preferred background task handler. Drivers invoke Execute
// and persist Message/Detail onto the execution record.
type TaskHandler interface {
	Execute(ctx context.Context, payload []byte) (*TaskResultDTO, error)
}

// TaskExecutionDTO represents a single task execution record.
type TaskExecutionDTO struct {
	ID           uint64     `json:"id,string"`
	TaskID       string     `json:"task_id"`
	TaskType     string     `json:"task_type"`
	TaskName     string     `json:"task_name"`
	Status       string     `json:"status"`
	Retryable    bool       `json:"retryable"`
	MaxRetry     int        `json:"max_retry"`
	RetryCount   int        `json:"retry_count"`
	Log          string     `json:"log"`
	ErrorMessage string     `json:"error_message"`
	Result       string     `json:"result"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	Duration     int64      `json:"duration"`
	Payload      string     `json:"payload"`
	TriggeredBy  string     `json:"triggered_by"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Canonical triggered_by values persisted on task executions and shown in admin UI.
const (
	TaskTriggerSystem   = "system"
	TaskTriggerManual   = "manual"
	TaskTriggerRetry    = "retry"
	TaskTriggerSchedule = "schedule"
)

// TaskService defines the unified contract for dispatching and tracking background tasks.
type TaskService interface {
	Dispatch(ctx context.Context, taskType string, payload []byte, triggeredBy string) (string, error)
	Retry(ctx context.Context, id uint64) (string, error)
	ListTasks() []TaskMetaDTO
	GetTaskMeta(taskType string) (TaskMetaDTO, bool)
	ValidatePayload(taskType string, payload []byte) ([]byte, error)
	ReloadScheduler() error
	AppendLog(ctx context.Context, format string, args ...any)
	ListExecutions(ctx context.Context, taskType, status string, page, pageSize int) ([]TaskExecutionDTO, int64, error)
	GetExecution(ctx context.Context, id uint64) (*TaskExecutionDTO, error)
	GetExecutionByTaskID(ctx context.Context, taskID string) (*TaskExecutionDTO, error)
}
