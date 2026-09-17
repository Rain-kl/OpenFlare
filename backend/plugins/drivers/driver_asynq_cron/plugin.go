// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package driver_asynq_cron provides the Asynq cron schedule driver plugin for Cordis.
package driver_asynq_cron

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/hibiken/asynq"
)

// Option configures the Asynq cron scheduler driver plugin.
type Option func(*Plugin)

// WithRedisOpt sets the Redis connection options for Asynq scheduler.
func WithRedisOpt(opt asynq.RedisConnOpt) Option {
	return func(p *Plugin) {
		p.redisOpt = opt
	}
}

// WithLocation sets the timezone location for the cron scheduler.
func WithLocation(loc *time.Location) Option {
	return func(p *Plugin) {
		p.location = loc
	}
}

// WithSchedulerOpts sets custom Asynq SchedulerOpts.
func WithSchedulerOpts(opts *asynq.SchedulerOpts) Option {
	return func(p *Plugin) {
		p.schedulerOpts = opts
	}
}

// WithScheduler injects a pre-configured Asynq Scheduler instance.
func WithScheduler(sched *asynq.Scheduler) Option {
	return func(p *Plugin) {
		p.scheduler = sched
	}
}

// Plugin implements core.Plugin and core.Driver for Asynq Cron Scheduler.
type Plugin struct {
	mu            sync.RWMutex
	redisOpt      asynq.RedisConnOpt
	location      *time.Location
	schedulerOpts *asynq.SchedulerOpts
	scheduler     *asynq.Scheduler
	running       bool
	coreCtx       *core.Context
}

// New creates a new Asynq Cron Scheduler driver plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{
		redisOpt: RedisOpt,
		location: time.Local,
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
	return "driver_asynq_cron"
}

type redisCronConfig struct {
	Enabled  bool     `config:"enabled" env:"REDIS_ENABLED" default:"false" autoEnable:"REDIS_ADDR"`
	Addrs    []string `config:"addrs" env:"REDIS_ADDR"`
	Username string   `config:"username" env:"REDIS_USERNAME"`
	Password string   `config:"password" env:"REDIS_PASSWORD" secret:"true"`
	DB       int      `config:"db" env:"REDIS_DB"`
}

// DeclareConfig declares configuration bindings for driver_asynq_cron.
func (p *Plugin) DeclareConfig() []core.ConfigBinding {
	return []core.ConfigBinding{
		{Prefix: "redis", Target: &redisCronConfig{}},
	}
}

// ConfigEnabled gates plugin activation when Redis is enabled.
func (p *Plugin) ConfigEnabled(view core.ConfigView) bool {
	return view.Bool("redis.enabled", false)
}

// Apply mounts the Asynq Cron Scheduler driver into the micro-kernel Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	var rCfg redisCronConfig
	_ = ctx.Config().Bind("redis", &rCfg)

	p.mu.Lock()
	p.coreCtx = ctx
	if p.redisOpt == nil {
		addr := "127.0.0.1:6379"
		if len(rCfg.Addrs) > 0 && rCfg.Addrs[0] != "" {
			addr = rCfg.Addrs[0]
		}
		p.redisOpt = asynq.RedisClientOpt{
			Addr:     addr,
			Username: rCfg.Username,
			Password: rCfg.Password,
			DB:       rCfg.DB,
		}
	}
	p.mu.Unlock()

	core.Bind[contracts.DBService](ctx, setDBService)
	core.Bind[contracts.TaskService](ctx, setTaskService)

	ctx.OnDispose(func() error {
		setDBService(nil)
		setTaskService(nil)
		return nil
	})

	ctx.OnDispose(func() error {
		return p.Stop(context.Background())
	})

	return ctx.RegisterDriver(p)
}

// Type returns DriverTypeScheduler.
func (p *Plugin) Type() core.DriverType {
	return core.DriverTypeScheduler
}

// Start registers scheduled cron entries from Context and boots the Asynq scheduler.
func (p *Plugin) Start(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return nil
	}

	if p.scheduler == nil {
		p.scheduler = p.initScheduler()
	}

	if p.coreCtx != nil && p.coreCtx.Schedules() != nil {
		for _, sd := range p.coreCtx.Schedules().Schedules() {
			payloadBytes, err := encodePayload(sd.Payload)
			if err != nil {
				return fmt.Errorf("driver_asynq_cron: encode payload for task %q failed: %w", sd.TaskType, err)
			}

			task := asynq.NewTask(sd.TaskType, payloadBytes)
			asynqOpts := buildAsynqOptions(sd.Options)

			if _, err := p.scheduler.Register(sd.Spec, task, asynqOpts...); err != nil {
				return fmt.Errorf("driver_asynq_cron: register schedule %q for task %q failed: %w", sd.Spec, sd.TaskType, err)
			}
		}
	}

	if err := p.scheduler.Start(); err != nil {
		return fmt.Errorf("driver_asynq_cron: start scheduler failed: %w", err)
	}

	p.running = true
	return nil
}

// Stop gracefully shuts down the Asynq cron scheduler.
func (p *Plugin) Stop(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	p.running = false

	if p.scheduler != nil {
		p.scheduler.Shutdown()
		p.scheduler = nil
	}

	return nil
}

// IsRunning returns whether the scheduler is running.
func (p *Plugin) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// Scheduler returns the underlying Asynq scheduler instance.
func (p *Plugin) Scheduler() *asynq.Scheduler {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.scheduler
}

func encodePayload(payload any) ([]byte, error) {
	if payload == nil {
		return nil, nil
	}

	switch val := payload.(type) {
	case []byte:
		return val, nil
	case string:
		return []byte(val), nil
	default:
		return json.Marshal(val)
	}
}

func buildAsynqOptions(opts map[string]any) []asynq.Option {
	if len(opts) == 0 {
		return nil
	}

	var res []asynq.Option

	if q, ok := opts["queue"].(string); ok && q != "" {
		res = append(res, asynq.Queue(q))
	}

	if retry, ok := opts["retry"].(int); ok {
		res = append(res, asynq.MaxRetry(retry))
	} else if maxRetry, ok := opts["max_retry"].(int); ok {
		res = append(res, asynq.MaxRetry(maxRetry))
	}

	if timeout, ok := opts["timeout"].(time.Duration); ok {
		res = append(res, asynq.Timeout(timeout))
	}

	if retention, ok := opts["retention"].(time.Duration); ok {
		res = append(res, asynq.Retention(retention))
	}

	return res
}

func (p *Plugin) initScheduler() *asynq.Scheduler {
	opts := p.schedulerOpts
	if opts == nil {
		opts = &asynq.SchedulerOpts{
			Location: p.location,
		}
	} else if opts.Location == nil && p.location != nil {
		opts.Location = p.location
	}

	opt := p.resolveRedisOpt()
	return asynq.NewScheduler(opt, opts)
}

func (p *Plugin) resolveRedisOpt() asynq.RedisConnOpt {
	if p.redisOpt != nil {
		return p.redisOpt
	}
	if RedisOpt != nil {
		return RedisOpt
	}
	return asynq.RedisClientOpt{
		Addr: "127.0.0.1:6379",
	}
}
