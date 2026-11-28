// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"time"
)

// DNS account verification statuses.
const (
	DNSAccountStatusReady = "ready"
	DNSAccountStatusError = "error"
)

// DNSAccountScope identifies a DNS zone visible to the provider credential.
type DNSAccountScope struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DNSAccount OpenFlare DNS 账号实体。
type DNSAccount struct {
	ID                 uint              `json:"id" gorm:"primaryKey;autoIncrement"`
	Name               string            `json:"name" gorm:"size:255;not null"`
	Type               string            `json:"type" gorm:"size:64;not null"`
	Authorization      string            `json:"-" gorm:"type:text;not null"`
	Scopes             []DNSAccountScope `json:"scopes" gorm:"serializer:json;type:text;not null;default:'[]'"`
	VerificationStatus string            `json:"verification_status" gorm:"size:16;not null;default:''"`
	LastCheckedAt      *time.Time        `json:"last_checked_at"`
	CreatedAt          time.Time         `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time         `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名。
func (DNSAccount) TableName() string {
	return "of_dns_accounts"
}
