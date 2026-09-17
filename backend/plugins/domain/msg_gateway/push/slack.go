// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"context"
	"errors"
	"fmt"

	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/slack"
)

func init() {
	Register("slack", &SlackPusher{})
}

// SlackPusher 基于 nikoksr/notify 的 Slack 推送实现
type SlackPusher struct{}

// Send 发送 Slack 通知
func (p *SlackPusher) Send(ctx context.Context, cfg Config, target string, body map[string]any, _ string, _ map[string]any) (string, error) {
	token := cfg.Key
	if token == "" {
		token = cfg.Secret
	}
	channelID := cfg.URL
	if target != "" {
		channelID = target
	}
	if channelID == "" {
		channelID = cfg.Other
	}

	if token == "" {
		return "", errors.New("slack: bot/api token is required")
	}
	if channelID == "" {
		return "", errors.New("slack: channel ID is required")
	}

	title := bodyTitle(body)
	content := bodyContent(body, "*%s*: %v", "\n")

	slackService := slack.New(token)
	slackService.AddReceivers(channelID)

	notifier := notify.New()
	notifier.UseServices(slackService)

	if err := notifier.Send(ctx, title, content); err != nil {
		return "", fmt.Errorf("slack: notify send failed: %w", err)
	}

	return "ok", nil
}

// ValidateConfig 校验 Slack 配置
func (p *SlackPusher) ValidateConfig(cfg Config) error {
	token := cfg.Key
	if token == "" {
		token = cfg.Secret
	}
	if token == "" {
		return errors.New("slack token is required")
	}
	return nil
}
