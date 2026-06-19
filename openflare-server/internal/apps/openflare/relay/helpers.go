// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package relay

import (
	"net"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
)

func normalizeRelayStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "healthy":
		return "healthy"
	case "unhealthy":
		return "unhealthy"
	default:
		return "unknown"
	}
}

func normalizeReleaseChannel(channel string) string {
	if strings.ToLower(strings.TrimSpace(channel)) == "preview" {
		return "preview"
	}
	return "stable"
}

func resolveReportedNodeIP(reportedIP string, remoteAddr string) string {
	reported := normalizeNodeIP(reportedIP)
	remote := normalizeRemoteAddr(remoteAddr)
	if reported == "" {
		return remote
	}
	if isPublicNodeIP(reported) {
		return reported
	}
	if isPublicNodeIP(remote) {
		return remote
	}
	return reported
}

func normalizeNodeIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}
	raw = strings.Trim(raw, "[]")
	return raw
}

func normalizeRemoteAddr(remoteAddr string) string {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return normalizeNodeIP(remoteAddr)
	}
	return normalizeNodeIP(host)
}

func isPublicNodeIP(raw string) bool {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return false
	}
	return true
}

func buildRelayConfig(node *model.OpenFlareNode) *Config {
	if node == nil {
		return nil
	}
	return &Config{
		BindPort:         node.RelayBindPort,
		VhostHTTPPort:    node.RelayVhostHTTPPort,
		AuthToken:        node.RelayAuthToken,
		LogLevel:         "info",
		WebServerEnabled: node.RelayWebServerEnabled,
	}
}

// BuildSettings returns runtime settings shared by relay and flared clients.
func BuildSettings(node *model.OpenFlareNode, updateNow bool, updateChannel, updateTag string) *Settings {
	autoUpdate := false
	if node != nil {
		autoUpdate = node.AutoUpdateEnabled
	}
	if strings.TrimSpace(updateChannel) == "" {
		updateChannel = "stable"
	}
	return &Settings{
		HeartbeatInterval:       model.AgentHeartbeatInterval,
		WebsocketUpgradeEnabled: model.AgentWebsocketUpgradeEnabled,
		AutoUpdate:              autoUpdate,
		UpdateRepo:              model.AgentUpdateRepo,
		UpdateNow:               updateNow,
		UpdateChannel:           updateChannel,
		UpdateTag:               strings.TrimSpace(updateTag),
	}
}
