// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"fmt"
	"time"
)

const (
	nodeMetricSnapshotTableName     = "of_node_metric_snapshots"
	nodeMetricSnapshotInsertColumns = "id, node_id, captured_at, cpu_usage_percent, memory_used_bytes, memory_total_bytes, storage_used_bytes, storage_total_bytes, disk_read_bytes, disk_write_bytes, network_rx_bytes, network_tx_bytes, created_at"

	nodeEdgeHealthTableName     = "of_node_edge_health"
	nodeEdgeHealthInsertColumns = "id, node_id, captured_at, status, connections, created_at"

	nodeObsFrpsTableName     = "of_node_obs_frps"
	nodeObsFrpsInsertColumns = "id, node_id, captured_at, frps_connections, frps_proxy_count, frps_client_count, frps_proxies, created_at"

	nodeObsFrpcTableName     = "of_node_obs_frpc"
	nodeObsFrpcInsertColumns = "id, node_id, captured_at, tunnel_status, connected_relays_count, created_at"
)

// NodeMetricSnapshot stores periodic node resource utilization metrics in ClickHouse.
type NodeMetricSnapshot struct {
	ID                uint64    `gorm:"column:id"`
	NodeID            string    `gorm:"column:node_id"`
	CapturedAt        time.Time `gorm:"column:captured_at"`
	CPUUsagePercent   float64   `gorm:"column:cpu_usage_percent"`
	MemoryUsedBytes   int64     `gorm:"column:memory_used_bytes"`
	MemoryTotalBytes  int64     `gorm:"column:memory_total_bytes"`
	StorageUsedBytes  int64     `gorm:"column:storage_used_bytes"`
	StorageTotalBytes int64     `gorm:"column:storage_total_bytes"`
	DiskReadBytes     int64     `gorm:"column:disk_read_bytes"`
	DiskWriteBytes    int64     `gorm:"column:disk_write_bytes"`
	NetworkRxBytes    int64     `gorm:"column:network_rx_bytes"`
	NetworkTxBytes    int64     `gorm:"column:network_tx_bytes"`
	CreatedAt         time.Time `gorm:"column:created_at"`
}

// TableName returns the ClickHouse table name.
func (NodeMetricSnapshot) TableName() string {
	return nodeMetricSnapshotTableName
}

// InsertColumns returns comma-separated column names for batch insert.
func (NodeMetricSnapshot) InsertColumns() string {
	return nodeMetricSnapshotInsertColumns
}

// BatchInsertSQL returns the INSERT prefix used by native batch writers.
func (NodeMetricSnapshot) BatchInsertSQL() string {
	return fmt.Sprintf("INSERT INTO %s (%s)", nodeMetricSnapshotTableName, nodeMetricSnapshotInsertColumns)
}

// NodeEdgeHealth stores L2 OpenResty health snapshots (connections + status).
type NodeEdgeHealth struct {
	ID          uint64    `gorm:"column:id"`
	NodeID      string    `gorm:"column:node_id"`
	CapturedAt  time.Time `gorm:"column:captured_at"`
	Status      string    `gorm:"column:status"`
	Connections int64     `gorm:"column:connections"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

// TableName returns the ClickHouse table name.
func (NodeEdgeHealth) TableName() string {
	return nodeEdgeHealthTableName
}

// InsertColumns returns comma-separated column names for batch insert.
func (NodeEdgeHealth) InsertColumns() string {
	return nodeEdgeHealthInsertColumns
}

// BatchInsertSQL returns the INSERT prefix used by native batch writers.
func (NodeEdgeHealth) BatchInsertSQL() string {
	return fmt.Sprintf("INSERT INTO %s (%s)", nodeEdgeHealthTableName, nodeEdgeHealthInsertColumns)
}

// AccessLogHourly is a Server-side hourly rollup of access logs.
type AccessLogHourly struct {
	NodeID        string    `gorm:"column:node_id"`
	Hour          time.Time `gorm:"column:hour"`
	Host          string    `gorm:"column:host"`
	RequestCount  int64     `gorm:"column:request_count"`
	ErrorCount    int64     `gorm:"column:error_count"`
	BytesSent     int64     `gorm:"column:bytes_sent"`
	RequestLength int64     `gorm:"column:request_length"`
}

// NodeObsFrps stores FRPS observability snapshots in ClickHouse.
type NodeObsFrps struct {
	ID              uint64    `gorm:"column:id"`
	NodeID          string    `gorm:"column:node_id"`
	CapturedAt      time.Time `gorm:"column:captured_at"`
	FrpsConnections int32     `gorm:"column:frps_connections"`
	FrpsProxyCount  int32     `gorm:"column:frps_proxy_count"`
	FrpsClientCount int32     `gorm:"column:frps_client_count"`
	FrpsProxies     string    `gorm:"column:frps_proxies"`
	CreatedAt       time.Time `gorm:"column:created_at"`
}

// TableName returns the ClickHouse table name.
func (NodeObsFrps) TableName() string {
	return nodeObsFrpsTableName
}

// InsertColumns returns comma-separated column names for batch insert.
func (NodeObsFrps) InsertColumns() string {
	return nodeObsFrpsInsertColumns
}

// BatchInsertSQL returns the INSERT prefix used by native batch writers.
func (NodeObsFrps) BatchInsertSQL() string {
	return fmt.Sprintf("INSERT INTO %s (%s)", nodeObsFrpsTableName, nodeObsFrpsInsertColumns)
}

// NodeObsFrpc stores FRPC observability snapshots in ClickHouse.
type NodeObsFrpc struct {
	ID                   uint64    `gorm:"column:id"`
	NodeID               string    `gorm:"column:node_id"`
	CapturedAt           time.Time `gorm:"column:captured_at"`
	TunnelStatus         string    `gorm:"column:tunnel_status"`
	ConnectedRelaysCount int32     `gorm:"column:connected_relays_count"`
	CreatedAt            time.Time `gorm:"column:created_at"`
}

// TableName returns the ClickHouse table name.
func (NodeObsFrpc) TableName() string {
	return nodeObsFrpcTableName
}

// InsertColumns returns comma-separated column names for batch insert.
func (NodeObsFrpc) InsertColumns() string {
	return nodeObsFrpcInsertColumns
}

// BatchInsertSQL returns the INSERT prefix used by native batch writers.
func (NodeObsFrpc) BatchInsertSQL() string {
	return fmt.Sprintf("INSERT INTO %s (%s)", nodeObsFrpcTableName, nodeObsFrpcInsertColumns)
}
