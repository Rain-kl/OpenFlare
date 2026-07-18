// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"fmt"
	"time"
)

const (
	nodeAccessLogTableName     = "of_node_access_logs"
	nodeAccessLogInsertColumns = "id, node_id, logged_at, remote_addr, region, host, path, user_agent, status_code, bytes_sent, request_length, request_time_ms, created_at"
)

// NodeAccessLog stores OpenFlare edge node access records in ClickHouse.
type NodeAccessLog struct {
	ID            uint64    `gorm:"column:id"`
	NodeID        string    `gorm:"column:node_id"`
	LoggedAt      time.Time `gorm:"column:logged_at"`
	RemoteAddr    string    `gorm:"column:remote_addr"`
	Region        string    `gorm:"column:region"`
	Host          string    `gorm:"column:host"`
	Path          string    `gorm:"column:path"`
	UserAgent     string    `gorm:"column:user_agent"`
	StatusCode    int32     `gorm:"column:status_code"`
	BytesSent     uint64    `gorm:"column:bytes_sent"`
	RequestLength uint64    `gorm:"column:request_length"`
	RequestTimeMs uint32    `gorm:"column:request_time_ms"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

// TableName returns the ClickHouse table name.
func (NodeAccessLog) TableName() string {
	return nodeAccessLogTableName
}

// InsertColumns returns comma-separated column names for batch insert.
func (NodeAccessLog) InsertColumns() string {
	return nodeAccessLogInsertColumns
}

// BatchInsertSQL returns the INSERT prefix used by native batch writers.
func (NodeAccessLog) BatchInsertSQL() string {
	return fmt.Sprintf("INSERT INTO %s (%s)", nodeAccessLogTableName, nodeAccessLogInsertColumns)
}
