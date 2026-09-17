// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"context"
	"errors"
	"fmt"

	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/discord"
)

func init() {
	Register("discord", &DiscordPusher{})
}

// DiscordPusher 基于 nikoksr/notify 的 Discord 推送实现
type DiscordPusher struct{}

// Send 发送 Discord 通知
func (p *DiscordPusher) Send(ctx context.Context, cfg Config, target string, body map[string]any, _ string, _ map[string]any) (string, error) {
	botToken := cfg.Key
	if botToken == "" {
		botToken = cfg.Secret
	}
	channelID := cfg.URL
	if target != "" {
		channelID = target
	}
	if channelID == "" {
		channelID = cfg.Other
	}

	if botToken == "" {
		return "", errors.New("discord: bot token is required")
	}
	if channelID == "" {
		return "", errors.New("discord: channel ID is required")
	}

	title := bodyTitle(body)
	content := bodyContent(body, "**%s**: %v", "\n")

	discordService := discord.New()
	if err := discordService.AuthenticateWithBotToken(botToken); err != nil {
		return "", fmt.Errorf("discord: auth failed: %w", err)
	}
	discordService.AddReceivers(channelID)

	notifier := notify.New()
	notifier.UseServices(discordService)

	if err := notifier.Send(ctx, title, content); err != nil {
		return "", fmt.Errorf("discord: notify send failed: %w", err)
	}

	return "ok", nil
}

// ValidateConfig 校验 Discord 配置
func (p *DiscordPusher) ValidateConfig(cfg Config) error {
	botToken := cfg.Key
	if botToken == "" {
		botToken = cfg.Secret
	}
	if botToken == "" {
		return errors.New("bot token is required")
	}
	return nil
}
