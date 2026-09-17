// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package entity defines GORM table mapping entities for msg_gateway.
package entity

import "time"

// MessageChannel is an admin-configured messaging adapter entity.
type MessageChannel struct {
	ID          uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	Type        string    `json:"type" gorm:"size:32;not null"`
	Name        string    `json:"name" gorm:"size:64;not null"`
	OwnerScope  string    `json:"owner_scope" gorm:"size:32;not null;default:'system'"`
	OwnerID     *uint64   `json:"owner_id,omitempty"`
	Credentials string    `json:"credentials" gorm:"type:text;not null"`
	Extra       string    `json:"extra" gorm:"type:text"`
	Enabled     bool      `json:"enabled" gorm:"default:false;not null"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名
func (MessageChannel) TableName() string {
	return "w_message_channels"
}

// MessageBinding maps a platform user to a Wavelet user on one channel.
type MessageBinding struct {
	ID             uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	ChannelID      uint64    `json:"channel_id" gorm:"not null;index"`
	PlatformUserID string    `json:"platform_user_id" gorm:"size:128;not null;index"`
	UserID         uint64    `json:"user_id" gorm:"not null;index"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName 表名
func (MessageBinding) TableName() string {
	return "w_message_bindings"
}

// MessagePairingCode is a one-time bind code.
type MessagePairingCode struct {
	ID             uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	Code           string    `json:"code" gorm:"size:32;uniqueIndex;not null"`
	ChannelID      uint64    `json:"channel_id" gorm:"not null;index"`
	PlatformUserID string    `json:"platform_user_id" gorm:"size:128;not null;index"`
	UserID         uint64    `json:"user_id" gorm:"not null;index"`
	ExpiresAt      time.Time `json:"expires_at" gorm:"not null;index"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName 表名
func (MessagePairingCode) TableName() string {
	return "w_message_pairing_codes"
}
