// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package relay

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/agent"
	ofgeoip "github.com/Rain-kl/Wavelet/internal/apps/openflare/geoip"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

const nodeStatusOnline = "online"

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
		"update_channel":   releaseChannelStable,
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
	if !node.GeoManualOverride {
		beforeGeo := node.GeoName
		beforeLat := node.GeoLatitude
		beforeLon := node.GeoLongitude
		ofgeoip.ApplyNodeGeoFromIP(ctx, node, node.IP)
		if node.GeoName != beforeGeo {
			changes["geo_name"] = node.GeoName
		}
		if !coordinatesEqual(beforeLat, node.GeoLatitude) {
			changes["geo_latitude"] = node.GeoLatitude
		}
		if !coordinatesEqual(beforeLon, node.GeoLongitude) {
			changes["geo_longitude"] = node.GeoLongitude
		}
	}
	if !previous.UpdateRequested {
		delete(changes, "update_requested")
	}
	if previous.UpdateChannel == releaseChannelStable {
		delete(changes, "update_channel")
	}
	if previous.UpdateTag == "" {
		delete(changes, "update_tag")
	}

	node.Version = payload.Version
	node.ExtVersion = payload.ExtVersion
	node.RelayStatus = payload.RelayStatus
	node.UpdateRequested = false
	node.UpdateChannel = releaseChannelStable
	node.UpdateTag = ""
	lastSeen := now
	node.LastSeenAt = &lastSeen
	node.Status = nodeStatusOnline

	if err := repository.UpdateOpenFlareNodeColumns(ctx, node, changes); err != nil {
		return nil, fmt.Errorf("update relay heartbeat: %w", err)
	}
	if err := reconcileRelayHealthEvents(ctx, node.NodeID, payload.RelayStatus, now); err != nil {
		return nil, fmt.Errorf("reconcile relay health events: %w", err)
	}
	agent.RefreshAccessTokenCache(ctx, node)
	persistRelayHeartbeatObservability(ctx, node.NodeID, payload, now)

	return &HeartbeatResponse{
		RelayConfig:   buildRelayConfig(ctx, node),
		RelaySettings: BuildSettings(ctx, node, updateNow, updateChannel, updateTag),
	}, nil
}

func coordinatesEqual(before *float64, after *float64) bool {
	if before == nil || after == nil {
		return before == after
	}
	return *before == *after
}
