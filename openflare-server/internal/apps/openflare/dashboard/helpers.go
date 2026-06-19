// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

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
)

func computeNodeStatus(node *model.OpenFlareNode) string {
	if node == nil {
		return nodeStatusOffline
	}
	if node.LastSeenAt == nil || node.LastSeenAt.IsZero() {
		return nodeStatusPending
	}
	if time.Since(*node.LastSeenAt) > model.NodeOfflineThreshold {
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
