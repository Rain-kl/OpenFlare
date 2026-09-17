// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_inproc_worker

import (
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"context"
	"errors"
	"fmt"
)

type inprocTaskService struct {
	taskReg extpoints.TaskExtension
}

func newInprocTaskService(taskReg extpoints.TaskExtension) contracts.TaskService {
	return &inprocTaskService{
		taskReg: taskReg,
	}
}

func (s *inprocTaskService) Dispatch(ctx context.Context, taskType string, payload []byte, triggeredBy string) (string, error) {
	return DispatchTask(ctx, taskType, payload, triggeredBy)
}

func (s *inprocTaskService) Retry(ctx context.Context, id uint64) (string, error) {
	db := getDB(ctx)
	if db == nil {
		return "", errors.New("driver_inproc_worker: db not initialized")
	}
	var exec taskExecution
	if err := db.Where("id = ?", id).First(&exec).Error; err != nil {
		return "", fmt.Errorf("driver_inproc_worker: task execution not found: %w", err)
	}
	if exec.Status != taskExecutionStatusFailed {
		return "", fmt.Errorf("driver_inproc_worker: only failed tasks can be retried, current status: %s", exec.Status)
	}
	if !exec.Retryable {
		return "", errors.New("driver_inproc_worker: task is not retryable")
	}
	return DispatchTask(ctx, exec.TaskType, []byte(exec.Payload), "retry")
}

func (s *inprocTaskService) ListTasks() []contracts.TaskMetaDTO {
	if s.taskReg == nil {
		return nil
	}
	tasks := s.taskReg.Tasks()
	res := make([]contracts.TaskMetaDTO, 0, len(tasks))
	for _, td := range tasks {
		res = append(res, td.ToDTO())
	}
	return res
}

func (s *inprocTaskService) GetTaskMeta(taskType string) (contracts.TaskMetaDTO, bool) {
	if s.taskReg == nil {
		return contracts.TaskMetaDTO{}, false
	}
	for _, td := range s.taskReg.Tasks() {
		if td.Type == taskType || td.Pattern == taskType {
			return td.ToDTO(), true
		}
	}
	return contracts.TaskMetaDTO{}, false
}

func (s *inprocTaskService) ValidatePayload(_ string, payload []byte) ([]byte, error) {
	return payload, nil
}

func (s *inprocTaskService) ReloadScheduler() error {
	return nil
}

func (s *inprocTaskService) AppendLog(_ context.Context, _ string, _ ...any) {
}

func (s *inprocTaskService) ListExecutions(ctx context.Context, taskType, status string, page, pageSize int) ([]contracts.TaskExecutionDTO, int64, error) {
	db := getDB(ctx)
	if db == nil {
		return []contracts.TaskExecutionDTO{}, 0, nil
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	query := db.Model(&taskExecution{})
	if taskType != "" {
		query = query.Where("task_type = ?", taskType)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []taskExecution
	offset := (page - 1) * pageSize
	if err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	res := make([]contracts.TaskExecutionDTO, 0, len(rows))
	for i := range rows {
		res = append(res, toExecutionDTO(&rows[i]))
	}
	return res, total, nil
}

func findExecution(ctx context.Context, query any, args ...any) (*contracts.TaskExecutionDTO, error) {
	db := getDB(ctx)
	if db == nil {
		return nil, errors.New("driver_inproc_worker: db not initialized")
	}
	var exec taskExecution
	if err := db.Where(query, args...).First(&exec).Error; err != nil {
		return nil, err
	}
	dto := toExecutionDTO(&exec)
	return &dto, nil
}

func (s *inprocTaskService) GetExecution(ctx context.Context, id uint64) (*contracts.TaskExecutionDTO, error) {
	return findExecution(ctx, "id = ?", id)
}

func (s *inprocTaskService) GetExecutionByTaskID(ctx context.Context, taskID string) (*contracts.TaskExecutionDTO, error) {
	return findExecution(ctx, "task_id = ?", taskID)
}

func toExecutionDTO(exec *taskExecution) contracts.TaskExecutionDTO {
	return contracts.TaskExecutionDTO{
		ID:           exec.ID,
		TaskID:       exec.TaskID,
		TaskType:     exec.TaskType,
		TaskName:     exec.TaskName,
		Status:       string(exec.Status),
		Retryable:    exec.Retryable,
		MaxRetry:     exec.MaxRetry,
		RetryCount:   exec.RetryCount,
		Log:          exec.Log,
		ErrorMessage: exec.ErrorMessage,
		Result:       exec.Result,
		StartedAt:    exec.StartedAt,
		FinishedAt:   exec.FinishedAt,
		Duration:     exec.Duration,
		Payload:      exec.Payload,
		TriggeredBy:  exec.TriggeredBy,
		CreatedAt:    exec.CreatedAt,
		UpdatedAt:    exec.UpdatedAt,
	}
}
