// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package testhelper

import (
	"context"
	"fmt"
	"time"

	"Wavelet/core/contracts"
	"Wavelet/openflare/plugins/server/kernel/repository"
	"Wavelet/pkg/idgen"
)

// NoopTaskService is a contracts.TaskService that records dispatch and ignores the rest.
type NoopTaskService struct {
	LastType    string
	LastPayload []byte
}

var _ contracts.TaskService = (*NoopTaskService)(nil)

// Dispatch dispatches a task mock execution.
func (s *NoopTaskService) Dispatch(ctx context.Context, taskType string, payload []byte, triggeredBy string) (string, error) {
	s.LastType = taskType
	s.LastPayload = payload
	taskID := fmt.Sprintf("test-task-%d", time.Now().UnixNano())
	gdb := repository.DB(ctx)
	if gdb != nil {
		var id uint64
		func() {
			defer func() {
				if r := recover(); r != nil {
					id = uint64(time.Now().UnixNano())
				}
			}()
			id = idgen.NextUint64ID()
		}()
		_ = gdb.Table("w_task_executions").Create(&contracts.TaskExecutionDTO{
			ID:          id,
			TaskID:      taskID,
			TaskType:    taskType,
			Payload:     string(payload),
			TriggeredBy: triggeredBy,
			Status:      "pending",
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}).Error
	}
	return taskID, nil
}

// Retry retries a task mock execution.
func (s *NoopTaskService) Retry(context.Context, uint64) (string, error) { return "", nil }

// ListTasks lists task mock metadata.
func (s *NoopTaskService) ListTasks() []contracts.TaskMetaDTO { return nil }

// GetTaskMeta returns task mock metadata.
func (s *NoopTaskService) GetTaskMeta(string) (contracts.TaskMetaDTO, bool) {
	return contracts.TaskMetaDTO{}, false
}

// ValidatePayload validates task payload.
func (s *NoopTaskService) ValidatePayload(_ string, payload []byte) ([]byte, error) {
	return payload, nil
}

// ReloadScheduler reloads task scheduler.
func (s *NoopTaskService) ReloadScheduler() error { return nil }

// AppendLog appends log message.
func (s *NoopTaskService) AppendLog(context.Context, string, ...any) {}

// ListExecutions lists task executions.
func (s *NoopTaskService) ListExecutions(context.Context, string, string, int, int) ([]contracts.TaskExecutionDTO, int64, error) {
	return nil, 0, nil
}

// GetExecution gets task execution by ID.
func (s *NoopTaskService) GetExecution(context.Context, uint64) (*contracts.TaskExecutionDTO, error) {
	return &contracts.TaskExecutionDTO{TaskID: "test-task"}, nil
}

// GetExecutionByTaskID gets task execution by taskID.
func (s *NoopTaskService) GetExecutionByTaskID(ctx context.Context, taskID string) (*contracts.TaskExecutionDTO, error) {
	gdb := repository.DB(ctx)
	if gdb != nil {
		var exec contracts.TaskExecutionDTO
		if err := gdb.Table("w_task_executions").Where("task_id = ?", taskID).First(&exec).Error; err == nil {
			return &exec, nil
		}
	}
	return &contracts.TaskExecutionDTO{ID: 1, TaskID: taskID, Payload: string(s.LastPayload)}, nil
}
