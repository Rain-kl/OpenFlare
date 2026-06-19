package protocol

type WSMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

type AgentNodeSystemProfile struct {
	Hostname         string `json:"hostname"`
	OSName           string `json:"os_name"`
	OSVersion        string `json:"os_version"`
	KernelVersion    string `json:"kernel_version"`
	Architecture     string `json:"architecture"`
	CPUModel         string `json:"cpu_model"`
	CPUCores         int    `json:"cpu_cores"`
	TotalMemoryBytes int64  `json:"total_memory_bytes"`
	TotalDiskBytes   int64  `json:"total_disk_bytes"`
	UptimeSeconds    int64  `json:"uptime_seconds"`
	ReportedAtUnix   int64  `json:"reported_at_unix"`
}

type AgentNodeMetricSnapshot struct {
	CapturedAtUnix    int64   `json:"captured_at_unix"`
	CPUUsagePercent   float64 `json:"cpu_usage_percent"`
	MemoryUsedBytes   int64   `json:"memory_used_bytes"`
	MemoryTotalBytes  int64   `json:"memory_total_bytes"`
	StorageUsedBytes  int64   `json:"storage_used_bytes"`
	StorageTotalBytes int64   `json:"storage_total_bytes"`
	DiskReadBytes     int64   `json:"disk_read_bytes"`
	DiskWriteBytes    int64   `json:"disk_write_bytes"`
	NetworkRxBytes    int64   `json:"network_rx_bytes"`
	NetworkTxBytes    int64   `json:"network_tx_bytes"`
}

type AgentNodeHealthEvent struct {
	EventType       string            `json:"event_type"`
	Severity        string            `json:"severity"`
	Message         string            `json:"message"`
	TriggeredAtUnix int64             `json:"triggered_at_unix"`
	Metadata        map[string]string `json:"metadata"`
}

type RelayProxyStat struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Status        string `json:"status"`
	ClientVersion string `json:"client_version"`
	LastStartTime string `json:"last_start_time"`
	LastCloseTime string `json:"last_close_time"`
	ClientAddr    string `json:"client_addr"`
}

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

type RelayConfig struct {
	BindPort         int    `json:"bind_port"`
	VhostHTTPPort    int    `json:"vhost_http_port"`
	AuthToken        string `json:"auth_token"`
	LogLevel         string `json:"log_level"`
	WebServerEnabled bool   `json:"web_server_enabled"`
}

type RelaySettings struct {
	HeartbeatInterval       int    `json:"heartbeat_interval"`
	WebsocketUpgradeEnabled bool   `json:"websocket_upgrade_enabled"`
	AutoUpdate              bool   `json:"auto_update"`
	UpdateRepo              string `json:"update_repo"`
	UpdateNow               bool   `json:"update_now"`
	UpdateChannel           string `json:"update_channel"`
	UpdateTag               string `json:"update_tag"`
}

type RelayHeartbeatResponse struct {
	RelayConfig   *RelayConfig   `json:"relay_config"`
	RelaySettings *RelaySettings `json:"relay_settings"`
}

type ActiveConfigMeta struct {
	Version  string `json:"version"`
	Checksum string `json:"checksum"`
}

type FlaredConnectedRelay struct {
	RelayNodeID string `json:"relay_node_id"`
	Status      string `json:"status"`
	ProxyCount  int    `json:"proxy_count"`
}

type FlaredHeartbeatPayload struct {
	ClientVersion   string                 `json:"client_version"`
	FrpVersion      string                 `json:"frp_version"`
	IP              string                 `json:"ip"`
	TunnelStatus    string                 `json:"tunnel_status"`
	ConnectedRelays []FlaredConnectedRelay `json:"connected_relays"`
	CurrentVersion  string                 `json:"current_version"`
	CurrentChecksum string                 `json:"current_checksum"`
}

type FlaredHeartbeatResponse struct {
	ActiveConfig   *ActiveConfigMeta `json:"active_config"`
	TunnelSettings *RelaySettings    `json:"tunnel_settings"`
}

type FlaredTunnelConfigResponse struct {
	Version  string             `json:"version"`
	Checksum string             `json:"checksum"`
	Relays   []FlaredRelayInfo  `json:"relays"`
	Proxies  []FlaredProxyEntry `json:"proxies"`
}

type FlaredRelayInfo struct {
	RelayNodeID string `json:"relay_node_id"`
	Address     string `json:"address"`
	AuthToken   string `json:"auth_token"`
	ProxyURL    string `json:"proxy_url"`
}

type FlaredProxyEntry struct {
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	LocalAddr     string   `json:"local_addr"`
	LocalPort     int      `json:"local_port"`
	CustomDomains []string `json:"custom_domains"`
}

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
