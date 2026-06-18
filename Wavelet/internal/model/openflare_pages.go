// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
)

const (
	PagesDeploymentStatusUploaded = "uploaded"
	PagesDeploymentStatusActive   = "active"
)

// PagesProject OpenFlare Pages 静态托管项目。
type PagesProject struct {
	ID                 uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name               string    `json:"name" gorm:"size:255;not null"`
	Slug               string    `json:"slug" gorm:"uniqueIndex;size:128;not null"`
	Description        string    `json:"description" gorm:"type:text;not null;default:''"`
	Enabled            bool      `json:"enabled" gorm:"not null;default:true"`
	SPAFallbackEnabled bool      `json:"spa_fallback_enabled" gorm:"not null;default:false"`
	SPAFallbackPath    string    `json:"spa_fallback_path" gorm:"size:512;not null;default:'/index.html'"`
	APIProxyEnabled    bool      `json:"api_proxy_enabled" gorm:"not null;default:false"`
	APIProxyPath       string    `json:"api_proxy_path" gorm:"size:255;not null;default:''"`
	APIProxyPass       string    `json:"api_proxy_pass" gorm:"size:2048;not null;default:''"`
	APIProxyRewrite    string    `json:"api_proxy_rewrite" gorm:"size:255;not null;default:''"`
	ActiveDeploymentID *uint     `json:"active_deployment_id" gorm:"index"`
	RootDir            string    `json:"root_dir" gorm:"size:512;not null;default:''"`
	EntryFile          string    `json:"entry_file" gorm:"size:512;not null;default:'index.html'"`
	CreatedAt          time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名。
func (PagesProject) TableName() string {
	return "of_pages_projects"
}

// PagesDeployment OpenFlare Pages 不可变部署记录。
type PagesDeployment struct {
	ID               uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	ProjectID        uint       `json:"project_id" gorm:"not null;index"`
	DeploymentNumber int        `json:"deployment_number" gorm:"not null"`
	Checksum         string     `json:"checksum" gorm:"size:64;not null;index"`
	Status           string     `json:"status" gorm:"size:32;not null;default:'uploaded';index"`
	UploadID         uint64     `json:"upload_id,string" gorm:"not null;default:0;index"`
	ArtifactPath     string     `json:"artifact_path,omitempty" gorm:"size:2048;not null;default:''"` // legacy only
	FileCount        int        `json:"file_count" gorm:"not null;default:0"`
	TotalSize        int64      `json:"total_size" gorm:"not null;default:0"`
	CreatedBy        string     `json:"created_by" gorm:"size:64;not null;default:''"`
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

// HasPagesProjectsTable 判断 Pages 项目表是否已迁移。
func HasPagesProjectsTable(ctx context.Context) bool {
	return db.DB(ctx).Migrator().HasTable(&PagesProject{})
}

// ListPagesProjects 列出全部 Pages 项目。
func ListPagesProjects(ctx context.Context) ([]PagesProject, error) {
	var projects []PagesProject
	if err := db.DB(ctx).Order("id desc").Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}

// GetPagesProjectByID 按 ID 查询 Pages 项目。
func GetPagesProjectByID(ctx context.Context, id uint) (*PagesProject, error) {
	var project PagesProject
	if err := db.DB(ctx).First(&project, id).Error; err != nil {
		return nil, err
	}
	return &project, nil
}

// GetPagesProjectBySlug 按 slug 查询 Pages 项目。
func GetPagesProjectBySlug(ctx context.Context, slug string) (*PagesProject, error) {
	var project PagesProject
	if err := db.DB(ctx).Where("slug = ?", slug).First(&project).Error; err != nil {
		return nil, err
	}
	return &project, nil
}

// CreatePagesProjectRecord 创建 Pages 项目。
func CreatePagesProjectRecord(ctx context.Context, project *PagesProject) error {
	return db.DB(ctx).Create(project).Error
}

// ListPagesDeployments 列出项目的全部部署。
func ListPagesDeployments(ctx context.Context, projectID uint) ([]PagesDeployment, error) {
	var deployments []PagesDeployment
	if err := db.DB(ctx).Where("project_id = ?", projectID).Order("id desc").Find(&deployments).Error; err != nil {
		return nil, err
	}
	return deployments, nil
}

// GetPagesDeploymentByID 按 ID 查询 Pages 部署。
func GetPagesDeploymentByID(ctx context.Context, id uint) (*PagesDeployment, error) {
	var deployment PagesDeployment
	if err := db.DB(ctx).First(&deployment, id).Error; err != nil {
		return nil, err
	}
	return &deployment, nil
}

// ListPagesDeploymentFiles 列出部署文件清单。
func ListPagesDeploymentFiles(ctx context.Context, deploymentID uint) ([]PagesDeploymentFile, error) {
	var files []PagesDeploymentFile
	if err := db.DB(ctx).Where("deployment_id = ?", deploymentID).Order("path asc").Find(&files).Error; err != nil {
		return nil, err
	}
	return files, nil
}

// CountPagesDeploymentsByProjectID 统计项目部署数量。
func CountPagesDeploymentsByProjectID(ctx context.Context, projectID uint) (int64, error) {
	var count int64
	if err := db.DB(ctx).Model(&PagesDeployment{}).Where("project_id = ?", projectID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountProxyRoutesByPagesProjectID 统计引用 Pages 项目的代理规则数量。
func CountProxyRoutesByPagesProjectID(ctx context.Context, projectID uint) (int64, error) {
	if !HasProxyRoutesTable(ctx) {
		return 0, nil
	}
	var count int64
	if err := db.DB(ctx).Model(&ProxyRoute{}).Where("pages_project_id = ?", projectID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
