// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"time"
)

// Schedule 定时任务配置表
type Schedule struct {
	ID        uint64    `json:"id,string" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"size:128;not null"`
	TaskType  string    `json:"task_type" gorm:"size:64;not null"`
	Cron      string    `json:"cron" gorm:"size:64;not null"`
	Payload   string    `json:"payload" gorm:"type:text"`
	IsActive  bool      `json:"is_active" gorm:"not null;default:true"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名
func (Schedule) TableName() string {
	return "w_schedules"
}
