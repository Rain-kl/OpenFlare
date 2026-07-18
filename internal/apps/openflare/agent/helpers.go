// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"strings"
	"time"

	ofgeoip "github.com/Rain-kl/Wavelet/internal/apps/openflare/geoip"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

const (
	openrestyStatusHealthy   = "healthy"
	openrestyStatusUnhealthy = "unhealthy"
	openrestyStatusUnknown   = "unknown"
	releaseChannelStable     = "stable"
	randomTokenBytes         = 16
	maxDatabaseTextLength    = 16000

	defaultAgentHeartbeatInterval = 3000 // 默认心跳间隔 3 秒（毫秒）
	defaultAgentUpdateRepo        = "Rain-kl/OpenFlare"
)

func newRandomToken() (string, error) {
	buf := make([]byte, randomTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func newServerNodeID() (string, error) {
	token, err := newRandomToken()
	if err != nil {
		return "", err
	}
	return "node-" + token, nil
}

func normalizeOpenrestyStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case openrestyStatusHealthy:
		return openrestyStatusHealthy
	case openrestyStatusUnhealthy:
		return openrestyStatusUnhealthy
	default:
		return openrestyStatusUnknown
	}
}

func normalizeNodePayload(payload NodePayload) NodePayload {
	payload.Name = strings.TrimSpace(payload.Name)
	payload.IP = strings.TrimSpace(payload.IP)
	payload.Version = strings.TrimSpace(payload.Version)
	payload.ExtVersion = strings.TrimSpace(payload.ExtVersion)
	payload.CurrentVersion = strings.TrimSpace(payload.CurrentVersion)
	payload.LastError = truncateForDatabase(payload.LastError, maxDatabaseTextLength)
	payload.OpenrestyStatus = normalizeOpenrestyStatus(payload.OpenrestyStatus)
	payload.OpenrestyMessage = truncateForDatabase(payload.OpenrestyMessage, maxDatabaseTextLength)
	// Align L2 edge_health with top-level status/message (PG is latest-state authority).
	if payload.EdgeHealth != nil {
		if s := strings.TrimSpace(payload.EdgeHealth.Status); s != "" {
			if payload.OpenrestyStatus == "" || payload.OpenrestyStatus == openrestyStatusUnknown {
				payload.OpenrestyStatus = normalizeOpenrestyStatus(s)
			}
		}
		if m := strings.TrimSpace(payload.EdgeHealth.Message); m != "" && payload.OpenrestyMessage == "" {
			payload.OpenrestyMessage = truncateForDatabase(m, maxDatabaseTextLength)
		}
		// CH series status must match the same authority as PG after normalize.
		payload.EdgeHealth.Status = payload.OpenrestyStatus
		payload.EdgeHealth.Message = payload.OpenrestyMessage
	}
	return payload
}

func validateNodePayload(payload NodePayload) error {
	if payload.IP == "" {
		return errPayload(errIPRequired)
	}
	if net.ParseIP(payload.IP) == nil {
		return errPayload(errIPInvalid)
	}
	if payload.Version == "" {
		return errPayload(errAgentVersionRequired)
	}
	return nil
}

type payloadError string

func (e payloadError) Error() string { return string(e) }

func errPayload(message string) error { return payloadError(message) }

func applyNodeRuntime(ctx context.Context, node *model.OpenFlareNode, payload NodePayload, preserveName bool) {
	if !preserveName || strings.TrimSpace(node.Name) == "" {
		if strings.TrimSpace(payload.Name) != "" {
			node.Name = strings.TrimSpace(payload.Name)
		}
	}
	if !node.IPManualOverride {
		node.IP = strings.TrimSpace(payload.IP)
	}
	node.Version = strings.TrimSpace(payload.Version)
	node.ExtVersion = strings.TrimSpace(payload.ExtVersion)
	node.OpenrestyStatus = normalizeOpenrestyStatus(payload.OpenrestyStatus)
	node.OpenrestyMessage = truncateForDatabase(payload.OpenrestyMessage, maxDatabaseTextLength)
	node.Status = nodeStatusOnline
	node.CurrentVersion = strings.TrimSpace(payload.CurrentVersion)
	now := time.Now()
	node.LastSeenAt = &now
	node.LastError = truncateForDatabase(payload.LastError, maxDatabaseTextLength)
	if !node.GeoManualOverride {
		ofgeoip.ApplyNodeGeoFromIP(ctx, node, node.IP)
	}
}

func truncateForDatabase(value string, maxVal int) string {
	if maxVal <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxVal {
		return string(runes)
	}
	return string(runes[:maxVal])
}

func resolveReportedNodeIP(reportedIP string, remoteAddr string) string {
	reported := normalizeIP(reportedIP)
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

func normalizeIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	host := raw
	if strings.Contains(raw, ":") {
		if h, _, err := net.SplitHostPort(raw); err == nil {
			host = h
		}
	}
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return ""
}

func normalizeRemoteAddr(remoteAddr string) string {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return normalizeIP(remoteAddr)
	}
	return normalizeIP(host)
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

func buildAgentSettings(ctx context.Context, node *model.OpenFlareNode, updateNow bool, updateChannel string, updateTag string, restartOpenrestyNow bool) *Settings {
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
		RestartOpenrestyNow:     restartOpenrestyNow,
	}
}

func collectHeartbeatChanges(previous *model.OpenFlareNode, current *model.OpenFlareNode) map[string]any {
	if previous == nil || current == nil {
		return map[string]any{}
	}
	changes := make(map[string]any)
	appendIfChanged := func(key string, before any, after any) {
		if before != after {
			changes[key] = after
		}
	}
	appendIfChanged("name", previous.Name, current.Name)
	appendIfChanged("ip", previous.IP, current.IP)
	appendIfChanged("geo_name", previous.GeoName, current.GeoName)
	appendIfChanged("version", previous.Version, current.Version)
	appendIfChanged("ext_version", previous.ExtVersion, current.ExtVersion)
	appendIfChanged("openresty_status", previous.OpenrestyStatus, current.OpenrestyStatus)
	appendIfChanged("openresty_message", previous.OpenrestyMessage, current.OpenrestyMessage)
	appendIfChanged("status", previous.Status, current.Status)
	appendIfChanged("current_version", previous.CurrentVersion, current.CurrentVersion)
	appendIfChanged("last_error", previous.LastError, current.LastError)
	appendIfChanged("update_requested", previous.UpdateRequested, current.UpdateRequested)
	appendIfChanged("update_channel", previous.UpdateChannel, current.UpdateChannel)
	appendIfChanged("update_tag", previous.UpdateTag, current.UpdateTag)
	appendIfChanged("restart_openresty_requested", previous.RestartOpenrestyRequested, current.RestartOpenrestyRequested)
	if !coordinatesEqual(previous.GeoLatitude, current.GeoLatitude) {
		changes["geo_latitude"] = current.GeoLatitude
	}
	if !coordinatesEqual(previous.GeoLongitude, current.GeoLongitude) {
		changes["geo_longitude"] = current.GeoLongitude
	}
	if !lastSeenAtEqual(previous.LastSeenAt, current.LastSeenAt) {
		changes["last_seen_at"] = current.LastSeenAt
	}
	return changes
}

func coordinatesEqual(before *float64, after *float64) bool {
	if before == nil || after == nil {
		return before == after
	}
	return *before == *after
}

func lastSeenAtEqual(before *time.Time, after *time.Time) bool {
	if before == nil || after == nil {
		return before == after
	}
	return before.Equal(*after)
}

func normalizeApplyLogPayload(payload ApplyLogPayload) ApplyLogPayload {
	payload.NodeID = strings.TrimSpace(payload.NodeID)
	payload.Version = strings.TrimSpace(payload.Version)
	payload.Result = strings.ToLower(strings.TrimSpace(payload.Result))
	payload.Message = truncateForDatabase(strings.TrimSpace(payload.Message), maxDatabaseTextLength)
	payload.Checksum = strings.TrimSpace(payload.Checksum)
	payload.MainConfigChecksum = strings.TrimSpace(payload.MainConfigChecksum)
	payload.RouteConfigChecksum = strings.TrimSpace(payload.RouteConfigChecksum)
	return payload
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique")
}

// RefreshAccessTokenCache updates the in-memory node cache after heartbeat mutations.
func RefreshAccessTokenCache(_ context.Context, node *model.OpenFlareNode) {
	if node == nil {
		return
	}
	tokenCache.storeNode(node.AccessToken, cloneNode(node))
}

func cloneNode(node *model.OpenFlareNode) *model.OpenFlareNode {
	if node == nil {
		return nil
	}
	cloned := *node
	return &cloned
}
