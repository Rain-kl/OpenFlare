// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"Wavelet/pkg/idgen"
	"Wavelet/plugins/domain/msg_gateway/model/entity"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// CreateMessageChannel inserts a channel row.
func CreateMessageChannel(ctx context.Context, ch *entity.MessageChannel) error {
	if ch.ID == 0 {
		ch.ID = idgen.NextUint64ID()
	}
	return GetDB(ctx).Create(ch).Error
}

// UpdateMessageChannel saves a channel row.
func UpdateMessageChannel(ctx context.Context, ch *entity.MessageChannel) error {
	return GetDB(ctx).Save(ch).Error
}

// GetMessageChannel loads a channel by id.
func GetMessageChannel(ctx context.Context, id uint64) (*entity.MessageChannel, error) {
	var ch entity.MessageChannel
	if err := GetDB(ctx).Where("id = ?", id).First(&ch).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &ch, nil
}

// ListMessageChannels returns all channels newest first.
func ListMessageChannels(ctx context.Context) ([]entity.MessageChannel, error) {
	var rows []entity.MessageChannel
	if err := GetDB(ctx).Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// DeleteMessageChannel removes pairings, bindings, then the channel.
func DeleteMessageChannel(ctx context.Context, id uint64) error {
	return GetDB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("channel_id = ?", id).Delete(&entity.MessagePairingCode{}).Error; err != nil {
			return err
		}
		if err := tx.Where("channel_id = ?", id).Delete(&entity.MessageBinding{}).Error; err != nil {
			return err
		}
		return tx.Delete(&entity.MessageChannel{}, id).Error
	})
}

// CreateMessageBinding inserts a binding.
func CreateMessageBinding(ctx context.Context, b *entity.MessageBinding) error {
	if b.ID == 0 {
		b.ID = idgen.NextUint64ID()
	}
	return GetDB(ctx).Create(b).Error
}

// GetBindingByChannelPlatform finds a binding for a platform user on a channel.
func GetBindingByChannelPlatform(ctx context.Context, channelID uint64, platformUserID string) (*entity.MessageBinding, error) {
	var b entity.MessageBinding
	err := GetDB(ctx).Where("channel_id = ? AND platform_user_id = ?", channelID, platformUserID).First(&b).Error
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &b, nil
}

// ListBindingsByUser lists bindings for a Wavelet user.
func ListBindingsByUser(ctx context.Context, userID uint64) ([]entity.MessageBinding, error) {
	var rows []entity.MessageBinding
	if err := GetDB(ctx).Where("user_id = ?", userID).Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListBindingsByChannel lists bindings on one messaging channel.
func ListBindingsByChannel(ctx context.Context, channelID uint64) ([]entity.MessageBinding, error) {
	var rows []entity.MessageBinding
	if err := GetDB(ctx).Where("channel_id = ?", channelID).Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetMessageBinding loads a binding by id.
func GetMessageBinding(ctx context.Context, id uint64) (*entity.MessageBinding, error) {
	var b entity.MessageBinding
	if err := GetDB(ctx).Where("id = ?", id).First(&b).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &b, nil
}

// DeleteMessageBinding deletes a binding by id.
func DeleteMessageBinding(ctx context.Context, id uint64) error {
	return GetDB(ctx).Delete(&entity.MessageBinding{}, id).Error
}

// UpsertPairingCode reuses an unexpired code for the same channel+platform user.
func UpsertPairingCode(ctx context.Context, channelID uint64, platformUserID, code string, expiresAt time.Time) (*entity.MessagePairingCode, error) {
	var existing entity.MessagePairingCode
	err := GetDB(ctx).
		Where("channel_id = ? AND platform_user_id = ? AND expires_at > ?", channelID, platformUserID, time.Now()).
		First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	row := &entity.MessagePairingCode{
		Code:           code,
		ChannelID:      channelID,
		PlatformUserID: platformUserID,
		ExpiresAt:      expiresAt,
	}
	if err := GetDB(ctx).Create(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

// GetPairingCode loads a pairing code by normalized code string.
func GetPairingCode(ctx context.Context, code string) (*entity.MessagePairingCode, error) {
	var row entity.MessagePairingCode
	if err := GetDB(ctx).Where("code = ?", code).First(&row).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &row, nil
}

// DeletePairingCode removes a pairing code.
func DeletePairingCode(ctx context.Context, code string) error {
	return GetDB(ctx).Where("code = ?", code).Delete(&entity.MessagePairingCode{}).Error
}

// DeleteExpiredPairingCodes removes expired pairing rows.
func DeleteExpiredPairingCodes(ctx context.Context) error {
	return GetDB(ctx).Where("expires_at <= ?", time.Now()).Delete(&entity.MessagePairingCode{}).Error
}

// ListEnabledMessageChannels returns enabled channels.
func ListEnabledMessageChannels(ctx context.Context) ([]entity.MessageChannel, error) {
	var rows []entity.MessageChannel
	if err := GetDB(ctx).Where("enabled = ?", true).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
