// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package entity provides database model entities for the auth domain plugin.
package entity

import (
	"time"
)

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
