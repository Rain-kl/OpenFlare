// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	pkgmail "Wavelet/pkg/mail"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
)

func init() {
	Register("email", &EmailPusher{})
}

// EmailPusher 基于 pkg/mail 的 SMTP 邮件推送实现
type EmailPusher struct{}

// Send 发送邮件
func (p *EmailPusher) Send(ctx context.Context, cfg Config, target string, body map[string]any, _ string, ext map[string]any) (string, error) {
	if cfg.URL == "" || cfg.Key == "" || cfg.Secret == "" {
		return "", errors.New("email: SMTP configuration (url, key, secret) is incomplete")
	}
	if target == "" {
		return "", errors.New("email: target email address is required")
	}

	title := bodyTitle(body)
	content := bodyContent(body, "<p><b>%s</b>: %v</p>", "")

	fromName := "System Notification"
	if ext != nil {
		if fn, ok := ext["from_name"].(string); ok && fn != "" {
			fromName = fn
		}
	}

	htmlBody := fmt.Sprintf(`<html><body><h2>%s</h2><div>%s</div></body></html>`, title, content)

	host, portStr, err := net.SplitHostPort(cfg.URL)
	port := 25
	if err != nil {
		host = cfg.URL
	} else if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
		port = p
	}

	mailCfg := pkgmail.Config{
		Host:     host,
		Port:     port,
		Username: cfg.Key,
		Password: cfg.Secret,
		FromName: fromName,
	}

	if err := pkgmail.SendMail(ctx, mailCfg, target, title, htmlBody); err != nil {
		return "", fmt.Errorf("email: send smtp mail failed: %w", err)
	}

	return "", nil
}

// ValidateConfig 校验邮件 SMTP 配置
func (p *EmailPusher) ValidateConfig(cfg Config) error {
	if cfg.URL == "" {
		return errors.New("SMTP host:port is required")
	}
	if cfg.Key == "" {
		return errors.New("SMTP username is required")
	}
	if cfg.Secret == "" {
		return errors.New("SMTP password is required")
	}
	return nil
}
