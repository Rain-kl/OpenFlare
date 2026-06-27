// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package flared

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/agent"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/routeidentity"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

const (
	nodeStatusOnline         = "online"
	applyResultOK            = "success"
	applyResultWarn          = "warning"
	applyResultFail          = "failed"
	maxApplyLogMessageLength = 16000
)

// Heartbeat processes an OpenFlared heartbeat and returns runtime settings.
func Heartbeat(ctx context.Context, node *model.OpenFlareNode, payload HeartbeatPayload) (*HeartbeatResponse, error) {
	if node == nil {
		return nil, fmt.Errorf("tunnel client node is nil")
	}
	if node.NodeType != "tunnel_client" {
		return nil, fmt.Errorf("node %s is not a tunnel_client", node.NodeID)
	}

	payload = normalizeFlaredHeartbeatPayload(payload)
	previous := *node
	updateNow := node.UpdateRequested
	updateChannel := normalizeReleaseChannel(node.UpdateChannel)
	updateTag := strings.TrimSpace(node.UpdateTag)

	now := time.Now().UTC()
	changes := map[string]any{
		"version":          payload.ClientVersion,
		"ext_version":      payload.FrpVersion,
		"current_version":  payload.CurrentVersion,
		"last_seen_at":     now,
		"status":           nodeStatusOnline,
		"update_requested": false,
		"update_channel":   updateChannelStable,
		"update_tag":       "",
	}
	if !previous.UpdateRequested {
		delete(changes, "update_requested")
	}
	if previous.UpdateChannel == updateChannelStable {
		delete(changes, "update_channel")
	}
	if previous.UpdateTag == "" {
		delete(changes, "update_tag")
	}
	if !node.IPManualOverride && payload.IP != "" && previous.IP != payload.IP {
		changes["ip"] = payload.IP
		node.IP = payload.IP
	}

	node.Version = payload.ClientVersion
	node.ExtVersion = payload.FrpVersion
	node.CurrentVersion = payload.CurrentVersion
	node.UpdateRequested = false
	node.UpdateChannel = updateChannelStable
	node.UpdateTag = ""
	lastSeen := now
	node.LastSeenAt = &lastSeen
	node.Status = nodeStatusOnline

	if err := db.DB(ctx).Model(node).Updates(changes).Error; err != nil {
		return nil, fmt.Errorf("update flared heartbeat: %w", err)
	}
	agent.RefreshAccessTokenCache(ctx, node)
	persistFlaredObservability(ctx, node.NodeID, payload, now)

	activeConfig, err := getActiveConfigMeta(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return &HeartbeatResponse{
		ActiveConfig:   activeConfig,
		TunnelSettings: buildTunnelSettings(ctx, node, updateNow, updateChannel, updateTag),
	}, nil
}

// GetTunnelConfig builds the full tunnel routing config for an OpenFlared client.
func GetTunnelConfig(ctx context.Context, node *model.OpenFlareNode) (*TunnelConfigResponse, error) {
	if node == nil {
		return nil, fmt.Errorf("node is nil")
	}

	activeVersion, err := getActiveConfigMeta(ctx)
	if err != nil {
		return nil, fmt.Errorf("no active config version: %w", err)
	}

	routes, err := model.ListProxyRoutes(ctx)
	if err != nil {
		return nil, fmt.Errorf("get proxy routes: %w", err)
	}

	relayNodes, err := listTunnelRelayNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("get relay nodes: %w", err)
	}

	relays := make([]RelayInfo, 0, len(relayNodes))
	for i := range relayNodes {
		relayNode := relayNodes[i]
		if relayNode.RelayStatus == "healthy" || relayNode.Status == nodeStatusOnline {
			relays = append(relays, RelayInfo{
				RelayNodeID: relayNode.NodeID,
				Address:     relayClientAddress(&relayNode),
				AuthToken:   relayNode.RelayAuthToken,
				ProxyURL:    strings.TrimSpace(relayNode.RelayClientProxyURL),
			})
		}
	}

	proxies := make([]ProxyEntry, 0)
	for _, route := range routes {
		if route == nil || route.UpstreamType != "tunnel" || route.TunnelNodeID == nil || *route.TunnelNodeID != node.ID {
			continue
		}
		if !route.Enabled {
			continue
		}
		domains, decodeErr := routeidentity.DecodeDomains(route.Domains, route.Domain)
		if decodeErr != nil {
			continue
		}
		localAddr, localPort := parseTunnelTargetAddr(route.TunnelTargetAddr)
		proxies = append(proxies, ProxyEntry{
			Name:          fmt.Sprintf("%s-%s", node.NodeID, sanitizeProxyName(domains[0])),
			Type:          "http",
			LocalAddr:     localAddr,
			LocalPort:     localPort,
			CustomDomains: domains,
		})
	}

	return &TunnelConfigResponse{
		Version:  activeVersion.Version,
		Checksum: activeVersion.Checksum,
		Relays:   relays,
		Proxies:  proxies,
	}, nil
}

// ReportApplyLog records an apply result from OpenFlared.
func ReportApplyLog(ctx context.Context, payload ApplyLogPayload) (*model.OpenFlareApplyLog, error) {
	now := time.Now().UTC()
	payload = normalizeApplyLogPayload(payload)
	if payload.NodeID == "" {
		return nil, errors.New("node_id 不能为空")
	}
	if payload.Version == "" {
		return nil, errors.New("version 不能为空")
	}
	if payload.Result != applyResultOK && payload.Result != applyResultWarn && payload.Result != applyResultFail {
		return nil, errors.New("result 仅支持 success、warning 或 failed")
	}

	latest, err := model.GetLatestOpenFlareApplyLogByNodeID(ctx, payload.NodeID)
	if err != nil {
		return nil, err
	}
	if model.IsRepeatSuccessApplyLog(latest, payload.Version, payload.Checksum, payload.Result) {
		if err := updateFlaredNodeFromApplyLog(ctx, payload, now); err != nil {
			return nil, err
		}
		return latest, nil
	}

	log := &model.OpenFlareApplyLog{
		NodeID:              payload.NodeID,
		Version:             payload.Version,
		Result:              payload.Result,
		Message:             payload.Message,
		Checksum:            payload.Checksum,
		MainConfigChecksum:  payload.MainConfigChecksum,
		RouteConfigChecksum: payload.RouteConfigChecksum,
		SupportFileCount:    payload.SupportFileCount,
		CreatedAt:           now,
	}

	err = db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(log).Error; err != nil {
			return err
		}
		return updateFlaredNodeFromApplyLogTx(tx, payload, now)
	})
	if err != nil {
		return nil, err
	}
	return log, nil
}

func updateFlaredNodeFromApplyLog(ctx context.Context, payload ApplyLogPayload, now time.Time) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New("database not initialized")
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		return updateFlaredNodeFromApplyLogTx(tx, payload, now)
	})
}

func updateFlaredNodeFromApplyLogTx(tx *gorm.DB, payload ApplyLogPayload, now time.Time) error {
	var node model.OpenFlareNode
	if err := tx.Where("node_id = ?", payload.NodeID).First(&node).Error; err != nil {
		return err
	}
	node.Status = nodeStatusOnline
	lastSeen := now
	node.LastSeenAt = &lastSeen
	if payload.Result == applyResultOK {
		node.CurrentVersion = payload.Version
		node.LastError = ""
	} else {
		node.LastError = payload.Message
	}
	return tx.Model(&node).Select("status", "last_seen_at", "current_version", "last_error").Updates(&node).Error
}

func normalizeApplyLogPayload(payload ApplyLogPayload) ApplyLogPayload {
	payload.NodeID = strings.TrimSpace(payload.NodeID)
	payload.Version = strings.TrimSpace(payload.Version)
	payload.Result = strings.ToLower(strings.TrimSpace(payload.Result))
	payload.Message = strings.TrimSpace(payload.Message)
	payload.Checksum = strings.TrimSpace(payload.Checksum)
	payload.MainConfigChecksum = strings.TrimSpace(payload.MainConfigChecksum)
	payload.RouteConfigChecksum = strings.TrimSpace(payload.RouteConfigChecksum)
	if len(payload.Message) > maxApplyLogMessageLength {
		payload.Message = payload.Message[:maxApplyLogMessageLength]
	}
	return payload
}
