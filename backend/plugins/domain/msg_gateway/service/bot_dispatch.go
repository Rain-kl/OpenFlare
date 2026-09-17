// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/msg_gateway/consts"
	"Wavelet/plugins/domain/msg_gateway/dao"
	"Wavelet/plugins/domain/msg_gateway/model/do"
	"Wavelet/plugins/domain/msg_gateway/model/entity"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	// TaskDispatchBotMsg is the queue pattern for bot downlink dispatch.
	TaskDispatchBotMsg = consts.TaskDispatchBotMsg
	// TaskTypeDispatchBotMsg is the admin type identifier for bot downlink dispatch.
	TaskTypeDispatchBotMsg = consts.TaskTypeDispatchBotMsg

	taskQueueDefault    = "default"
	taskParamTypeString = "string"
	paramNameText       = "text"
)

// BotDispatchMeta describes the bot downlink dispatch task.
var BotDispatchMeta = contracts.TaskMetaDTO{
	Type:        TaskTypeDispatchBotMsg,
	AsynqTask:   TaskDispatchBotMsg,
	Name:        "分发 Bot 消息",
	DisplayName: "分发 Bot 消息",
	Description: "向已绑定的平台账号异步下发 Bot 文本消息",
	Category:    "messaging",
	Queue:       taskQueueDefault,
	Retryable:   true,
	Params: []contracts.TaskParamDTO{
		{Name: paramNameText, Label: "消息内容", Type: consts.TypeText, Required: true, Placeholder: "要发送的文本", Description: "下发给绑定用户的文本"},
		{Name: "channel_id", Label: "频道 ID", Type: taskParamTypeString, Required: false, Placeholder: "留空表示全部启用频道", Description: "仅向指定频道的绑定发送"},
		{Name: "user_id", Label: "用户 ID", Type: taskParamTypeString, Required: false, Placeholder: "留空表示频道下全部绑定", Description: "仅向指定 Wavelet 用户的绑定发送"},
	},
}

type botDispatchPayload struct {
	Text      string `json:"text"`
	ChannelID uint64 `json:"channel_id,string"`
	UserID    uint64 `json:"user_id,string"`
}

// BotDispatchHandler sends a text message through enabled bot channels.
type BotDispatchHandler struct{}

// ValidatePayload requires a non-empty message body.
func (h *BotDispatchHandler) ValidatePayload(payload []byte) ([]byte, error) {
	p, err := parseBotDispatchPayload(payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(p)
}

// Execute delivers the text to matching channel bindings.
func (h *BotDispatchHandler) Execute(ctx context.Context, payload []byte) (*contracts.TaskResultDTO, error) {
	p, err := parseBotDispatchPayload(payload)
	if err != nil {
		return nil, err
	}

	channels, err := dao.ListEnabledMessageChannels(ctx)
	if err != nil {
		return nil, err
	}
	if p.ChannelID != 0 {
		filtered := channels[:0]
		for i := range channels {
			if channels[i].ID == p.ChannelID {
				filtered = append(filtered, channels[i])
			}
		}
		channels = filtered
		if len(channels) == 0 {
			return nil, consts.ErrChannelNotFound
		}
	}

	sent := 0
	failed := 0
	for i := range channels {
		n, ferr := dispatchOnChannel(ctx, &channels[i], p.UserID, p.Text)
		sent += n
		failed += ferr
	}
	msg := fmt.Sprintf("Bot 消息已尝试发送，成功 %d，失败 %d", sent, failed)
	if svc := GetTaskService(ctx); svc != nil {
		svc.AppendLog(ctx, "%s", msg)
	}
	if sent == 0 && failed > 0 {
		return nil, errors.New(msg)
	}
	return &contracts.TaskResultDTO{Message: msg}, nil
}

func parseBotDispatchPayload(payload []byte) (botDispatchPayload, error) {
	var p botDispatchPayload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &p); err != nil {
			return p, fmt.Errorf("%s: %w", consts.ErrInvalidJSONFormat, err)
		}
	}
	p.Text = strings.TrimSpace(p.Text)
	if p.Text == "" {
		return p, errors.New(consts.ErrBotDispatchTextRequired)
	}
	return p, nil
}

func dispatchOnChannel(ctx context.Context, row *entity.MessageChannel, userID uint64, text string) (sent, failed int) {
	factory, ok := Lookup(row.Type)
	if !ok {
		logger.ErrorF(ctx, "bot dispatch: %s type=%s", consts.ErrBotChannelNotRegistered, row.Type)
		return 0, 1
	}
	cfg, err := channelConfigFromRow(row)
	if err != nil {
		logger.ErrorF(ctx, "bot dispatch: decode channel %d: %v", row.ID, err)
		return 0, 1
	}
	ch, err := factory(cfg, nil)
	if err != nil {
		logger.ErrorF(ctx, "bot dispatch: create adapter %d: %v", row.ID, err)
		return 0, 1
	}
	if err := ch.Connect(ctx); err != nil {
		logger.ErrorF(ctx, "bot dispatch: connect channel %d: %v", row.ID, err)
		return 0, 1
	}
	defer func() { _ = ch.Disconnect(ctx) }()

	bindings, err := dao.ListBindingsByChannel(ctx, row.ID)
	if err != nil {
		logger.ErrorF(ctx, "bot dispatch: list bindings %d: %v", row.ID, err)
		return 0, 1
	}
	for i := range bindings {
		if userID != 0 && bindings[i].UserID != userID {
			continue
		}
		to := do.Recipient{
			ChatID:         bindings[i].PlatformUserID,
			PlatformUserID: bindings[i].PlatformUserID,
		}
		if err := ch.Send(ctx, to, do.OutboundMessage{Text: text}); err != nil {
			logger.ErrorF(ctx, "bot dispatch: send channel=%d user=%d: %v", row.ID, bindings[i].UserID, err)
			failed++
			continue
		}
		sent++
	}
	return sent, failed
}

func channelConfigFromRow(row *entity.MessageChannel) (do.ChannelConfig, error) {
	creds, err := DecryptCredentials(row.Credentials)
	if err != nil {
		return do.ChannelConfig{}, err
	}
	if creds == nil {
		creds = map[string]string{}
	}
	if creds["bot_token"] == "" && creds["token"] != "" {
		creds["bot_token"] = creds["token"]
	}
	if creds["app_secret"] == "" && creds["client_secret"] != "" {
		creds["app_secret"] = creds["client_secret"]
	}
	extra := ParseExtra(row.Extra)
	if extra["base_url"] == "" && creds["api_base"] != "" {
		extra["base_url"] = creds["api_base"]
	}
	return do.ChannelConfig{
		ID:          row.ID,
		Type:        row.Type,
		Name:        row.Name,
		Credentials: creds,
		Extra:       extra,
	}, nil
}
