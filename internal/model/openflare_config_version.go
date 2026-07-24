// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"time"

	"gorm.io/gorm"
)

// ConfigVersionSummary is the list view for config versions.
type ConfigVersionSummary struct {
	ID        string    `json:"id" gorm:"-"`
	Version   string    `json:"version" gorm:"primaryKey;column:version"`
	Checksum  string    `json:"checksum"`
	IsActive  bool      `json:"is_active"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// AfterFind hook for ConfigVersionSummary.
func (cvs *ConfigVersionSummary) AfterFind(_ *gorm.DB) (err error) {
	cvs.ID = cvs.Version
	return
}

// ConfigVersion stores a published OpenResty configuration snapshot.
type ConfigVersion struct {
	ID               string    `json:"id" gorm:"-"`
	Version          string    `json:"version" gorm:"primaryKey;size:32;not null"`
	SnapshotJSON     string    `json:"snapshot_json" gorm:"type:text;not null"`
	MainConfig       string    `json:"main_config" gorm:"type:text;not null;default:''"`
	RenderedConfig   string    `json:"rendered_config" gorm:"type:text;not null"`
	SupportFilesJSON string    `json:"support_files_json" gorm:"type:text;not null;default:'[]'"`
	Checksum         string    `json:"checksum" gorm:"size:64;not null"`
	IsActive         bool      `json:"is_active" gorm:"not null;default:false;index"`
	CreatedBy        string    `json:"created_by" gorm:"size:64;not null"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// AfterFind hook for ConfigVersion.
func (cv *ConfigVersion) AfterFind(_ *gorm.DB) (err error) {
	cv.ID = cv.Version
	return
}

// AfterCreate hook for ConfigVersion.
func (cv *ConfigVersion) AfterCreate(_ *gorm.DB) (err error) {
	cv.ID = cv.Version
	return
}

// TableName returns the GORM table name.
func (ConfigVersion) TableName() string {
	return "of_config_versions"
}
