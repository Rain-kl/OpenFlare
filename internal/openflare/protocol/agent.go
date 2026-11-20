// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package protocol defines the communication protocol between OpenFlare server, agent, and relay components.
package protocol

import (
	"encoding/json"
	"errors"
	"strings"
)

// APIResponse is a generic API response wrapper.
type APIResponse[T any] struct {
	ErrorMsg string `json:"error_msg"`
	Data     T      `json:"data"`
}

// AgentSettings holds the Edge heartbeat interval and queued restart command.
type AgentSettings struct {
	HeartbeatInterval int  `json:"heartbeat_interval"`
	RestartEdgeNow    bool `json:"restart_edge_now"`
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
	LatestConfigVersion string                        `json:"latest_config_version,omitempty"`
	LastError           string                        `json:"last_error"`
	OpenrestyStatus     string                        `json:"openresty_status"`
	OpenrestyMessage    string                        `json:"openresty_message"`
	Profile             *NodeSystemProfile            `json:"profile,omitempty"`
	HostMetrics         *NodeMetricSnapshot           `json:"host_metrics,omitempty"`
	EdgeHealth          *NodeEdgeHealth               `json:"edge_health,omitempty"`
	AccessLogs          []NodeAccessLog               `json:"access_logs,omitempty"`
	Buffered            []BufferedObservabilityRecord `json:"buffered,omitempty"`
	HealthEvents        []NodeHealthEvent             `json:"health_events"`
	Engine              string                        `json:"engine,omitempty"`
	ContractVersion     int                           `json:"contract_version,omitempty"`
	ConfigSyncVersion   int                           `json:"config_sync_version"`
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

// RegisterNodeRequest carries enrollment data separately from heartbeat telemetry.
type RegisterNodeRequest struct {
	NodePayload
	RegistrationKey string `json:"registration_key,omitempty"`
}

// RegisterNodeResponse is the node registration response.
type RegisterNodeResponse struct {
	NodeID      string `json:"node_id"`
	AccessToken string `json:"agent_token"`
	Name        string `json:"name"`
}

// MarshalJSON retains access_token for consumers of older server responses.
func (r RegisterNodeResponse) MarshalJSON() ([]byte, error) {
	type response RegisterNodeResponse
	return json.Marshal(struct {
		response
		LegacyAccessToken string `json:"access_token"`
	}{response(r), r.AccessToken})
}

// UnmarshalJSON accepts either spelling used by released servers.
func (r *RegisterNodeResponse) UnmarshalJSON(data []byte) error {
	type response RegisterNodeResponse
	var decoded struct {
		response
		LegacyAccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	canonical := strings.TrimSpace(decoded.AccessToken)
	legacy := strings.TrimSpace(decoded.LegacyAccessToken)
	if canonical != "" && legacy != "" && canonical != legacy {
		return errors.New("registration response contains conflicting agent tokens")
	}
	*r = RegisterNodeResponse(decoded.response)
	r.AccessToken = canonical
	if canonical == "" {
		r.AccessToken = legacy
	}
	return nil
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
	PackageSize  int64  `json:"package_size"`
	FileCount    int    `json:"file_count"`
	TotalSize    int64  `json:"total_size"`
}

// EdgeConfigSyncVersion is the resource manifest wire protocol understood by the controller.
const EdgeConfigSyncVersion = 1

// ConfigManifest advertises resources from all configuration providers.
type ConfigManifest struct {
	Version   int                       `json:"version"`
	Resources map[string]ConfigResource `json:"resources"`
}

// ConfigResource describes the exact JSON bytes of one configuration resource.
type ConfigResource struct {
	SHA256     string `json:"sha256"`
	InlineJSON string `json:"inline_json,omitempty"`
	Revision   string `json:"revision,omitempty"`
	Metadata   any    `json:"metadata,omitempty"`
}
