// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"time"
)

// PushHistory 推送日志/历史实体
type PushHistory struct {
	ID        uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	EventKey  string    `json:"event_key" gorm:"size:80;not null;index"`
	Channel   string    `json:"channel" gorm:"size:50;not null"`
	Target    string    `json:"target" gorm:"size:255;not null"`
	Title     string    `json:"title" gorm:"size:255;not null"`
	Content   string    `json:"content" gorm:"type:text;not null"`
	Level     string    `json:"level" gorm:"size:20;not null"`
	Status    string    `json:"status" gorm:"size:20;not null"` // success / failed
	ErrorMsg  string    `json:"error_msg" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime;index"`
}

// TableName 指定表名
func (PushHistory) TableName() string {
	return "w_push_histories"
}
