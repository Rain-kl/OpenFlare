package service

import (
	"fmt"
	"log/slog"
	"openflare/common"
	"openflare/model"
	"strings"
	"time"
)

// RelayHeartbeatPayload is the payload sent by OpenFlareRelay in each heartbeat.
type RelayHeartbeatPayload struct {
	Version        string                   `json:"version"`
	ExtVersion     string                   `json:"frp_version"`
	RelayStatus    string                   `json:"relay_status"`
	FrpsConnCount  int                      `json:"frps_connections"`
	FrpsProxyCount int                      `json:"frps_proxy_count"`
	Name           string                   `json:"name"`
	IP             string                   `json:"ip"`
	Profile        *AgentNodeSystemProfile  `json:"profile,omitempty"`
	Snapshot       *AgentNodeMetricSnapshot `json:"snapshot,omitempty"`
	HealthEvents   []AgentNodeHealthEvent   `json:"health_events,omitempty"`
}

// RelayConfig is the frps configuration sent to the Relay.
type RelayConfig struct {
	BindPort      int    `json:"bind_port"`
	VhostHTTPPort int    `json:"vhost_http_port"`
	AuthToken     string `json:"auth_token"`
	LogLevel      string `json:"log_level"`
}

// RelaySettings contains runtime settings for the Relay.
type RelaySettings struct {
	HeartbeatInterval       int  `json:"heartbeat_interval"`
	WebsocketUpgradeEnabled bool `json:"websocket_upgrade_enabled"`
}

// RelayHeartbeatResponse is the response returned to the Relay from a heartbeat.
type RelayHeartbeatResponse struct {
	RelayConfig   *RelayConfig   `json:"relay_config"`
	RelaySettings *RelaySettings `json:"relay_settings"`
}

// HeartbeatRelay processes a relay heartbeat, updates node status, and returns config.
func HeartbeatRelay(node *model.Node, payload RelayHeartbeatPayload) (*RelayHeartbeatResponse, error) {
	if node == nil {
		return nil, fmt.Errorf("relay node is nil")
	}
	slog.Debug("relay heartbeat received", "node_id", node.NodeID)

	payload.Version = strings.TrimSpace(payload.Version)
	payload.ExtVersion = strings.TrimSpace(payload.ExtVersion)
	payload.RelayStatus = normalizeRelayStatus(payload.RelayStatus)
	payload.Name = strings.TrimSpace(payload.Name)
	payload.IP = strings.TrimSpace(payload.IP)

	changes := make(map[string]any)
	appendRelayChange := func(key string, before any, after any) {
		if before != after {
			changes[key] = after
		}
	}
	now := time.Now()
	appendRelayChange("version", node.Version, payload.Version)
	appendRelayChange("ext_version", node.ExtVersion, payload.ExtVersion)
	appendRelayChange("relay_status", node.RelayStatus, payload.RelayStatus)

	if payload.Name != "" && strings.TrimSpace(node.Name) == "" {
		appendRelayChange("name", node.Name, payload.Name)
		node.Name = payload.Name
	}
	if payload.IP != "" && !node.IPManualOverride {
		appendRelayChange("ip", node.IP, payload.IP)
		node.IP = payload.IP
		if !node.GeoManualOverride {
			applyGeoInfoFromIP(node, node.IP)
			changes["geo_name"] = node.GeoName
			changes["geo_latitude"] = node.GeoLatitude
			changes["geo_longitude"] = node.GeoLongitude
		}
	}
	if !node.LastSeenAt.Equal(now) {
		changes["last_seen_at"] = now
	}
	changes["status"] = NodeStatusOnline

	node.Version = payload.Version
	node.ExtVersion = payload.ExtVersion
	node.RelayStatus = payload.RelayStatus

	node.LastSeenAt = now
	node.Status = NodeStatusOnline

	if len(changes) > 0 {
		if err := model.DB.Model(node).Updates(changes).Error; err != nil {
			return nil, fmt.Errorf("update relay heartbeat: %w", err)
		}
	}
	refreshAccessTokenCache(node)
	persistRelayHeartbeatObservability(node.NodeID, payload, node.LastSeenAt)

	return &RelayHeartbeatResponse{
		RelayConfig:   buildRelayConfig(node),
		RelaySettings: buildRelaySettings(),
	}, nil
}

func persistRelayHeartbeatObservability(nodeID string, payload RelayHeartbeatPayload, reportedAt time.Time) {
	persistHeartbeatObservability(nodeID, AgentNodePayload{
		Profile:      payload.Profile,
		Snapshot:     payload.Snapshot,
		HealthEvents: payload.HealthEvents,
	}, reportedAt)

	frpsObs := &model.NodeObservationFrps{
		NodeID:          nodeID,
		CapturedAt:      reportedAt,
		FrpsConnections: payload.FrpsConnCount,
		FrpsProxyCount:  payload.FrpsProxyCount,
	}
	_ = frpsObs.Insert()
}

func buildRelayConfig(node *model.Node) *RelayConfig {
	if node == nil {
		return nil
	}
	return &RelayConfig{
		BindPort:      node.RelayBindPort,
		VhostHTTPPort: node.RelayVhostHTTPPort,
		AuthToken:     node.RelayAuthToken,
		LogLevel:      "info",
	}
}

func buildRelaySettings() *RelaySettings {
	return &RelaySettings{
		HeartbeatInterval:       common.AgentHeartbeatInterval,
		WebsocketUpgradeEnabled: common.AgentWebsocketUpgradeEnabled,
	}
}

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

// FlaredHeartbeatPayload is the payload sent by OpenFlared in each heartbeat.
type FlaredHeartbeatPayload struct {
	ClientVersion   string                 `json:"client_version"`
	FrpVersion      string                 `json:"frp_version"`
	TunnelStatus    string                 `json:"tunnel_status"`
	ConnectedRelays []FlaredConnectedRelay `json:"connected_relays"`
	CurrentVersion  string                 `json:"current_version"`
	CurrentChecksum string                 `json:"current_checksum"`
}

// FlaredConnectedRelay describes the status of a relay connection from a client.
type FlaredConnectedRelay struct {
	RelayNodeID string `json:"relay_node_id"`
	Status      string `json:"status"`
	ProxyCount  int    `json:"proxy_count"`
}

// FlaredHeartbeatResponse is the response returned to the OpenFlared client.
type FlaredHeartbeatResponse struct {
	ActiveConfig   *ActiveConfigMeta `json:"active_config"`
	TunnelSettings *RelaySettings    `json:"tunnel_settings"`
}

// FlaredTunnelConfigResponse is the full tunnel routing config sent to the client.
type FlaredTunnelConfigResponse struct {
	Version  string             `json:"version"`
	Checksum string             `json:"checksum"`
	Relays   []FlaredRelayInfo  `json:"relays"`
	Proxies  []FlaredProxyEntry `json:"proxies"`
}

// FlaredRelayInfo describes a relay that the client should connect to.
type FlaredRelayInfo struct {
	RelayNodeID string `json:"relay_node_id"`
	Address     string `json:"address"`
	AuthToken   string `json:"auth_token"`
	ProxyURL    string `json:"proxy_url"`
}

// FlaredProxyEntry describes a single frpc proxy definition.
type FlaredProxyEntry struct {
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	LocalAddr     string   `json:"local_addr"`
	LocalPort     int      `json:"local_port"`
	CustomDomains []string `json:"custom_domains"`
}

// GetFlaredTunnelConfig builds the full tunnel routing config for an OpenFlared client.
func GetFlaredTunnelConfig(node *model.Node) (*FlaredTunnelConfigResponse, error) {
	if node == nil {
		return nil, fmt.Errorf("node is nil")
	}

	activeVersion, err := model.GetActiveConfigVersion()
	if err != nil {
		return nil, fmt.Errorf("no active config version: %w", err)
	}

	// Get all enabled proxy routes with tunnel upstream targeting this tunnel
	routes, err := model.GetEnabledProxyRoutes()
	if err != nil {
		return nil, fmt.Errorf("get proxy routes: %w", err)
	}

	// Get all online tunnel relay nodes
	relayNodes, err := model.ListNodesByType("tunnel_relay")
	if err != nil {
		return nil, fmt.Errorf("get relay nodes: %w", err)
	}

	// Build relay info
	relays := make([]FlaredRelayInfo, 0, len(relayNodes))
	for _, node := range relayNodes {
		if node.RelayStatus == "healthy" || node.Status == NodeStatusOnline {
			addr := strings.TrimSpace(node.RelayClientAccessAddr)
			if addr == "" {
				addr = fmt.Sprintf("%s:%d", strings.TrimSpace(node.IP), node.RelayBindPort)
			}
			relays = append(relays, FlaredRelayInfo{
				RelayNodeID: node.NodeID,
				Address:     addr,
				AuthToken:   node.RelayAuthToken,
				ProxyURL:    strings.TrimSpace(node.RelayClientProxyURL),
			})
		}
	}

	// Build proxy entries from routes
	proxies := make([]FlaredProxyEntry, 0)
	for _, route := range routes {
		if route.UpstreamType != "tunnel" || route.TunnelNodeID == nil || *route.TunnelNodeID != node.ID {
			continue
		}
		if !route.Enabled {
			continue
		}
		domains, err := decodeStoredDomains(route.Domains, route.Domain)
		if err != nil {
			continue
		}
		localAddr, localPort := parseTunnelTargetAddr(route.TunnelTargetAddr)
		proxies = append(proxies, FlaredProxyEntry{
			Name:          fmt.Sprintf("%s-%s", node.NodeID, sanitizeProxyName(domains[0])),
			Type:          "http",
			LocalAddr:     localAddr,
			LocalPort:     localPort,
			CustomDomains: domains,
		})
	}

	return &FlaredTunnelConfigResponse{
		Version:  activeVersion.Version,
		Checksum: activeVersion.Checksum,
		Relays:   relays,
		Proxies:  proxies,
	}, nil
}

func parseTunnelTargetAddr(addr string) (string, int) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "127.0.0.1", 80
	}
	host, portStr, err := splitHostPort(addr)
	if err != nil {
		return addr, 80
	}
	port := 80
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		port = 80
	}
	return host, port
}

func splitHostPort(addr string) (string, string, error) {
	lastColon := strings.LastIndex(addr, ":")
	if lastColon < 0 {
		return addr, "", fmt.Errorf("no port")
	}
	return addr[:lastColon], addr[lastColon+1:], nil
}

func sanitizeProxyName(domain string) string {
	return strings.ReplaceAll(strings.ReplaceAll(domain, ".", "-"), "*", "wildcard")
}
