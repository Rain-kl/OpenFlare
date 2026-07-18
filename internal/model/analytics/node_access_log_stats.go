// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

// NodeAccessLogBucketAggregate is a folded bucket aggregate row.
type NodeAccessLogBucketAggregate struct {
	BucketEpoch      int64 `gorm:"column:bucket_epoch"`
	RequestCount     int64 `gorm:"column:request_count"`
	SuccessCount     int64 `gorm:"column:success_count"`
	ClientErrorCount int64 `gorm:"column:client_error_count"`
	ServerErrorCount int64 `gorm:"column:server_error_count"`
	UniqueIPCount    int64 `gorm:"column:unique_ip_count"`
	UniqueHostCount  int64 `gorm:"column:unique_host_count"`
	BytesSent        int64 `gorm:"column:bytes_sent"`
	RequestLength    int64 `gorm:"column:request_length"`
}

// NodeAccessLogWAFIPAggregate is a per-IP aggregate row for WAF automatic rules.
type NodeAccessLogWAFIPAggregate struct {
	RemoteAddr       string
	RequestCount     int64
	Status404Count   int64
	ClientErrorCount int64
	ServerErrorCount int64
	IPHostCount      int64
	LastSeenEpoch    int64
	StatusCounts     map[int]int64
}

// NodeAccessLogBucketDimension is a bucket dimension value.
type NodeAccessLogBucketDimension struct {
	BucketEpoch int64  `gorm:"column:bucket_epoch"`
	Value       string `gorm:"column:value"`
}

// NodeAccessLogIPAggregate is an IP aggregate row.
type NodeAccessLogIPAggregate struct {
	RemoteAddr       string `gorm:"column:remote_addr"`
	RequestCount     int64  `gorm:"column:request_count"`
	SuccessCount     int64  `gorm:"column:success_count"`
	ClientErrorCount int64  `gorm:"column:client_error_count"`
	ServerErrorCount int64  `gorm:"column:server_error_count"`
	LastSeenEpoch    int64  `gorm:"column:last_seen_epoch"`
}

// NodeAccessLogIPSummary is an IP summary row.
type NodeAccessLogIPSummary struct {
	RemoteAddr      string  `gorm:"column:remote_addr"`
	Region          string  `gorm:"column:region"`
	TotalRequests   int64   `gorm:"column:total_requests"`
	Success2xxCount int64   `gorm:"column:success_2xx_count"`
	SuccessRatio    float64 `gorm:"column:success_ratio"`
	BytesReceived   int64   `gorm:"column:request_length"`
	BytesSent       int64   `gorm:"column:bytes_sent"`
	// RecentRequests is deprecated (always 0); kept for wire compatibility.
	RecentRequests int64 `gorm:"column:recent_requests"`
	LastSeenEpoch  int64 `gorm:"column:last_seen_epoch"`
}

// NodeAccessLogIPTrend is an IP trend bucket row.
type NodeAccessLogIPTrend struct {
	BucketEpoch  int64 `gorm:"column:bucket_epoch"`
	RequestCount int64 `gorm:"column:request_count"`
}
