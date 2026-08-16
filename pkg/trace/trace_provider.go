// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"context"
	"os"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func newTracerProvider(cfg Config) (*sdktrace.TracerProvider, error) {
	// 获取主机名和容器信息
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	// 业务属性不绑定 schema URL，合并时继承 resource.Default() 的 SDK 内置版本，避免 semconv 与 otel/sdk 升级不同步。
	r, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			attribute.String("service.name", cfg.AppName),
			attribute.String("host.name", hostname),
			attribute.String("k8s.namespace.name", os.Getenv("KUBERNETES_NAMESPACE")),
			attribute.String("k8s.pod.name", os.Getenv("KUBERNETES_POD_NAME")),
			attribute.String("k8s.pod.uid", os.Getenv("KUBERNETES_POD_UID")),
		),
	)
	if err != nil {
		return nil, err
	}

	// 初始化 Exporter
	traceExporter, err := otlptracegrpc.New(context.Background())
	if err != nil {
		return nil, err
	}

	// 初始化 Trace
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(r),
		sdktrace.WithSampler(ParentBasedRatioSampler(cfg.SamplingRate)),
	)
	return tracerProvider, nil
}
