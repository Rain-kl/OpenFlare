// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package analytics defines ClickHouse analytics domain models.
package analytics

import (
	"fmt"
	"time"
)

const (
	userAccessLogTableName     = "w_user_access_logs"
	userAccessLogInsertColumns = "id, user_id, path, method, ip, user_agent, headers, status, latency, created_at"
)

// UserAccessLog stores HTTP access records in ClickHouse.
type UserAccessLog struct {
	ID        uint64    `gorm:"column:id"`
	UserID    uint64    `gorm:"column:user_id"`
	Path      string    `gorm:"column:path"`
	Method    string    `gorm:"column:method"`
	IP        string    `gorm:"column:ip"`
	UserAgent string    `gorm:"column:user_agent"`
	Headers   string    `gorm:"column:headers"`
	Status    int32     `gorm:"column:status"`
	Latency   int64     `gorm:"column:latency"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

// TableName returns the ClickHouse table name.
func (UserAccessLog) TableName() string {
	return userAccessLogTableName
}

// InsertColumns returns comma-separated column names for batch insert.
func (UserAccessLog) InsertColumns() string {
	return userAccessLogInsertColumns
}

// BatchInsertSQL returns the INSERT prefix used by native batch writers.
func (UserAccessLog) BatchInsertSQL() string {
	return fmt.Sprintf("INSERT INTO %s (%s)", userAccessLogTableName, userAccessLogInsertColumns)
}
