// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/dingding"
)

func init() {
	Register("dingtalk", &DingTalkPusher{})
}

// DingTalkPusher 基于 nikoksr/notify 的钉钉机器人推送实现
type DingTalkPusher struct{}

// Send 发送钉钉通知
func (p *DingTalkPusher) Send(ctx context.Context, cfg Config, _ string, body map[string]any, _ string, _ map[string]any) (string, error) {
	token := cfg.Key
	if token == "" {
		if u, err := url.Parse(cfg.URL); err == nil {
			token = u.Query().Get("access_token")
		}
	}
	if token == "" {
		token = cfg.URL
	}
	if token == "" {
		return "", errors.New("dingtalk: access token or webhook URL is required")
	}

	title := bodyTitle(body)
	content := bodyContent(body, "**%s**: %v", "\n\n")

	dingService := dingding.New(&dingding.Config{
		Token:  token,
		Secret: cfg.Secret,
	})

	notifier := notify.New()
	notifier.UseServices(dingService)

	if err := notifier.Send(ctx, title, content); err != nil {
		return "", fmt.Errorf("dingtalk: notify send failed: %w", err)
	}

	return "ok", nil
}

// ValidateConfig 校验钉钉配置
func (p *DingTalkPusher) ValidateConfig(cfg Config) error {
	if cfg.URL == "" && cfg.Key == "" {
		return errors.New("webhook URL or access token is required")
	}
	if cfg.URL != "" && !strings.HasPrefix(cfg.URL, "https://") {
		return errors.New("webhook URL must use https:// protocol")
	}
	return nil
}
