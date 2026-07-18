// Package protocol defines the communication protocol between OpenFlare server, agent, and relay components.
package protocol

import "encoding/json"

// APIResponse is a generic API response wrapper.
type APIResponse[T any] struct {
	ErrorMsg string `json:"error_msg"`
	Data     T      `json:"data"`
}

// HeartbeatData is the heartbeat request payload from agent.
type HeartbeatData struct {
	AgentSettings *AgentSettings    `json:"agent_settings"`
	ActiveConfig  *ActiveConfigMeta `json:"active_config"`
	WAFIPGroups   []WAFIPGroup      `json:"waf_ip_groups,omitempty"`
}

// HeartbeatResult is the heartbeat response payload.
type HeartbeatResult struct {
	AgentSettings *AgentSettings
	ActiveConfig  *ActiveConfigMeta
	WAFIPGroups   []WAFIPGroup
}

// AgentSettings holds agent configuration settings.
type AgentSettings struct {
	HeartbeatInterval       int    `json:"heartbeat_interval"`
	WebsocketUpgradeEnabled bool   `json:"websocket_upgrade_enabled"`
	AutoUpdate              bool   `json:"auto_update"`
	UpdateRepo              string `json:"update_repo"`
	UpdateNow               bool   `json:"update_now"`
	UpdateChannel           string `json:"update_channel"`
	UpdateTag               string `json:"update_tag"`
	RestartOpenrestyNow     bool   `json:"restart_openresty_now"`
}

// WSMessageType constants define WebSocket message types.
const (
	WSMessageTypeStatus          = "status"
	WSMessageTypeSettings        = "settings"
	WSMessageTypeActiveConfig    = "active_config"
	WSMessageTypeForceSyncConfig = "force_sync_config"
	WSMessageTypeWAFIPGroups     = "waf_ip_groups"
	WSMessageTypePing            = "ping"
	WSMessageTypePong            = "pong"
)

// WSMessage represents a WebSocket message.
type WSMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// WSOutboundMessage represents an outbound WebSocket message.
type WSOutboundMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

// WebSocketConnection defines the WebSocket connection interface.
type WebSocketConnection interface {
	URL() string
	SendStatus(payload NodePayload) error
	SendPong() error
	Receive() (WSMessage, error)
	Close() error
}

// OpenrestyStatus constants define OpenResty health status values.
const (
	OpenrestyStatusHealthy   = "healthy"
	OpenrestyStatusUnhealthy = "unhealthy"
	OpenrestyStatusUnknown   = "unknown"
)

// NodePayload is the agent node registration / heartbeat payload.
// schema_version 2: host_metrics + edge_health + access_logs facts only (no business pre-aggregation).
// Agents are destroy/rebuild upgraded; no wire-level compatibility aliases.
type NodePayload struct {
	SchemaVersion       int                           `json:"schema_version,omitempty"`
	NodeID              string                        `json:"node_id"`
	Name                string                        `json:"name"`
	IP                  string                        `json:"ip"`
	Version             string                        `json:"version"`
	ExtVersion          string                        `json:"ext_version"`
	CurrentVersion      string                        `json:"current_version"`
	LastError           string                        `json:"last_error"`
	OpenrestyStatus     string                        `json:"openresty_status"`
	OpenrestyMessage    string                        `json:"openresty_message"`
	Profile             *NodeSystemProfile            `json:"profile,omitempty"`
	HostMetrics         *NodeMetricSnapshot           `json:"host_metrics,omitempty"`
	EdgeHealth          *NodeEdgeHealth               `json:"edge_health,omitempty"`
	AccessLogs          []NodeAccessLog               `json:"access_logs,omitempty"`
	Buffered            []BufferedObservabilityRecord `json:"buffered,omitempty"`
	HealthEvents        []NodeHealthEvent             `json:"health_events"`
	WAFIPGroupChecksums map[string]string             `json:"waf_ip_group_checksums,omitempty"`
}

// NodeEdgeHealth is an instantaneous OpenResty health snapshot (L2).
type NodeEdgeHealth struct {
	CapturedAtUnix int64  `json:"captured_at_unix"`
	Status         string `json:"status"`
	Message        string `json:"message"`
	Connections    int64  `json:"connections"`
}

// NodeSystemProfile describes the system profile of a node.
type NodeSystemProfile struct {
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

// NodeMetricSnapshot is a metric snapshot of a node.
type NodeMetricSnapshot struct {
	CapturedAtUnix    int64   `json:"captured_at_unix"`
	CPUUsagePercent   float64 `json:"cpu_usage_percent"`
	MemoryUsedBytes   int64   `json:"memory_used_bytes"`
	MemoryTotalBytes  int64   `json:"memory_total_bytes"`
	StorageUsedBytes  int64   `json:"storage_used_bytes"`
	StorageTotalBytes int64   `json:"storage_total_bytes"`
	DiskReadBytes     int64   `json:"disk_read_bytes"`
	DiskWriteBytes    int64   `json:"disk_write_bytes"`
}

// NodeAccessLog is an access log entry from agent (L1 business fact).
type NodeAccessLog struct {
	LoggedAtUnix  int64  `json:"logged_at_unix"`
	RemoteAddr    string `json:"remote_addr"`
	Host          string `json:"host"`
	Path          string `json:"path"`
	UserAgent     string `json:"user_agent,omitempty"`
	CacheStatus   string `json:"cache_status,omitempty"` // $upstream_cache_status
	StatusCode    int    `json:"status_code"`
	BytesSent     int64  `json:"bytes_sent"`      // body bytes = 已提供数据
	RequestLength int64  `json:"request_length"`  // 接收数据
	RequestTimeMs int64  `json:"request_time_ms"` // optional
}

// BufferedObservabilityRecord is a buffered observability record (facts only).
type BufferedObservabilityRecord struct {
	CapturedAtUnix int64               `json:"captured_at_unix,omitempty"`
	HostMetrics    *NodeMetricSnapshot `json:"host_metrics,omitempty"`
	EdgeHealth     *NodeEdgeHealth     `json:"edge_health,omitempty"`
	AccessLogs     []NodeAccessLog     `json:"access_logs,omitempty"`
}

// NodeHealthEvent represents a node health event.
type NodeHealthEvent struct {
	EventType       string            `json:"event_type"`
	Severity        string            `json:"severity"`
	Message         string            `json:"message"`
	TriggeredAtUnix int64             `json:"triggered_at_unix"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// RegisterNodeResponse is the node registration response.
type RegisterNodeResponse struct {
	NodeID      string `json:"node_id"`
	AccessToken string `json:"agent_token"`
	Name        string `json:"name"`
}

// ActiveConfigResponse is the active configuration response.
type ActiveConfigResponse struct {
	Version          string        `json:"version"`
	Checksum         string        `json:"checksum"`
	SourceConfigJSON string        `json:"source_config_json"`
	SupportFiles     []SupportFile `json:"support_files"`
	CreatedAt        string        `json:"created_at"`
}

// WAFIPGroup defines a WAF IP group.
type WAFIPGroup struct {
	ID       uint     `json:"id"`
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Enabled  bool     `json:"enabled"`
	IPList   []string `json:"ip_list"`
	Checksum string   `json:"checksum"`
}

// WAFIPGroupSyncRequest is a WAF IP group sync request.
type WAFIPGroupSyncRequest struct {
	IDs       []uint            `json:"ids,omitempty"`
	Checksums map[string]string `json:"checksums,omitempty"`
}

// WAFIPGroupSyncResponse is a WAF IP group sync response.
type WAFIPGroupSyncResponse struct {
	Groups []WAFIPGroup `json:"groups"`
}

// SupportFile represents a support file for relay.
type SupportFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// PagesDeploymentHashResponse is the upload SHA-256 hash for a Pages deployment package.
type PagesDeploymentHashResponse struct {
	DeploymentID uint   `json:"deployment_id"`
	Hash         string `json:"hash"`
}

// PagesProjectLatestHashResponse is the hash of a project's currently active Pages deployment.
// Agents poll this like a "latest" pointer without caring about historical deployment IDs.
type PagesProjectLatestHashResponse struct {
	ProjectID    uint   `json:"project_id"`
	DeploymentID uint   `json:"deployment_id"`
	Hash         string `json:"hash"`
}
