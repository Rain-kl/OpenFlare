// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package protocol

// AgentNodeSystemProfile is an alias for NodeSystemProfile used by server.
type AgentNodeSystemProfile = NodeSystemProfile

// AgentNodeMetricSnapshot is an alias for NodeMetricSnapshot used by server.
type AgentNodeMetricSnapshot = NodeMetricSnapshot

// AgentNodeHealthEvent is an alias for NodeHealthEvent used by server.
type AgentNodeHealthEvent = NodeHealthEvent

// RelayProxyStat holds relay proxy statistics.
type RelayProxyStat struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Status        string `json:"status"`
	ClientVersion string `json:"client_version"`
	LastStartTime string `json:"last_start_time"`
	LastCloseTime string `json:"last_close_time"`
	ClientAddr    string `json:"client_addr"`
}

// RelayHeartbeatPayload is the relay heartbeat payload.
type RelayHeartbeatPayload struct {
	Version         string                   `json:"version"`
	ExtVersion      string                   `json:"frp_version"`
	RelayStatus     string                   `json:"relay_status"`
	FrpsConnCount   int                      `json:"frps_connections"`
	FrpsProxyCount  int                      `json:"frps_proxy_count"`
	FrpsClientCount int                      `json:"frps_client_count"`
	FrpsProxies     []RelayProxyStat         `json:"frps_proxies,omitempty"`
	Name            string                   `json:"name"`
	IP              string                   `json:"ip"`
	Profile         *AgentNodeSystemProfile  `json:"profile,omitempty"`
	Snapshot        *AgentNodeMetricSnapshot `json:"snapshot,omitempty"`
	HealthEvents    []AgentNodeHealthEvent   `json:"health_events,omitempty"`
}

// RelayConfig holds relay configuration.
type RelayConfig struct {
	BindPort         int    `json:"bind_port"`
	VhostHTTPPort    int    `json:"vhost_http_port"`
	AuthToken        string `json:"auth_token"`
	LogLevel         string `json:"log_level"`
	WebServerEnabled bool   `json:"web_server_enabled"`
	WebServerPort    int    `json:"web_server_port"`
}

// RelaySettings holds relay runtime settings.
type RelaySettings struct {
	HeartbeatInterval       int    `json:"heartbeat_interval"`
	WebsocketUpgradeEnabled bool   `json:"websocket_upgrade_enabled"`
	AutoUpdate              bool   `json:"auto_update"`
	UpdateRepo              string `json:"update_repo"`
	UpdateNow               bool   `json:"update_now"`
	UpdateChannel           string `json:"update_channel"`
	UpdateTag               string `json:"update_tag"`
}

// RelayHeartbeatResponse is the relay heartbeat response.
type RelayHeartbeatResponse struct {
	RelayConfig   *RelayConfig   `json:"relay_config"`
	RelaySettings *RelaySettings `json:"relay_settings"`
}

// ActiveConfigMeta holds active configuration metadata.
type ActiveConfigMeta struct {
	Version  string `json:"version"`
	Checksum string `json:"checksum"`
}

// FlaredConnectedRelay describes a connected relay info for flared.
type FlaredConnectedRelay struct {
	RelayNodeID string `json:"relay_node_id"`
	Status      string `json:"status"`
	ProxyCount  int    `json:"proxy_count"`
}

// FlaredHeartbeatPayload is the flared heartbeat payload.
type FlaredHeartbeatPayload struct {
	ClientVersion   string                 `json:"client_version"`
	FrpVersion      string                 `json:"frp_version"`
	IP              string                 `json:"ip"`
	TunnelStatus    string                 `json:"tunnel_status"`
	ConnectedRelays []FlaredConnectedRelay `json:"connected_relays"`
	CurrentVersion  string                 `json:"current_version"`
	CurrentChecksum string                 `json:"current_checksum"`
}

// FlaredHeartbeatResponse is the flared heartbeat response.
type FlaredHeartbeatResponse struct {
	ActiveConfig   *ActiveConfigMeta `json:"active_config"`
	TunnelSettings *RelaySettings    `json:"tunnel_settings"`
}

// FlaredTunnelConfigResponse is the flared tunnel configuration response.
type FlaredTunnelConfigResponse struct {
	Version  string             `json:"version"`
	Checksum string             `json:"checksum"`
	Relays   []FlaredRelayInfo  `json:"relays"`
	Proxies  []FlaredProxyEntry `json:"proxies"`
}

// FlaredRelayInfo holds flared relay information.
type FlaredRelayInfo struct {
	RelayNodeID string `json:"relay_node_id"`
	Address     string `json:"address"`
	AuthToken   string `json:"auth_token"`
	ProxyURL    string `json:"proxy_url"`
}

// FlaredProxyEntry represents a flared proxy entry.
type FlaredProxyEntry struct {
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	LocalAddr     string   `json:"local_addr"`
	LocalPort     int      `json:"local_port"`
	CustomDomains []string `json:"custom_domains"`
}

// ApplyLogPayload is the apply log payload for flared.
type ApplyLogPayload struct {
	NodeID              string `json:"node_id"`
	Version             string `json:"version"`
	Result              string `json:"result"`
	Message             string `json:"message"`
	Checksum            string `json:"checksum"`
	MainConfigChecksum  string `json:"main_config_checksum"`
	RouteConfigChecksum string `json:"route_config_checksum"`
	SupportFileCount    int    `json:"support_file_count"`
}
