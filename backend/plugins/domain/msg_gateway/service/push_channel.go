// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/plugins/domain/msg_gateway/consts"
	"Wavelet/plugins/domain/msg_gateway/dao"
	"Wavelet/plugins/domain/msg_gateway/model/do"
	"Wavelet/plugins/domain/msg_gateway/model/entity"
	pkgpush "Wavelet/plugins/domain/msg_gateway/push"
	"context"
	"errors"
	"fmt"
)

// ListPushChannels returns every configured push channel.
func ListPushChannels(ctx context.Context) ([]entity.PushChannel, error) {
	return dao.ListPushChannelsRecord(ctx)
}

// CreatePushChannel validates uniqueness and persists a new push channel.
func CreatePushChannel(ctx context.Context, req do.CreatePushChannelRequest) (entity.PushChannel, error) {
	count, err := dao.CountPushChannelsByNameRecord(ctx, req.Name)
	if err != nil {
		return entity.PushChannel{}, err
	}
	if count > 0 {
		return entity.PushChannel{}, errors.New(consts.ErrChannelNameExists)
	}

	channel := entity.PushChannel{
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Token:       req.Token,
		URL:         req.URL,
		Other:       req.Other,
		Enabled:     req.Enabled,
	}
	if err := channel.Validate(); err != nil {
		return entity.PushChannel{}, err
	}
	if err := dao.CreatePushChannelRecord(ctx, &channel); err != nil {
		return entity.PushChannel{}, err
	}
	return channel, nil
}

// UpdatePushChannel replaces the mutable fields of an existing push channel.
func UpdatePushChannel(ctx context.Context, id uint64, req do.UpdatePushChannelRequest) (entity.PushChannel, error) {
	channel, err := dao.GetPushChannelByIDRecord(ctx, id)
	if err != nil {
		return entity.PushChannel{}, err
	}

	channel.Description = req.Description
	channel.Type = req.Type
	channel.Token = req.Token
	channel.URL = req.URL
	channel.Other = req.Other
	channel.Enabled = req.Enabled
	if err := channel.Validate(); err != nil {
		return entity.PushChannel{}, err
	}
	if err := dao.SavePushChannelRecord(ctx, &channel); err != nil {
		return entity.PushChannel{}, err
	}
	return channel, nil
}

// DeletePushChannel removes a push channel by id.
func DeletePushChannel(ctx context.Context, id uint64) error {
	channel, err := dao.GetPushChannelByIDRecord(ctx, id)
	if err != nil {
		return err
	}
	return dao.DeletePushChannelRecord(ctx, &channel)
}

// LoadChannelForTest resolves the credentials under test, either from a stored
// channel name or from the ad-hoc values sent by the caller.
func LoadChannelForTest(ctx context.Context, req do.TestPushChannelRequest) (string, string, string, string, error) {
	if req.Name != "" {
		channel, err := dao.GetPushChannelByNameRecord(ctx, req.Name)
		if err != nil {
			return "", "", "", "", errors.New(consts.ErrChannelNotFoundText)
		}
		return channel.URL, channel.Token, channel.Other, channel.Type, nil
	}
	return req.URL, req.Token, req.Other, req.Type, nil
}

// PreparePushChannelTest builds the connectivity probe payload for a channel.
func PreparePushChannelTest(ctx context.Context, req do.TestPushChannelRequest) (do.SendPayload, error) {
	url, token, other, channelType, err := LoadChannelForTest(ctx, req)
	if err != nil {
		return do.SendPayload{}, err
	}

	tempChannel := entity.PushChannel{
		Name:    "test_temp",
		URL:     url,
		Token:   token,
		Other:   other,
		Type:    channelType,
		Enabled: true,
	}
	if err := tempChannel.Validate(); err != nil {
		return do.SendPayload{}, err
	}
	url = tempChannel.URL

	var config pkgpush.Config
	var renderedJSON string
	switch channelType {
	case consts.ChannelLark:
		config = pkgpush.Config{Channel: consts.ChannelLark, URL: url, Secret: token}
		renderedJSON = other
	case consts.ChannelEmail:
		config = pkgpush.Config{Channel: consts.ChannelEmail, URL: url, Key: token, Secret: other}
	case consts.ChannelTelegram:
		config = pkgpush.Config{Channel: consts.ChannelTelegram, URL: url, Secret: token, Key: other}
	default:
		config = pkgpush.Config{Channel: consts.ChannelCustom, URL: url}
		customPushReq := do.CustomPushRequest{
			Title:       "通道测试通知",
			Content:     "这是一条来自系统的消息通道连通性测试消息。",
			Description: "系统通道测试",
			URL:         "https://example.com",
			To:          req.Target,
		}
		renderedJSON = RenderCustomPayload(other, customPushReq)
	}

	return do.SendPayload{
		EventKey: "test_channel",
		Config:   config,
		Target:   req.Target,
		Body: do.NotificationMessage{
			Title:   "通道测试通知",
			Content: "这是一条来自系统的消息通道连通性测试消息。",
			Level:   consts.DefaultLevelInfo,
		},
		Template: renderedJSON,
	}, nil
}

// RunPushTest validates an ad-hoc channel config and sends a connectivity probe.
func RunPushTest(ctx context.Context, cfg pkgpush.Config, target string) error {
	pusher, err := pkgpush.GetPusher(cfg.Channel)
	if err != nil {
		return err
	}
	if err := pusher.ValidateConfig(cfg); err != nil {
		return fmt.Errorf("%s: %w", consts.ErrValidationFailed, err)
	}

	testBody := map[string]any{
		consts.KeyTitle:   "测试通道推送",
		consts.KeyContent: "当您收到这条消息，说明当前渠道连通性测试通过。",
		consts.KeyLevel:   consts.DefaultLevelInfo,
	}
	if _, err := pusher.Send(ctx, cfg, target, testBody, "", nil); err != nil {
		return err
	}
	return nil
}
