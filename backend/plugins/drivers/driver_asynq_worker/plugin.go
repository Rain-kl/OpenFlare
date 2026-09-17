// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package driver_asynq_worker provides the Asynq worker driver plugin for Cordis.
package driver_asynq_worker

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

const (
	defaultConcurrency     = 10
	defaultShutdownTimeout = 10 * time.Second
)

// Option configures the Asynq worker driver plugin.
type Option func(*Plugin)

// WithRedisOpt sets the Redis connection options for Asynq.
func WithRedisOpt(opt asynq.RedisConnOpt) Option {
	return func(p *Plugin) {
		p.redisOpt = opt
	}
}

// WithConcurrency sets the worker concurrency limit.
func WithConcurrency(concurrency int) Option {
	return func(p *Plugin) {
		p.concurrency = concurrency
	}
}

// WithQueues sets the queue priorities mapping.
func WithQueues(queues map[string]int) Option {
	return func(p *Plugin) {
		p.queues = queues
	}
}

// WithStrictPriority sets whether to process queues strictly in priority order.
func WithStrictPriority(strict bool) Option {
	return func(p *Plugin) {
		p.strictPriority = strict
	}
}

// WithShutdownTimeout sets the timeout for graceful worker shutdown.
func WithShutdownTimeout(d time.Duration) Option {
	return func(p *Plugin) {
		p.shutdownTimeout = d
	}
}

// WithServer injects an existing Asynq server instance.
func WithServer(srv *asynq.Server) Option {
	return func(p *Plugin) {
		p.server = srv
	}
}

// Plugin implements core.Plugin and core.Driver for Asynq background worker server.
type Plugin struct {
	mu              sync.RWMutex
	redisOpt        asynq.RedisConnOpt
	concurrency     int
	queues          map[string]int
	strictPriority  bool
	shutdownTimeout time.Duration
	server          *asynq.Server
	mux             *asynq.ServeMux
	running         bool
	coreCtx         *core.Context
	taskSvc         contracts.TaskService
}

// New creates a new Asynq Worker driver plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{
		concurrency:     defaultConcurrency,
		shutdownTimeout: defaultShutdownTimeout,
		queues:          map[string]int{"default": 1},
	}

	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}

	return p
}

// Name returns the unique plugin identifier.
func (p *Plugin) Name() string {
	return "driver_asynq_worker"
}

// DeclareConfig declares configuration bindings consumed by the Asynq worker driver.
func (p *Plugin) DeclareConfig() []core.ConfigBinding {
	return []core.ConfigBinding{
		{Prefix: "worker", Target: &workerConfig{}},
		{Prefix: "redis", Target: &redisWorkerConfig{}},
	}
}

// ConfigEnabled gates plugin activation when Redis is enabled.
func (p *Plugin) ConfigEnabled(view core.ConfigView) bool {
	return view.Bool("redis.enabled", false)
}

// Apply mounts the Asynq Worker driver into the micro-kernel Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	var wCfg workerConfig
	_ = ctx.Config().Bind("worker", &wCfg)
	var rCfg redisWorkerConfig
	_ = ctx.Config().Bind("redis", &rCfg)

	p.mu.Lock()
	p.coreCtx = ctx
	if p.concurrency == defaultConcurrency && wCfg.Concurrency > 0 {
		p.concurrency = wCfg.Concurrency
	}
	p.strictPriority = wCfg.StrictPriority
	if len(p.queues) == 1 && p.queues["default"] == 1 && len(wCfg.Queues) > 0 {
		qMap := make(map[string]int, len(wCfg.Queues))
		for _, q := range wCfg.Queues {
			qMap[q.Name] = q.Priority
		}
		p.queues = qMap
	}
	if p.redisOpt == nil {
		p.redisOpt = NewRedisConnOptWithConfig(rCfg)
	}
	RedisOpt = p.redisOpt
	ResetAsynqClient()
	p.mu.Unlock()

	core.Bind[contracts.DBService](ctx, setDBService)
	ctx.OnDispose(func() error {
		setDBService(nil)
		return nil
	})

	// 1. Provide contracts.TaskService
	p.taskSvc = &taskServiceImpl{}
	core.Provide[contracts.TaskService](ctx, p.taskSvc)
	SetActiveTaskExtension(ctx.Tasks())

	ctx.OnDispose(func() error {
		SetActiveTaskExtension(nil)
		SetRedisClient(nil)
		ResetAsynqClient()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), p.shutdownTimeout)
		defer cancel()
		return p.Stop(shutdownCtx)
	})

	return ctx.RegisterDriver(p)
}

// Type returns DriverTypeWorker.
func (p *Plugin) Type() core.DriverType {
	return core.DriverTypeWorker
}

// Start boots the Asynq worker server and starts processing background tasks.
func (p *Plugin) Start(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return nil
	}

	mux := asynq.NewServeMux()

	if p.coreCtx != nil && p.coreCtx.Tasks() != nil {
		appCtx := p.coreCtx.Root()
		for _, td := range p.coreCtx.Tasks().Tasks() {
			handler, err := toAsynqHandler(td.Pattern, td.Handler)
			if err != nil {
				return fmt.Errorf("driver_asynq_worker: invalid handler for task pattern %q: %w", td.Pattern, err)
			}
			mux.Handle(td.Pattern, asynq.HandlerFunc(func(c context.Context, t *asynq.Task) error {
				return handler.ProcessTask(core.WithAppContext(c, appCtx), t)
			}))
		}
	}

	opt := p.redisOpt
	if opt == nil {
		opt = NewRedisConnOpt()
	}
	RedisOpt = opt

	if getRedisClient() == nil {
		if mk, ok := opt.(interface{ MakeRedisClient() interface{} }); ok {
			if client, ok := mk.MakeRedisClient().(redis.UniversalClient); ok {
				SetRedisClient(client)
			}
		}
	}

	if p.server == nil {
		p.server = asynq.NewServer(
			opt,
			asynq.Config{
				Concurrency:     p.concurrency,
				Queues:          p.queues,
				StrictPriority:  p.strictPriority,
				ShutdownTimeout: p.shutdownTimeout,
			},
		)
	}

	if err := p.server.Start(mux); err != nil {
		return fmt.Errorf("driver_asynq_worker: start server failed: %w", err)
	}

	p.mux = mux
	p.running = true
	return nil
}

// Stop gracefully shuts down the Asynq worker server.
func (p *Plugin) Stop(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	p.running = false

	if p.server != nil {
		p.server.Stop()
		p.server.Shutdown()
		p.server = nil
	}

	return nil
}

// IsRunning returns whether the worker server is running.
func (p *Plugin) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// Server returns the underlying Asynq server instance.
func (p *Plugin) Server() *asynq.Server {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.server
}

// Mux returns the underlying Asynq serve mux.
func (p *Plugin) Mux() *asynq.ServeMux {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mux
}

func toAsynqHandler(pattern string, h any) (asynq.Handler, error) {
	if h == nil {
		return nil, errors.New("nil handler")
	}

	if th, ok := h.(TaskHandler); ok {
		RegisterHandler(pattern, th)
		return asynq.HandlerFunc(ProcessTask), nil
	}
	if th, ok := h.(contracts.TaskHandler); ok {
		RegisterHandler(pattern, contractTaskAdapter{inner: th})
		return asynq.HandlerFunc(ProcessTask), nil
	}
	if fn, ok := h.(func(context.Context, []byte) (*contracts.TaskResultDTO, error)); ok {
		RegisterHandler(pattern, contractFuncAdapter{fn: fn})
		return asynq.HandlerFunc(ProcessTask), nil
	}

	inner, err := toRawAsynqHandler(h)
	if err != nil {
		return nil, err
	}
	RegisterHandler(pattern, &asynqHandlerAdapter{inner: inner})
	return asynq.HandlerFunc(ProcessTask), nil
}

type contractTaskAdapter struct {
	inner contracts.TaskHandler
}

func (a contractTaskAdapter) Execute(ctx context.Context, payload []byte) (*TaskResult, error) {
	res, err := a.inner.Execute(ctx, payload)
	if err != nil {
		return nil, err
	}
	return dtoToTaskResult(res), nil
}

func (a contractTaskAdapter) ValidatePayload(payload []byte) ([]byte, error) {
	if v, ok := a.inner.(PayloadValidator); ok {
		return v.ValidatePayload(payload)
	}
	return payload, nil
}

type contractFuncAdapter struct {
	fn func(context.Context, []byte) (*contracts.TaskResultDTO, error)
}

func (a contractFuncAdapter) Execute(ctx context.Context, payload []byte) (*TaskResult, error) {
	res, err := a.fn(ctx, payload)
	if err != nil {
		return nil, err
	}
	return dtoToTaskResult(res), nil
}

func dtoToTaskResult(res *contracts.TaskResultDTO) *TaskResult {
	if res == nil {
		return &TaskResult{Message: "ok"}
	}
	out := &TaskResult{Message: res.Message}
	if res.Detail == nil {
		return out
	}
	if s, ok := res.Detail.(string); ok {
		out.Detail = s
		return out
	}
	b, err := json.Marshal(res.Detail)
	if err != nil {
		out.Detail = fmt.Sprint(res.Detail)
		return out
	}
	out.Detail = string(b)
	return out
}

func toRawAsynqHandler(h any) (asynq.Handler, error) {
	switch fn := h.(type) {
	case asynq.HandlerFunc:
		return fn, nil
	case asynq.Handler:
		return fn, nil
	case func(context.Context, *asynq.Task) error:
		return asynq.HandlerFunc(fn), nil
	case func(context.Context, []byte) error:
		return asynq.HandlerFunc(func(c context.Context, t *asynq.Task) error {
			c, payload, _ := extractTaskTraceContext(c, t.Payload())
			return fn(c, payload)
		}), nil
	case func(context.Context) error:
		return asynq.HandlerFunc(func(c context.Context, _ *asynq.Task) error {
			return fn(c)
		}), nil
	case func([]byte) error:
		return asynq.HandlerFunc(func(c context.Context, t *asynq.Task) error {
			_, payload, _ := extractTaskTraceContext(c, t.Payload())
			return fn(payload)
		}), nil
	case func() error:
		return asynq.HandlerFunc(func(_ context.Context, _ *asynq.Task) error {
			return fn()
		}), nil
	default:
		return nil, fmt.Errorf("unsupported task handler type: %T", h)
	}
}

type asynqTaskCtxKey struct{}

type asynqHandlerAdapter struct {
	inner asynq.Handler
}

func (a *asynqHandlerAdapter) Execute(ctx context.Context, payload []byte) (*TaskResult, error) {
	t, _ := ctx.Value(asynqTaskCtxKey{}).(*asynq.Task)
	if t == nil {
		t = asynq.NewTask("", payload)
	}
	if err := a.inner.ProcessTask(ctx, t); err != nil {
		return nil, err
	}
	return &TaskResult{Message: "ok"}, nil
}

type taskServiceImpl struct{}

func (s *taskServiceImpl) Dispatch(ctx context.Context, taskType string, payload []byte, triggeredBy string) (string, error) {
	return DispatchTask(ctx, taskType, payload, triggeredBy)
}

func (s *taskServiceImpl) ListTasks() []contracts.TaskMetaDTO {
	all := GetDispatchableTasks()
	res := make([]contracts.TaskMetaDTO, 0, len(all))
	for _, m := range all {
		params := make([]contracts.TaskParamDTO, 0, len(m.Params))
		for _, param := range m.Params {
			params = append(params, contracts.TaskParamDTO{
				Name:        param.Name,
				Label:       param.Label,
				Type:        param.Type,
				Description: param.Description,
				Placeholder: param.Placeholder,
				Required:    param.Required,
			})
		}
		taskType := m.Type
		if taskType == "" {
			taskType = m.AsynqTask
		}
		if taskType == "" {
			taskType = m.Name
		}
		asynqTask := m.AsynqTask
		if asynqTask == "" {
			asynqTask = taskType
		}
		name := m.Name
		if name == "" {
			name = taskType
		}
		res = append(res, contracts.TaskMetaDTO{
			Type:         taskType,
			AsynqTask:    asynqTask,
			Name:         name,
			DisplayName:  name,
			Description:  m.Description,
			Category:     m.Category,
			SupportsTime: m.SupportsTime,
			Params:       params,
			MaxRetry:     m.MaxRetry,
			Queue:        m.Queue,
			Retryable:    m.Retryable,
		})
	}
	return res
}

func (s *taskServiceImpl) GetTaskMeta(taskType string) (contracts.TaskMetaDTO, bool) {
	m := GetTaskMeta(taskType)
	if m == nil {
		return contracts.TaskMetaDTO{}, false
	}
	params := make([]contracts.TaskParamDTO, 0, len(m.Params))
	for _, param := range m.Params {
		params = append(params, contracts.TaskParamDTO{
			Name:        param.Name,
			Label:       param.Label,
			Type:        param.Type,
			Description: param.Description,
			Placeholder: param.Placeholder,
			Required:    param.Required,
		})
	}
	tType := m.Type
	if tType == "" {
		tType = m.AsynqTask
	}
	if tType == "" {
		tType = m.Name
	}
	asynqTask := m.AsynqTask
	if asynqTask == "" {
		asynqTask = tType
	}
	name := m.Name
	if name == "" {
		name = tType
	}
	return contracts.TaskMetaDTO{
		Type:         tType,
		AsynqTask:    asynqTask,
		Name:         name,
		DisplayName:  name,
		Description:  m.Description,
		Category:     m.Category,
		SupportsTime: m.SupportsTime,
		Params:       params,
		MaxRetry:     m.MaxRetry,
		Queue:        m.Queue,
		Retryable:    m.Retryable,
	}, true
}

func (s *taskServiceImpl) ListExecutions(ctx context.Context, taskType, status string, page, pageSize int) ([]contracts.TaskExecutionDTO, int64, error) {
	db := getDB(ctx)
	if db == nil {
		return nil, 0, errors.New("db not initialized")
	}
	query := db.Model(&TaskExecution{})
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
	var rows []TaskExecution
	offset := (page - 1) * pageSize
	if err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	res := make([]contracts.TaskExecutionDTO, 0, len(rows))
	for i := range rows {
		res = append(res, toTaskExecutionDTO(&rows[i]))
	}
	return res, total, nil
}

func (s *taskServiceImpl) Retry(ctx context.Context, id uint64) (string, error) {
	return RetryTask(ctx, id)
}

func (s *taskServiceImpl) ValidatePayload(taskType string, payload []byte) ([]byte, error) {
	meta := GetTaskMeta(taskType)
	if meta == nil {
		return payload, nil
	}
	return ValidateAndNormalizePayload(meta.AsynqTask, payload)
}

func (s *taskServiceImpl) ReloadScheduler() error {
	return nil
}

func (s *taskServiceImpl) AppendLog(ctx context.Context, format string, args ...any) {
	AppendLog(ctx, format, args...)
}

func (s *taskServiceImpl) GetExecution(ctx context.Context, id uint64) (*contracts.TaskExecutionDTO, error) {
	exec, err := GetTaskExecutionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	dto := toTaskExecutionDTO(exec)
	return &dto, nil
}

func (s *taskServiceImpl) GetExecutionByTaskID(ctx context.Context, taskID string) (*contracts.TaskExecutionDTO, error) {
	exec, err := GetTaskExecutionByTaskID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	dto := toTaskExecutionDTO(exec)
	return &dto, nil
}

func toTaskExecutionDTO(exec *TaskExecution) contracts.TaskExecutionDTO {
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
