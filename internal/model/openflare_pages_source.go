// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import "time"

// PagesProjectSource 保存 Pages 项目的持久部署源配置。
//
// RemoteURL 可能包含签名参数，禁止直接序列化 model；对外接口必须映射到
// pages 包内的脱敏 source view。
type PagesProjectSource struct {
	ID                   uint      `json:"-" gorm:"primaryKey;autoIncrement"`
	ProjectID            uint      `json:"-" gorm:"not null;uniqueIndex:idx_of_pages_project_sources_project_id"`
	SourceType           string    `json:"-" gorm:"size:32;not null;default:''"`
	RemoteURL            string    `json:"-" gorm:"type:text;not null;default:''"`
	RemoteNetworkPolicy  string    `json:"-" gorm:"size:32;not null;default:''"`
	GitHubRepository     string    `json:"-" gorm:"column:github_repository;size:255;not null;default:''"`
	ReleaseSelector      string    `json:"-" gorm:"size:16;not null;default:''"`
	ReleaseTag           string    `json:"-" gorm:"size:255;not null;default:''"`
	AssetName            string    `json:"-" gorm:"size:255;not null;default:''"`
	AutoUpdateEnabled    bool      `json:"-" gorm:"not null;default:false"`
	CheckIntervalMinutes int       `json:"-" gorm:"not null;default:0"`
	ConfigVersion        int       `json:"-" gorm:"not null;default:0"`
	SourceIdentity       string    `json:"-" gorm:"type:char(64);not null;default:''"`
	CreatedAt            time.Time `json:"-" gorm:"autoCreateTime"`
	UpdatedAt            time.Time `json:"-" gorm:"autoUpdateTime"`
}

// TableName 返回 Pages 项目部署源配置表名。
func (PagesProjectSource) TableName() string {
	return "of_pages_project_sources"
}

// PagesProjectSourceRuntime 保存 Pages 项目部署源的可变运行态。
//
// Runtime 不冗余 project_id；调用方通过 SourceID 关联配置，并在最终提交时
// 同时校验 source config version 与 project content config version。
type PagesProjectSourceRuntime struct {
	SourceID            uint       `json:"-" gorm:"primaryKey;autoIncrement:false"`
	ETag                string     `json:"-" gorm:"column:etag;size:512;not null;default:''"`
	LastSeenRevision    string     `json:"-" gorm:"type:char(64);not null;default:''"`
	LastSeenDetail      string     `json:"-" gorm:"type:text;not null;default:''"`
	LastAppliedRevision string     `json:"-" gorm:"type:char(64);not null;default:''"`
	LastAppliedDetail   string     `json:"-" gorm:"type:text;not null;default:''"`
	SyncStatus          string     `json:"-" gorm:"size:32;not null;default:''"`
	LastError           string     `json:"-" gorm:"type:text;not null;default:''"`
	LastCheckedAt       *time.Time `json:"-"`
	LastSyncedAt        *time.Time `json:"-"`
	NextCheckAt         *time.Time `json:"-" gorm:"index:idx_of_pages_project_source_runtime_next_check_at"`
	LeaseExpiresAt      *time.Time `json:"-"`
	LeaseToken          string     `json:"-" gorm:"size:64;not null;default:''"`
	UpdatedAt           time.Time  `json:"-" gorm:"autoUpdateTime"`
}

// TableName 返回 Pages 项目部署源运行态表名。
func (PagesProjectSourceRuntime) TableName() string {
	return "of_pages_project_source_runtime"
}
