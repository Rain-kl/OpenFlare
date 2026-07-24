// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// CreateSchedule 创建定时任务
func CreateSchedule(ctx context.Context, schedule *model.Schedule) error {
	return db.DB(ctx).Create(schedule).Error
}

// UpdateSchedule 更新定时任务
func UpdateSchedule(ctx context.Context, schedule *model.Schedule) error {
	return db.DB(ctx).Save(schedule).Error
}

// DeleteSchedule 删除定时任务
func DeleteSchedule(ctx context.Context, id uint64) error {
	return db.DB(ctx).Delete(&model.Schedule{}, id).Error
}

// GetScheduleByID 根据 ID 获取定时任务
func GetScheduleByID(ctx context.Context, id uint64) (*model.Schedule, error) {
	var schedule model.Schedule
	if err := db.DB(ctx).Where("id = ?", id).First(&schedule).Error; err != nil {
		return nil, err
	}
	return &schedule, nil
}

// ListSchedules 获取所有定时任务
func ListSchedules(ctx context.Context) ([]model.Schedule, error) {
	var schedules []model.Schedule
	if err := db.DB(ctx).Order("id DESC").Find(&schedules).Error; err != nil {
		return nil, err
	}
	return schedules, nil
}

// ListActiveSchedules 获取所有启用的定时任务
func ListActiveSchedules(ctx context.Context) ([]model.Schedule, error) {
	var schedules []model.Schedule
	if err := db.DB(ctx).Where("is_active = ?", true).Find(&schedules).Error; err != nil {
		return nil, err
	}
	return schedules, nil
}
