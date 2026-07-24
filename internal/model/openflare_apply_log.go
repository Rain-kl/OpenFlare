// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"strings"
	"time"
)

// OpenFlareApplyLogQuery filters apply logs for list queries.
type OpenFlareApplyLogQuery struct {
	NodeID   string
	PageNo   int
	PageSize int
}

// OpenFlareApplyLog stores node configuration apply results.
type OpenFlareApplyLog struct {
	ID                  uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeID              string    `json:"node_id" gorm:"index;size:64;not null"`
	Version             string    `json:"version" gorm:"size:32;not null"`
	Result              string    `json:"result" gorm:"size:32;not null"`
	Message             string    `json:"message" gorm:"type:text"`
	Checksum            string    `json:"checksum" gorm:"size:64;not null;default:''"`
	MainConfigChecksum  string    `json:"main_config_checksum" gorm:"size:64;not null;default:''"`
	RouteConfigChecksum string    `json:"route_config_checksum" gorm:"size:64;not null;default:''"`
	SupportFileCount    int       `json:"support_file_count" gorm:"not null;default:0"`
	CreatedAt           time.Time `json:"created_at" gorm:"autoCreateTime;index"`
}

// TableName returns the GORM table name.
func (OpenFlareApplyLog) TableName() string {
	return "of_apply_logs"
}

// IsRepeatSuccessApplyLog reports whether the payload repeats an already-recorded success entry.
func IsRepeatSuccessApplyLog(latest *OpenFlareApplyLog, version, checksum, result string) bool {
	if latest == nil || result != "success" {
		return false
	}
	return latest.Result == "success" &&
		strings.TrimSpace(latest.Version) == strings.TrimSpace(version) &&
		strings.TrimSpace(latest.Checksum) == strings.TrimSpace(checksum)
}
