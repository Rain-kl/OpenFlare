// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package flared

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/relay"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

type configVersionRow struct {
	Version  string `gorm:"column:version"`
	Checksum string `gorm:"column:checksum"`
}

func (configVersionRow) TableName() string {
	return "of_config_versions"
}

func normalizeReleaseChannel(channel string) string {
	if strings.ToLower(strings.TrimSpace(channel)) == "preview" {
		return "preview"
	}
	return "stable"
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
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New("database not initialized")
	}
	if !conn.Migrator().HasTable(&configVersionRow{}) {
		return nil, gorm.ErrRecordNotFound
	}

	var version configVersionRow
	err := conn.Where("is_active = ?", true).Order("id desc").First(&version).Error
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

func decodeStoredDomains(raw string, fallbackDomain string) ([]string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		domain := strings.ToLower(strings.TrimSpace(fallbackDomain))
		if domain == "" {
			return nil, errors.New("domain is required")
		}
		return []string{domain}, nil
	}
	var domains []string
	if err := json.Unmarshal([]byte(text), &domains); err != nil {
		return nil, errors.New("domains payload is invalid")
	}
	normalized := make([]string, 0, len(domains))
	for _, item := range domains {
		domain := strings.ToLower(strings.TrimSpace(item))
		if domain == "" {
			continue
		}
		normalized = append(normalized, domain)
	}
	if len(normalized) == 0 {
		return nil, errors.New("domain is required")
	}
	return normalized, nil
}

func parseTunnelTargetAddr(addr string) (string, int) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "127.0.0.1", 80
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		lastColon := strings.LastIndex(addr, ":")
		if lastColon < 0 {
			return addr, 80
		}
		host = addr[:lastColon]
		portStr = addr[lastColon+1:]
	}
	port := 80
	if _, scanErr := fmt.Sscanf(portStr, "%d", &port); scanErr != nil {
		port = 80
	}
	if host == "" {
		host = "127.0.0.1"
	}
	return host, port
}

func sanitizeProxyName(domain string) string {
	return strings.ReplaceAll(strings.ReplaceAll(domain, ".", "-"), "*", "wildcard")
}

func buildTunnelSettings(node *model.OpenFlareNode, updateNow bool, updateChannel, updateTag string) *relay.Settings {
	return relay.BuildSettings(node, updateNow, updateChannel, updateTag)
}
