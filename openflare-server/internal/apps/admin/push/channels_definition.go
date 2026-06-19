// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import "sync"

const (
	// KeyURL represents the URL field key
	KeyURL = "url"
	// KeyToken represents the Token field key
	KeyToken = "token"
	// KeyOther represents the Other field key
	KeyOther = "other"

	// TypeText represents standard text input type
	TypeText = "text"
	// TypePassword represents password input type
	TypePassword = "password"
	// TypeTextarea represents textarea input type
	TypeTextarea = "textarea"
)

// Field represents a form field configuration for a channel.
type Field struct {
	Key         string `json:"key"`         // unique key for the field (e.g. url, token, other)
	Label       string `json:"label"`       // human readable label (e.g. "Webhook 地址")
	Type        string `json:"type"`        // input type: "text" | "password" | "textarea"
	Required    bool   `json:"required"`    // whether this field is required
	Placeholder string `json:"placeholder"` // input placeholder
	Description string `json:"description"` // field explanation/help text
}

// Definition represents the metadata and form schema for a notification channel.
type Definition struct {
	Type        string  `json:"type"`        // channel type (e.g., custom, lark, email)
	Name        string  `json:"name"`        // display name
	Description string  `json:"description"` // short description
	Fields      []Field `json:"fields"`      // form fields
}

var (
	defMu       sync.RWMutex
	definitions = make(map[string]Definition)
)

// RegisterChannelDefinition registers a channel definition.
func RegisterChannelDefinition(def Definition) {
	defMu.Lock()
	defer defMu.Unlock()
	definitions[def.Type] = def
}

// ListDefinitions returns all registered channel definitions.
func ListDefinitions() []Definition {
	defMu.RLock()
	defer defMu.RUnlock()

	// We want a stable order: custom, lark, telegram, email
	order := []string{channelCustom, channelLark, channelTelegram, channelEmail}
	res := make([]Definition, 0, len(definitions))
	for _, t := range order {
		if d, ok := definitions[t]; ok {
			res = append(res, d)
		}
	}
	// Add any others
	for t, d := range definitions {
		found := false
		for _, o := range order {
			if o == t {
				found = true
				break
			}
		}
		if !found {
			res = append(res, d)
		}
	}
	return res
}

func init() {
	// Register custom webhook channel
	RegisterChannelDefinition(Definition{
		Type:        channelCustom,
		Name:        "自定义消息通道",
		Description: "使用自定义 HTTP POST 请求向外部 Webhook 发送数据。",
		Fields: []Field{
			{
				Key:         KeyURL,
				Label:       "请求地址",
				Type:        TypeText,
				Required:    true,
				Placeholder: "在此填写完整的请求地址，必须使用 HTTPS 协议",
				Description: "接口请求的完整 HTTPS URL，例如 https://api.example.com/webhook",
			},
			{
				Key:         KeyOther,
				Label:       "请求体 (JSON)",
				Type:        TypeTextarea,
				Required:    true,
				Placeholder: "在此输入请求体，支持模板变量，必须为合法的 JSON 格式",
				Description: "可使用的变量：$title, $description, $content, $url, $to。例如 {\"text\": \"$content\"}",
			},
		},
	})

	// Register Lark robot channel
	RegisterChannelDefinition(Definition{
		Type:        channelLark,
		Name:        "飞书群机器人",
		Description: "配置飞书群自定义机器人的 Webhook 接口投递。",
		Fields: []Field{
			{
				Key:         KeyURL,
				Label:       "Webhook 地址",
				Type:        TypeText,
				Required:    true,
				Placeholder: "https://open.feishu.cn/open-apis/bot/v2/hook/YOUR_TOKEN",
				Description: "从飞书群机器人设置中复制 of Webhook URL",
				// Note: using 'of' was in feishu.go, let's keep original wording or fix it
			},
			{
				Key:         KeyToken,
				Label:       "签名校验密钥 (Secret) (可选)",
				Type:        TypeText,
				Required:    false,
				Placeholder: "可选，若机器人启用了安全设置中的签名校验，请在此输入",
				Description: "飞书群机器人安全设置中的签名校验 Key",
			},
			{
				Key:         KeyOther,
				Label:       "自定义卡片 JSON 模版 (可选)",
				Type:        TypeTextarea,
				Required:    false,
				Placeholder: "可选，留空则默认使用系统内置的精美互动卡片",
				Description: "若填写，必须是合法的飞书卡片 JSON 格式",
			},
		},
	})

	// Register Telegram channel
	RegisterChannelDefinition(Definition{
		Type:        channelTelegram,
		Name:        "Telegram 机器人",
		Description: "配置 Telegram 机器人推送消息。",
		Fields: []Field{
			{
				Key:         KeyURL,
				Label:       "API 基础地址 (可选)",
				Type:        TypeText,
				Required:    false,
				Placeholder: "https://api.telegram.org",
				Description: "接口请求的 HTTPS 基础地址，留空默认为 https://api.telegram.org",
			},
			{
				Key:         KeyToken,
				Label:       "机器人 Token (Bot Token)",
				Type:        TypePassword,
				Required:    true,
				Placeholder: "在此输入 Telegram 机器人的 Bot Token",
				Description: "通过 BotFather 申请到的机器人 Access Token",
			},
			{
				Key:         KeyOther,
				Label:       "默认会话 ID (Chat ID) (可选)",
				Type:        TypeText,
				Required:    false,
				Placeholder: "例如 -100123456789 或 @channel_name",
				Description: "默认的消息接收 Chat ID。如果通知事件中未配置 targets，将推送到此 ID",
			},
		},
	})

	// Register Email channel
	RegisterChannelDefinition(Definition{
		Type:        channelEmail,
		Name:        "邮件推送通道",
		Description: "邮件推送通道直接使用系统全局 SMTP 设置进行发送，无需在此填写服务器配置。",
		Fields:      []Field{},
	})
}
