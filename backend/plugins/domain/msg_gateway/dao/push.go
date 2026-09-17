// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"Wavelet/plugins/domain/msg_gateway/model/do"
	"Wavelet/plugins/domain/msg_gateway/model/entity"
	"context"
	"time"

	"gorm.io/gorm"
)

const (
	activePushChannelCacheTTL = 24 * time.Hour
	activePushEventCacheTTL   = 24 * time.Hour
)

// ListPushChannelsRecord returns all push channels ordered by creation time descending.
func ListPushChannelsRecord(ctx context.Context) ([]entity.PushChannel, error) {
	var channels []entity.PushChannel
	if err := GetDB(ctx).Order("created_at DESC").Find(&channels).Error; err != nil {
		return nil, err
	}
	return channels, nil
}

// GetPushChannelByIDRecord loads a push channel by primary key.
func GetPushChannelByIDRecord(ctx context.Context, id uint64) (entity.PushChannel, error) {
	var channel entity.PushChannel
	if err := GetDB(ctx).Where("id = ?", id).First(&channel).Error; err != nil {
		return entity.PushChannel{}, mapNotFound(err)
	}
	return channel, nil
}

// GetPushChannelByNameRecord loads a push channel by its unique name.
func GetPushChannelByNameRecord(ctx context.Context, name string) (*entity.PushChannel, error) {
	var channel entity.PushChannel
	if err := GetDB(ctx).Where("name = ?", name).First(&channel).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &channel, nil
}

// CountPushChannelsByNameRecord returns how many channels share the given name.
func CountPushChannelsByNameRecord(ctx context.Context, name string) (int64, error) {
	var count int64
	if err := GetDB(ctx).Model(&entity.PushChannel{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CreatePushChannelRecord persists a new channel and invalidates cache.
func CreatePushChannelRecord(ctx context.Context, channel *entity.PushChannel) error {
	if err := GetDB(ctx).Create(channel).Error; err != nil {
		return err
	}
	DeleteActivePushChannelCache(ctx, channel.Name)
	return nil
}

// SavePushChannelRecord updates a channel and invalidates cache.
func SavePushChannelRecord(ctx context.Context, channel *entity.PushChannel) error {
	if err := GetDB(ctx).Save(channel).Error; err != nil {
		return err
	}
	DeleteActivePushChannelCache(ctx, channel.Name)
	return nil
}

// DeletePushChannelRecord removes a channel and invalidates cache.
func DeletePushChannelRecord(ctx context.Context, channel *entity.PushChannel) error {
	if err := GetDB(ctx).Delete(channel).Error; err != nil {
		return err
	}
	DeleteActivePushChannelCache(ctx, channel.Name)
	return nil
}

func getCachedOrQuery[T any](ctx context.Context, cacheKey string, ttl time.Duration, query func(db *gorm.DB, dest *T) error) (*T, error) {
	if cache := GetCache(ctx); cache != nil {
		var val T
		if err := cache.Get(ctx, cacheKey, &val); err == nil {
			return &val, nil
		}
	}

	db := GetDB(ctx)
	var val T
	if err := query(db, &val); err != nil {
		return nil, err
	}

	if cache := GetCache(ctx); cache != nil {
		_ = cache.Set(ctx, cacheKey, val, ttl)
	}

	return &val, nil
}

// GetActivePushChannelByName loads an enabled push channel, preferring the cache layer.
func GetActivePushChannelByName(ctx context.Context, name string) (*entity.PushChannel, error) {
	channel, err := getCachedOrQuery(ctx, "push:channel:active:"+name, activePushChannelCacheTTL, func(db *gorm.DB, dest *entity.PushChannel) error {
		return db.Where("name = ? AND enabled = ?", name, true).First(dest).Error
	})
	if err != nil {
		return nil, mapNotFound(err)
	}
	return channel, nil
}

// DeleteActivePushChannelCache drops the cached enabled-channel entry.
func DeleteActivePushChannelCache(ctx context.Context, name string) {
	if cache := GetCache(ctx); cache != nil {
		_ = cache.Delete(ctx, "push:channel:active:"+name)
	}
}

// ListPushEventsRecord returns all push events ordered by creation time descending.
func ListPushEventsRecord(ctx context.Context) ([]entity.PushEvent, error) {
	var events []entity.PushEvent
	if err := GetDB(ctx).Order("created_at DESC").Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

// GetPushEventByIDRecord loads a push event by primary key.
func GetPushEventByIDRecord(ctx context.Context, id uint64) (entity.PushEvent, error) {
	var event entity.PushEvent
	if err := GetDB(ctx).First(&event, id).Error; err != nil {
		return entity.PushEvent{}, mapNotFound(err)
	}
	return event, nil
}

// GetPushEventByKeyRecord loads a push event by event key.
func GetPushEventByKeyRecord(ctx context.Context, key string) (entity.PushEvent, error) {
	var event entity.PushEvent
	if err := GetDB(ctx).Where("event_key = ?", key).First(&event).Error; err != nil {
		return entity.PushEvent{}, mapNotFound(err)
	}
	return event, nil
}

// CountPushEventsByKeyRecord returns how many events use the given event key.
func CountPushEventsByKeyRecord(ctx context.Context, key string) (int64, error) {
	var count int64
	if err := GetDB(ctx).Model(&entity.PushEvent{}).Where("event_key = ?", key).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CreatePushEventRecord persists a new push event and invalidates cache.
func CreatePushEventRecord(ctx context.Context, event *entity.PushEvent) error {
	if err := GetDB(ctx).Create(event).Error; err != nil {
		return err
	}
	DeleteActivePushEventCache(ctx, event.EventKey)
	return nil
}

// SavePushEventRecord updates a push event and invalidates cache.
func SavePushEventRecord(ctx context.Context, event *entity.PushEvent) error {
	if err := GetDB(ctx).Save(event).Error; err != nil {
		return err
	}
	DeleteActivePushEventCache(ctx, event.EventKey)
	return nil
}

// UpdatePushEventEnabledRecord toggles the enabled flag for a push event.
func UpdatePushEventEnabledRecord(ctx context.Context, event *entity.PushEvent, enabled bool) error {
	event.Enabled = enabled
	if err := GetDB(ctx).Model(event).Update("enabled", enabled).Error; err != nil {
		return err
	}
	DeleteActivePushEventCache(ctx, event.EventKey)
	return nil
}

// DeletePushEventRecord removes a push event and invalidates cache.
func DeletePushEventRecord(ctx context.Context, event *entity.PushEvent) error {
	if err := GetDB(ctx).Delete(event).Error; err != nil {
		return err
	}
	DeleteActivePushEventCache(ctx, event.EventKey)
	return nil
}

// ListActivePushEventsByTaskTypeRecord returns enabled events bound to a task type.
func ListActivePushEventsByTaskTypeRecord(ctx context.Context, taskType string) ([]entity.PushEvent, error) {
	var events []entity.PushEvent
	if err := GetDB(ctx).Where("task_type = ? AND enabled = ?", taskType, true).Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

// GetActivePushEventByKey loads an enabled push event, preferring the cache layer.
func GetActivePushEventByKey(ctx context.Context, key string) (*entity.PushEvent, error) {
	event, err := getCachedOrQuery(ctx, "push:event:active:"+key, activePushEventCacheTTL, func(db *gorm.DB, dest *entity.PushEvent) error {
		return db.Where("event_key = ? AND enabled = ?", key, true).First(dest).Error
	})
	if err != nil {
		return nil, mapNotFound(err)
	}
	return event, nil
}

// DeleteActivePushEventCache drops the cached enabled-event entry.
func DeleteActivePushEventCache(ctx context.Context, key string) {
	if cache := GetCache(ctx); cache != nil {
		_ = cache.Delete(ctx, "push:event:active:"+key)
	}
}

// ListPushHistoriesRecord returns paginated push history records.
func ListPushHistoriesRecord(ctx context.Context, filter do.PushHistoryListFilter) (int64, []entity.PushHistory, error) {
	query := GetDB(ctx).Model(&entity.PushHistory{}).Order("created_at DESC")
	if filter.EventKey != "" {
		query = query.Where("event_key = ?", filter.EventKey)
	}
	if filter.Channel != "" {
		query = query.Where("channel = ?", filter.Channel)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}

	var results []entity.PushHistory
	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Offset(offset).Limit(filter.PageSize).Find(&results).Error; err != nil {
		return 0, nil, err
	}

	return total, results, nil
}

// CreatePushHistoryRecord persists a push history audit record.
func CreatePushHistoryRecord(ctx context.Context, history *entity.PushHistory) error {
	return GetDB(ctx).Create(history).Error
}

// PushHistoryQuery returns a scoped query builder for push histories.
func PushHistoryQuery(ctx context.Context) *gorm.DB {
	return GetDB(ctx).Model(&entity.PushHistory{})
}

// DeletePushHistoriesBeforeRecord deletes push history records created before cutoff time.
func DeletePushHistoriesBeforeRecord(ctx context.Context, cutoff time.Time) (int64, error) {
	db := GetDB(ctx)
	if db == nil {
		return 0, nil
	}
	result := db.Where("created_at < ?", cutoff).Delete(&entity.PushHistory{})
	return result.RowsAffected, result.Error
}
