// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/pkg/httppool"
)

func init() {
	Register("lark", &LarkPusher{})
}

const (
	msgTypeInteractive = "interactive"
)

// LarkPusher 飞书 Webhook 机器人推送实现
type LarkPusher struct{}

type larkTextContent struct {
	Text string `json:"text"`
}

type larkCardHeaderTitle struct {
	Content string `json:"content"`
	Tag     string `json:"tag"`
}

type larkCardHeader struct {
	Template string              `json:"template"` // "blue", "orange", "red" etc.
	Title    larkCardHeaderTitle `json:"title"`
}

type larkCardElementText struct {
	Content string `json:"content"`
	Tag     string `json:"tag"` // "lark_md"
}

type larkCardElement struct {
	Tag  string              `json:"tag"` // "div"
	Text larkCardElementText `json:"text"`
}

type larkCardContent struct {
	Header   larkCardHeader    `json:"header"`
	Elements []larkCardElement `json:"elements"`
}

type larkMessageRequest struct {
	MessageType string           `json:"msg_type"`
	Timestamp   string           `json:"timestamp,omitempty"`
	Sign        string           `json:"sign,omitempty"`
	Content     larkTextContent  `json:"content,omitempty"`
	Card        *larkCardContent `json:"card,omitempty"`
}

type larkMessageResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// Send 执行飞书消息发送
//
//nolint:nestif,cyclop
func (p *LarkPusher) Send(ctx context.Context, cfg Config, _ string, body map[string]any, template string, _ map[string]any) error {
	if cfg.URL == "" {
		return errors.New("lark: URL is required")
	}

	var req larkMessageRequest

	// 1. 如果有自定义模板，我们尝试进行解析
	if template != "" {
		rendered := ParseTemplate(template, body)

		// 尝试解析原生的 Lark Card
		var customCard larkCardContent
		var rawMap map[string]any
		_ = json.Unmarshal([]byte(rendered), &rawMap)

		if rawMap != nil && rawMap["elements"] != nil {
			// 如果包含 elements 字段，说明是用户定制的原生飞书卡片 JSON
			if err := json.Unmarshal([]byte(rendered), &customCard); err == nil {
				req.MessageType = msgTypeInteractive
				req.Card = &customCard
			} else {
				req.MessageType = "text"
				req.Content.Text = rendered
			}
		} else {
			// 说明配置的是系统统一通知消息 of JSON 模板：{"title": "...", "content": "...", "level": "..."}
			type larkNotificationMessage struct {
				Title   string `json:"title"`
				Content string `json:"content"`
				Level   string `json:"level"`
			}
			var msg larkNotificationMessage
			if err := json.Unmarshal([]byte(rendered), &msg); err == nil && (msg.Title != "" || msg.Content != "") {
				title := msg.Title
				if title == "" {
					title = defaultTitle
				}
				content := msg.Content
				level := strings.ToUpper(msg.Level)
				if level == "" {
					level = levelInfo
				}

				headerColor := "blue"
				switch level {
				case "IMPORTANT":
					headerColor = "orange"
				case "CRITICAL":
					headerColor = "red"
				}

				req.MessageType = msgTypeInteractive
				req.Card = &larkCardContent{
					Header: larkCardHeader{
						Template: headerColor,
						Title: larkCardHeaderTitle{
							Content: title,
							Tag:     "plain_text",
						},
					},
					Elements: []larkCardElement{
						{
							Tag: "div",
							Text: larkCardElementText{
								Content: content,
								Tag:     "lark_md",
							},
						},
					},
				}
			} else {
				// 兜底：如果无法按 JSON 解析出结构化字段，当做普通文本发送
				req.MessageType = "text"
				req.Content.Text = rendered
			}
		}
	} else {
		// 2. 如果无模板，默认生成一个精美的飞书互动卡片
		title := defaultTitle
		if t, ok := body["title"].(string); ok && t != "" {
			title = t
		}

		content := ""
		if c, ok := body["content"].(string); ok && c != "" {
			content = c
		} else {
			// 兜底：如果连 content 都没有，把 body 里的所有值拼成 markdown
			var parts []string
			for k, v := range body {
				parts = append(parts, fmt.Sprintf("**%s**: %v", k, v))
			}
			content = strings.Join(parts, "\n")
		}

		level := levelInfo
		if l, ok := body["level"].(string); ok && l != "" {
			level = strings.ToUpper(l)
		}

		// 根据级别确定飞书卡片头部的背景色模板
		headerColor := "blue"
		switch level {
		case "IMPORTANT":
			headerColor = "orange"
		case "CRITICAL":
			headerColor = "red"
		}

		req.MessageType = msgTypeInteractive
		req.Card = &larkCardContent{
			Header: larkCardHeader{
				Template: headerColor,
				Title: larkCardHeaderTitle{
					Content: title,
					Tag:     "plain_text",
				},
			},
			Elements: []larkCardElement{
				{
					Tag: "div",
					Text: larkCardElementText{
						Content: content,
						Tag:     "lark_md",
					},
				},
			},
		}
	}

	// 3. 计算签名 (如果配置了 secret)
	if cfg.Secret != "" {
		timestamp := time.Now().Unix()
		sign, err := larkSign(cfg.Secret, timestamp)
		if err != nil {
			return fmt.Errorf("lark: sign failed: %w", err)
		}
		req.Timestamp = strconv.FormatInt(timestamp, 10)
		req.Sign = sign
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("lark: marshal request failed: %w", err)
	}

	// 4. 发送 POST 请求
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("lark: create http request failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := httppool.NewClient(defaultHTTPClientTimeout)
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("lark: http request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("lark: http status %s", resp.Status)
	}

	var res larkMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return fmt.Errorf("lark: decode response failed: %w", err)
	}

	if res.Code != 0 {
		return fmt.Errorf("lark: send message failed, code %d: %s", res.Code, res.Msg)
	}

	return nil
}

// ValidateConfig 校验飞书配置
func (p *LarkPusher) ValidateConfig(cfg Config) error {
	if cfg.URL == "" {
		return errors.New("webhook URL is required")
	}
	if !strings.HasPrefix(cfg.URL, "http://") && !strings.HasPrefix(cfg.URL, "https://") {
		return errors.New("webhook URL must start with http:// or https://")
	}
	return nil
}

func larkSign(secret string, timestamp int64) (string, error) {
	stringToSign := fmt.Sprintf("%v", timestamp) + "\n" + secret
	h := hmac.New(sha256.New, []byte(stringToSign))
	_, err := h.Write(nil)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}
