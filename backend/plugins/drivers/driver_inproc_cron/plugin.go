// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package driver_inproc_cron provides the zero-dependency in-process cron scheduler driver plugin for Cordis.
package driver_inproc_cron

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"context"
	"sync"
)

// Plugin implements core.Plugin and core.Driver for in-process cron job scheduling.
type Plugin struct {
	mu        sync.RWMutex
	coreCtx   *core.Context
	scheduler *inprocScheduler
}

// New creates a new in-process cron scheduler driver plugin.
func New() *Plugin {
	return &Plugin{}
}

// Name returns the unique identifier of the plugin.
func (p *Plugin) Name() string {
	return "driver_inproc_cron"
}

// Manifest returns the plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        "driver_inproc_cron",
		Version:     "1.0.0",
		Description: "Zero-dependency in-process cron scheduler driver plugin",
		Author:      "Wavelet Team",
	}
}

type redisGateConfig struct {
	Enabled bool `config:"enabled" env:"REDIS_ENABLED" default:"false" autoEnable:"REDIS_ADDR"`
}

// DeclareConfig declares configuration bindings for driver_inproc_cron.
func (p *Plugin) DeclareConfig() []core.ConfigBinding {
	return []core.ConfigBinding{
		{Prefix: "redis", Target: &redisGateConfig{}},
	}
}

// ConfigEnabled gates plugin activation when Redis is disabled.
func (p *Plugin) ConfigEnabled(view core.ConfigView) bool {
	return !view.Bool("redis.enabled", false)
}

// Apply registers the scheduler driver into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	p.mu.Lock()
	p.coreCtx = ctx
	p.mu.Unlock()

	ctx.OnDispose(func() error {
		return p.Stop(context.Background())
	})

	return ctx.RegisterDriver(p)
}

// Type returns DriverTypeScheduler.
func (p *Plugin) Type() core.DriverType {
	return core.DriverTypeScheduler
}

// Start boots the in-process cron scheduler. ctx is the app-lifetime context;
// cron-dispatched tasks carry it so cancellation propagates on shutdown.
func (p *Plugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.scheduler == nil {
		taskSvc, _ := core.Inject[contracts.TaskService](p.coreCtx)
		p.scheduler = newInprocScheduler(p.coreCtx.Schedules(), p.coreCtx.Tasks(), taskSvc)
	}

	runCtx := ctx
	if p.coreCtx != nil {
		runCtx = core.WithAppContext(ctx, p.coreCtx.Root())
	}
	return p.scheduler.Start(runCtx)
}

// Stop terminates the in-process cron scheduler.
func (p *Plugin) Stop(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.scheduler != nil {
		p.scheduler.Stop()
		p.scheduler = nil
	}
	return nil
}
