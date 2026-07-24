// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"time"
)

// DNSAccount OpenFlare DNS 账号实体。
type DNSAccount struct {
	ID            uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name          string    `json:"name" gorm:"size:255;not null"`
	Type          string    `json:"type" gorm:"size:64;not null"`
	Authorization string    `json:"-" gorm:"type:text;not null"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名。
func (DNSAccount) TableName() string {
	return "of_dns_accounts"
}
