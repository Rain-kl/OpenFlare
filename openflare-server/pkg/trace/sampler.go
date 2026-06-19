// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// ParentBasedRatioSampler 创建父级感知的概率采样器
// - 如果父 Span 已采样，则子 Span 也采样
// - 如果父 Span 未采样，则子 Span 也不采样
// - 如果是根 Span，按 samplingRate 概率采样
func ParentBasedRatioSampler(samplingRate float64) sdktrace.Sampler {
	return sdktrace.ParentBased(
		sdktrace.TraceIDRatioBased(samplingRate),
	)
}
