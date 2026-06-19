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
	Register("telegram", &TelegramPusher{})
}

// TelegramPusher Telegram 机器人推送实现
type TelegramPusher struct{}

type telegramMessageRequest struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

type telegramErrorResponse struct {
	Ok          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}

// Send 执行 Telegram 消息发送
//
//nolint:cyclop
func (p *TelegramPusher) Send(ctx context.Context, cfg Config, target string, body map[string]any, template string, _ map[string]any) error {
	if cfg.Secret == "" {
		return errors.New("telegram: Bot Token (Secret) is required")
	}

	chatID := target
	if chatID == "" {
		chatID = cfg.Key // Use default chat ID (Key) if target is blank
	}
	if chatID == "" {
		return errors.New("telegram: chat_id (target or default Key) is required")
	}

	baseURL := cfg.URL
	if baseURL == "" {
		baseURL = "https://api.telegram.org"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	title := defaultTitle
	if t, ok := body["title"].(string); ok && t != "" {
		title = t
	}
	content := ""
	if c, ok := body["content"].(string); ok && c != "" {
		content = c
	} else {
		var parts []string
		for k, v := range body {
			parts = append(parts, fmt.Sprintf("<b>%s</b>: %v", k, v))
		}
		content = strings.Join(parts, "\n")
	}
	level := levelInfo
	if l, ok := body["level"].(string); ok && l != "" {
		level = strings.ToUpper(l)
	}

	var text string
	if template != "" {
		text = ParseTemplate(template, body)
	} else {
		text = fmt.Sprintf("<b>[%s] %s</b>\n\n%s", escapeHTML(level), escapeHTML(title), escapeHTML(content))
	}

	// Try sending with HTML parse mode
	err := p.sendMessage(ctx, baseURL, cfg.Secret, chatID, text, "HTML")
	if err != nil {
		// Fallback: send as plain text without parse mode
		plainText := text
		if template == "" {
			plainText = fmt.Sprintf("[%s] %s\n\n%s", level, title, content)
		}
		fallbackErr := p.sendMessage(ctx, baseURL, cfg.Secret, chatID, plainText, "")
		if fallbackErr != nil {
			return fmt.Errorf("telegram: send message failed (fallback also failed): %w (original HTML error: %v)", fallbackErr, err)
		}
	}

	return nil
}

// ValidateConfig 校验 Telegram 配置
func (p *TelegramPusher) ValidateConfig(cfg Config) error {
	if cfg.Secret == "" {
		return errors.New("bot Token (Secret) is required")
	}
	if cfg.URL != "" {
		if !strings.HasPrefix(cfg.URL, "http://") && !strings.HasPrefix(cfg.URL, "https://") {
			return errors.New("API base URL must start with http:// or https://")
		}
	}
	return nil
}

func (p *TelegramPusher) sendMessage(ctx context.Context, baseURL, token, chatID, text, parseMode string) error {
	apiURL := fmt.Sprintf("%s/bot%s/sendMessage", baseURL, token)

	reqPayload := telegramMessageRequest{
		ChatID:    chatID,
		Text:      text,
		ParseMode: parseMode,
	}

	jsonData, err := json.Marshal(reqPayload)
	if err != nil {
		return fmt.Errorf("marshal request failed: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("create http request failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := httppool.NewClient(defaultHTTPClientTimeout)
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var errRes telegramErrorResponse
		if decodeErr := json.NewDecoder(resp.Body).Decode(&errRes); decodeErr == nil {
			return fmt.Errorf("http status %d: %s", resp.StatusCode, errRes.Description)
		}
		return fmt.Errorf("http status %s", resp.Status)
	}

	return nil
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
