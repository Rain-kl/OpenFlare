// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"time"
)

// Pages deployment status constants.
const (
	PagesDeploymentStatusUploaded = "uploaded"
	PagesDeploymentStatusActive   = "active"
)

// PagesProject OpenFlare Pages 静态托管项目。
type PagesProject struct {
	ID                   uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name                 string    `json:"name" gorm:"size:255;not null"`
	Slug                 string    `json:"slug" gorm:"uniqueIndex;size:128;not null"`
	Description          string    `json:"description" gorm:"type:text;not null;default:''"`
	Enabled              bool      `json:"enabled" gorm:"not null;default:true"`
	SPAFallbackEnabled   bool      `json:"spa_fallback_enabled" gorm:"not null;default:false"`
	SPAFallbackPath      string    `json:"spa_fallback_path" gorm:"size:512;not null;default:'/index.html'"`
	APIProxyEnabled      bool      `json:"api_proxy_enabled" gorm:"not null;default:false"`
	APIProxyPath         string    `json:"api_proxy_path" gorm:"size:255;not null;default:''"`
	APIProxyPass         string    `json:"api_proxy_pass" gorm:"size:2048;not null;default:''"`
	APIProxyRewrite      string    `json:"api_proxy_rewrite" gorm:"size:255;not null;default:''"`
	ActiveDeploymentID   *uint     `json:"active_deployment_id" gorm:"index"`
	RootDir              string    `json:"root_dir" gorm:"size:512;not null;default:''"`
	EntryFile            string    `json:"entry_file" gorm:"size:512;not null;default:'index.html'"`
	ContentConfigVersion int       `json:"-" gorm:"not null;default:0"`
	CreatedAt            time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名。
func (PagesProject) TableName() string {
	return "of_pages_projects"
}

// PagesDeployment OpenFlare Pages 不可变部署记录。
type PagesDeployment struct {
	ID               uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	ProjectID        uint       `json:"project_id" gorm:"not null;index;uniqueIndex:idx_of_pages_deployments_project_number,priority:1;uniqueIndex:idx_of_pages_deployments_source_revision,priority:1,where:source_identity IS NOT NULL AND source_revision IS NOT NULL"`
	DeploymentNumber int        `json:"deployment_number" gorm:"not null;uniqueIndex:idx_of_pages_deployments_project_number,priority:2"`
	Checksum         string     `json:"checksum" gorm:"size:64;not null;index"`
	Status           string     `json:"status" gorm:"size:32;not null;default:'uploaded';index"`
	UploadID         uint64     `json:"upload_id,string" gorm:"not null;default:0;index"`
	ArtifactPath     string     `json:"artifact_path,omitempty" gorm:"size:2048;not null;default:''"` // legacy only
	FileCount        int        `json:"file_count" gorm:"not null;default:0"`
	TotalSize        int64      `json:"total_size" gorm:"not null;default:0"`
	CreatedBy        string     `json:"created_by" gorm:"size:64;not null;default:''"`
	SourceType       string     `json:"source_type" gorm:"size:32;not null;default:''"`
	SourceIdentity   *string    `json:"-" gorm:"type:char(64);uniqueIndex:idx_of_pages_deployments_source_revision,priority:2,where:source_identity IS NOT NULL AND source_revision IS NOT NULL"`
	SourceRevision   *string    `json:"-" gorm:"type:char(64);uniqueIndex:idx_of_pages_deployments_source_revision,priority:3,where:source_identity IS NOT NULL AND source_revision IS NOT NULL"`
	SourceLabel      string     `json:"source_label" gorm:"size:255;not null;default:''"`
	SourceMeta       string     `json:"-" gorm:"type:text;not null;default:''"`
	TriggerType      string     `json:"trigger_type" gorm:"size:32;not null;default:''"`
	CreatedAt        time.Time  `json:"created_at" gorm:"autoCreateTime"`
	ActivatedAt      *time.Time `json:"activated_at"`
}

// TableName 表名。
func (PagesDeployment) TableName() string {
	return "of_pages_deployments"
}

// PagesDeploymentFile OpenFlare Pages 部署文件清单。
type PagesDeploymentFile struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	DeploymentID uint      `json:"deployment_id" gorm:"not null;index"`
	Path         string    `json:"path" gorm:"size:2048;not null"`
	Size         int64     `json:"size" gorm:"not null;default:0"`
	Checksum     string    `json:"checksum" gorm:"size:64;not null"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName 表名。
func (PagesDeploymentFile) TableName() string {
	return "of_pages_deployment_files"
}
