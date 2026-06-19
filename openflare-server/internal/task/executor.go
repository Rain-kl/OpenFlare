// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db/idgen"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	otel_trace "github.com/Rain-kl/Wavelet/pkg/trace"
	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// handlerRegistry 已注册的任务处理器
var handlerRegistry = make(map[string]TaskHandler)

// CompletedHandler is called when a task execution completes.
type CompletedHandler func(ctx context.Context, execution *model.TaskExecution, result *TaskResult, execErr error)

var taskCompletedHandlers []CompletedHandler

// OnTaskCompleted registers a handler for task completion events.
// Handlers must be registered during application bootstrap before processing tasks.
func OnTaskCompleted(handler CompletedHandler) {
	taskCompletedHandlers = append(taskCompletedHandlers, handler)
}

// RegisterHandler 注册任务处理器
// 传入任务类型标识（对应 constants.go 中的 AsynqTask 常量）和 TaskHandler 实现
func RegisterHandler(asynqTaskType string, handler TaskHandler) {
	handlerRegistry[asynqTaskType] = handler
}

// getHandler 获取已注册的处理器
func getHandler(asynqTaskType string) (TaskHandler, bool) {
	h, ok := handlerRegistry[asynqTaskType]
	return h, ok
}

// ValidateAndNormalizePayload 校验并标准化任务参数。
// 如果 Handler 实现了 PayloadValidator，调用其 ValidatePayload 方法；
// 否则直接返回原始 payload。
func ValidateAndNormalizePayload(asynqTaskType string, payload []byte) ([]byte, error) {
	handler, ok := getHandler(asynqTaskType)
	if !ok {
		return payload, nil
	}
	if validator, ok := handler.(PayloadValidator); ok {
		return validator.ValidatePayload(payload)
	}
	return payload, nil
}

// contextKey 用于 context 存取 taskID
type contextKey string

const taskIDKey contextKey = "task_execution_task_id"
const traceEnvelopeVersion = 1

type traceEnvelope struct {
	WaveletTraceEnvelope bool              `json:"_wavelet_trace_envelope"`
	Version              int               `json:"version"`
	TraceContext         map[string]string `json:"trace_context,omitempty"`
	Payload              []byte            `json:"payload"`
}

// withTaskID 将 taskID 注入 context
func withTaskID(ctx context.Context, taskID string) context.Context {
	return context.WithValue(ctx, taskIDKey, taskID)
}

// GetTaskID 从 context 中获取 taskID
func GetTaskID(ctx context.Context) string {
	if v, ok := ctx.Value(taskIDKey).(string); ok {
		return v
	}
	return ""
}

// IsFinalAttempt 判断当前任务执行是否为最后一次重试尝试（若再次失败即为最终失败）
func IsFinalAttempt(ctx context.Context) bool {
	retryCount, hasRetryCount := asynq.GetRetryCount(ctx)
	maxRetry, hasMaxRetry := asynq.GetMaxRetry(ctx)
	if !hasRetryCount || !hasMaxRetry {
		return true
	}
	return retryCount >= maxRetry
}

// AppendLog 追加日志到任务执行记录
// 在 TaskHandler.Execute 中调用，日志会自动追加到 TaskExecution.Log 字段
func AppendLog(ctx context.Context, format string, args ...interface{}) {
	taskID := GetTaskID(ctx)
	if taskID == "" {
		// 上下文中没有 taskID，降级到普通日志
		logger.InfoF(ctx, format, args...)
		return
	}

	logLine := fmt.Sprintf(format, args...)
	if err := model.AppendTaskExecutionLog(ctx, taskID, logLine); err != nil {
		logger.ErrorF(ctx, "[TaskExecutor] 追加任务日志失败 taskID=%s: %v", taskID, err)
	}
}

// DispatchTask 下发任务（创建 TaskExecution 记录 → 入队 Asynq）
func DispatchTask(ctx context.Context, taskType string, payload []byte, triggeredBy string) (string, error) {
	meta := GetTaskMeta(taskType)
	if meta == nil {
		return "", fmt.Errorf(errUnknownTaskType, taskType)
	}

	// 生成唯一的 TaskID
	taskID := generateTaskID(taskType, triggeredBy)

	// 创建任务执行记录
	execution := &model.TaskExecution{
		TaskID:      taskID,
		TaskType:    meta.AsynqTask,
		TaskName:    meta.Name,
		Status:      model.TaskExecutionStatusPending,
		Retryable:   meta.Retryable,
		MaxRetry:    meta.MaxRetry,
		RetryCount:  0,
		Payload:     string(payload),
		TriggeredBy: triggeredBy,
	}

	if err := model.CreateTaskExecution(ctx, execution); err != nil {
		return "", fmt.Errorf(errCreateTaskExecutionFailed, err)
	}

	// 入队 Asynq
	taskInfo := asynq.NewTask(meta.AsynqTask, injectTaskTraceContext(ctx, payload))
	if _, err := AsynqClient.Enqueue(
		taskInfo,
		asynq.TaskID(taskID),
		asynq.MaxRetry(meta.MaxRetry),
		asynq.Queue(meta.Queue),
	); err != nil {
		// 入队失败，更新执行记录状态
		execution.Status = model.TaskExecutionStatusFailed
		execution.ErrorMessage = fmt.Sprintf("入队失败: %v", err)
		now := time.Now()
		execution.StartedAt = &now
		execution.FinishedAt = &now
		_ = model.UpdateTaskExecution(ctx, execution)
		return "", fmt.Errorf(errTaskEnqueueFailed, err)
	}

	if err := model.AppendTaskExecutionLog(ctx, taskID, fmt.Sprintf("[系统] 任务已成功入队，等待调度执行 (队列: %s, 最大重试次数: %d)", meta.Queue, meta.MaxRetry)); err != nil {
		logger.ErrorF(ctx, "[TaskExecutor] 追加入队日志失败 taskID=%s: %v", taskID, err)
	}

	return taskID, nil
}

// RetryTask 重试失败的任务
func RetryTask(ctx context.Context, id uint64) (string, error) {
	execution, err := model.GetTaskExecutionByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf(errTaskExecutionNotFound, err)
	}

	if execution.Status != model.TaskExecutionStatusFailed {
		return "", fmt.Errorf(errRetryOnlyFailedTask, execution.Status)
	}

	if !execution.Retryable {
		return "", errors.New(errTaskNotRetryable)
	}

	// 生成新的 TaskID
	newTaskID := generateRetryTaskID(execution.TaskID, execution.RetryCount+1)

	// 创建新的执行记录
	newExecution := &model.TaskExecution{
		TaskID:      newTaskID,
		TaskType:    execution.TaskType,
		TaskName:    execution.TaskName,
		Status:      model.TaskExecutionStatusPending,
		Retryable:   execution.Retryable,
		MaxRetry:    execution.MaxRetry,
		RetryCount:  execution.RetryCount + 1,
		Payload:     execution.Payload,
		TriggeredBy: "retry",
	}

	if err := model.CreateTaskExecution(ctx, newExecution); err != nil {
		return "", fmt.Errorf(errCreateRetryExecutionFailed, err)
	}

	meta := GetTaskMeta(execution.TaskType)
	queueName := QueueDefault
	if meta != nil {
		queueName = meta.Queue
	}

	// 入队 Asynq
	taskInfo := asynq.NewTask(execution.TaskType, injectTaskTraceContext(ctx, []byte(execution.Payload)))
	if _, err := AsynqClient.Enqueue(
		taskInfo,
		asynq.TaskID(newTaskID),
		asynq.MaxRetry(execution.MaxRetry),
		asynq.Queue(queueName),
	); err != nil {
		newExecution.Status = model.TaskExecutionStatusFailed
		newExecution.ErrorMessage = fmt.Sprintf("重试入队失败: %v", err)
		now := time.Now()
		newExecution.StartedAt = &now
		newExecution.FinishedAt = &now
		_ = model.UpdateTaskExecution(ctx, newExecution)
		return "", fmt.Errorf(errRetryTaskEnqueueFailed, err)
	}

	if err := model.AppendTaskExecutionLog(ctx, newTaskID, fmt.Sprintf("[系统] 手动触发重试，已重新创建任务并入队 (原任务ID: %s, 重试次数: %d/%d)", execution.TaskID, execution.RetryCount+1, execution.MaxRetry)); err != nil {
		logger.ErrorF(ctx, "[TaskExecutor] 追加重试日志失败 taskID=%s: %v", newTaskID, err)
	}

	return newTaskID, nil
}

// ProcessTask Asynq 实际调用的统一处理函数
// Worker 注册时统一使用此函数，内部自动分发到对应的 TaskHandler
func ProcessTask(ctx context.Context, t *asynq.Task) error {
	taskPayload := t.Payload()
	ctx, taskPayload, hasRemoteTraceContext := extractTaskTraceContext(ctx, taskPayload)

	// 初始化 Trace
	ctx, span := otel_trace.Start(ctx, "TaskProcess_"+t.Type(), trace.WithSpanKind(trace.SpanKindConsumer))
	defer span.End()

	// 添加任务信息到 Span
	span.SetAttributes(
		attribute.String("task.type", t.Type()),
		attribute.Int("task.payload_size", len(taskPayload)),
		attribute.Bool("task.trace_context_propagated", hasRemoteTraceContext),
		attribute.String("task.id", t.ResultWriter().TaskID()),
	)

	taskID := t.ResultWriter().TaskID()

	// 注入 taskID 到 context
	ctx = withTaskID(ctx, taskID)

	// 查找处理器
	handler, ok := getHandler(t.Type())
	if !ok {
		err := fmt.Errorf(errUnregisteredTaskHandler, t.Type())
		logger.ErrorF(ctx, "[TaskExecutor] %v", err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	// 加载或动态创建执行记录
	now := time.Now()
	execution, err := getOrCreateTaskExecution(ctx, taskID, t, taskPayload, now)
	if err == nil {
		updateExecutionOnStart(ctx, execution, now)
	}

	if execution != nil {
		AppendLog(ctx, "[系统] 开始执行异步任务 [名称: %s, 类型: %s]，重试次数: %d/%d",
			execution.TaskName, t.Type(), execution.RetryCount, execution.MaxRetry)
	} else {
		AppendLog(ctx, "[系统] 开始执行异步任务 [类型: %s]", t.Type())
	}

	// 开始计时
	start := time.Now()

	// 执行业务逻辑
	result, execErr := handler.Execute(ctx, taskPayload)

	// 计算耗时并归档记录
	duration := time.Since(start)
	finishTime := time.Now()

	completeTaskExecution(ctx, execution, t, duration, finishTime, result, execErr, span)

	if execution == nil && execErr != nil {
		span.SetStatus(codes.Error, execErr.Error())
		return execErr
	}

	return execErr
}

func injectTaskTraceContext(ctx context.Context, payload []byte) []byte {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	if len(carrier) == 0 {
		return payload
	}

	envelope := traceEnvelope{
		WaveletTraceEnvelope: true,
		Version:              traceEnvelopeVersion,
		TraceContext:         map[string]string(carrier),
		Payload:              payload,
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		logger.ErrorF(ctx, "[TaskExecutor] 序列化任务 Trace 上下文失败: %v", err)
		return payload
	}
	return data
}

func extractTaskTraceContext(ctx context.Context, payload []byte) (context.Context, []byte, bool) {
	var envelope traceEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return ctx, payload, false
	}
	if !envelope.WaveletTraceEnvelope || envelope.Version != traceEnvelopeVersion {
		return ctx, payload, false
	}
	if len(envelope.TraceContext) == 0 {
		return ctx, envelope.Payload, false
	}

	extractedCtx := otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(envelope.TraceContext))
	return extractedCtx, envelope.Payload, true
}

func updateExecutionOnStart(ctx context.Context, execution *model.TaskExecution, now time.Time) {
	if execution == nil {
		return
	}
	dirty := false
	if retryCount, hasRetry := asynq.GetRetryCount(ctx); hasRetry && execution.RetryCount != retryCount {
		execution.RetryCount = retryCount
		dirty = true
	}
	if execution.Status != model.TaskExecutionStatusRunning {
		execution.Status = model.TaskExecutionStatusRunning
		execution.StartedAt = &now
		dirty = true
	}
	if dirty {
		if updateErr := model.UpdateTaskExecution(ctx, execution); updateErr != nil {
			logger.ErrorF(ctx, "[TaskExecutor] 更新执行状态失败 taskID=%s: %v", execution.TaskID, updateErr)
		}
	}
}

// getOrCreateTaskExecution 获取已有的任务执行记录，如果不存在则针对已知任务类型动态创建记录
func getOrCreateTaskExecution(ctx context.Context, taskID string, t *asynq.Task, payload []byte, now time.Time) (*model.TaskExecution, error) {
	execution, err := model.GetTaskExecutionByTaskID(ctx, taskID)
	if err == nil {
		return execution, nil
	}

	meta := GetTaskMetaByAsynqTask(t.Type())
	if meta == nil {
		return nil, err
	}

	execution = &model.TaskExecution{
		TaskID:      taskID,
		TaskType:    meta.AsynqTask,
		TaskName:    meta.Name,
		Status:      model.TaskExecutionStatusRunning,
		Retryable:   meta.Retryable,
		MaxRetry:    meta.MaxRetry,
		RetryCount:  0,
		Payload:     string(payload),
		TriggeredBy: "schedule",
		StartedAt:   &now,
	}

	if createErr := model.CreateTaskExecution(ctx, execution); createErr != nil {
		logger.ErrorF(ctx, "[TaskExecutor] 动态创建执行记录失败 taskID=%s: %v", taskID, createErr)
		return nil, createErr
	}

	return execution, nil
}

// completeTaskExecution 完成并更新任务执行记录的状态和执行结果
func completeTaskExecution(ctx context.Context, execution *model.TaskExecution, t *asynq.Task, duration time.Duration, finishTime time.Time, result *TaskResult, execErr error, span trace.Span) {
	if execution == nil {
		return
	}

	execution.Duration = duration.Milliseconds()
	execution.FinishedAt = &finishTime

	if execErr != nil {
		handleFailedTask(ctx, execution, t, duration, execErr, span)
	} else {
		handleSuccessfulTask(ctx, execution, t, duration, result)
	}

	if err := model.UpdateTaskExecution(ctx, execution); err != nil {
		logger.ErrorF(ctx, "[TaskExecutor] 更新执行记录失败 taskID=%s: %v", execution.TaskID, err)
	}
	if shouldFlushTaskExecutionLog(ctx, execErr) {
		if err := model.FlushTaskExecutionLog(ctx, execution.TaskID); err != nil {
			logger.ErrorF(ctx, "[TaskExecutor] 持久化任务日志失败 taskID=%s: %v", execution.TaskID, err)
		}
	}

	notifyTaskCompleted(ctx, execution, result, execErr)
}

func notifyTaskCompleted(ctx context.Context, execution *model.TaskExecution, result *TaskResult, execErr error) {
	if len(taskCompletedHandlers) == 0 {
		return
	}

	asyncCtx := context.WithoutCancel(ctx)
	for _, handler := range taskCompletedHandlers {
		go handler(asyncCtx, execution, result, execErr)
	}
}

func shouldFlushTaskExecutionLog(ctx context.Context, execErr error) bool {
	if execErr == nil {
		return true
	}

	retryCount, hasRetryCount := asynq.GetRetryCount(ctx)
	maxRetry, hasMaxRetry := asynq.GetMaxRetry(ctx)
	if !hasRetryCount || !hasMaxRetry {
		return true
	}
	return retryCount >= maxRetry
}

func handleFailedTask(ctx context.Context, execution *model.TaskExecution, t *asynq.Task, duration time.Duration, execErr error, span trace.Span) {
	execution.Status = model.TaskExecutionStatusFailed
	execution.ErrorMessage = execErr.Error()
	logger.ErrorF(ctx, "[TaskExecutor] 任务处理失败 Type: %s TaskID: %s Duration: %d ms Error: %v", t.Type(), execution.TaskID, duration.Milliseconds(), execErr)
	span.SetStatus(codes.Error, execErr.Error())
	span.RecordError(execErr)

	AppendLog(ctx, "[系统] 任务执行失败，耗时: %d ms，错误原因: %v", duration.Milliseconds(), execErr)
}

func handleSuccessfulTask(ctx context.Context, execution *model.TaskExecution, t *asynq.Task, duration time.Duration, result *TaskResult) {
	execution.Status = model.TaskExecutionStatusSucceeded
	execution.ErrorMessage = "" // 清除历史重试失败遗留的错误信息
	if result != nil {
		execution.Result = result.Message
		if result.Detail != "" {
			execution.Result = fmt.Sprintf("%s\n%s", result.Message, result.Detail)
		}
	}
	logger.InfoF(ctx, "[TaskExecutor] 任务处理完成 Type: %s TaskID: %s Duration: %d ms", t.Type(), execution.TaskID, duration.Milliseconds())

	resultMsg := "成功"
	if result != nil {
		resultMsg = result.Message
	}
	AppendLog(ctx, "[系统] 任务执行成功，耗时: %d ms，执行结果: %s", duration.Milliseconds(), resultMsg)
}

// generateTaskID 生成任务 ID
func generateTaskID(taskType string, triggeredBy string) string {
	return fmt.Sprintf("%s_%s_%d", triggeredBy, taskType, idgen.NextUint64ID())
}

// generateRetryTaskID 生成重试任务 ID
func generateRetryTaskID(originalTaskID string, retryCount int) string {
	return fmt.Sprintf("retry_%d_%s", retryCount, originalTaskID)
}
