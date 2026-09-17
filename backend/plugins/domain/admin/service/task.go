// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/admin/errs"
	"Wavelet/plugins/domain/admin/model"
	"Wavelet/plugins/domain/admin/repository"
	"context"
	"fmt"
	"strings"

	"github.com/robfig/cron/v3"
)

// ListTaskTypes returns every dispatchable task type declared in the task registry.
func ListTaskTypes(ctx context.Context) []contracts.TaskMetaDTO {
	taskSvc := GetTaskService(ctx)
	if taskSvc == nil {
		return []contracts.TaskMetaDTO{}
	}
	return taskSvc.ListTasks()
}

// DispatchTask validates and enqueues a manual task run, returning the new task id.
func DispatchTask(ctx context.Context, req model.DispatchTaskRequest) (string, error) {
	taskSvc, err := requireTaskService(ctx)
	if err != nil {
		return "", err
	}

	if _, ok := taskSvc.GetTaskMeta(req.TaskType); !ok {
		return "", errs.ErrInvalidTaskType
	}

	validated, err := validateTaskPayload(taskSvc, req.TaskType, req.Payload)
	if err != nil {
		return "", err
	}

	taskID, err := taskSvc.Dispatch(ctx, req.TaskType, validated, contracts.TaskTriggerManual)
	if err != nil {
		return "", fmt.Errorf("%s: %w", errs.TaskDispatchFailed, err)
	}
	return taskID, nil
}

// validateTaskPayload normalises an optional raw payload through the task registry.
func validateTaskPayload(taskSvc contracts.TaskService, name, payload string) ([]byte, error) {
	var payloadBytes []byte
	if strings.TrimSpace(payload) != "" {
		payloadBytes = []byte(payload)
	}

	validated, err := taskSvc.ValidatePayload(name, payloadBytes)
	if err != nil {
		return nil, errs.NewInvalidInputError(err.Error())
	}
	return validated, nil
}

// ListTaskExecutions pages task execution records for the console.
func ListTaskExecutions(
	ctx context.Context,
	req model.ListTaskExecutionsRequest,
) ([]model.TaskExecution, int64, error) {
	taskSvc, err := requireTaskService(ctx)
	if err != nil {
		return nil, 0, err
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	filterType := req.TaskType
	if filterType != "" {
		if meta, ok := taskSvc.GetTaskMeta(filterType); ok {
			filterType = meta.AsynqTask
		}
	}

	rows, total, err := taskSvc.ListExecutions(ctx, filterType, req.Status, req.Page, req.PageSize)
	if err != nil {
		return nil, 0, err
	}
	executions := make([]model.TaskExecution, 0, len(rows))
	for i := range rows {
		executions = append(executions, executionFromDTO(rows[i]))
	}
	return executions, total, nil
}

// TaskExecution loads a single execution record including its buffered log.
func TaskExecution(ctx context.Context, id uint64) (*model.TaskExecution, error) {
	taskSvc, err := requireTaskService(ctx)
	if err != nil {
		return nil, err
	}
	dto, err := taskSvc.GetExecution(ctx, id)
	if err != nil || dto == nil {
		return nil, err
	}
	row := executionFromDTO(*dto)
	return &row, nil
}

func normalizeTaskTrigger(v string) string {
	switch v {
	case contracts.TaskTriggerManual, contracts.TaskTriggerSystem, contracts.TaskTriggerRetry, contracts.TaskTriggerSchedule:
		return v
	case "inproc_cron", "cron":
		return contracts.TaskTriggerSchedule
	case "http":
		return contracts.TaskTriggerSystem
	case "":
		return contracts.TaskTriggerSystem
	default:
		return v
	}
}

func executionFromDTO(dto contracts.TaskExecutionDTO) model.TaskExecution {
	return model.TaskExecution{
		ID:           dto.ID,
		TaskID:       dto.TaskID,
		TaskType:     dto.TaskType,
		TaskName:     dto.TaskName,
		Status:       model.TaskExecutionStatus(dto.Status),
		Retryable:    dto.Retryable,
		MaxRetry:     dto.MaxRetry,
		RetryCount:   dto.RetryCount,
		Log:          dto.Log,
		ErrorMessage: dto.ErrorMessage,
		Result:       dto.Result,
		StartedAt:    dto.StartedAt,
		FinishedAt:   dto.FinishedAt,
		Duration:     dto.Duration,
		Payload:      dto.Payload,
		TriggeredBy:  normalizeTaskTrigger(dto.TriggeredBy),
		CreatedAt:    dto.CreatedAt,
		UpdatedAt:    dto.UpdatedAt,
	}
}

// RetryTask re-dispatches a failed execution as a new task run.
func RetryTask(ctx context.Context, id uint64) (string, error) {
	taskSvc, err := requireTaskService(ctx)
	if err != nil {
		return "", err
	}

	newTaskID, err := taskSvc.Retry(ctx, id)
	if err != nil {
		return "", err
	}
	return newTaskID, nil
}

// IsRetryConflictError reports whether the task registry rejected the retry request
// because of the record state rather than an infrastructure failure.
func IsRetryConflictError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, errs.RemoteTaskNotFailedMsg) ||
		strings.Contains(msg, errs.RemoteTaskNotRetryableMsg) ||
		strings.Contains(msg, errs.RemoteTaskMaxRetryMsg)
}

// IsRetryMissingError reports whether the referenced execution record is absent.
func IsRetryMissingError(err error) bool {
	return strings.Contains(err.Error(), errs.RemoteTaskNotFoundMsg)
}

// ListSchedules returns every dynamic schedule definition.
func ListSchedules(ctx context.Context) ([]model.Schedule, error) {
	return repository.ListSchedulesRecord(ctx)
}

// CreateSchedule validates a schedule definition, persists it and reloads the scheduler.
func CreateSchedule(ctx context.Context, req model.CreateScheduleRequest) (*model.Schedule, error) {
	if _, err := cron.ParseStandard(req.Cron); err != nil {
		return nil, errs.ErrInvalidCronExpression
	}

	taskSvc, err := requireTaskService(ctx)
	if err != nil {
		return nil, err
	}

	meta, ok := taskSvc.GetTaskMeta(req.TaskType)
	if !ok {
		return nil, errs.ErrInvalidTaskType
	}

	validated, err := validateTaskPayload(taskSvc, meta.Name, req.Payload)
	if err != nil {
		return nil, err
	}

	schedule := &model.Schedule{
		Name:     req.Name,
		TaskType: req.TaskType,
		Cron:     req.Cron,
		Payload:  string(validated),
		IsActive: *req.IsActive,
	}

	if err := repository.CreateScheduleRecord(ctx, schedule); err != nil {
		return nil, fmt.Errorf("%s: %w", errs.ScheduleSaveFailed, err)
	}

	reloadScheduler(ctx, taskSvc)
	return schedule, nil
}

// UpdateSchedule rewrites an existing schedule definition and reloads the scheduler.
func UpdateSchedule(ctx context.Context, id uint64, req model.UpdateScheduleRequest) (*model.Schedule, error) {
	schedule, err := repository.GetScheduleByID(ctx, id)
	if err != nil {
		return nil, errs.ErrScheduleNotFound
	}

	if _, err := cron.ParseStandard(req.Cron); err != nil {
		return nil, errs.ErrInvalidCronExpression
	}

	taskSvc, err := requireTaskService(ctx)
	if err != nil {
		return nil, err
	}

	meta, ok := taskSvc.GetTaskMeta(req.TaskType)
	if !ok {
		return nil, errs.ErrInvalidTaskType
	}

	validated, err := validateTaskPayload(taskSvc, meta.Name, req.Payload)
	if err != nil {
		return nil, err
	}

	schedule.Name = req.Name
	schedule.TaskType = req.TaskType
	schedule.Cron = req.Cron
	schedule.Payload = string(validated)
	schedule.IsActive = *req.IsActive

	if err := repository.UpdateScheduleRecord(ctx, schedule); err != nil {
		return nil, fmt.Errorf("%s: %w", errs.ScheduleSaveFailed, err)
	}

	reloadScheduler(ctx, taskSvc)
	return schedule, nil
}

// DeleteSchedule removes a schedule definition and reloads the scheduler.
func DeleteSchedule(ctx context.Context, id uint64) error {
	if err := repository.DeleteScheduleRecord(ctx, id); err != nil {
		return fmt.Errorf("%s: %w", errs.ScheduleDeleteFailed, err)
	}

	if taskSvc := GetTaskService(ctx); taskSvc != nil {
		reloadScheduler(ctx, taskSvc)
	}
	return nil
}

// reloadScheduler triggers the hot reload, degrading gracefully when the scheduler rejects it.
func reloadScheduler(ctx context.Context, taskSvc contracts.TaskService) {
	if err := taskSvc.ReloadScheduler(); err != nil {
		logger.ErrorF(ctx, "[TaskAdmin] 重载调度器失败: %v", err)
	}
}
