// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package analytics defines ClickHouse analytics domain models and query DTOs
// (pure data, no IO).
package analytics

import "time"

// AccessLogFilter scopes user access log queries.
// 单一权威字段集（CH 原字段，Task 1 迁入）：禁止追加仅某实现使用的字段（避免双字段集分叉）。
type AccessLogFilter struct {
	// UserIDs filters by user IDs. nil means no user filter; an empty slice means no matches.
	UserIDs []uint64
	Path    string
	// StartTime filters created_at >= StartTime when non-nil.
	StartTime *time.Time
	// EndTime filters created_at <= EndTime when non-nil（闭区间，与 CH/GORM 实现一致）。
	EndTime *time.Time
}

// NodeAccessLogFilter scopes ClickHouse node access log queries.
type NodeAccessLogFilter struct {
	NodeID     string
	RemoteAddr string
	Host       string
	// Hosts exact-matches any host (case-insensitive). Prefer over Host for multi-domain scopes.
	Hosts []string
	Path  string
	// StatusCode filters by exact HTTP status code when > 0.
	StatusCode int
	Since      time.Time
	Until      time.Time
	Page       int
	PageSize   int
	SortBy     string
	SortOrder  string
}

// NodeObservabilityFilter scopes ClickHouse node observability queries.
type NodeObservabilityFilter struct {
	NodeID string
	Since  time.Time
	Limit  int
}

// DailyTrend is a single day's access count.
type DailyTrend struct {
	Date  string
	Count uint64
}

// BrowserShare is a browser group's share of access logs.
type BrowserShare struct {
	Browser string
	Count   uint64
}

// TopUser is an active user ranked by access count.
type TopUser struct {
	UserID uint64
	Count  uint64
}

// NodeAccessLogRegionCount aggregates access log regions.
type NodeAccessLogRegionCount struct {
	Region string
	Count  int64
}

// NodeAccessLogTrafficSummary is a window-level access log traffic summary.
type NodeAccessLogTrafficSummary struct {
	RequestCount  int64
	ErrorCount    int64
	UniqueIPCount int64
	BytesSent     int64
	RequestLength int64
	NodeCount     int64
}

// NodeAccessLogValueCount is a grouped value count (status_code, host, ...).
type NodeAccessLogValueCount struct {
	Value string
	Count int64
}

// NodeAccessLogNodeAggregate is per-node traffic over a window.
type NodeAccessLogNodeAggregate struct {
	NodeID        string
	RequestCount  int64
	ErrorCount    int64
	UniqueIPCount int64
}

// BatchWriterStats is a point-in-time snapshot of a batch writer queue and failure counters.
type BatchWriterStats struct {
	Name        string `json:"name"`
	Depth       int    `json:"depth"`
	Cap         int    `json:"cap"`
	Drops       int64  `json:"drops"`
	FlushErrors int64  `json:"flush_errors"`
	Running     bool   `json:"running"`
}

// ClickHouseOperationalStats summarizes ClickHouse merge/mutation pressure
// and in-process batch writer queue health.
type ClickHouseOperationalStats struct {
	Database         string `json:"database"`
	ActiveParts      int64  `json:"active_parts"`
	TotalRows        int64  `json:"total_rows"`
	PendingMutations int64  `json:"pending_mutations"`
	AsyncInsertQueue int64  `json:"async_insert_queue"`
	AsyncInsertBytes int64  `json:"async_insert_bytes"`
	// BatchWriters reports in-process queue depth/drops/flush errors for CH writers.
	BatchWriters []BatchWriterStats `json:"batch_writers,omitempty"`
}
