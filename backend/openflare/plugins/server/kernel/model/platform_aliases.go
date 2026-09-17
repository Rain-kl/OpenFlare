// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"time"

	"Wavelet/core/contracts"
)

const (
	tokenByteLength = 24
	maskThreshold   = 8
)

// User represents a user identity view.
type User struct {
	ID          uint64    `json:"id,string" gorm:"primaryKey"`
	Username    string    `json:"username"`
	Password    string    `json:"-"`
	Nickname    string    `json:"nickname"`
	Email       string    `json:"email"`
	IsAdmin     bool      `json:"is_admin"`
	IsActive    bool      `json:"is_active"`
	LastLoginAt time.Time `json:"last_login_at"`
}

func (User) TableName() string {
	return "w_users"
}

func (u *User) SetEncryptedPassword(pwd string) error {
	u.Password = pwd
	return nil
}

// AccessToken represents an access token view.
type AccessToken struct {
	ID          uint64    `json:"id" gorm:"primaryKey"`
	UserID      uint64    `json:"user_id"`
	Name        string    `json:"name"`
	Token       string    `json:"token"`
	MaskedToken string    `json:"masked_token"`
	TokenHash   string    `json:"token_hash"`
	IsAdmin     bool      `json:"is_admin"`
	ExpiredAt   time.Time `json:"expired_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (AccessToken) TableName() string {
	return "w_access_tokens"
}

// AuthSource represents an authentication source view.
type AuthSource struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	DisplayName string `json:"display_name"`
	IconURL     string `json:"icon_url"`
	IsActive    bool   `json:"is_active"`
}

// TaskExecution represents task execution entity.
type TaskExecution struct {
	ID           uint64     `json:"id" gorm:"primaryKey"`
	TaskID       string     `json:"task_id" gorm:"size:64;index"`
	TaskType     string     `json:"task_type" gorm:"size:100;index"`
	TaskName     string     `json:"task_name" gorm:"size:255"`
	Status       string     `json:"status" gorm:"size:20;index"`
	Retryable    bool       `json:"retryable"`
	MaxRetry     int        `json:"max_retry"`
	RetryCount   int        `json:"retry_count"`
	Log          string     `json:"log" gorm:"type:text"`
	ErrorMessage string     `json:"error_message" gorm:"type:text"`
	Result       string     `json:"result" gorm:"type:text"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	Duration     int64      `json:"duration"`
	Payload      string     `json:"payload" gorm:"type:text"`
	TriggeredBy  string     `json:"triggered_by" gorm:"size:100"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (TaskExecution) TableName() string {
	return "w_task_executions"
}

type UploadStatus = string

const (
	UploadStatusPending UploadStatus = "pending"
	UploadStatusUsed    UploadStatus = "used"
	UploadStatusDeleted UploadStatus = "deleted"
)

// UploadMetadata represents upload metadata JSON.
type UploadMetadata = contracts.UploadMetadataDTO

// Upload represents file upload entity.
type Upload struct {
	ID        uint64                      `json:"id" gorm:"primaryKey"`
	UserID    uint64                      `json:"user_id" gorm:"index"`
	FileName  string                      `json:"file_name" gorm:"size:255"`
	FilePath  string                      `json:"file_path" gorm:"size:500"`
	MimeType  string                      `json:"mime_type" gorm:"size:100"`
	Size      int64                       `json:"size"`
	Hash      string                      `json:"hash" gorm:"size:64"`
	Status    string                      `json:"status" gorm:"type:varchar(20)"`
	Type      string                      `json:"type" gorm:"size:50;index"`
	Metadata  contracts.UploadMetadataDTO `json:"metadata"`
	CreatedAt time.Time                   `json:"created_at"`
	UpdatedAt time.Time                   `json:"updated_at"`
}

func (Upload) TableName() string {
	return "w_uploads"
}

func (u *Upload) ToDTO() contracts.UploadDTO {
	return contracts.UploadDTO{
		ID:        u.ID,
		UserID:    u.UserID,
		FileName:  u.FileName,
		FilePath:  u.FilePath,
		MimeType:  u.MimeType,
		Size:      u.Size,
		Hash:      u.Hash,
		Status:    u.Status,
		Type:      u.Type,
		Metadata:  u.Metadata,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func FromUploadDTO(d contracts.UploadDTO) Upload {
	return Upload{
		ID:        d.ID,
		UserID:    d.UserID,
		FileName:  d.FileName,
		FilePath:  d.FilePath,
		MimeType:  d.MimeType,
		Size:      d.Size,
		Hash:      d.Hash,
		Status:    d.Status,
		Type:      d.Type,
		Metadata:  d.Metadata,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

const UploadStatDimensionTotal = "total"

// UploadStat tracks upload statistics by dimension.
type UploadStat struct {
	ID        uint64 `gorm:"primaryKey"`
	Dimension string `gorm:"size:50;not null"`
	TargetID  uint64 `gorm:"not null"`
	TotalSize int64  `gorm:"not null"`
	FileCount int    `gorm:"not null"`
}

func (UploadStat) TableName() string {
	return "w_upload_stats"
}

// GenerateTokenString 生成加密安全的随机 Token 值
func GenerateTokenString() (string, error) {
	bytes := make([]byte, tokenByteLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "at_" + hex.EncodeToString(bytes), nil
}

// HashToken 计算 Token 的 SHA-256 哈希值用于数据库存储与查询
func HashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

// MaskTokenString 生成脱敏显示的 Token，仅保留前缀和最后四位
func MaskTokenString(token string) string {
	if len(token) <= maskThreshold {
		return "at_****"
	}
	return fmt.Sprintf("%s...%s", token[:7], token[len(token)-4:])
}
