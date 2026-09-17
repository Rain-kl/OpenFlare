// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package cache_memory provides the in-memory cache infrastructure plugin for Cordis.
package cache_memory

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
)

const defaultRAMCapacity = 10000

// Option configures the cache_memory plugin.
type Option func(*Plugin)

// WithCapacity sets the maximum capacity for the in-memory cache.
func WithCapacity(capacity int) Option {
	return func(p *Plugin) {
		p.capacity = capacity
	}
}

// Plugin implements core.Plugin to provide contracts.CacheService using in-memory storage.
type Plugin struct {
	capacity int
}

// New creates a new in-memory cache infrastructure plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{
		capacity: defaultRAMCapacity,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// Name returns the unique identifier of the cache_memory plugin.
func (p *Plugin) Name() string {
	return "cache_memory"
}

// Manifest returns the plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        "cache_memory",
		Version:     "1.0.0",
		Description: "Zero-dependency pure in-memory cache infrastructure plugin",
		Author:      "Wavelet Team",
	}
}

// redisGateConfig declares the Redis gate configuration for cache_memory.
type redisGateConfig struct {
	Enabled bool `config:"enabled" env:"REDIS_ENABLED" default:"false" autoEnable:"REDIS_ADDR"`
}

// DeclareConfig declares the configuration bindings consumed by this plugin.
func (p *Plugin) DeclareConfig() []core.ConfigBinding {
	return []core.ConfigBinding{
		{Prefix: "redis", Target: &redisGateConfig{}},
	}
}

// ConfigEnabled gates plugin activation when Redis is disabled.
func (p *Plugin) ConfigEnabled(view core.ConfigView) bool {
	return !view.Bool("redis.enabled", false)
}

// Apply mounts the in-memory cache service into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	svc, err := newMemoryCacheService(p.capacity, ctx.Events())
	if err != nil {
		return err
	}

	core.Provide[contracts.CacheService](ctx, svc)
	core.Provide[contracts.LimiterService](ctx, newMemoryLimiterService())
	return nil
}
