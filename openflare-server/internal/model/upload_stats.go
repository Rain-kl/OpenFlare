// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import "time"

// Upload stats dimension keys stored in w_upload_stats.dimension.
const (
	UploadStatDimensionTotal    = "total"
	UploadStatDimensionType     = "type"
	UploadStatDimensionCategory = "category"
	UploadStatDimensionTrend    = "trend"
)

// UploadStat stores incremental upload statistics keyed by dimension and stat_key.
type UploadStat struct {
	Dimension string    `json:"dimension" gorm:"primaryKey;size:32;not null"`
	StatKey   string    `json:"stat_key" gorm:"primaryKey;size:64;not null;default:''"`
	FileCount int64     `json:"file_count" gorm:"not null;default:0"`
	FileSize  int64     `json:"file_size" gorm:"not null;default:0"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the upload stats table name.
func (UploadStat) TableName() string {
	return "w_upload_stats"
}
