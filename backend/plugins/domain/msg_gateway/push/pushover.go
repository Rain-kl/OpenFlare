// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"context"
	"errors"
	"fmt"

	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/pushover"
)

func init() {
	Register("pushover", &PushoverPusher{})
}

// PushoverPusher 基于 nikoksr/notify 的 Pushover 移动端推送实现
type PushoverPusher struct{}

// Send 发送 Pushover 通知
func (p *PushoverPusher) Send(ctx context.Context, cfg Config, target string, body map[string]any, _ string, _ map[string]any) (string, error) {
	appToken := cfg.Key
	if appToken == "" {
		appToken = cfg.Secret
	}
	if appToken == "" {
		return "", errors.New("pushover: app token is required")
	}

	userKey := cfg.URL
	if userKey == "" {
		userKey = cfg.Other
	}
	if target != "" {
		userKey = target
	}
	if userKey == "" {
		return "", errors.New("pushover: user key is required")
	}

	title := bodyTitle(body)
	content := bodyContent(body, "%s: %v", "\n")

	poService := pushover.New(appToken)
	poService.AddReceivers(userKey)

	notifier := notify.New()
	notifier.UseServices(poService)

	if err := notifier.Send(ctx, title, content); err != nil {
		return "", fmt.Errorf("pushover: notify send failed: %w", err)
	}

	return "ok", nil
}

// ValidateConfig 校验 Pushover 配置
func (p *PushoverPusher) ValidateConfig(cfg Config) error {
	appToken := cfg.Key
	if appToken == "" {
		appToken = cfg.Secret
	}
	if appToken == "" {
		return errors.New("app token is required")
	}
	return nil
}
