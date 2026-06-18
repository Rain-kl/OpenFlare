// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Tracer 全局 OpenTelemetry Tracer 实例
var Tracer trace.Tracer
var shutdownFuncs []func(context.Context) error

func init() {
	// 初始化 Propagator
	prop := newPropagator()
	otel.SetTextMapPropagator(prop)

	// 初始化 Tracer 实例为 No-op 默认以避免未初始化前或测试环境崩溃
	Tracer = otel.GetTracerProvider().Tracer("github.com/Rain-kl/OpenFlare")
}

// Config 链路追踪配置
type Config struct {
	AppName      string
	SamplingRate float64
	TracerName   string
}

// Init 初始化 Tracer Provider 并关联全局 Tracer 实例
func Init(cfg Config) {
	tracerProvider, err := newTracerProvider(cfg)
	if err != nil {
		log.Fatalf("[Trace] init trace provider failed: %v", err)
	}
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)
	otel.SetTracerProvider(tracerProvider)

	// 更新 Tracer
	tracerName := cfg.TracerName
	if tracerName == "" {
		tracerName = "github.com/Rain-kl/OpenFlare"
	}
	Tracer = tracerProvider.Tracer(tracerName)
}

// Shutdown 关闭所有 Trace Provider
func Shutdown(ctx context.Context) {
	for _, fn := range shutdownFuncs {
		_ = fn(ctx)
	}
}

// Start 创建一个新的 Trace Span
func Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return Tracer.Start(ctx, name, opts...)
}
