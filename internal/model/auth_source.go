// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

// 认证源类型
const (
	AuthSourceTypeOIDC = "oidc"
)

// Shared GORM column name constants used across model package.
const (
	colName            = "name"
	colEnabled         = "enabled"
	colRemark          = "remark"
	tableOfProxyRoutes = "of_proxy_routes"
)

var authSourceNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,79}$`)

// AuthSource 认证源实体
type AuthSource struct {
	ID                     uint64    `json:"id" gorm:"primaryKey"`
	Name                   string    `json:"name" gorm:"uniqueIndex;size:80;not null"`
	Type                   string    `json:"type" gorm:"size:20;not null"`
	DisplayName            string    `json:"display_name" gorm:"size:100"`
	IsActive               bool      `json:"is_active" gorm:"index;not null;default:false"`
	ClientID               string    `json:"client_id" gorm:"size:255"`
	ClientSecret           string    `json:"-" gorm:"size:1024"`
	OpenIDDiscoveryURL     string    `json:"openid_discovery_url" gorm:"column:openid_discovery_url;size:1024"`
	Scopes                 string    `json:"scopes" gorm:"size:255"`
	IconURL                string    `json:"icon_url" gorm:"size:1024"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
	ClientSecretConfigured bool      `json:"client_secret_configured" gorm:"-"`
}

// TableName 表名
func (AuthSource) TableName() string {
	return "w_auth_sources"
}

// ExternalAccount 外部账号绑定实体
type ExternalAccount struct {
	ID               uint64    `json:"id" gorm:"primaryKey"`
	AuthSourceID     uint64    `json:"auth_source_id" gorm:"uniqueIndex:idx_external_accounts_source_external,priority:1;index"`
	UserID           uint64    `json:"user_id" gorm:"index;not null"`
	ExternalID       string    `json:"external_id" gorm:"uniqueIndex:idx_external_accounts_source_external,priority:2;size:255;not null"`
	ExternalUsername string    `json:"external_username" gorm:"size:255"`
	Email            string    `json:"email" gorm:"size:255"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// TableName 表名
func (ExternalAccount) TableName() string {
	return "w_external_accounts"
}

// ExternalAccountView 外部帐号绑定视图（脱敏展示用）
type ExternalAccountView struct {
	ID               uint64    `json:"id"`
	AuthSourceID     uint64    `json:"auth_source_id"`
	AuthSourceName   string    `json:"auth_source_name"`
	AuthSourceType   string    `json:"auth_source_type"`
	AuthSourceLabel  string    `json:"auth_source_label"`
	ExternalUsername string    `json:"external_username"`
	Email            string    `json:"email"`
	CreatedAt        time.Time `json:"created_at"`
}

// Normalize 对认证源字段进行标准化处理
func (source *AuthSource) Normalize() {
	source.Type = strings.ToLower(strings.TrimSpace(source.Type))
	source.Name = strings.TrimSpace(source.Name)
	source.DisplayName = strings.TrimSpace(source.DisplayName)
	source.ClientID = strings.TrimSpace(source.ClientID)
	source.ClientSecret = strings.TrimSpace(source.ClientSecret)
	source.OpenIDDiscoveryURL = strings.TrimSpace(source.OpenIDDiscoveryURL)
	source.Scopes = strings.TrimSpace(source.Scopes)
	source.IconURL = strings.TrimSpace(source.IconURL)
	if source.DisplayName == "" {
		source.DisplayName = source.Name
	}
	if source.Type == AuthSourceTypeOIDC && source.Scopes == "" {
		source.Scopes = "openid profile email"
	}
}

// Validate 校验认证源字段合法性
func (source *AuthSource) Validate() error {
	source.Normalize()
	if source.Name == "" {
		return errors.New(errAuthSourceNameRequired)
	}
	if !authSourceNamePattern.MatchString(source.Name) {
		return errors.New(errAuthSourceNameInvalid)
	}
	if source.Type != AuthSourceTypeOIDC {
		return errors.New(errAuthSourceTypeUnsupported)
	}
	if source.OpenIDDiscoveryURL == "" {
		return errors.New(errAuthSourceDiscoveryURLRequired)
	}
	if source.IsActive && (source.ClientID == "" || source.ClientSecret == "") {
		return errors.New(errAuthSourceClientCredentialsRequired)
	}
	return nil
}

// Sanitize 脱敏处理，将 ClientSecret 清空并设置 ClientSecretConfigured 标志
func (source *AuthSource) Sanitize() {
	source.ClientSecretConfigured = source.ClientSecret != ""
	source.ClientSecret = ""
}
