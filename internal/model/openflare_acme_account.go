// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"time"
)

// AcmeAccount OpenFlare ACME 账号实体。
type AcmeAccount struct {
	ID         uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Email      string    `json:"email" gorm:"size:255"`
	URL        string    `json:"url" gorm:"size:255"`
	PrivateKey string    `json:"-" gorm:"type:text;not null"`
	CreatedAt  time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名。
func (AcmeAccount) TableName() string {
	return "of_acme_accounts"
}
