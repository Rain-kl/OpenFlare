// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
)

const activePushEventCacheTTL = 24 * time.Hour

// ListPushEvents returns all push events ordered by creation time descending.
func ListPushEvents(ctx context.Context) ([]model.PushEvent, error) {
	var events []model.PushEvent
	if err := db.DB(ctx).Order("created_at DESC").Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

// GetPushEventByID loads a push event by primary key.
func GetPushEventByID(ctx context.Context, id uint64) (model.PushEvent, error) {
	var event model.PushEvent
	if err := db.DB(ctx).First(&event, id).Error; err != nil {
		return model.PushEvent{}, err
	}
	return event, nil
}

// GetPushEventByKey loads a push event by event key.
func GetPushEventByKey(ctx context.Context, key string) (model.PushEvent, error) {
	var event model.PushEvent
	if err := db.DB(ctx).Where("event_key = ?", key).First(&event).Error; err != nil {
		return model.PushEvent{}, err
	}
	return event, nil
}

// CountPushEventsByKey returns how many events use the given event key.
func CountPushEventsByKey(ctx context.Context, key string) (int64, error) {
	var count int64
	if err := db.DB(ctx).Model(&model.PushEvent{}).Where("event_key = ?", key).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CreatePushEvent persists a new push event and invalidates cache.
func CreatePushEvent(ctx context.Context, event *model.PushEvent) error {
	if err := db.DB(ctx).Create(event).Error; err != nil {
		return err
	}
	DeleteActivePushEventCache(ctx, event.EventKey)
	return nil
}

// SavePushEvent updates a push event and invalidates cache.
func SavePushEvent(ctx context.Context, event *model.PushEvent) error {
	if err := db.DB(ctx).Save(event).Error; err != nil {
		return err
	}
	DeleteActivePushEventCache(ctx, event.EventKey)
	return nil
}

// UpdatePushEventEnabled toggles the enabled flag for a push event.
func UpdatePushEventEnabled(ctx context.Context, event *model.PushEvent, enabled bool) error {
	event.Enabled = enabled
	if err := db.DB(ctx).Model(event).Update("enabled", enabled).Error; err != nil {
		return err
	}
	DeleteActivePushEventCache(ctx, event.EventKey)
	return nil
}

// DeletePushEvent removes a push event and invalidates cache.
func DeletePushEvent(ctx context.Context, event *model.PushEvent) error {
	if err := db.DB(ctx).Delete(event).Error; err != nil {
		return err
	}
	DeleteActivePushEventCache(ctx, event.EventKey)
	return nil
}

// ListActivePushEventsByTaskType returns enabled events bound to a task type.
func ListActivePushEventsByTaskType(ctx context.Context, taskType string) ([]model.PushEvent, error) {
	var events []model.PushEvent
	if err := db.DB(ctx).Where("task_type = ? AND enabled = ?", taskType, true).Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

// GetActivePushEventByKey 获取启用的通知事件 (优先从 Redis 缓存获取)。
func GetActivePushEventByKey(ctx context.Context, key string) (*model.PushEvent, error) {
	cacheKey := "push:event:active:" + key
	var event model.PushEvent
	if db.Redis != nil {
		if err := db.GetJSON(ctx, cacheKey, &event); err == nil {
			return &event, nil
		}
	}

	if err := db.DB(ctx).Where("event_key = ? AND enabled = ?", key, true).First(&event).Error; err != nil {
		return nil, err
	}

	if db.Redis != nil {
		_ = db.SetJSON(ctx, cacheKey, event, activePushEventCacheTTL)
	}

	return &event, nil
}

// DeleteActivePushEventCache 清理启用通知事件的缓存。
func DeleteActivePushEventCache(ctx context.Context, key string) {
	if db.Redis != nil {
		_ = db.Redis.Del(ctx, db.PrefixedKey("push:event:active:"+key)).Err()
	}
}
