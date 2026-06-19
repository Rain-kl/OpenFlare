// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
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

// CreateSchedule 创建定时任务
func CreateSchedule(ctx context.Context, schedule *Schedule) error {
	return db.DB(ctx).Create(schedule).Error
}

// UpdateSchedule 更新定时任务
func UpdateSchedule(ctx context.Context, schedule *Schedule) error {
	return db.DB(ctx).Save(schedule).Error
}

// DeleteSchedule 删除定时任务
func DeleteSchedule(ctx context.Context, id uint64) error {
	return db.DB(ctx).Delete(&Schedule{}, id).Error
}

// GetScheduleByID 根据 ID 获取定时任务
func GetScheduleByID(ctx context.Context, id uint64) (*Schedule, error) {
	var schedule Schedule
	if err := db.DB(ctx).Where("id = ?", id).First(&schedule).Error; err != nil {
		return nil, err
	}
	return &schedule, nil
}

// ListSchedules 获取所有定时任务
func ListSchedules(ctx context.Context) ([]Schedule, error) {
	var schedules []Schedule
	if err := db.DB(ctx).Order("id DESC").Find(&schedules).Error; err != nil {
		return nil, err
	}
	return schedules, nil
}

// ListActiveSchedules 获取所有启用的定时任务
func ListActiveSchedules(ctx context.Context) ([]Schedule, error) {
	var schedules []Schedule
	if err := db.DB(ctx).Where("is_active = ?", true).Find(&schedules).Error; err != nil {
		return nil, err
	}
	return schedules, nil
}
