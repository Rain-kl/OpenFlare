// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"sync"

	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

// AccessLogHooks 节点访问日志异步入队回调（由 chwriter 装配）。
type AccessLogHooks struct {
	QueueNodeAccessLogs func(logs []analyticsmodel.NodeAccessLog)
}

// ObservabilityHooks 可观测异步入队回调（由 chwriter 装配）。
type ObservabilityHooks struct {
	QueueMetricSnapshot func(record analyticsmodel.NodeMetricSnapshot)
	QueueEdgeHealth     func(record analyticsmodel.NodeEdgeHealth)
	QueueNodeObsFrps    func(record analyticsmodel.NodeObsFrps)
	QueueNodeObsFrpc    func(record analyticsmodel.NodeObsFrpc)
}

var (
	hooksMu            sync.RWMutex
	accessLogHooks     AccessLogHooks
	observabilityHooks ObservabilityHooks
)

// SetAccessLogHooks 注册节点访问日志异步入队回调。
func SetAccessLogHooks(h AccessLogHooks) {
	hooksMu.Lock()
	accessLogHooks = h
	hooksMu.Unlock()
}

// SetObservabilityHooks 注册可观测异步入队回调。
func SetObservabilityHooks(h ObservabilityHooks) {
	hooksMu.Lock()
	observabilityHooks = h
	hooksMu.Unlock()
}

// currentAccessLogHooks 返回当前 hooks 快照（未注册时为 zero value，调用方判空跳过）。
func currentAccessLogHooks() AccessLogHooks {
	hooksMu.RLock()
	defer hooksMu.RUnlock()
	return accessLogHooks
}

// currentObservabilityHooks 返回当前 hooks 快照（未注册时为 zero value，调用方判空跳过）。
func currentObservabilityHooks() ObservabilityHooks {
	hooksMu.RLock()
	defer hooksMu.RUnlock()
	return observabilityHooks
}
