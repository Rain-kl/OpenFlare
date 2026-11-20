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

	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"

	ofgeoip "github.com/Rain-kl/Wavelet/internal/apps/openflare/geoip"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/geoip/iputil"
)

const (
	openrestyStatusHealthy   = "healthy"
	openrestyStatusUnhealthy = "unhealthy"
	openrestyStatusUnknown   = "unknown"
	releaseChannelStable     = "stable"
	randomTokenBytes         = 16
	maxDatabaseTextLength    = 16000

	defaultAgentHeartbeatInterval = 3000 // 默认心跳间隔 3 秒（毫秒）
	enginePingora                 = "pingora"
	engineOpenResty               = "openresty"
	nodeTypeEdge                  = "edge_node"
	unspecifiedNodeIP             = "0.0.0.0"
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

func normalizeNodePayload(payload protocol.NodePayload) protocol.NodePayload {
	payload.Name = strings.TrimSpace(payload.Name)
	payload.IP = strings.TrimSpace(payload.IP)
	payload.Version = strings.TrimSpace(payload.Version)
	payload.ExtVersion = strings.TrimSpace(payload.ExtVersion)
	payload.CurrentVersion = strings.TrimSpace(payload.CurrentVersion)
	payload.LastError = truncateForDatabase(payload.LastError, maxDatabaseTextLength)
	payload.OpenrestyStatus = normalizeOpenrestyStatus(payload.OpenrestyStatus)
	payload.OpenrestyMessage = truncateForDatabase(payload.OpenrestyMessage, maxDatabaseTextLength)
	payload.Engine = normalizeEngine(payload.Engine)
	if payload.ContractVersion < 0 {
		payload.ContractVersion = 0
	}
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

func normalizeEngine(engine string) string {
	switch strings.ToLower(strings.TrimSpace(engine)) {
	case enginePingora:
		return enginePingora
	default:
		return engineOpenResty
	}
}

func validateNodePayload(payload protocol.NodePayload) error {
	if payload.IP == "" {
		return errPayload(errIPRequired)
	}
	if net.ParseIP(payload.IP) == nil {
		return errPayload(errIPInvalid)
	}
	if payload.Version == "" {
		return errPayload(errAgentVersionRequired)
	}
	if payload.Engine != enginePingora {
		return errPayload(errPingoraNodeRequired)
	}
	if payload.ContractVersion != edgeconfig.ContractVersion || payload.ConfigSyncVersion != protocol.EdgeConfigSyncVersion {
		return errPayload(errPingoraContractUnsupported)
	}
	return nil
}

type payloadError string

func (e payloadError) Error() string { return string(e) }

func errPayload(message string) error { return payloadError(message) }

func applyNodeRuntime(ctx context.Context, node *model.OpenFlareNode, payload protocol.NodePayload, preserveName bool) {
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
	node.Engine = normalizeEngine(payload.Engine)
	node.ContractVersion = payload.ContractVersion
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
	if isPublicNodeIP(reported) {
		return reported
	}
	if isPublicNodeIP(remote) {
		return remote
	}
	return unspecifiedNodeIP
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
	return iputil.IsPublicRoutableString(raw)
}

func buildAgentSettings(ctx context.Context, restartEdgeNow bool) *protocol.AgentSettings {
	heartbeatInterval, _ := repository.GetIntByKey(ctx, model.ConfigKeyAgentHeartbeatInterval)
	if heartbeatInterval <= 0 {
		heartbeatInterval = defaultAgentHeartbeatInterval
	}
	return &protocol.AgentSettings{HeartbeatInterval: heartbeatInterval, RestartEdgeNow: restartEdgeNow}
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
	appendIfChanged("engine", previous.Engine, current.Engine)
	appendIfChanged("contract_version", previous.ContractVersion, current.ContractVersion)
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

func normalizeApplyLogPayload(payload protocol.ApplyLogPayload) protocol.ApplyLogPayload {
	payload.NodeID = strings.TrimSpace(payload.NodeID)
	payload.Version = strings.TrimSpace(payload.Version)
	payload.Result = strings.ToLower(strings.TrimSpace(payload.Result))
	payload.Message = truncateForDatabase(strings.TrimSpace(payload.Message), maxDatabaseTextLength)
	payload.Checksum = strings.TrimSpace(payload.Checksum)
	payload.MainConfigChecksum = strings.TrimSpace(payload.MainConfigChecksum)
	payload.RouteConfigChecksum = strings.TrimSpace(payload.RouteConfigChecksum)
	payload.Engine = normalizeEngine(payload.Engine)
	payload.PayloadChecksum = strings.TrimSpace(payload.PayloadChecksum)
	if payload.ReleaseSequence < 0 {
		payload.ReleaseSequence = 0
	}
	return payload
}

func sameEngineApplyLog(latest *model.OpenFlareApplyLog, engine string) bool {
	return latest != nil && strings.EqualFold(strings.TrimSpace(latest.Engine), engine)
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
