// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
)

const activePushChannelCacheTTL = 24 * time.Hour

// ListPushChannels returns all push channels ordered by creation time descending.
func ListPushChannels(ctx context.Context) ([]model.PushChannel, error) {
	var channels []model.PushChannel
	if err := db.DB(ctx).Order("created_at DESC").Find(&channels).Error; err != nil {
		return nil, err
	}
	return channels, nil
}

// GetPushChannelByID loads a push channel by primary key.
func GetPushChannelByID(ctx context.Context, id uint64) (model.PushChannel, error) {
	var channel model.PushChannel
	if err := db.DB(ctx).Where("id = ?", id).First(&channel).Error; err != nil {
		return model.PushChannel{}, err
	}
	return channel, nil
}

// GetPushChannelByName 根据名称获取消息通道。
func GetPushChannelByName(ctx context.Context, name string) (*model.PushChannel, error) {
	var channel model.PushChannel
	if err := db.DB(ctx).Where("name = ?", name).First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

// CountPushChannelsByName returns how many channels share the given name.
func CountPushChannelsByName(ctx context.Context, name string) (int64, error) {
	var count int64
	if err := db.DB(ctx).Model(&model.PushChannel{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CreatePushChannel persists a new channel and invalidates cache.
func CreatePushChannel(ctx context.Context, channel *model.PushChannel) error {
	if err := db.DB(ctx).Create(channel).Error; err != nil {
		return err
	}
	DeleteActivePushChannelCache(ctx, channel.Name)
	return nil
}

// SavePushChannel updates a channel and invalidates cache.
func SavePushChannel(ctx context.Context, channel *model.PushChannel) error {
	if err := db.DB(ctx).Save(channel).Error; err != nil {
		return err
	}
	DeleteActivePushChannelCache(ctx, channel.Name)
	return nil
}

// DeletePushChannel removes a channel and invalidates cache.
func DeletePushChannel(ctx context.Context, channel *model.PushChannel) error {
	if err := db.DB(ctx).Delete(channel).Error; err != nil {
		return err
	}
	DeleteActivePushChannelCache(ctx, channel.Name)
	return nil
}

// GetActivePushChannelByName 根据名称获取启用的消息通道 (优先从 Redis 缓存获取)。
func GetActivePushChannelByName(ctx context.Context, name string) (*model.PushChannel, error) {
	cacheKey := "push:channel:active:" + name
	var channel model.PushChannel
	if db.Redis != nil {
		if err := db.GetJSON(ctx, cacheKey, &channel); err == nil {
			return &channel, nil
		}
	}

	if err := db.DB(ctx).Where("name = ? AND enabled = ?", name, true).First(&channel).Error; err != nil {
		return nil, err
	}

	if db.Redis != nil {
		_ = db.SetJSON(ctx, cacheKey, channel, activePushChannelCacheTTL)
	}

	return &channel, nil
}

// DeleteActivePushChannelCache 清理启用消息通道的缓存。
func DeleteActivePushChannelCache(ctx context.Context, name string) {
	if db.Redis != nil {
		_ = db.Redis.Del(ctx, db.PrefixedKey("push:channel:active:"+name)).Err()
	}
}
