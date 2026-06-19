// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"errors"
	"strings"
	"time"
)

// PushEvent 系统通知事件模型
type PushEvent struct {
	ID        uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	EventKey  string    `json:"event_key" gorm:"uniqueIndex;size:80;not null"`       // 如 admin_login
	Name      string    `json:"name" gorm:"size:100;not null"`                       // 如 管理员登录
	TaskType  string    `json:"task_type" gorm:"size:100;index;not null;default:''"` // 关联的异步任务类型
	Channels  []string  `json:"channels" gorm:"type:text;serializer:json;not null"`  // 推送渠道列表，如 ["lark"]
	Targets   []string  `json:"targets" gorm:"type:text;serializer:json;not null"`   // 推送目标用户/邮箱列表
	Template  string    `json:"template" gorm:"type:text;not null"`                  // 消息模板 JSON
	Enabled   bool      `json:"enabled" gorm:"index;not null;default:false"`         // 是否启用
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime;index"`
}

// TableName 指定 GORM 表名
func (PushEvent) TableName() string {
	return "w_push_events"
}

// Validate 基础校验
func (pe *PushEvent) Validate() error {
	pe.EventKey = strings.TrimSpace(pe.EventKey)
	pe.Name = strings.TrimSpace(pe.Name)
	pe.Template = strings.TrimSpace(pe.Template)

	if pe.EventKey == "" {
		return errors.New("event key is required")
	}
	if pe.Name == "" {
		return errors.New("event name is required")
	}
	if pe.Template == "" {
		return errors.New("event template is required")
	}
	if pe.Enabled && len(pe.Channels) == 0 {
		return errors.New("cannot enable event without any push channels configured")
	}
	return nil
}
