// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	analyticsrepo "github.com/Rain-kl/Wavelet/internal/repository/analytics"
	"gorm.io/gorm"
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

// OpenFlareRequestReport stores aggregated traffic windows per node in ClickHouse (database: openflare, table: of_node_request_reports).
// ClickHouse DDL is managed by goose; reads/writes go through internal/repository/analytics.
type OpenFlareRequestReport struct {
	ID                  uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeID              string    `json:"node_id" gorm:"index;size:64;not null"`
	WindowStartedAt     time.Time `json:"window_started_at" gorm:"index"`
	WindowEndedAt       time.Time `json:"window_ended_at" gorm:"index"`
	RequestCount        int64     `json:"request_count"`
	ErrorCount          int64     `json:"error_count"`
	UniqueVisitorCount  int64     `json:"unique_visitor_count"`
	StatusCodesJSON     string    `json:"status_codes_json" gorm:"type:text"`
	TopDomainsJSON      string    `json:"top_domains_json" gorm:"type:text"`
	SourceCountriesJSON string    `json:"source_countries_json" gorm:"type:text"`
	CreatedAt           time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName returns the GORM table name.
func (OpenFlareRequestReport) TableName() string {
	return "of_node_request_reports"
}

// OpenFlareAccessLog stores a single access log row in ClickHouse (database: openflare, table: of_node_access_logs).
// ClickHouse DDL is managed by goose; reads/writes go through internal/repository/analytics.
type OpenFlareAccessLog struct {
	ID         uint64    `json:"id,string" gorm:"column:id"`
	NodeID     string    `json:"node_id" gorm:"index;size:64;not null"`
	LoggedAt   time.Time `json:"logged_at" gorm:"index"`
	RemoteAddr string    `json:"remote_addr" gorm:"index;size:128"`
	Region     string    `json:"region" gorm:"size:128"`
	Host       string    `json:"host" gorm:"index;size:255"`
	Path       string    `json:"path" gorm:"size:2048"`
	StatusCode int       `json:"status_code" gorm:"index"`
	CreatedAt  time.Time `json:"created_at" gorm:"autoCreateTime"`
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

// OpenFlareNodeObservationOpenresty stores openresty network observations in ClickHouse (database: openflare, table: of_node_obs_openresty).
// ClickHouse DDL is managed by goose; reads/writes go through internal/repository/analytics.
type OpenFlareNodeObservationOpenresty struct {
	ID                   uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeID               string    `json:"node_id" gorm:"index;size:64;not null"`
	CapturedAt           time.Time `json:"captured_at" gorm:"index"`
	OpenrestyRxBytes     int64     `json:"openresty_rx_bytes"`
	OpenrestyTxBytes     int64     `json:"openresty_tx_bytes"`
	OpenrestyConnections int64     `json:"openresty_connections"`
	CreatedAt            time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName returns the GORM table name.
func (OpenFlareNodeObservationOpenresty) TableName() string {
	return "of_node_obs_openresty"
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
	Path       string
	Since      time.Time
	Until      time.Time
	Page       int
	PageSize   int
	SortBy     string
	SortOrder  string
}

// OpenFlareAccessLogBucketQuery filters folded access log queries (v1 stub).
type OpenFlareAccessLogBucketQuery struct {
	NodeID      string
	RemoteAddr  string
	Host        string
	Path        string
	Since       time.Time
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
	Page       int
	PageSize   int
	SortBy     string
	SortOrder  string
}

// OpenFlareAccessLogIPSummaryRow is an IP summary row (v1 stub).
type OpenFlareAccessLogIPSummaryRow struct {
	RemoteAddr     string `json:"remote_addr"`
	TotalRequests  int64  `json:"total_requests"`
	RecentRequests int64  `json:"recent_requests"`
	LastSeenEpoch  int64  `json:"last_seen_epoch"`
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

func isMissingTableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such table") ||
		strings.Contains(msg, "doesn't exist") ||
		strings.Contains(msg, "does not exist")
}

// InsertOpenFlareMetricSnapshot inserts a metric snapshot into ClickHouse.
func InsertOpenFlareMetricSnapshot(ctx context.Context, record *OpenFlareMetricSnapshot) error {
	return currentObservabilityStore().InsertMetricSnapshot(ctx, record)
}

// InsertOpenFlareRequestReport inserts a request report into ClickHouse.
func InsertOpenFlareRequestReport(ctx context.Context, record *OpenFlareRequestReport) error {
	return currentObservabilityStore().InsertRequestReport(ctx, record)
}

// InsertOpenFlareNodeObservationOpenresty inserts an OpenResty observation into ClickHouse.
func InsertOpenFlareNodeObservationOpenresty(ctx context.Context, record *OpenFlareNodeObservationOpenresty) error {
	return currentObservabilityStore().InsertNodeObservationOpenresty(ctx, record)
}

// InsertOpenFlareNodeObservationFrps inserts an FRPS observation into ClickHouse.
func InsertOpenFlareNodeObservationFrps(ctx context.Context, record *OpenFlareNodeObservationFrps) error {
	return currentObservabilityStore().InsertNodeObservationFrps(ctx, record)
}

// InsertOpenFlareNodeObservationFrpc inserts an FRPC observation into ClickHouse.
func InsertOpenFlareNodeObservationFrpc(ctx context.Context, record *OpenFlareNodeObservationFrpc) error {
	return currentObservabilityStore().InsertNodeObservationFrpc(ctx, record)
}

// ListOpenFlareMetricSnapshotsSince returns metric snapshots since the given time.
func ListOpenFlareMetricSnapshotsSince(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareMetricSnapshot, error) {
	return currentObservabilityStore().ListMetricSnapshots(ctx, nodeID, since, limit)
}

// ListOpenFlareRequestReportsSince returns request reports since the given time.
func ListOpenFlareRequestReportsSince(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareRequestReport, error) {
	return currentObservabilityStore().ListRequestReports(ctx, nodeID, since, limit)
}

// OpenFlareTrafficHourly is an hourly traffic rollup row.
type OpenFlareTrafficHourly struct {
	NodeID             string    `json:"node_id"`
	Hour               time.Time `json:"hour"`
	RequestCount       int64     `json:"request_count"`
	ErrorCount         int64     `json:"error_count"`
	UniqueVisitorCount int64     `json:"unique_visitor_count"`
}

// ListOpenFlareTrafficHourlySince returns hourly traffic rollup rows since the given time.
func ListOpenFlareTrafficHourlySince(ctx context.Context, nodeID string, since time.Time) ([]*OpenFlareTrafficHourly, error) {
	rows, err := analyticsrepo.ListNodeTrafficHourly(ctx, analyticsrepo.NodeObservabilityFilter{
		NodeID: nodeID,
		Since:  since,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*OpenFlareTrafficHourly, len(rows))
	for index, row := range rows {
		result[index] = &OpenFlareTrafficHourly{
			NodeID:             row.NodeID,
			Hour:               row.Hour,
			RequestCount:       row.RequestCount,
			ErrorCount:         row.ErrorCount,
			UniqueVisitorCount: row.UniqueVisitorCount,
		}
	}
	return result, nil
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

// OpenFlareOpenrestyHourly is an hourly OpenResty observation aggregation row.
type OpenFlareOpenrestyHourly struct {
	Hour             time.Time `json:"hour"`
	OpenrestyRxBytes int64     `json:"openresty_rx_bytes"`
	OpenrestyTxBytes int64     `json:"openresty_tx_bytes"`
	ReportedNodes    int       `json:"reported_nodes"`
}

// ListOpenFlareMetricHourlySince returns hourly metric aggregates since the given time.
func ListOpenFlareMetricHourlySince(ctx context.Context, nodeID string, since time.Time) ([]*OpenFlareMetricHourly, error) {
	rows, err := analyticsrepo.ListNodeMetricHourly(ctx, analyticsrepo.NodeObservabilityFilter{
		NodeID: nodeID,
		Since:  since,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*OpenFlareMetricHourly, len(rows))
	for index, row := range rows {
		result[index] = &OpenFlareMetricHourly{
			Hour:                      row.Hour,
			AverageCPUUsagePercent:    row.AverageCPUUsagePercent,
			AverageMemoryUsagePercent: row.AverageMemoryUsagePercent,
			NetworkRxBytes:            row.NetworkRxBytes,
			NetworkTxBytes:            row.NetworkTxBytes,
			DiskReadBytes:             row.DiskReadBytes,
			DiskWriteBytes:            row.DiskWriteBytes,
			ReportedNodes:             row.ReportedNodes,
		}
	}
	return result, nil
}

// ListOpenFlareOpenrestyHourlySince returns hourly OpenResty aggregates since the given time.
func ListOpenFlareOpenrestyHourlySince(ctx context.Context, nodeID string, since time.Time) ([]*OpenFlareOpenrestyHourly, error) {
	rows, err := analyticsrepo.ListNodeOpenrestyHourly(ctx, analyticsrepo.NodeObservabilityFilter{
		NodeID: nodeID,
		Since:  since,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*OpenFlareOpenrestyHourly, len(rows))
	for index, row := range rows {
		result[index] = &OpenFlareOpenrestyHourly{
			Hour:             row.Hour,
			OpenrestyRxBytes: row.OpenrestyRxBytes,
			OpenrestyTxBytes: row.OpenrestyTxBytes,
			ReportedNodes:    row.ReportedNodes,
		}
	}
	return result, nil
}

// ListOpenFlareActiveHealthEvents returns active health events across all nodes.
func ListOpenFlareActiveHealthEvents(ctx context.Context) ([]*OpenFlareHealthEvent, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var rows []*OpenFlareHealthEvent
	if err := conn.Where("status = ?", "active").Order("last_triggered_at desc").Find(&rows).Error; err != nil {
		if isMissingTableError(err) {
			return []*OpenFlareHealthEvent{}, nil
		}
		return nil, err
	}
	return rows, nil
}

// ListOpenFlareHealthEvents returns health events for a node.
func ListOpenFlareHealthEvents(ctx context.Context, nodeID string, activeOnly bool, limit int) ([]*OpenFlareHealthEvent, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	query := conn.Model(&OpenFlareHealthEvent{}).Where("node_id = ?", nodeID).Order("last_triggered_at desc")
	if activeOnly {
		query = query.Where("status = ?", "active")
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	var rows []*OpenFlareHealthEvent
	if err := query.Find(&rows).Error; err != nil {
		if isMissingTableError(err) {
			return []*OpenFlareHealthEvent{}, nil
		}
		return nil, err
	}
	return rows, nil
}

// DeleteOpenFlareMetricSnapshotsBefore deletes metric snapshots captured before cutoff.
func DeleteOpenFlareMetricSnapshotsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return currentObservabilityStore().DeleteMetricSnapshotsBefore(ctx, cutoff)
}

// DeleteAllOpenFlareMetricSnapshots deletes all metric snapshots.
func DeleteAllOpenFlareMetricSnapshots(ctx context.Context) (int64, error) {
	return currentObservabilityStore().DeleteAllMetricSnapshots(ctx)
}

// DeleteOpenFlareRequestReportsBefore deletes request reports ending before cutoff.
func DeleteOpenFlareRequestReportsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return currentObservabilityStore().DeleteRequestReportsBefore(ctx, cutoff)
}

// DeleteAllOpenFlareRequestReports deletes all request reports.
func DeleteAllOpenFlareRequestReports(ctx context.Context) (int64, error) {
	return currentObservabilityStore().DeleteAllRequestReports(ctx)
}

// DeleteOpenFlareNodeObservationOpenrestyBefore deletes OpenResty observations captured before cutoff.
func DeleteOpenFlareNodeObservationOpenrestyBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return currentObservabilityStore().DeleteNodeObservationOpenrestyBefore(ctx, cutoff)
}

// DeleteAllOpenFlareNodeObservationOpenresty deletes all OpenResty observations.
func DeleteAllOpenFlareNodeObservationOpenresty(ctx context.Context) (int64, error) {
	return currentObservabilityStore().DeleteAllNodeObservationOpenresty(ctx)
}

// DeleteOpenFlareNodeObservationFrpsBefore deletes FRPS observations captured before cutoff.
func DeleteOpenFlareNodeObservationFrpsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return currentObservabilityStore().DeleteNodeObservationFrpsBefore(ctx, cutoff)
}

// DeleteAllOpenFlareNodeObservationFrps deletes all FRPS observations.
func DeleteAllOpenFlareNodeObservationFrps(ctx context.Context) (int64, error) {
	return currentObservabilityStore().DeleteAllNodeObservationFrps(ctx)
}

// DeleteOpenFlareNodeObservationFrpcBefore deletes FRPC observations captured before cutoff.
func DeleteOpenFlareNodeObservationFrpcBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return currentObservabilityStore().DeleteNodeObservationFrpcBefore(ctx, cutoff)
}

// DeleteAllOpenFlareNodeObservationFrpc deletes all FRPC observations.
func DeleteAllOpenFlareNodeObservationFrpc(ctx context.Context) (int64, error) {
	return currentObservabilityStore().DeleteAllNodeObservationFrpc(ctx)
}

// DeleteOpenFlareHealthEventsByNodeID deletes all health events for a node.
func DeleteOpenFlareHealthEventsByNodeID(ctx context.Context, nodeID string) (int64, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return 0, errors.New(errDatabaseNotInitialized)
	}
	result := conn.Where("node_id = ?", nodeID).Delete(&OpenFlareHealthEvent{})
	if result.Error != nil {
		if isMissingTableError(result.Error) {
			return 0, nil
		}
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// GetOpenFlareNodeSystemProfile returns the system profile for a node.
func GetOpenFlareNodeSystemProfile(ctx context.Context, nodeID string) (*OpenFlareNodeSystemProfile, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var profile OpenFlareNodeSystemProfile
	if err := conn.Where("node_id = ?", nodeID).First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || isMissingTableError(err) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &profile, nil
}

// ListOpenFlareNodeObservationOpenresty returns openresty observations.
func ListOpenFlareNodeObservationOpenresty(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareNodeObservationOpenresty, error) {
	return currentObservabilityStore().ListNodeObservationOpenresty(ctx, nodeID, since, limit)
}

// ListOpenFlareNodeObservationFrpc returns frpc observations.
func ListOpenFlareNodeObservationFrpc(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareNodeObservationFrpc, error) {
	return currentObservabilityStore().ListNodeObservationFrpc(ctx, nodeID, since, limit)
}

// ListOpenFlareNodeObservationFrps returns frps observations.
func ListOpenFlareNodeObservationFrps(ctx context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareNodeObservationFrps, error) {
	return currentObservabilityStore().ListNodeObservationFrps(ctx, nodeID, since, limit)
}
