// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/lark"
)

func init() {
	Register("lark", &LarkPusher{})
}

// LarkPusher 基于 nikoksr/notify 的飞书 Webhook 机器人推送实现
type LarkPusher struct{}

// Send 发送飞书通知
func (p *LarkPusher) Send(ctx context.Context, cfg Config, _ string, body map[string]any, _ string, _ map[string]any) (string, error) {
	if cfg.URL == "" {
		return "", errors.New("lark: webhook URL is required")
	}

	title := bodyTitle(body)
	content := bodyContent(body, "**%s**: %v", "\n")

	larkService := lark.NewWebhookService(cfg.URL)
	notifier := notify.New()
	notifier.UseServices(larkService)

	if err := notifier.Send(ctx, title, content); err != nil {
		return "", fmt.Errorf("lark: notify send failed: %w", err)
	}

	return "ok", nil
}

// ValidateConfig 校验飞书机器人配置
func (p *LarkPusher) ValidateConfig(cfg Config) error {
	if cfg.URL == "" {
		return errors.New("webhook URL is required")
	}
	if !strings.HasPrefix(cfg.URL, "https://") {
		return errors.New("webhook URL must use https:// protocol")
	}
	return nil
}
