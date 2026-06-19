// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"time"
)

// UploadStatus 上传状态
type UploadStatus string

// 上传状态
const (
	UploadStatusPending UploadStatus = "pending" // 待使用
	UploadStatusUsed    UploadStatus = "used"    // 已使用
	UploadStatusDeleted UploadStatus = "deleted" // 已删除
)

// UploadMetadata 自定义可扩展的 JSON 字段存储非核心或可选的文件元数据
type UploadMetadata struct {
	Width        int            `json:"width,omitempty"`         // 图像/视频宽度 (px)
	Height       int            `json:"height,omitempty"`        // 图像/视频高度 (px)
	Duration     float64        `json:"duration,omitempty"`      // 音视频时长 (s)
	OriginalMime string         `json:"original_mime,omitempty"` // 原始 MIME 类型
	UserAgent    string         `json:"user_agent,omitempty"`    // 上传者的 UA
	ClientIP     string         `json:"client_ip,omitempty"`     // 上传者 IP
	Bucket       string         `json:"bucket,omitempty"`        // 存储桶名称 (适用于 S3 等)
	Extra        map[string]any `json:"extra,omitempty"`         // 其它任意业务自定义元数据
}

// Upload 上传文件记录
type Upload struct {
	ID         uint64         `json:"id,string" gorm:"primaryKey"`
	UserID     uint64         `json:"user_id,string" gorm:"index;not null"`
	FileName   string         `json:"file_name" gorm:"size:255;not null"`             // 原始文件名 (例如: image.png)
	FilePath   string         `json:"file_path" gorm:"size:500;not null;index"`       // 文件相对路径 / S3 Key
	FileSize   int64          `json:"file_size" gorm:"not null"`                      // 文件大小（字节）
	MimeType   string         `json:"mime_type" gorm:"size:100;not null"`             // 媒体类型 (MIME, 如 image/png)
	Extension  string         `json:"extension" gorm:"size:50;not null"`              // 文件后缀名 (不含点，如 png, pdf)
	Hash       string         `json:"hash" gorm:"size:64;index"`                      // 文件哈希 (SHA-256/MD5，可用于排重)
	Type       string         `json:"type" gorm:"column:type;size:50;not null;index"` // 业务标识类型 (如 avatar, doc, attachment)
	Status     UploadStatus   `json:"status" gorm:"type:varchar(20);not null"`        // 状态
	AccessMode int            `json:"access_mode" gorm:"column:access_mode;not null;default:0"`
	Metadata   UploadMetadata `json:"metadata" gorm:"serializer:json;type:jsonb"` // 业务扩展元数据
	CreatedAt  time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名
func (Upload) TableName() string {
	return "w_uploads"
}
