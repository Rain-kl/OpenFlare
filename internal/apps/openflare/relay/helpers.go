// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package relay

import (
	"context"
	"net"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

const (
	relayStatusUnhealthy = "unhealthy"
	releaseChannelStable = "stable"

	defaultAgentHeartbeatInterval = 3000 // 默认心跳间隔 3 秒（毫秒）
	defaultAgentUpdateRepo        = "Rain-kl/OpenFlare"
)

func normalizeRelayStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "healthy":
		return "healthy"
	case relayStatusUnhealthy:
		return relayStatusUnhealthy
	default:
		return "unknown"
	}
}

func normalizeReleaseChannel(channel string) string {
	if strings.ToLower(strings.TrimSpace(channel)) == "preview" {
		return "preview"
	}
	return releaseChannelStable
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

func buildRelayConfig(ctx context.Context, node *model.OpenFlareNode) *Config {
	if node == nil {
		return nil
	}
	webServerPort, err := repository.GetIntByKey(ctx, model.ConfigKeyRelayFRPSWebUIPort)
	if err != nil || webServerPort <= 0 {
		webServerPort = node.RelayBindPort + 500
	}
	return &Config{
		BindPort:         node.RelayBindPort,
		VhostHTTPPort:    node.RelayVhostHTTPPort,
		AuthToken:        node.RelayAuthToken,
		LogLevel:         "info",
		WebServerEnabled: node.RelayWebServerEnabled,
		WebServerPort:    webServerPort,
	}
}

// BuildSettings returns runtime settings shared by relay and flared clients.
func BuildSettings(ctx context.Context, node *model.OpenFlareNode, updateNow bool, updateChannel, updateTag string) *Settings {
	autoUpdate := false
	if node != nil {
		autoUpdate = node.AutoUpdateEnabled
	}
	if strings.TrimSpace(updateChannel) == "" {
		updateChannel = releaseChannelStable
	}

	// 从 SystemConfig 读取配置，使用默认值作为降级
	heartbeatInterval, _ := repository.GetIntByKey(ctx, model.ConfigKeyAgentHeartbeatInterval)
	if heartbeatInterval <= 0 {
		heartbeatInterval = defaultAgentHeartbeatInterval
	}
	wsUpgradeEnabled, _ := repository.GetBoolByKey(ctx, model.ConfigKeyAgentWebsocketUpgradeEnabled)
	updateRepo, _ := repository.GetSystemConfigByKey(ctx, model.ConfigKeyAgentUpdateRepo)
	if strings.TrimSpace(updateRepo.Value) == "" {
		updateRepo.Value = defaultAgentUpdateRepo
	}

	return &Settings{
		HeartbeatInterval:       heartbeatInterval,
		WebsocketUpgradeEnabled: wsUpgradeEnabled,
		AutoUpdate:              autoUpdate,
		UpdateRepo:              updateRepo.Value,
		UpdateNow:               updateNow,
		UpdateChannel:           updateChannel,
		UpdateTag:               strings.TrimSpace(updateTag),
	}
}
