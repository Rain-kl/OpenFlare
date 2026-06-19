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
)

const (
	openrestyStatusHealthy   = "healthy"
	openrestyStatusUnhealthy = "unhealthy"
	openrestyStatusUnknown   = "unknown"
	releaseChannelStable     = "stable"
)

func newRandomToken() (string, error) {
	buf := make([]byte, 16)
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
	payload.LastError = truncateForDatabase(payload.LastError, 16000)
	payload.OpenrestyStatus = normalizeOpenrestyStatus(payload.OpenrestyStatus)
	payload.OpenrestyMessage = truncateForDatabase(payload.OpenrestyMessage, 16000)
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

func applyNodeRuntime(node *model.OpenFlareNode, payload NodePayload, preserveName bool) {
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
	node.OpenrestyMessage = truncateForDatabase(payload.OpenrestyMessage, 16000)
	node.Status = nodeStatusOnline
	node.CurrentVersion = strings.TrimSpace(payload.CurrentVersion)
	now := time.Now()
	node.LastSeenAt = &now
	node.LastError = truncateForDatabase(payload.LastError, 16000)
	if !node.GeoManualOverride {
		applyGeoInfoFromIP(node, node.IP)
	}
}

func applyGeoInfoFromIP(node *model.OpenFlareNode, rawIP string) {
	if node == nil {
		return
	}
	node.GeoName = ""
	node.GeoLatitude = nil
	node.GeoLongitude = nil
	ip := net.ParseIP(strings.TrimSpace(rawIP))
	if ip == nil {
		return
	}
	info, err := ofgeoip.GeoInfoFromIP(ip)
	if err != nil || info == nil {
		return
	}
	if strings.TrimSpace(info.Name) != "" {
		node.GeoName = strings.TrimSpace(info.Name)
	}
	if info.Latitude != nil && info.Longitude != nil {
		node.GeoLatitude = cloneCoordinate(info.Latitude)
		node.GeoLongitude = cloneCoordinate(info.Longitude)
	}
}

func cloneCoordinate(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func truncateForDatabase(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max])
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

func buildAgentSettings(node *model.OpenFlareNode, updateNow bool, updateChannel string, updateTag string, restartOpenrestyNow bool) *Settings {
	autoUpdate := false
	if node != nil {
		autoUpdate = node.AutoUpdateEnabled
	}
	if strings.TrimSpace(updateChannel) == "" {
		updateChannel = releaseChannelStable
	}
	return &Settings{
		HeartbeatInterval:       model.AgentHeartbeatInterval,
		WebsocketUpgradeEnabled: model.AgentWebsocketUpgradeEnabled,
		AutoUpdate:              autoUpdate,
		UpdateRepo:              model.AgentUpdateRepo,
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
	payload.Message = truncateForDatabase(strings.TrimSpace(payload.Message), 16000)
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
func RefreshAccessTokenCache(ctx context.Context, node *model.OpenFlareNode) {
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
