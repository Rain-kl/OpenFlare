// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"fmt"
	"time"
)

const (
	dnsMetricSnapshotTableName     = "of_dns_metric_snapshots"
	dnsMetricSnapshotInsertColumns = "id, instance_id, captured_at, queries_total, queries_success, queries_nxdomain, queries_servfail, queries_ratelimited, qps, latency_p50_ms, latency_p95_ms, latency_p99_ms, cache_hit_ratio, snapshot_version, cpu, ram_percent, created_at"
)

// DNSMetricSnapshot stores periodic SmartDNS instance throughput and latency percentiles in logstore.
type DNSMetricSnapshot struct {
	ID                 uint64    `gorm:"column:id"`
	InstanceID         string    `gorm:"column:instance_id"`
	CapturedAt         time.Time `gorm:"column:captured_at"`
	QueriesTotal       uint64    `gorm:"column:queries_total"`
	QueriesSuccess     uint64    `gorm:"column:queries_success"`
	QueriesNXDomain    uint64    `gorm:"column:queries_nxdomain"`
	QueriesServFail    uint64    `gorm:"column:queries_servfail"`
	QueriesRateLimited uint64    `gorm:"column:queries_ratelimited"`
	QPS                float64   `gorm:"column:qps"`
	LatencyP50Ms       float64   `gorm:"column:latency_p50_ms"`
	LatencyP95Ms       float64   `gorm:"column:latency_p95_ms"`
	LatencyP99Ms       float64   `gorm:"column:latency_p99_ms"`
	CacheHitRatio      float64   `gorm:"column:cache_hit_ratio"`
	SnapshotVersion    uint64    `gorm:"column:snapshot_version"`
	CPUUsagePercent    float64   `gorm:"column:cpu"`
	MemoryUsagePercent float64   `gorm:"column:ram_percent"`
	CreatedAt          time.Time `gorm:"column:created_at"`
}

// TableName returns the database table name.
func (DNSMetricSnapshot) TableName() string {
	return dnsMetricSnapshotTableName
}

// InsertColumns returns comma-separated column names for batch insert.
func (DNSMetricSnapshot) InsertColumns() string {
	return dnsMetricSnapshotInsertColumns
}

// BatchInsertSQL returns the INSERT prefix used by native batch writers.
func (DNSMetricSnapshot) BatchInsertSQL() string {
	return fmt.Sprintf("INSERT INTO %s (%s)", dnsMetricSnapshotTableName, dnsMetricSnapshotInsertColumns)
}
