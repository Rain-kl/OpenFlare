// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/telegram"
)

func init() {
	Register("telegram", &TelegramPusher{})
}

// TelegramPusher 基于 nikoksr/notify 的 Telegram 机器人推送实现
type TelegramPusher struct{}

// Send 执行 Telegram 消息发送
func (p *TelegramPusher) Send(ctx context.Context, cfg Config, target string, body map[string]any, _ string, _ map[string]any) (string, error) {
	botToken := cfg.Secret
	if botToken == "" {
		botToken = cfg.Key
	}
	if botToken == "" {
		return "", errors.New("telegram: bot token is required")
	}

	chatIDStr := target
	if chatIDStr == "" {
		chatIDStr = cfg.Other
	}
	if chatIDStr == "" {
		return "", errors.New("telegram: chat_id is required")
	}

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("telegram: invalid chat_id %q: %w", chatIDStr, err)
	}

	title := bodyTitle(body)
	content := bodyContent(body, "%s: %v", "\n")

	tgService, err := telegram.New(botToken)
	if err != nil {
		return "", fmt.Errorf("telegram: init service failed: %w", err)
	}
	tgService.AddReceivers(chatID)

	notifier := notify.New()
	notifier.UseServices(tgService)

	if err := notifier.Send(ctx, title, content); err != nil {
		return "", fmt.Errorf("telegram: notify send failed: %w", err)
	}

	return "ok", nil
}

// ValidateConfig 校验 Telegram 机器人配置
func (p *TelegramPusher) ValidateConfig(cfg Config) error {
	token := cfg.Secret
	if token == "" {
		token = cfg.Key
	}
	if token == "" {
		return errors.New("bot token is required")
	}
	return nil
}
