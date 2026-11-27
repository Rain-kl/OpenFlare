// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"fmt"
	"time"
)

const (
	nodeMetricSnapshotTableName     = "of_node_metric_snapshots"
	nodeMetricSnapshotInsertColumns = "id, node_id, captured_at, cpu, gpu, ram, ram_total, swap, swap_total, load, load5, load15, temp, disk, disk_total, disk_read_bytes, disk_write_bytes, net_in, net_out, net_total_up, net_total_down, process, connections, connections_udp, uptime, created_at"

	nodePingRecordTableName     = "of_node_ping_records"
	nodePingRecordInsertColumns = "id, node_id, task_id, recorded_at, value, created_at"
)

// NodeMetricSnapshot stores periodic Komari-aligned node resource metrics in logstore (PG/SQLite/ClickHouse).
type NodeMetricSnapshot struct {
	ID                uint64    `gorm:"column:id"`
	NodeID            string    `gorm:"column:node_id"`
	CapturedAt        time.Time `gorm:"column:captured_at"`
	CPUUsagePercent   float64   `gorm:"column:cpu"`
	GPUUsagePercent   float64   `gorm:"column:gpu"`
	MemoryUsedBytes   int64     `gorm:"column:ram"`
	MemoryTotalBytes  int64     `gorm:"column:ram_total"`
	SwapUsedBytes     int64     `gorm:"column:swap"`
	SwapTotalBytes    int64     `gorm:"column:swap_total"`
	Load1             float64   `gorm:"column:load"`
	Load5             float64   `gorm:"column:load5"`
	Load15            float64   `gorm:"column:load15"`
	Temp              float64   `gorm:"column:temp"`
	StorageUsedBytes  int64     `gorm:"column:disk"`
	StorageTotalBytes int64     `gorm:"column:disk_total"`
	DiskReadBytes     int64     `gorm:"column:disk_read_bytes"`
	DiskWriteBytes    int64     `gorm:"column:disk_write_bytes"`
	NetIn             int64     `gorm:"column:net_in"`
	NetOut            int64     `gorm:"column:net_out"`
	NetworkTxBytes    int64     `gorm:"column:net_total_up"`
	NetworkRxBytes    int64     `gorm:"column:net_total_down"`
	ProcessCount      int32     `gorm:"column:process"`
	TCPConnections    int32     `gorm:"column:connections"`
	UDPConnections    int32     `gorm:"column:connections_udp"`
	UptimeSeconds     int64     `gorm:"column:uptime"`
	CreatedAt         time.Time `gorm:"column:created_at"`
}

// TableName returns the table name.
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

// NodePingRecord stores latency monitoring measurements in logstore (PG/SQLite/ClickHouse).
type NodePingRecord struct {
	ID         uint64    `gorm:"column:id"`
	NodeID     string    `gorm:"column:node_id"`
	TaskID     uint64    `gorm:"column:task_id"`
	RecordedAt time.Time `gorm:"column:recorded_at"`
	Value      int32     `gorm:"column:value"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

// TableName returns the table name.
func (NodePingRecord) TableName() string {
	return nodePingRecordTableName
}

// InsertColumns returns comma-separated column names for batch insert.
func (NodePingRecord) InsertColumns() string {
	return nodePingRecordInsertColumns
}

// BatchInsertSQL returns the INSERT prefix used by native batch writers.
func (NodePingRecord) BatchInsertSQL() string {
	return fmt.Sprintf("INSERT INTO %s (%s)", nodePingRecordTableName, nodePingRecordInsertColumns)
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

// NodeTrafficHourly is an hourly traffic rollup row.
type NodeTrafficHourly struct {
	NodeID             string
	Hour               time.Time
	RequestCount       int64
	ErrorCount         int64
	UniqueVisitorCount int64
}

// NodeMetricHourly is an hourly metric snapshot aggregation row.
type NodeMetricHourly struct {
	Hour                      time.Time
	AverageCPUUsagePercent    float64
	AverageMemoryUsagePercent float64
	NetworkRxBytes            int64
	NetworkTxBytes            int64
	DiskReadBytes             int64
	DiskWriteBytes            int64
	ReportedNodes             int
}
