// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"log/slog"

	ofws "github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// HandleWSStatus processes an agent websocket status payload (replaces HTTP heartbeat in WS mode).
func HandleWSStatus(ctx context.Context, nodeID, remoteAddr string, rawPayload json.RawMessage) {
	var payload NodePayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		slog.Debug("agent ws status payload decode failed", "node_id", nodeID, "error", err)
		return
	}

	authNode, err := model.GetOpenFlareNodeByNodeID(ctx, nodeID)
	if err != nil {
		slog.Debug("agent ws status reload node failed", "node_id", nodeID, "error", err)
		return
	}

	payload.IP = resolveReportedNodeIP(payload.IP, remoteAddr)
	response, err := HeartbeatNode(ctx, authNode, payload)
	if err != nil {
		slog.Debug("agent ws status handling failed", "node_id", nodeID, "error", err)
		return
	}

	settingsSent := false
	if response.AgentSettings != nil {
		settingsSent = ofws.SendAgentSettings(nodeID, response.AgentSettings)
	}
	activeConfigSent := false
	if response.ActiveConfig != nil {
		activeConfigSent = ofws.SendAgentActiveConfig(nodeID, response.ActiveConfig)
	}
	wafIPGroupsSent := false
	if len(response.WAFIPGroups) > 0 {
		wafIPGroupsSent = ofws.SendAgentWAFIPGroups(nodeID, response.WAFIPGroups)
	}

	slog.Debug("agent ws status processed",
		"node_id", nodeID,
		"current_version", payload.CurrentVersion,
		"openresty_status", payload.OpenrestyStatus,
		"settings_sent", settingsSent,
		"active_config_sent", activeConfigSent,
		"waf_ip_groups_sent", wafIPGroupsSent,
	)
}
