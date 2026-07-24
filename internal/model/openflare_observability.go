// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"time"
)

// OpenFlareMetricSnapshot stores a node capacity snapshot in ClickHouse (database: openflare, table: of_node_metric_snapshots).
// ClickHouse DDL is managed by goose; reads/writes go through internal/repository/analytics.
type OpenFlareMetricSnapshot struct {
	ID                uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeID            string    `json:"node_id" gorm:"index;size:64;not null"`
	CapturedAt        time.Time `json:"captured_at" gorm:"index"`
	CPUUsagePercent   float64   `json:"cpu_usage_percent"`
	MemoryUsedBytes   int64     `json:"memory_used_bytes"`
	MemoryTotalBytes  int64     `json:"memory_total_bytes"`
	StorageUsedBytes  int64     `json:"storage_used_bytes"`
	StorageTotalBytes int64     `json:"storage_total_bytes"`
	DiskReadBytes     int64     `json:"disk_read_bytes"`
	DiskWriteBytes    int64     `json:"disk_write_bytes"`
	NetworkRxBytes    int64     `json:"network_rx_bytes"`
	NetworkTxBytes    int64     `json:"network_tx_bytes"`
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName returns the GORM table name.
func (OpenFlareMetricSnapshot) TableName() string {
	return "of_node_metric_snapshots"
}

// OpenFlareAccessLog stores a single access log row in ClickHouse (database: openflare, table: of_node_access_logs).
// ClickHouse DDL is managed by goose; reads/writes go through internal/repository/analytics.
type OpenFlareAccessLog struct {
	ID            uint64    `json:"id,string" gorm:"column:id"`
	NodeID        string    `json:"node_id" gorm:"index;size:64;not null"`
	LoggedAt      time.Time `json:"logged_at" gorm:"index"`
	RemoteAddr    string    `json:"remote_addr" gorm:"index;size:128"`
	Region        string    `json:"region" gorm:"size:128"`
	Host          string    `json:"host" gorm:"index;size:255"`
	Path          string    `json:"path" gorm:"size:2048"`
	UserAgent     string    `json:"user_agent" gorm:"column:user_agent;size:512"`
	CacheStatus   string    `json:"cache_status" gorm:"column:cache_status;size:32"`
	StatusCode    int       `json:"status_code" gorm:"index"`
	BytesSent     int64     `json:"bytes_sent" gorm:"column:bytes_sent;not null;default:0"`
	RequestLength int64     `json:"request_length" gorm:"column:request_length;not null;default:0"`
	RequestTimeMs int64     `json:"request_time_ms" gorm:"column:request_time_ms;not null;default:0"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName returns the GORM table name.
func (OpenFlareAccessLog) TableName() string {
	return "of_node_access_logs"
}

// OpenFlareAccessLogRegionCount aggregates access log regions.
type OpenFlareAccessLogRegionCount struct {
	Region string `json:"region"`
	Count  int64  `json:"count"`
}

// OpenFlareHealthEvent stores node health alert events.
type OpenFlareHealthEvent struct {
	ID               uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeID           string     `json:"node_id" gorm:"index;size:64;not null"`
	EventType        string     `json:"event_type" gorm:"index;size:64;not null"`
	Severity         string     `json:"severity" gorm:"size:16;not null"`
	Status           string     `json:"status" gorm:"index;size:16;not null"`
	Message          string     `json:"message" gorm:"type:text"`
	FirstTriggeredAt time.Time  `json:"first_triggered_at" gorm:"index"`
	LastTriggeredAt  time.Time  `json:"last_triggered_at" gorm:"index"`
	ReportedAt       time.Time  `json:"reported_at" gorm:"index"`
	ResolvedAt       *time.Time `json:"resolved_at" gorm:"index"`
	MetadataJSON     string     `json:"metadata_json" gorm:"type:text"`
	CreatedAt        time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the GORM table name.
func (OpenFlareHealthEvent) TableName() string {
	return "of_node_health_events"
}

// OpenFlareNodeSystemProfile stores the latest node system profile.
type OpenFlareNodeSystemProfile struct {
	ID               uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeID           string    `json:"node_id" gorm:"uniqueIndex;size:64;not null"`
	Hostname         string    `json:"hostname" gorm:"size:255"`
	OSName           string    `json:"os_name" gorm:"size:128"`
	OSVersion        string    `json:"os_version" gorm:"size:128"`
	KernelVersion    string    `json:"kernel_version" gorm:"size:128"`
	Architecture     string    `json:"architecture" gorm:"size:64"`
	CPUModel         string    `json:"cpu_model" gorm:"size:255"`
	CPUCores         int       `json:"cpu_cores"`
	TotalMemoryBytes int64     `json:"total_memory_bytes"`
	TotalDiskBytes   int64     `json:"total_disk_bytes"`
	UptimeSeconds    int64     `json:"uptime_seconds"`
	ReportedAt       time.Time `json:"reported_at" gorm:"index"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the GORM table name.
func (OpenFlareNodeSystemProfile) TableName() string {
	return "of_node_system_profiles"
}

// OpenFlareEdgeHealth is L2 OpenResty health (of_node_edge_health).
type OpenFlareEdgeHealth struct {
	ID          uint      `json:"id"`
	NodeID      string    `json:"node_id"`
	CapturedAt  time.Time `json:"captured_at"`
	Status      string    `json:"status"`
	Connections int64     `json:"connections"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName returns the ClickHouse table name.
func (OpenFlareEdgeHealth) TableName() string {
	return "of_node_edge_health"
}

// OpenFlareNodeObservationFrpc stores tunnel client frpc observations in ClickHouse (database: openflare, table: of_node_obs_frpc).
// ClickHouse DDL is managed by goose; reads/writes go through internal/repository/analytics.
type OpenFlareNodeObservationFrpc struct {
	ID                   uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeID               string    `json:"node_id" gorm:"index;size:64;not null"`
	CapturedAt           time.Time `json:"captured_at" gorm:"index"`
	TunnelStatus         string    `json:"tunnel_status" gorm:"size:16"`
	ConnectedRelaysCount int       `json:"connected_relays_count"`
	CreatedAt            time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName returns the GORM table name.
func (OpenFlareNodeObservationFrpc) TableName() string {
	return "of_node_obs_frpc"
}

// OpenFlareNodeObservationFrps stores tunnel relay frps observations in ClickHouse (database: openflare, table: of_node_obs_frps).
// ClickHouse DDL is managed by goose; reads/writes go through internal/repository/analytics.
type OpenFlareNodeObservationFrps struct {
	ID              uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeID          string    `json:"node_id" gorm:"index;size:64;not null"`
	CapturedAt      time.Time `json:"captured_at" gorm:"index"`
	FrpsConnections int       `json:"frps_connections"`
	FrpsProxyCount  int       `json:"frps_proxy_count"`
	FrpsClientCount int       `json:"frps_client_count"`
	FrpsProxies     string    `json:"frps_proxies" gorm:"type:text"`
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName returns the GORM table name.
func (OpenFlareNodeObservationFrps) TableName() string {
	return "of_node_obs_frps"
}

// OpenFlareAccessLogQuery filters access log list queries.
type OpenFlareAccessLogQuery struct {
	NodeID     string
	RemoteAddr string
	Host       string
	// Hosts exact-matches any host (case-insensitive). Prefer over Host for multi-domain scopes.
	Hosts     []string
	Path      string
	Since     time.Time
	Until     time.Time
	Page      int
	PageSize  int
	SortBy    string
	SortOrder string
}

// OpenFlareAccessLogBucketQuery filters folded access log queries (v1 stub).
type OpenFlareAccessLogBucketQuery struct {
	NodeID      string
	RemoteAddr  string
	Host        string
	Hosts       []string
	Path        string
	Since       time.Time
	Until       time.Time
	Page        int
	PageSize    int
	SortBy      string
	SortOrder   string
	FoldMinutes int
}

// OpenFlareAccessLogBucketRow is a folded access log bucket row (v1 stub).
type OpenFlareAccessLogBucketRow struct {
	BucketEpoch      int64 `json:"bucket_epoch"`
	RequestCount     int64 `json:"request_count"`
	UniqueIPCount    int64 `json:"unique_ip_count"`
	UniqueHostCount  int64 `json:"unique_host_count"`
	SuccessCount     int64 `json:"success_count"`
	ClientErrorCount int64 `json:"client_error_count"`
	ServerErrorCount int64 `json:"server_error_count"`
	BytesSent        int64 `json:"bytes_sent"`
	RequestLength    int64 `json:"request_length"`
}

// OpenFlareAccessLogBucketIPQuery filters folded IP summary queries (v1 stub).
type OpenFlareAccessLogBucketIPQuery struct {
	NodeID          string
	RemoteAddr      string
	Host            string
	Path            string
	BucketStartedAt time.Time
	FoldMinutes     int
	Page            int
	PageSize        int
	SortBy          string
	SortOrder       string
}

// OpenFlareAccessLogBucketIPRow is a folded IP row (v1 stub).
type OpenFlareAccessLogBucketIPRow struct {
	RemoteAddr       string `json:"remote_addr"`
	RequestCount     int64  `json:"request_count"`
	SuccessCount     int64  `json:"success_count"`
	ClientErrorCount int64  `json:"client_error_count"`
	ServerErrorCount int64  `json:"server_error_count"`
	LastSeenEpoch    int64  `json:"last_seen_epoch"`
}

// OpenFlareAccessLogIPSummaryQuery filters IP summary list queries (v1 stub).
type OpenFlareAccessLogIPSummaryQuery struct {
	NodeID     string
	RemoteAddr string
	Host       string
	Since      time.Time
	Until      time.Time
	Page       int
	PageSize   int
	SortBy     string
	SortOrder  string
}

// OpenFlareAccessLogIPSummaryRow is an IP summary row (v1 stub).
type OpenFlareAccessLogIPSummaryRow struct {
	RemoteAddr      string  `json:"remote_addr"`
	Region          string  `json:"region"`
	TotalRequests   int64   `json:"total_requests"`
	Success2xxCount int64   `json:"success_2xx_count"`
	SuccessRatio    float64 `json:"success_ratio"`
	BytesReceived   int64   `json:"bytes_received"`
	BytesSent       int64   `json:"bytes_sent"`
	// RecentRequests is deprecated and always 0.
	RecentRequests int64 `json:"recent_requests"`
	LastSeenEpoch  int64 `json:"last_seen_epoch"`
}

// OpenFlareAccessLogIPTrendQuery filters IP trend queries (v1 stub).
type OpenFlareAccessLogIPTrendQuery struct {
	NodeID        string
	RemoteAddr    string
	Host          string
	Since         time.Time
	BucketMinutes int
}

// OpenFlareAccessLogIPTrendRow is an IP trend bucket row (v1 stub).
type OpenFlareAccessLogIPTrendRow struct {
	BucketEpoch  int64 `json:"bucket_epoch"`
	RequestCount int64 `json:"request_count"`
}

// OpenFlareAccessLogWAFIPAggregate is a per-IP aggregate row for WAF automatic rules.
type OpenFlareAccessLogWAFIPAggregate struct {
	RemoteAddr       string
	RequestCount     int
	Status404Count   int
	ClientErrorCount int
	ServerErrorCount int
	IPHostCount      int
	LastSeenEpoch    int64
	StatusCounts     map[int]int
}

// OpenFlareTrafficHourly is an hourly traffic rollup row.
type OpenFlareTrafficHourly struct {
	NodeID             string    `json:"node_id"`
	Hour               time.Time `json:"hour"`
	RequestCount       int64     `json:"request_count"`
	ErrorCount         int64     `json:"error_count"`
	UniqueVisitorCount int64     `json:"unique_visitor_count"`
}

// OpenFlareAccessLogHourly is a per-node/host hourly access log rollup.
type OpenFlareAccessLogHourly struct {
	NodeID        string    `json:"node_id"`
	Hour          time.Time `json:"hour"`
	Host          string    `json:"host"`
	RequestCount  int64     `json:"request_count"`
	ErrorCount    int64     `json:"error_count"`
	BytesSent     int64     `json:"bytes_sent"`
	RequestLength int64     `json:"request_length"`
}

// OpenFlareMetricHourly is an hourly metric snapshot aggregation row.
type OpenFlareMetricHourly struct {
	Hour                      time.Time `json:"hour"`
	AverageCPUUsagePercent    float64   `json:"average_cpu_usage_percent"`
	AverageMemoryUsagePercent float64   `json:"average_memory_usage_percent"`
	NetworkRxBytes            int64     `json:"network_rx_bytes"`
	NetworkTxBytes            int64     `json:"network_tx_bytes"`
	DiskReadBytes             int64     `json:"disk_read_bytes"`
	DiskWriteBytes            int64     `json:"disk_write_bytes"`
	ReportedNodes             int       `json:"reported_nodes"`
}
