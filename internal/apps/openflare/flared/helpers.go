// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package flared

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/relay"
	"github.com/Rain-kl/Wavelet/internal/model"
)

const (
	updateChannelStable     = "stable"
	defaultTunnelTargetPort = 80
)

func normalizeReleaseChannel(channel string) string {
	if strings.ToLower(strings.TrimSpace(channel)) == "preview" {
		return "preview"
	}
	return updateChannelStable
}

func normalizeFlaredHeartbeatPayload(payload HeartbeatPayload) HeartbeatPayload {
	payload.ClientVersion = strings.TrimSpace(payload.ClientVersion)
	payload.FrpVersion = strings.TrimSpace(payload.FrpVersion)
	payload.IP = strings.TrimSpace(payload.IP)
	payload.TunnelStatus = strings.ToLower(strings.TrimSpace(payload.TunnelStatus))
	payload.CurrentVersion = strings.TrimSpace(payload.CurrentVersion)
	payload.CurrentChecksum = strings.TrimSpace(payload.CurrentChecksum)

	cleaned := make([]ConnectedRelay, 0, len(payload.ConnectedRelays))
	for _, item := range payload.ConnectedRelays {
		item.RelayNodeID = strings.TrimSpace(item.RelayNodeID)
		item.Status = strings.ToLower(strings.TrimSpace(item.Status))
		if item.RelayNodeID == "" {
			continue
		}
		if item.Status == "" {
			item.Status = "unknown"
		}
		cleaned = append(cleaned, item)
	}
	payload.ConnectedRelays = cleaned
	return payload
}

func getActiveConfigMeta(ctx context.Context) (*ActiveConfigMeta, error) {
	version, err := model.GetActiveConfigVersion(ctx)
	if err != nil {
		return nil, err
	}
	return &ActiveConfigMeta{
		Version:  version.Version,
		Checksum: version.Checksum,
	}, nil
}

func listTunnelRelayNodes(ctx context.Context) ([]model.OpenFlareNode, error) {
	nodes, err := model.ListOpenFlareNodes(ctx)
	if err != nil {
		return nil, err
	}
	relays := make([]model.OpenFlareNode, 0)
	for _, node := range nodes {
		if node.NodeType == "tunnel_relay" {
			relays = append(relays, node)
		}
	}
	return relays, nil
}

func relayClientAddress(node *model.OpenFlareNode) string {
	if node == nil {
		return ""
	}
	port := node.RelayBindPort
	if port <= 0 {
		port = 7000
	}
	addr := strings.TrimSpace(node.RelayClientAccessAddr)
	if addr == "" {
		addr = strings.TrimSpace(node.IP)
	}
	if addr == "" {
		return fmt.Sprintf("127.0.0.1:%d", port)
	}
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}
	if strings.Contains(addr, ":") && strings.Count(addr, ":") > 1 {
		return net.JoinHostPort(addr, strconv.Itoa(port))
	}
	return fmt.Sprintf("%s:%d", addr, port)
}

func parseTunnelTargetAddr(addr string) (string, int) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "127.0.0.1", defaultTunnelTargetPort
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		lastColon := strings.LastIndex(addr, ":")
		if lastColon < 0 {
			return addr, defaultTunnelTargetPort
		}
		host = addr[:lastColon]
		portStr = addr[lastColon+1:]
	}
	port := defaultTunnelTargetPort
	if _, scanErr := fmt.Sscanf(portStr, "%d", &port); scanErr != nil {
		port = defaultTunnelTargetPort
	}
	if host == "" {
		host = "127.0.0.1"
	}
	return host, port
}

func sanitizeProxyName(domain string) string {
	return strings.ReplaceAll(strings.ReplaceAll(domain, ".", "-"), "*", "wildcard")
}

func buildTunnelSettings(ctx context.Context, node *model.OpenFlareNode, updateNow bool, updateChannel, updateTag string) *relay.Settings {
	return relay.BuildSettings(ctx, node, updateNow, updateChannel, updateTag)
}
