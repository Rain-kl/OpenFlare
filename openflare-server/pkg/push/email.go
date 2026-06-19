// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
)

func init() {
	Register("email", &EmailPusher{})
}

// EmailPusher 极简 SMTP 邮件推送实现 (静态、解耦)
type EmailPusher struct{}

// Send 发送邮件
func (p *EmailPusher) Send(ctx context.Context, cfg Config, target string, body map[string]any, _ string, ext map[string]any) error {
	if cfg.URL == "" || cfg.Key == "" || cfg.Secret == "" {
		return errors.New("email: SMTP configuration (url, key, secret) is incomplete")
	}
	if target == "" {
		return errors.New("email: target email address is required")
	}

	title := defaultTitle
	if t, ok := body["title"].(string); ok && t != "" {
		title = t
	}

	content := ""
	if c, ok := body["content"].(string); ok && c != "" {
		content = c
	} else {
		// 自动格式化 map
		var parts []string
		for k, v := range body {
			parts = append(parts, fmt.Sprintf("<p><b>%s</b>: %v</p>", k, v))
		}
		content = strings.Join(parts, "")
	}

	// 邮件头和体
	from := cfg.Key
	to := target

	// 如果 ext 中指定了 from_name，我们在 From 头部包含它
	fromName := "System Notification"
	if ext != nil {
		if fn, ok := ext["from_name"].(string); ok && fn != "" {
			fromName = fn
		}
	}

	subjectHeader := fmt.Sprintf("Subject: %s\r\n", title)
	fromHeader := fmt.Sprintf("From: %s <%s>\r\n", fromName, from)
	toHeader := fmt.Sprintf("To: %s\r\n", to)
	mimeHeader := "MIME-version: 1.0;\r\nContent-Type: text/html; charset=\"UTF-8\";\r\n\r\n"

	// 拼装完整的邮件报文
	// 简单的 HTML 正文渲染
	htmlBody := fmt.Sprintf(`<html><body><h2>%s</h2><div>%s</div></body></html>`, title, content)
	msg := []byte(fromHeader + toHeader + subjectHeader + mimeHeader + htmlBody + "\r\n")

	// 解析 Host 和 Port
	host, port, err := net.SplitHostPort(cfg.URL)
	if err != nil {
		host = cfg.URL
		port = "25" // 默认 SMTP 端口
	}

	auth := smtp.PlainAuth("", cfg.Key, cfg.Secret, host)

	// 异步超时处理
	errChan := make(chan error, 1)
	go func() {
		errChan <- smtp.SendMail(host+":"+port, auth, from, []string{to}, msg)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("email: send smtp mail failed: %w", err)
		}
	}

	return nil
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
