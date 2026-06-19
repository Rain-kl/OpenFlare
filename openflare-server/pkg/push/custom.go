// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Rain-kl/Wavelet/pkg/httppool"
)

func init() {
	Register("custom", &CustomPusher{})
}

// CustomPusher 自定义 Webhook 发送实现
type CustomPusher struct{}

// Send 发送自定义 webhook
func (p *CustomPusher) Send(ctx context.Context, cfg Config, _ string, body map[string]any, template string, _ map[string]any) error {
	if cfg.URL == "" {
		return errors.New("custom: URL is required")
	}

	var reqBody []byte

	if template != "" {
		// 替换模板中的 {{key}} 占位符
		rendered := ParseTemplate(template, body)
		reqBody = []byte(rendered)
	} else {
		// 兜底：直接把 body 转为 JSON 字符串发送
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("custom: marshal body failed: %w", err)
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("custom: create http request failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// 如果配置了 Key 且格式为 "HeaderName:HeaderValue"，我们可以附加测试用 Header
	if cfg.Key != "" && strings.Contains(cfg.Key, ":") {
		parts := strings.SplitN(cfg.Key, ":", 2) //nolint:mnd
		httpReq.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	}

	client := httppool.NewClient(defaultHTTPClientTimeout)
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("custom: http request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("custom: http status %s", resp.Status)
	}

	return nil
}

// ValidateConfig 校验自定义配置
func (p *CustomPusher) ValidateConfig(cfg Config) error {
	if cfg.URL == "" {
		return errors.New("webhook URL is required")
	}
	if !strings.HasPrefix(cfg.URL, "http://") && !strings.HasPrefix(cfg.URL, "https://") {
		return errors.New("webhook URL must start with http:// or https://")
	}
	return nil
}
