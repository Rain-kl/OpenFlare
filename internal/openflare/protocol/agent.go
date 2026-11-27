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

// AgentSettings holds the Edge heartbeat interval and runtime commands.
type AgentSettings struct {
	HeartbeatInterval int                 `json:"heartbeat_interval"`
	RestartEdgeNow    bool                `json:"restart_edge_now"`
	EdgeUpdate        *EdgeUpdateSettings `json:"edge_update,omitempty"`
}

// EdgeUpdateSettings specifies a pinned manual update or periodic stable updates.
type EdgeUpdateSettings struct {
	Repo       string `json:"repo"`
	AutoUpdate bool   `json:"auto_update"`
	Tag        string `json:"tag,omitempty"`
}

// WSMessageType constants define WebSocket message types.
const (
	WSMessageTypeStatus          = "status"
	WSMessageTypeSettings        = "settings"
	WSMessageTypeActiveConfig    = "active_config"
	WSMessageTypeForceSyncConfig = "force_sync_config"
	WSMessageTypeWAFIPGroups     = "waf_ip_groups"
	WSMessageTypeQuarantined     = "quarantined"
	WSMessageTypePing            = "ping"
	WSMessageTypePong            = "pong"
	WSMessageTypePingTask        = "ping_task"
	WSMessageTypePingResult      = "ping_result"
	WSMessageTypeNetProbeTask    = "net_probe_task"
	WSMessageTypeNetProbeResult  = "net_probe_result"
	WSMessageTypeRemoteExec      = "remote_exec"
)

// PingTaskDispatch instructs an edge node to execute one latency probe.
type PingTaskDispatch struct {
	PingTaskID uint64 `json:"ping_task_id"`
	PingType   string `json:"ping_type"`
	PingTarget string `json:"ping_target"`
}

// PingResultPayload carries one latency probe measurement reported by an edge node.
type PingResultPayload struct {
	TaskID     uint64 `json:"task_id"`
	PingType   string `json:"ping_type"`
	Value      int    `json:"value"`
	FinishedAt string `json:"finished_at"`
}

// RemoteExecDispatch instructs an edge node to execute a shell command.
type RemoteExecDispatch struct {
	TaskID         string `json:"task_id"`
	Command        string `json:"command"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

// RemoteExecResultPayload carries a node's remote execution output and exit code.
type RemoteExecResultPayload struct {
	TaskID     string `json:"task_id"`
	Result     string `json:"result"`
	ExitCode   int    `json:"exit_code"`
	FinishedAt string `json:"finished_at"`
}

// PingBatchRequest configures a multi-sample latency probe task on an edge node.
type PingBatchRequest struct {
	Protocol   string  `json:"protocol"`
	Target     string  `json:"target"`
	Port       *uint16 `json:"port,omitempty"`
	Count      uint8   `json:"count"`
	IntervalMs uint64  `json:"interval_ms"`
	TimeoutMs  *uint64 `json:"timeout_ms,omitempty"`
}

// PingSample records a single sample inside a batch ping probe.
type PingSample struct {
	Seq     uint8 `json:"seq"`
	RTTMs   int64 `json:"rtt_ms"`
	Success bool  `json:"success"`
}

// PingBatchResult holds aggregated statistics for a multi-sample latency probe.
type PingBatchResult struct {
	Protocol   string       `json:"protocol"`
	Target     string       `json:"target"`
	ResolvedIP string       `json:"resolved_ip,omitempty"`
	Sent       uint8        `json:"sent"`
	Received   uint8        `json:"received"`
	LossRate   float64      `json:"loss_rate"`
	MinMs      float64      `json:"min_ms"`
	AvgMs      float64      `json:"avg_ms"`
	MaxMs      float64      `json:"max_ms"`
	P95Ms      float64      `json:"p95_ms"`
	JitterMs   float64      `json:"jitter_ms"`
	Samples    []PingSample `json:"samples"`
	Error      string       `json:"error,omitempty"`
}

// TracerouteRequest configures an active multi-round traceroute task on an edge node.
type TracerouteRequest struct {
	Target       string  `json:"target"`
	Protocol     string  `json:"protocol"`
	Port         *uint16 `json:"port,omitempty"`
	MaxHops      uint8   `json:"max_hops"`
	ProbesPerHop uint8   `json:"probes_per_hop"`
	TimeoutMs    *uint64 `json:"timeout_ms,omitempty"`
}

// TracerouteProbe represents one round sample at a traceroute hop.
type TracerouteProbe struct {
	Seq   uint8    `json:"seq"`
	IP    string   `json:"ip,omitempty"`
	RTTMs *float64 `json:"rtt_ms,omitempty"`
}

// TracerouteHop summarizes measurements at a single TTL hop.
type TracerouteHop struct {
	TTL      uint8             `json:"ttl"`
	IP       string            `json:"ip,omitempty"`
	AltIPs   []string          `json:"alt_ips,omitempty"`
	Sent     uint16            `json:"sent"`
	Recv     uint16            `json:"recv"`
	LossPct  float64           `json:"loss_pct"`
	MinMs    *float64          `json:"min_ms,omitempty"`
	AvgMs    *float64          `json:"avg_ms,omitempty"`
	MaxMs    *float64          `json:"max_ms,omitempty"`
	JitterMs *float64          `json:"jitter_ms,omitempty"`
	Probes   []TracerouteProbe `json:"probes"`
}

// TracerouteResult captures the completed traceroute output reported by a node.
type TracerouteResult struct {
	Target     string          `json:"target"`
	ResolvedIP string          `json:"resolved_ip,omitempty"`
	Protocol   string          `json:"protocol"`
	HopCount   uint8           `json:"hop_count"`
	Reached    bool            `json:"reached"`
	TotalRTTMs float64         `json:"total_rtt_ms"`
	Hops       []TracerouteHop `json:"hops"`
	Error      string          `json:"error,omitempty"`
}

// DNSQueryRequest configures a DNS probe query on an edge node.
type DNSQueryRequest struct {
	Domain     string  `json:"domain"`
	RecordType string  `json:"record_type"`
	Resolver   string  `json:"resolver"`
	ECSSubnet  string  `json:"ecs_subnet,omitempty"`
	TimeoutMs  *uint64 `json:"timeout_ms,omitempty"`
}

// DNSRecord represents a single DNS answer record returned by a probe.
type DNSRecord struct {
	Name       string `json:"name"`
	RecordType string `json:"record_type"`
	TTL        uint32 `json:"ttl"`
	Value      string `json:"value"`
}

// DNSQueryResult carries the structured response from a DNS probe query.
type DNSQueryResult struct {
	Domain        string      `json:"domain"`
	RecordType    string      `json:"record_type"`
	Resolver      string      `json:"resolver"`
	RCode         string      `json:"rcode"`
	Authoritative bool        `json:"authoritative"`
	Truncated     bool        `json:"truncated"`
	UsedTCP       bool        `json:"used_tcp"`
	LatencyMs     float64     `json:"latency_ms"`
	Records       []DNSRecord `json:"records"`
	Error         string      `json:"error,omitempty"`
}

// NetProbeTaskDispatch instructs an edge node to run a network diagnostic probe in its isolated worker.
type NetProbeTaskDispatch struct {
	ProbeID    string             `json:"probe_id"`
	Kind       string             `json:"kind"`
	Ping       *PingBatchRequest  `json:"ping,omitempty"`
	Traceroute *TracerouteRequest `json:"traceroute,omitempty"`
	DNS        *DNSQueryRequest   `json:"dns,omitempty"`
}

// NetProbeResultPayload carries a completed network diagnostic probe result from an edge node.
type NetProbeResultPayload struct {
	ProbeID    string            `json:"probe_id"`
	Kind       string            `json:"kind"`
	Ping       *PingBatchResult  `json:"ping,omitempty"`
	Traceroute *TracerouteResult `json:"traceroute,omitempty"`
	DNS        *DNSQueryResult   `json:"dns,omitempty"`
	Error      string            `json:"error,omitempty"`
	FinishedAt string            `json:"finished_at"`
}

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
	EdgeUpdateSupported bool                          `json:"edge_update_supported,omitempty"`
	EdgeUpdateReady     bool                          `json:"edge_update_ready,omitempty"`
}

// NodeEdgeHealth is an instantaneous OpenResty health snapshot (L2).
type NodeEdgeHealth struct {
	CapturedAtUnix int64  `json:"captured_at_unix"`
	Status         string `json:"status"`
	Message        string `json:"message"`
	Connections    int64  `json:"connections"`
}

// NodeSystemProfile describes the Komari-aligned system profile of a node.
type NodeSystemProfile struct {
	Hostname         string `json:"hostname"`
	OSName           string `json:"os_name"`
	OSVersion        string `json:"os_version"`
	KernelVersion    string `json:"kernel_version"`
	Architecture     string `json:"architecture"`
	CPUModel         string `json:"cpu_model"`
	CPUCores         int    `json:"cpu_cores"`
	CPUPhysicalCores int    `json:"cpu_physical_cores,omitempty"`
	Virtualization   string `json:"virtualization,omitempty"`
	GPUName          string `json:"gpu_name,omitempty"`
	TotalMemoryBytes int64  `json:"total_memory_bytes"`
	SwapTotalBytes   int64  `json:"swap_total_bytes,omitempty"`
	TotalDiskBytes   int64  `json:"total_disk_bytes"`
	IPv4             string `json:"ipv4,omitempty"`
	IPv6             string `json:"ipv6,omitempty"`
	ProbeVersion     string `json:"probe_version,omitempty"`
	UptimeSeconds    int64  `json:"uptime_seconds"`
	ReportedAtUnix   int64  `json:"reported_at_unix"`
}

// NodeMetricSnapshot is a Komari-aligned real-time metric snapshot of a node.
type NodeMetricSnapshot struct {
	CapturedAtUnix    int64   `json:"captured_at_unix"`
	CPUUsagePercent   float64 `json:"cpu_usage_percent"`
	GPUUsagePercent   float64 `json:"gpu_usage_percent,omitempty"`
	MemoryUsedBytes   int64   `json:"memory_used_bytes"`
	MemoryTotalBytes  int64   `json:"memory_total_bytes"`
	SwapUsedBytes     int64   `json:"swap_used_bytes,omitempty"`
	SwapTotalBytes    int64   `json:"swap_total_bytes,omitempty"`
	Load1             float64 `json:"load1,omitempty"`
	Load5             float64 `json:"load5,omitempty"`
	Load15            float64 `json:"load15,omitempty"`
	Temp              float64 `json:"temp,omitempty"`
	StorageUsedBytes  int64   `json:"storage_used_bytes"`
	StorageTotalBytes int64   `json:"storage_total_bytes"`
	DiskReadBytes     int64   `json:"disk_read_bytes"`
	DiskWriteBytes    int64   `json:"disk_write_bytes"`
	NetIn             int64   `json:"net_in,omitempty"`
	NetOut            int64   `json:"net_out,omitempty"`
	NetTotalUp        int64   `json:"net_total_up,omitempty"`
	NetTotalDown      int64   `json:"net_total_down,omitempty"`
	ProcessCount      int     `json:"process_count,omitempty"`
	TCPConnections    int     `json:"tcp_connections,omitempty"`
	UDPConnections    int     `json:"udp_connections,omitempty"`
	UptimeSeconds     int64   `json:"uptime_seconds,omitempty"`
}

// NodeAccessLog is an access log entry from agent (L1 business fact).
type NodeAccessLog struct {
	LoggedAtUnix   int64     `json:"logged_at_unix"`
	RemoteAddr     string    `json:"remote_addr"`
	Host           string    `json:"host"`
	Path           string    `json:"path"`
	UserAgent      string    `json:"user_agent,omitempty"`
	CacheStatus    string    `json:"cache_status,omitempty"` // $upstream_cache_status
	StatusCode     int       `json:"status_code"`
	BytesSent      int64     `json:"bytes_sent"`      // body bytes = 已提供数据
	RequestLength  int64     `json:"request_length"`  // 接收数据
	RequestTimeMs  int64     `json:"request_time_ms"` // optional
	TCPRTTUs       uint32    `json:"tcp_rtt_us,omitempty"`
	TCPMinRTTUs    uint32    `json:"tcp_min_rtt_us,omitempty"`
	TCPRTTVarUs    uint32    `json:"tcp_rtt_var_us,omitempty"`
	TCPRetrans     uint32    `json:"tcp_retrans,omitempty"`
	TCPSegsOut     uint32    `json:"tcp_segs_out,omitempty"`
	TLSHandshakeMs uint32    `json:"tls_handshake_ms,omitempty"`
	WAF            *WAFAudit `json:"waf,omitempty"`
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

// TLSKeyRequestItem identifies one public certificate chain whose private key the node needs in memory.
type TLSKeyRequestItem struct {
	CertificateID uint   `json:"certificate_id"`
	CertSHA256    string `json:"cert_sha256"`
}

// FetchTLSKeysRequest asks the controller for memory-only private keys matching an active release.
type FetchTLSKeysRequest struct {
	ReleaseSequence int64               `json:"release_sequence"`
	Certificates    []TLSKeyRequestItem `json:"certificates"`
}

// FetchTLSKeysResponse returns private keys indexed by decimal certificate ID for in-memory loading only.
type FetchTLSKeysResponse struct {
	ReleaseSequence int64             `json:"release_sequence"`
	Keys            map[string]string `json:"keys"`
}
