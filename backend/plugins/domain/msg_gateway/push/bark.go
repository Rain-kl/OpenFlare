// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/bark"
)

func init() {
	Register("bark", &BarkPusher{})
}

// BarkPusher 基于 nikoksr/notify 的 Bark iOS 客户端通知推送实现
type BarkPusher struct{}

// Send 发送 Bark 通知
func (p *BarkPusher) Send(ctx context.Context, cfg Config, target string, body map[string]any, _ string, _ map[string]any) (string, error) {
	deviceKey := cfg.Key
	if deviceKey == "" {
		deviceKey = cfg.Secret
	}
	if target != "" {
		deviceKey = target
	}
	if deviceKey == "" {
		return "", errors.New("bark: device key is required")
	}

	serverURL := strings.TrimRight(cfg.URL, "/")
	if serverURL == "" {
		serverURL = bark.DefaultServerURL
	}

	title := bodyTitle(body)
	content := bodyContent(body, "%s: %v", "\n")

	barkService := bark.NewWithServers(deviceKey, serverURL)
	notifier := notify.New()
	notifier.UseServices(barkService)

	if err := notifier.Send(ctx, title, content); err != nil {
		return "", fmt.Errorf("bark: notify send failed: %w", err)
	}

	return "ok", nil
}

// ValidateConfig 校验 Bark 配置
func (p *BarkPusher) ValidateConfig(cfg Config) error {
	deviceKey := cfg.Key
	if deviceKey == "" {
		deviceKey = cfg.Secret
	}
	if deviceKey == "" {
		return errors.New("device key is required")
	}
	if cfg.URL != "" && !strings.HasPrefix(cfg.URL, "http://") && !strings.HasPrefix(cfg.URL, "https://") {
		return errors.New("server URL must start with http:// or https://")
	}
	return nil
}
