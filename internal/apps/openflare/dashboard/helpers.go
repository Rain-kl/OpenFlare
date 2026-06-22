// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package dashboard provides helper utilities for dashboard API handlers.
package dashboard

import (
	"strings"
	"time"

	ofws "github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/model"
)

const (
	nodeStatusOnline  = "online"
	nodeStatusOffline = "offline"
	nodeStatusPending = "pending"

	dashboardDistributionLimit       = 8
	highCPUUsagePercentThreshold     = 80
	highMemoryUsagePercentThreshold  = 85
	highStorageUsagePercentThreshold = 85
)

func computeNodeStatus(node *model.OpenFlareNode) string {
	if node == nil {
		return nodeStatusOffline
	}
	if node.LastSeenAt == nil || node.LastSeenAt.IsZero() {
		return nodeStatusPending
	}
	// 使用默认阈值 2 分钟
	threshold := 2 * time.Minute
	if time.Since(*node.LastSeenAt) > threshold {
		return nodeStatusOffline
	}
	return nodeStatusOnline
}

func nodeViewLastSeenAt(node *model.OpenFlareNode) any {
	if node == nil {
		return time.Time{}
	}
	nodeType := strings.TrimSpace(node.NodeType)
	if nodeType == "" {
		nodeType = "edge_node"
	}
	if nodeType == "tunnel_relay" && ofws.IsRelayConnected(node.NodeID) {
		return ofws.RelayWSConnectedLastSeenValue
	}
	if nodeType == "tunnel_client" && ofws.IsFlaredConnected(node.NodeID) {
		return ofws.FlaredWSConnectedLastSeenValue
	}
	if ofws.IsAgentConnected(node.NodeID) {
		return ofws.AgentWSConnectedLastSeenValue
	}
	if node.LastSeenAt == nil {
		return time.Time{}
	}
	return *node.LastSeenAt
}
