// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package entity provides database model entities for the auth domain plugin.
package entity

import (
	"Wavelet/plugins/domain/auth/consts"
	"errors"
	"regexp"
	"strings"
	"time"
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
	if source.Type == consts.AuthSourceTypeOIDC && source.Scopes == "" {
		source.Scopes = "openid profile email"
	}
}

// Validate 校验认证源字段合法性
func (source *AuthSource) Validate() error {
	source.Normalize()
	if source.Name == "" {
		return errors.New(consts.ErrAuthSourceNameRequired)
	}
	if !authSourceNamePattern.MatchString(source.Name) {
		return errors.New(consts.ErrAuthSourceNameInvalid)
	}
	if source.Type != consts.AuthSourceTypeOIDC {
		return errors.New(consts.ErrAuthSourceTypeUnsupported)
	}
	if source.OpenIDDiscoveryURL == "" {
		//nolint:staticcheck // descriptive error constant
		return errors.New(consts.ErrAuthSourceDiscoveryURLRequired)
	}
	if source.IsActive && (source.ClientID == "" || source.ClientSecret == "") {
		return errors.New(consts.ErrAuthSourceClientCredentialsRequired)
	}
	return nil
}

// Sanitize 脱敏处理，将 ClientSecret 清空并设置 ClientSecretConfigured 标志
func (source *AuthSource) Sanitize() {
	source.ClientSecretConfigured = source.ClientSecret != ""
	source.ClientSecret = ""
}
