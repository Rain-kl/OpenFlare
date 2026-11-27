// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"fmt"
	"time"
)

const (
	networkQualitySampleTableName     = "of_network_quality_samples"
	networkQualitySampleInsertColumns = "id, node_id, node_asn, client_asn, client_country, client_region, ip_family, stats_version, source, sample_count, rtt_min_ms, rtt_p50_ms, rtt_p95_ms, rtt_hist, prefix_sketch, distinct_prefix_count, client_lat, client_lon, geo_accuracy_km, geo_source, retrans_rate, loss_rate, hop_count, captured_at, created_at"
)

// NetworkQualitySample stores 1-minute aggregated passive TCP or active traceroute network quality samples in logstore.
type NetworkQualitySample struct {
	ID                  uint64    `gorm:"column:id"`
	NodeID              string    `gorm:"column:node_id"`
	NodeASN             uint32    `gorm:"column:node_asn"`
	ClientASN           uint32    `gorm:"column:client_asn"`
	ClientCountry       string    `gorm:"column:client_country"`
	ClientRegion        string    `gorm:"column:client_region;default:''"`
	IPFamily            string    `gorm:"column:ip_family;default:'unknown'"`
	StatsVersion        int32     `gorm:"column:stats_version;default:1"`
	Source              string    `gorm:"column:source"`
	SampleCount         int32     `gorm:"column:sample_count"`
	RTTMinMs            float64   `gorm:"column:rtt_min_ms"`
	RTTP50Ms            float64   `gorm:"column:rtt_p50_ms"`
	RTTP95Ms            float64   `gorm:"column:rtt_p95_ms"`
	RTTHist             string    `gorm:"column:rtt_hist;default:''"`
	PrefixSketch        string    `gorm:"column:prefix_sketch;default:''"`
	DistinctPrefixCount int32     `gorm:"column:distinct_prefix_count;default:0"`
	ClientLat           *float64  `gorm:"column:client_lat"`
	ClientLon           *float64  `gorm:"column:client_lon"`
	GeoAccuracyKm       float64   `gorm:"column:geo_accuracy_km;default:0"`
	GeoSource           string    `gorm:"column:geo_source;default:''"`
	RetransRate         float64   `gorm:"column:retrans_rate"`
	LossRate            float64   `gorm:"column:loss_rate"`
	HopCount            int32     `gorm:"column:hop_count"`
	CapturedAt          time.Time `gorm:"column:captured_at"`
	CreatedAt           time.Time `gorm:"column:created_at"`
}

// TableName returns the database table name.
func (NetworkQualitySample) TableName() string {
	return networkQualitySampleTableName
}

// InsertColumns returns comma-separated column names for batch insert.
func (NetworkQualitySample) InsertColumns() string {
	return networkQualitySampleInsertColumns
}

// BatchInsertSQL returns the INSERT prefix used by native batch writers.
func (NetworkQualitySample) BatchInsertSQL() string {
	return fmt.Sprintf("INSERT INTO %s (%s)", networkQualitySampleTableName, networkQualitySampleInsertColumns)
}
