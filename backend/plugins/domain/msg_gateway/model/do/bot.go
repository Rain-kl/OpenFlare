// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package do defines domain objects, DTOs, and request/response payloads for msg_gateway.
package do

import "time"

// Capability describes what an adapter can send and receive.
type Capability struct {
	Text  bool
	Image bool
	File  bool
	Reply bool
	Group bool
}

// ChannelConfig is the decrypted runtime config passed to a factory.
type ChannelConfig struct {
	ID          uint64
	Type        string
	Name        string
	Credentials map[string]string
	Extra       map[string]string
}

// Recipient is the outbound destination on a platform.
type Recipient struct {
	ChatID         string
	PlatformUserID string
}

// Attachment is a downloaded inbound file sitting on local disk.
type Attachment struct {
	Path     string
	FileName string
	MIME     string
	Error    string
}

// InboundMessage is a normalized private-chat message.
type InboundMessage struct {
	ChannelID      uint64
	ChannelType    string
	PlatformUserID string
	ChatID         string
	MessageID      string
	Text           string
	Attachments    []Attachment
	BindingUserID  *uint64
}

// OutboundMessage is a reply or probe send.
type OutboundMessage struct {
	Text        string
	ReplyToID   string
	Attachments []Attachment
}

// BindRequest is the user bind body.
type BindRequest struct {
	ChannelID string `json:"channel_id"`
	Code      string `json:"code"`
}

// BindingDTO is a user-facing binding row.
type BindingDTO struct {
	ID             uint64    `json:"id,string"`
	UserID         uint64    `json:"user_id,string"`
	ChannelID      uint64    `json:"channel_id,string"`
	ChannelName    string    `json:"channel_name"`
	ChannelType    string    `json:"channel_type"`
	PlatformUserID string    `json:"platform_user_id"`
	CreatedAt      time.Time `json:"created_at"`
}

// PublicChannelDTO is an enabled channel a user can bind to.
type PublicChannelDTO struct {
	ID   uint64 `json:"id,string"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// Field is one admin form field.
type Field struct {
	Key      string `json:"key"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

// Definition describes a channel type form.
type Definition struct {
	Type   string  `json:"type"`
	Fields []Field `json:"fields"`
}

// ChannelDTO represents a channel for admin consumption.
type ChannelDTO struct {
	ID          uint64            `json:"id,string"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	OwnerScope  string            `json:"owner_scope"`
	OwnerID     *uint64           `json:"owner_id,string,omitempty"`
	Enabled     bool              `json:"enabled"`
	Credentials map[string]string `json:"credentials"`
	Extra       map[string]string `json:"extra"`
}

// CreateChannelRequest is admin create payload.
type CreateChannelRequest struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Enabled     *bool             `json:"enabled"`
	Credentials map[string]string `json:"credentials"`
	Extra       map[string]string `json:"extra"`
}

// UpdateChannelRequest is admin update payload.
type UpdateChannelRequest struct {
	Name        string            `json:"name"`
	Enabled     *bool             `json:"enabled"`
	Credentials map[string]string `json:"credentials"`
	Extra       map[string]string `json:"extra"`
}
