package protocol

type APIResponse[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type HeartbeatAPIResponse struct {
	Success       bool              `json:"success"`
	Message       string            `json:"message"`
	Data          any               `json:"data"`
	AgentSettings *AgentSettings    `json:"agent_settings,omitempty"`
	ActiveConfig  *ActiveConfigMeta `json:"active_config,omitempty"`
}

type HeartbeatResult struct {
	AgentSettings *AgentSettings
	ActiveConfig  *ActiveConfigMeta
}

type AgentSettings struct {
	HeartbeatInterval   int    `json:"heartbeat_interval"`
	AutoUpdate          bool   `json:"auto_update"`
	UpdateRepo          string `json:"update_repo"`
	UpdateNow           bool   `json:"update_now"`
	UpdateChannel       string `json:"update_channel"`
	UpdateTag           string `json:"update_tag"`
	RestartOpenrestyNow bool   `json:"restart_openresty_now"`
}

const (
	OpenrestyStatusHealthy   = "healthy"
	OpenrestyStatusUnhealthy = "unhealthy"
	OpenrestyStatusUnknown   = "unknown"
)

type NodePayload struct {
	NodeID                string                        `json:"node_id"`
	Name                  string                        `json:"name"`
	IP                    string                        `json:"ip"`
	AgentVersion          string                        `json:"agent_version"`
	NginxVersion          string                        `json:"nginx_version"`
	CurrentVersion        string                        `json:"current_version"`
	LastError             string                        `json:"last_error"`
	OpenrestyStatus       string                        `json:"openresty_status"`
	OpenrestyMessage      string                        `json:"openresty_message"`
	Profile               *NodeSystemProfile            `json:"profile,omitempty"`
	Snapshot              *NodeMetricSnapshot           `json:"snapshot,omitempty"`
	TrafficReport         *NodeTrafficReport            `json:"traffic_report,omitempty"`
	BufferedObservability []BufferedObservabilityRecord `json:"buffered_observability,omitempty"`
	HealthEvents          []NodeHealthEvent             `json:"health_events"`
}

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

type NodeMetricSnapshot struct {
	CapturedAtUnix       int64   `json:"captured_at_unix"`
	CPUUsagePercent      float64 `json:"cpu_usage_percent"`
	MemoryUsedBytes      int64   `json:"memory_used_bytes"`
	MemoryTotalBytes     int64   `json:"memory_total_bytes"`
	StorageUsedBytes     int64   `json:"storage_used_bytes"`
	StorageTotalBytes    int64   `json:"storage_total_bytes"`
	DiskReadBytes        int64   `json:"disk_read_bytes"`
	DiskWriteBytes       int64   `json:"disk_write_bytes"`
	NetworkRxBytes       int64   `json:"network_rx_bytes"`
	NetworkTxBytes       int64   `json:"network_tx_bytes"`
	OpenrestyRxBytes     int64   `json:"openresty_rx_bytes"`
	OpenrestyTxBytes     int64   `json:"openresty_tx_bytes"`
	OpenrestyConnections int64   `json:"openresty_connections"`
}

type NodeTrafficReport struct {
	WindowStartedAtUnix int64            `json:"window_started_at_unix"`
	WindowEndedAtUnix   int64            `json:"window_ended_at_unix"`
	RequestCount        int64            `json:"request_count"`
	ErrorCount          int64            `json:"error_count"`
	UniqueVisitorCount  int64            `json:"unique_visitor_count"`
	StatusCodes         map[string]int64 `json:"status_codes"`
	TopDomains          map[string]int64 `json:"top_domains"`
	SourceCountries     map[string]int64 `json:"source_countries"`
}

type BufferedObservabilityRecord struct {
	WindowStartedAtUnix int64               `json:"window_started_at_unix"`
	Snapshot            *NodeMetricSnapshot `json:"snapshot,omitempty"`
	TrafficReport       *NodeTrafficReport  `json:"traffic_report,omitempty"`
}

type NodeHealthEvent struct {
	EventType       string            `json:"event_type"`
	Severity        string            `json:"severity"`
	Message         string            `json:"message"`
	TriggeredAtUnix int64             `json:"triggered_at_unix"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

type RegisterNodeResponse struct {
	NodeID     string `json:"node_id"`
	AgentToken string `json:"agent_token"`
	Name       string `json:"name"`
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

type ActiveConfigResponse struct {
	Version        string        `json:"version"`
	Checksum       string        `json:"checksum"`
	MainConfig     string        `json:"main_config"`
	RouteConfig    string        `json:"route_config"`
	RenderedConfig string        `json:"rendered_config"`
	SupportFiles   []SupportFile `json:"support_files"`
	CreatedAt      string        `json:"created_at"`
}

type ActiveConfigMeta struct {
	Version  string `json:"version"`
	Checksum string `json:"checksum"`
}

type SupportFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}
