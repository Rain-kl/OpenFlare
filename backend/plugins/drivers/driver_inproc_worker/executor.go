// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_inproc_worker

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"Wavelet/pkg/idgen"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/util"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const defaultRetryBackoff = 500 * time.Millisecond

// TaskMessage represents an in-process task item in the queue.
type TaskMessage struct {
	ID        string
	TaskType  string
	Payload   []byte
	Source    string
	CreatedAt time.Time
	RetryLeft int
}

// InprocQueue manages in-memory task queuing and worker pool execution.
type InprocQueue struct {
	concurrency int
	queue       chan TaskMessage
	taskReg     extpoints.TaskExtension
	running     atomic.Bool
	stopCh      chan struct{}
	wg          sync.WaitGroup

	// baseCtx is the app-lifetime context captured at Start; task handlers
	// derive their timeouts from it so shutdown cancellation propagates.
	baseCtx context.Context
	appCtx  *core.Context
}

// NewInprocQueue creates a new InprocQueue with a given concurrency and queue capacity.
func NewInprocQueue(concurrency, queueCap int, taskReg extpoints.TaskExtension) *InprocQueue {
	if concurrency <= 0 {
		concurrency = 10
	}
	if queueCap <= 0 {
		queueCap = 1000
	}

	return &InprocQueue{
		concurrency: concurrency,
		queue:       make(chan TaskMessage, queueCap),
		taskReg:     taskReg,
		stopCh:      make(chan struct{}),
	}
}

// Enqueue puts a new task into the in-process queue.
// taskType may be the registration pattern or the admin type identifier.
func (q *InprocQueue) Enqueue(ctx context.Context, taskType string, payload []byte, source string) (string, error) {
	if !q.running.Load() {
		return "", errors.New("driver_inproc_worker: queue is not running")
	}

	td, ok := q.lookupTask(taskType)
	if !ok {
		return "", fmt.Errorf("driver_inproc_worker: unknown task type %q", taskType)
	}

	if source == "" {
		source = contracts.TaskTriggerManual
	}
	idType := td.Type
	if idType == "" {
		idType = td.Pattern
	}
	taskID := fmt.Sprintf("%s_%s_%d", source, idType, idgen.NextUint64ID())
	msg := TaskMessage{
		ID:        taskID,
		TaskType:  td.Pattern,
		Payload:   payload,
		Source:    source,
		CreatedAt: time.Now(),
		RetryLeft: td.Retry,
	}

	if err := q.createExecution(ctx, msg, td); err != nil {
		return "", err
	}

	q.appendExecutionLog(ctx, taskID, fmt.Sprintf("[系统] 任务已成功入队，等待调度执行 (最大重试次数: %d)", td.Retry))

	select {
	case q.queue <- msg:
		return taskID, nil
	default:
		q.failExecution(ctx, msg, errors.New("queue is full"), 0)
		return "", errors.New("driver_inproc_worker: queue is full")
	}
}

func (q *InprocQueue) lookupTask(taskType string) (extpoints.TaskDefinition, bool) {
	if q.taskReg == nil {
		return extpoints.TaskDefinition{}, false
	}
	return q.taskReg.Get(taskType)
}

// Start begins processing tasks with the worker pool. ctx is the app-lifetime
// context used as the parent for per-task execution contexts.
func (q *InprocQueue) Start(ctx context.Context) {
	if !q.running.CompareAndSwap(false, true) {
		return
	}

	if q.baseCtx == nil {
		q.baseCtx = ctx
	}
	for i := 0; i < q.concurrency; i++ {
		q.wg.Add(1)
		util.Go(func() {
			defer q.wg.Done()
			q.workerLoop(ctx)
		})
	}
}

// Stop gracefully waits for in-flight tasks and shuts down workers.
func (q *InprocQueue) Stop(ctx context.Context) error {
	if !q.running.CompareAndSwap(true, false) {
		return nil
	}

	close(q.stopCh)

	done := make(chan struct{})
	util.Go(func() {
		q.wg.Wait()
		close(done)
	})

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *InprocQueue) workerLoop(ctx context.Context) {
	for {
		select {
		case <-q.stopCh:
			return
		case <-ctx.Done():
			return
		case msg, ok := <-q.queue:
			if !ok {
				return
			}
			q.executeTask(ctx, msg)
		}
	}
}

func (q *InprocQueue) executeTask(ctx context.Context, msg TaskMessage) {
	td, ok := q.lookupTask(msg.TaskType)
	if !ok {
		logger.ErrorF(ctx, "driver_inproc_worker: no handler for task %q", msg.TaskType)
		q.failExecution(ctx, msg, fmt.Errorf("unregistered task handler: %s", msg.TaskType), 0)
		return
	}

	timeout := td.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	taskCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if q.appCtx != nil {
		taskCtx = core.WithAppContext(taskCtx, q.appCtx)
		ctx = core.WithAppContext(ctx, q.appCtx)
	}

	q.markRunning(ctx, msg)
	start := time.Now()
	result, err := invokeHandler(taskCtx, td.Handler, msg.Payload)
	duration := time.Since(start)

	if err != nil {
		q.failExecution(ctx, msg, err, duration)
		if msg.RetryLeft > 0 {
			msg.RetryLeft--
			util.Go(func() {
				select {
				case <-time.After(defaultRetryBackoff):
				case <-q.stopCh:
					return
				case <-ctx.Done():
					return
				}
				if q.running.Load() {
					select {
					case q.queue <- msg:
					default:
					}
				}
			})
		}
		return
	}
	q.succeedExecution(ctx, msg, duration, result)
}

func invokeHandler(ctx context.Context, handler any, payload []byte) (*contracts.TaskResultDTO, error) {
	if handler == nil {
		return nil, errors.New("nil task handler")
	}

	switch fn := handler.(type) {
	case contracts.TaskHandler:
		return fn.Execute(ctx, payload)
	case func(context.Context, []byte) (*contracts.TaskResultDTO, error):
		return fn(ctx, payload)
	case func(context.Context, []byte) error:
		return nil, fn(ctx, payload)
	case func(context.Context) error:
		return nil, fn(ctx)
	case func([]byte) error:
		return nil, fn(payload)
	case func() error:
		return nil, fn()
	default:
		return nil, fmt.Errorf("unsupported handler type: %T", handler)
	}
}

func (q *InprocQueue) createExecution(ctx context.Context, msg TaskMessage, td extpoints.TaskDefinition) error {
	db := getDB(ctx)
	if db == nil {
		return nil
	}

	name := td.Name
	if name == "" {
		name = td.DisplayName
	}
	if name == "" {
		name = td.Pattern
	}
	exec := &taskExecution{
		ID:          idgen.NextUint64ID(),
		TaskID:      msg.ID,
		TaskType:    td.Pattern,
		TaskName:    name,
		Status:      taskExecutionStatusPending,
		Retryable:   td.Retryable || td.Retry > 0,
		MaxRetry:    td.Retry,
		RetryCount:  0,
		Payload:     string(msg.Payload),
		TriggeredBy: msg.Source,
	}
	if err := db.Create(exec).Error; err != nil {
		return fmt.Errorf("driver_inproc_worker: create task execution: %w", err)
	}
	return nil
}

func (q *InprocQueue) markRunning(ctx context.Context, msg TaskMessage) {
	db := getDB(ctx)
	if db == nil {
		return
	}
	now := time.Now()
	updates := map[string]any{
		taskExecutionColStatus: taskExecutionStatusRunning,
		"started_at":           now,
	}
	if err := db.Model(&taskExecution{}).Where("task_id = ?", msg.ID).Updates(updates).Error; err != nil {
		logger.ErrorF(ctx, "driver_inproc_worker: mark running failed taskID=%s: %v", msg.ID, err)
		return
	}
	q.appendExecutionLog(ctx, msg.ID, fmt.Sprintf("[系统] 开始执行异步任务 [类型: %s]", msg.TaskType))
}

func (q *InprocQueue) succeedExecution(ctx context.Context, msg TaskMessage, duration time.Duration, result *contracts.TaskResultDTO) {
	db := getDB(ctx)
	if db == nil {
		return
	}
	now := time.Now()
	resultText := "ok"
	if result != nil {
		resultText = result.Message
		if result.Detail != nil {
			if s, ok := result.Detail.(string); ok && s != "" {
				resultText = result.Message + "\n" + s
			} else if b, err := json.Marshal(result.Detail); err == nil && len(b) > 0 && string(b) != "null" {
				resultText = result.Message + "\n" + string(b)
			}
		}
	}
	updates := map[string]any{
		taskExecutionColStatus: taskExecutionStatusSucceeded,
		"error_message":        "",
		"result":               resultText,
		"finished_at":          now,
		"duration":             duration.Milliseconds(),
	}
	if err := db.Model(&taskExecution{}).Where("task_id = ?", msg.ID).Updates(updates).Error; err != nil {
		logger.ErrorF(ctx, "driver_inproc_worker: mark succeeded failed taskID=%s: %v", msg.ID, err)
		return
	}
	q.appendExecutionLog(ctx, msg.ID, fmt.Sprintf("[系统] 任务执行成功，耗时: %d ms", duration.Milliseconds()))
}

func (q *InprocQueue) failExecution(ctx context.Context, msg TaskMessage, execErr error, duration time.Duration) {
	db := getDB(ctx)
	if db == nil {
		return
	}
	now := time.Now()
	updates := map[string]any{
		taskExecutionColStatus: taskExecutionStatusFailed,
		"error_message":        execErr.Error(),
		"finished_at":          now,
		"duration":             duration.Milliseconds(),
	}
	if err := db.Model(&taskExecution{}).Where("task_id = ?", msg.ID).Updates(updates).Error; err != nil {
		logger.ErrorF(ctx, "driver_inproc_worker: mark failed failed taskID=%s: %v", msg.ID, err)
		return
	}
	q.appendExecutionLog(ctx, msg.ID, fmt.Sprintf("[系统] 任务执行失败，耗时: %d ms，错误原因: %v", duration.Milliseconds(), execErr))
}

func (q *InprocQueue) appendExecutionLog(ctx context.Context, taskID, logLine string) {
	db := getDB(ctx)
	if db == nil {
		return
	}
	now := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s\n", now, logLine)
	var exec taskExecution
	if err := db.Where("task_id = ?", taskID).First(&exec).Error; err != nil {
		return
	}
	if err := db.Model(&exec).Update("log", exec.Log+line).Error; err != nil {
		logger.ErrorF(ctx, "driver_inproc_worker: append log failed taskID=%s: %v", taskID, err)
	}
}
