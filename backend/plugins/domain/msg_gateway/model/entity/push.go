// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"Wavelet/plugins/domain/msg_gateway/consts"
	"errors"
	"strings"
	"time"
)

// PushChannel 消息通道实体
type PushChannel struct {
	ID          uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string    `json:"name" gorm:"size:100;not null"`
	Description string    `json:"description" gorm:"size:255"`
	Type        string    `json:"type" gorm:"size:50;not null;index"`
	URL         string    `json:"url" gorm:"type:text"`
	Token       string    `json:"token" gorm:"type:text"`
	Other       string    `json:"other" gorm:"type:text"`
	Enabled     bool      `json:"enabled" gorm:"index;not null;default:true"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime;index"`
}

// TableName 指定 GORM 表名
func (PushChannel) TableName() string {
	return "w_push_channels"
}

// Validate 验证与标准化字段
func (c *PushChannel) Validate() error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return errors.New(consts.ErrChannelNameRequired)
	}
	c.Type = strings.TrimSpace(c.Type)
	if c.Type == "" {
		return errors.New(consts.ErrChannelTypeRequired)
	}
	return nil
}

// PushEvent 系统通知事件实体
type PushEvent struct {
	ID        uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	EventKey  string    `json:"event_key" gorm:"uniqueIndex;size:80;not null"`
	Name      string    `json:"name" gorm:"size:100;not null"`
	TaskType  string    `json:"task_type" gorm:"size:100;index;not null;default:''"`
	Channels  []string  `json:"channels" gorm:"type:text;serializer:json;not null"`
	Targets   []string  `json:"targets" gorm:"type:text;serializer:json;not null"`
	Template  string    `json:"template" gorm:"type:text;not null"`
	Enabled   bool      `json:"enabled" gorm:"index;not null;default:false"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime;index"`
}

// TableName 指定 GORM 表名
func (PushEvent) TableName() string {
	return "w_push_events"
}

// Validate 验证 PushEvent 实体字段
func (e *PushEvent) Validate() error {
	e.EventKey = strings.TrimSpace(e.EventKey)
	if e.EventKey == "" {
		return errors.New(consts.ErrEventKeyRequired)
	}
	e.Name = strings.TrimSpace(e.Name)
	if e.Name == "" {
		return errors.New(consts.ErrNameRequired)
	}
	return nil
}

// PushHistory 推送日志/历史实体
type PushHistory struct {
	ID        uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	EventKey  string    `json:"event_key" gorm:"size:80;not null;index"`
	Channel   string    `json:"channel" gorm:"size:50;not null;index"`
	Target    string    `json:"target" gorm:"size:255;not null"`
	Title     string    `json:"title" gorm:"size:255;not null"`
	Content   string    `json:"content" gorm:"type:text;not null"`
	Level     string    `json:"level" gorm:"size:20;not null;default:'INFO'"`
	Status    string    `json:"status" gorm:"size:20;not null;index"`
	ErrorMsg  string    `json:"error_msg" gorm:"type:text"`
	Payload   string    `json:"payload" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime;index"`
}

// TableName 指定 GORM 表名
func (PushHistory) TableName() string {
	return "w_push_histories"
}
