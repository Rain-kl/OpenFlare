// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package relay

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/agent"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
)

const nodeStatusOnline = "online"

// ProxyStat describes a single frps proxy reported by the relay.
type ProxyStat struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Status        string `json:"status"`
	ClientVersion string `json:"client_version"`
	LastStartTime string `json:"last_start_time"`
	LastCloseTime string `json:"last_close_time"`
	ClientAddr    string `json:"client_addr"`
}

// HeartbeatPayload is sent by OpenFlareRelay on each heartbeat.
type HeartbeatPayload struct {
	Version         string                    `json:"version"`
	ExtVersion      string                    `json:"frp_version"`
	RelayStatus     string                    `json:"relay_status"`
	FrpsConnCount   int                       `json:"frps_connections"`
	FrpsProxyCount  int                       `json:"frps_proxy_count"`
	FrpsClientCount int                       `json:"frps_client_count"`
	FrpsProxies     []ProxyStat               `json:"frps_proxies,omitempty"`
	Name            string                    `json:"name"`
	IP              string                    `json:"ip"`
	Profile         *agent.NodeSystemProfile  `json:"profile,omitempty"`
	Snapshot        *agent.NodeMetricSnapshot `json:"snapshot,omitempty"`
	HealthEvents    []agent.NodeHealthEvent   `json:"health_events,omitempty"`
}

// Config is the frps configuration sent to the relay.
type Config struct {
	BindPort         int    `json:"bind_port"`
	VhostHTTPPort    int    `json:"vhost_http_port"`
	AuthToken        string `json:"auth_token"`
	LogLevel         string `json:"log_level"`
	WebServerEnabled bool   `json:"web_server_enabled"`
}

// Settings contains runtime settings for relay and flared clients.
type Settings struct {
	HeartbeatInterval       int    `json:"heartbeat_interval"`
	WebsocketUpgradeEnabled bool   `json:"websocket_upgrade_enabled"`
	AutoUpdate              bool   `json:"auto_update"`
	UpdateRepo              string `json:"update_repo"`
	UpdateNow               bool   `json:"update_now"`
	UpdateChannel           string `json:"update_channel"`
	UpdateTag               string `json:"update_tag"`
}

// HeartbeatResponse is returned from a relay heartbeat.
type HeartbeatResponse struct {
	RelayConfig   *Config   `json:"relay_config"`
	RelaySettings *Settings `json:"relay_settings"`
}

// Heartbeat processes a relay heartbeat, updates node status, and returns config.
func Heartbeat(ctx context.Context, node *model.OpenFlareNode, payload HeartbeatPayload) (*HeartbeatResponse, error) {
	if node == nil {
		return nil, fmt.Errorf("relay node is nil")
	}

	payload.Version = strings.TrimSpace(payload.Version)
	payload.ExtVersion = strings.TrimSpace(payload.ExtVersion)
	payload.RelayStatus = normalizeRelayStatus(payload.RelayStatus)
	payload.Name = strings.TrimSpace(payload.Name)
	payload.IP = strings.TrimSpace(payload.IP)

	previous := *node
	updateNow := node.UpdateRequested
	updateChannel := normalizeReleaseChannel(node.UpdateChannel)
	updateTag := strings.TrimSpace(node.UpdateTag)

	now := time.Now().UTC()
	changes := map[string]any{
		"version":          payload.Version,
		"ext_version":      payload.ExtVersion,
		"relay_status":     payload.RelayStatus,
		"last_seen_at":     now,
		"status":           nodeStatusOnline,
		"update_requested": false,
		"update_channel":   "stable",
		"update_tag":       "",
	}
	if payload.Name != "" && strings.TrimSpace(node.Name) == "" {
		changes["name"] = payload.Name
		node.Name = payload.Name
	}
	if payload.IP != "" && !node.IPManualOverride {
		changes["ip"] = payload.IP
		node.IP = payload.IP
	}
	if !previous.UpdateRequested {
		delete(changes, "update_requested")
	}
	if previous.UpdateChannel == "stable" {
		delete(changes, "update_channel")
	}
	if previous.UpdateTag == "" {
		delete(changes, "update_tag")
	}

	node.Version = payload.Version
	node.ExtVersion = payload.ExtVersion
	node.RelayStatus = payload.RelayStatus
	node.UpdateRequested = false
	node.UpdateChannel = "stable"
	node.UpdateTag = ""
	lastSeen := now
	node.LastSeenAt = &lastSeen
	node.Status = nodeStatusOnline

	if err := db.DB(ctx).Model(node).Updates(changes).Error; err != nil {
		return nil, fmt.Errorf("update relay heartbeat: %w", err)
	}
	if err := reconcileRelayHealthEvents(ctx, node.NodeID, payload.RelayStatus, now); err != nil {
		return nil, fmt.Errorf("reconcile relay health events: %w", err)
	}
	agent.RefreshAccessTokenCache(ctx, node)
	persistRelayHeartbeatObservability(ctx, node.NodeID, payload, now)

	return &HeartbeatResponse{
		RelayConfig:   buildRelayConfig(node),
		RelaySettings: BuildSettings(node, updateNow, updateChannel, updateTag),
	}, nil
}
