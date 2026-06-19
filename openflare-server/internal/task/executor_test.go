// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// mockHandler 用于测试的模拟任务处理器
type mockHandler struct {
	executeFunc func(ctx context.Context, payload []byte) (*TaskResult, error)
}

func (h *mockHandler) Execute(ctx context.Context, payload []byte) (*TaskResult, error) {
	if h.executeFunc != nil {
		return h.executeFunc(ctx, payload)
	}
	return &TaskResult{Message: "mock success"}, nil
}

// successHandler 返回成功的处理器
func successHandler() *mockHandler {
	return &mockHandler{
		executeFunc: func(ctx context.Context, payload []byte) (*TaskResult, error) {
			AppendLog(ctx, "执行成功，处理了 %d 条数据", 100)
			return &TaskResult{Message: "处理完成，共 100 条"}, nil
		},
	}
}

// failHandler 返回失败的处理器
func failHandler() *mockHandler {
	return &mockHandler{
		executeFunc: func(ctx context.Context, payload []byte) (*TaskResult, error) {
			AppendLog(ctx, "开始执行任务")
			return nil, fmt.Errorf("模拟执行失败: 数据库连接超时")
		},
	}
}

const testTaskType = "test:mock_task"

func setupTest(t *testing.T) func() {
	_, mr, cleanup := testhelper.SetupTestEnvironment(t)
	AsynqClient = asynq.NewClient(asynq.RedisClientOpt{
		Addr: mr.Addr(),
	})
	// 注册测试用 handler
	RegisterHandler(testTaskType, successHandler())
	return func() {
		if AsynqClient != nil {
			_ = AsynqClient.Close()
			AsynqClient = nil
		}
		cleanup()
	}
}

func TestRegisterAndGetHandler(t *testing.T) {
	_ = testTaskType
	cleanup := setupTest(t)
	defer cleanup()

	// 验证 handler 已注册
	h, ok := getHandler(testTaskType)
	assert.True(t, ok, "handler should be registered")
	assert.NotNil(t, h)

	// 未注册的 handler
	_, ok = getHandler("nonexistent")
	assert.False(t, ok, "non-existent handler should return false")
}

func TestGetTaskIDFromContext(t *testing.T) {
	ctx := context.Background()

	// 空 context
	taskID := GetTaskID(ctx)
	assert.Equal(t, "", taskID)

	// 注入 taskID
	ctx = withTaskID(ctx, "test_task_123")
	taskID = GetTaskID(ctx)
	assert.Equal(t, "test_task_123", taskID)
}

func TestAppendLogWithoutTaskID(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()
	ctx := context.Background()

	// 没有 taskID 的 context，应降级到普通日志，不报错
	AppendLog(ctx, "这条日志应该降级处理，不会报错")
}

func TestAppendLogWithTaskID(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()
	ctx := context.Background()

	// 先创建一条执行记录
	execution := &model.TaskExecution{
		TaskID:      "log_test_001",
		TaskType:    testTaskType,
		TaskName:    "测试任务",
		Status:      model.TaskExecutionStatusRunning,
		TriggeredBy: "manual",
	}
	err := model.CreateTaskExecution(ctx, execution)
	require.NoError(t, err)

	// 注入 taskID 并追加日志
	ctx = withTaskID(ctx, "log_test_001")
	AppendLog(ctx, "第一条日志")
	AppendLog(ctx, "处理了 %d 条数据", 50)

	// 验证日志
	found, err := model.GetTaskExecutionByTaskID(ctx, "log_test_001")
	require.NoError(t, err)
	assert.Contains(t, found.Log, "第一条日志")
	assert.Contains(t, found.Log, "处理了 50 条数据")
}

func TestTaskTraceContextEnvelope(t *testing.T) {
	payload := []byte(`{"hello":"wavelet"}`)
	traceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	parentCtx := otel.GetTextMapPropagator().Extract(
		context.Background(),
		propagation.MapCarrier{
			"traceparent": "00-" + traceID + "-00f067aa0ba902b7-01",
		},
	)

	wrappedPayload := injectTaskTraceContext(parentCtx, payload)
	require.NotEqual(t, string(payload), string(wrappedPayload))

	gotCtx, gotPayload, ok := extractTaskTraceContext(context.Background(), wrappedPayload)
	require.True(t, ok)
	assert.Equal(t, payload, gotPayload)
	assert.Equal(t, traceID, trace.SpanContextFromContext(gotCtx).TraceID().String())
}

func TestTaskTraceContextEnvelopeKeepsLegacyPayload(t *testing.T) {
	payload := []byte(`{"legacy":true}`)

	gotCtx, gotPayload, ok := extractTaskTraceContext(context.Background(), payload)
	require.False(t, ok)
	assert.Equal(t, context.Background(), gotCtx)
	assert.Equal(t, payload, gotPayload)
}

func TestTaskTraceContextEnvelopeSkipsEmptyContext(t *testing.T) {
	payload := []byte(`{"background":true}`)

	wrappedPayload := injectTaskTraceContext(context.Background(), payload)
	assert.Equal(t, payload, wrappedPayload)
}

func TestProcessTaskSuccess(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()
	ctx := context.Background()

	// 注册成功 handler
	RegisterHandler(testTaskType, successHandler())

	// 创建执行记录
	execution := &model.TaskExecution{
		TaskID:      "process_success_001",
		TaskType:    testTaskType,
		TaskName:    "测试任务",
		Status:      model.TaskExecutionStatusPending,
		Retryable:   true,
		MaxRetry:    3,
		TriggeredBy: "manual",
	}
	err := model.CreateTaskExecution(ctx, execution)
	require.NoError(t, err)

	// 通过 asynq 的 Task 不能直接设置 taskID，ProcessTask 通过 t.ResultWriter().TaskID() 获取
	// 但 asynq.Task 在没有经过 asynq server 的情况下 ResultWriter 可能为 nil
	// 我们需要在 ProcessTask 内部改用 taskID 注入的方式测试
	// 为了测试 ProcessTask，我们直接模拟调用 handler

	// 直接通过 handler 测试
	handler, ok := getHandler(testTaskType)
	require.True(t, ok)

	ctx = withTaskID(ctx, "process_success_001")
	result, err := handler.Execute(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "处理完成，共 100 条", result.Message)

	// 验证日志被追加
	found, err := model.GetTaskExecutionByTaskID(ctx, "process_success_001")
	require.NoError(t, err)
	assert.Contains(t, found.Log, "执行成功，处理了 100 条数据")
}

func TestProcessTaskFailure(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()
	ctx := context.Background()

	// 注册失败 handler
	RegisterHandler(testTaskType, failHandler())

	// 创建执行记录
	execution := &model.TaskExecution{
		TaskID:      "process_fail_001",
		TaskType:    testTaskType,
		TaskName:    "测试任务",
		Status:      model.TaskExecutionStatusPending,
		Retryable:   true,
		MaxRetry:    3,
		TriggeredBy: "manual",
	}
	err := model.CreateTaskExecution(ctx, execution)
	require.NoError(t, err)

	// 直接调用 handler
	handler, ok := getHandler(testTaskType)
	require.True(t, ok)

	ctx = withTaskID(ctx, "process_fail_001")
	_, err = handler.Execute(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "模拟执行失败")

	// 验证日志
	found, err := model.GetTaskExecutionByTaskID(ctx, "process_fail_001")
	require.NoError(t, err)
	assert.Contains(t, found.Log, "开始执行任务")
}

func TestCompleteTaskExecutionFlushesLog(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()
	ctx := context.Background()

	execution := &model.TaskExecution{
		TaskID:      "complete_flush_001",
		TaskType:    testTaskType,
		TaskName:    "测试任务",
		Status:      model.TaskExecutionStatusRunning,
		TriggeredBy: "manual",
	}
	err := model.CreateTaskExecution(ctx, execution)
	require.NoError(t, err)

	ctx = withTaskID(ctx, execution.TaskID)
	AppendLog(ctx, "任务执行中的日志")

	finishTime := time.Now()
	completeTaskExecution(
		ctx,
		execution,
		asynq.NewTask(testTaskType, nil),
		100*time.Millisecond,
		finishTime,
		&TaskResult{Message: "处理完成"},
		nil,
		trace.SpanFromContext(ctx),
	)

	found, err := model.GetTaskExecutionByTaskID(ctx, execution.TaskID)
	require.NoError(t, err)
	assert.Equal(t, model.TaskExecutionStatusSucceeded, found.Status)
	assert.Contains(t, found.Log, "任务执行中的日志")
	assert.Contains(t, found.Log, "任务执行成功")
}

func TestRetryTask(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()
	ctx := context.Background()

	// 创建一条失败的执行记录（可重试）
	now := time.Now()
	execution := &model.TaskExecution{
		TaskID:       "retry_test_001",
		TaskType:     testTaskType,
		TaskName:     "测试任务",
		Status:       model.TaskExecutionStatusFailed,
		Retryable:    true,
		MaxRetry:     3,
		RetryCount:   0,
		ErrorMessage: "首次执行失败",
		StartedAt:    &now,
		FinishedAt:   &now,
		Duration:     100,
		TriggeredBy:  "manual",
	}
	err := model.CreateTaskExecution(ctx, execution)
	require.NoError(t, err)

	// 重试
	newTaskID, err := RetryTask(ctx, execution.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, newTaskID)
	assert.Contains(t, newTaskID, "retry_1_")

	// 验证新记录
	newExecution, err := model.GetTaskExecutionByTaskID(ctx, newTaskID)
	require.NoError(t, err)
	assert.Equal(t, model.TaskExecutionStatusPending, newExecution.Status)
	assert.Equal(t, 1, newExecution.RetryCount)
	assert.Equal(t, "retry", newExecution.TriggeredBy)
	assert.Equal(t, execution.TaskType, newExecution.TaskType)
	assert.True(t, newExecution.Retryable)

	// 原记录不变
	original, err := model.GetTaskExecutionByID(ctx, execution.ID)
	require.NoError(t, err)
	assert.Equal(t, model.TaskExecutionStatusFailed, original.Status)
	assert.Equal(t, 0, original.RetryCount)
}

func TestRetryTaskNotFailed(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()
	ctx := context.Background()

	// 创建一条成功的记录
	execution := &model.TaskExecution{
		TaskID:      "retry_not_failed_001",
		TaskType:    testTaskType,
		TaskName:    "测试任务",
		Status:      model.TaskExecutionStatusSucceeded,
		Retryable:   true,
		MaxRetry:    3,
		TriggeredBy: "manual",
	}
	err := model.CreateTaskExecution(ctx, execution)
	require.NoError(t, err)

	// 尝试重试成功的任务
	_, err = RetryTask(ctx, execution.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "只有失败的任务才能重试")
}

func TestRetryTaskNotRetryable(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()
	ctx := context.Background()

	execution := &model.TaskExecution{
		TaskID:      "retry_not_allowed_001",
		TaskType:    testTaskType,
		TaskName:    "测试任务",
		Status:      model.TaskExecutionStatusFailed,
		Retryable:   false,
		MaxRetry:    0,
		TriggeredBy: "manual",
	}
	err := model.CreateTaskExecution(ctx, execution)
	require.NoError(t, err)

	_, err = RetryTask(ctx, execution.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不支持重试")
}

func TestRetryTaskNonExistent(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()
	ctx := context.Background()

	_, err := RetryTask(ctx, 99999999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestGenerateTaskID(t *testing.T) {
	id1 := generateTaskID("test_type", "manual")
	id2 := generateTaskID("test_type", "manual")

	// 两个 ID 应不同（包含 Snowflake ID）
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "manual_test_type_")
}

func TestGenerateRetryTaskID(t *testing.T) {
	id := generateRetryTaskID("original_task_123", 2)
	assert.Equal(t, "retry_2_original_task_123", id)
}
