// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"slices"
	"sync"
)

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
		found := slices.Contains(order, t)
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
		Name:        "Custom notification channel",
		Description: "Send data to an external webhook using a custom HTTP POST request.",
		Fields: []Field{
			{
				Key:         KeyURL,
				Label:       "Request URL",
				Type:        TypeText,
				Required:    true,
				Placeholder: "Enter the complete request URL using HTTPS",
				Description: "Complete HTTPS endpoint URL, for example https://api.example.com/webhook",
			},
			{
				Key:         KeyOther,
				Label:       "Request body (JSON)",
				Type:        TypeTextarea,
				Required:    true,
				Placeholder: "Enter a valid JSON request body; template variables are supported",
				Description: `Available variables: $title, $description, $content, $url, $to. Example: {"text": "$content"}`,
			},
		},
	})

	// Register Lark robot channel
	RegisterChannelDefinition(Definition{
		Type:        channelLark,
		Name:        "Feishu group bot",
		Description: "Deliver notifications through a Feishu group bot webhook.",
		Fields: []Field{
			{
				Key:         KeyURL,
				Label:       "Webhook URL",
				Type:        TypeText,
				Required:    true,
				Placeholder: "https://open.feishu.cn/open-apis/bot/v2/hook/YOUR_TOKEN",
				Description: "Copy the webhook URL from the Feishu group bot settings",
				// Note: using 'of' was in feishu.go, let's keep original wording or fix it
			},
			{
				Key:         KeyToken,
				Label:       "Signing secret (optional)",
				Type:        TypeText,
				Required:    false,
				Placeholder: "Enter the secret if signature verification is enabled for the bot",
				Description: "Signature verification key from the Feishu group bot security settings",
			},
			{
				Key:         KeyOther,
				Label:       "Custom card JSON template (optional)",
				Type:        TypeTextarea,
				Required:    false,
				Placeholder: "Leave empty to use the built-in interactive card",
				Description: "If specified, this must be valid Feishu card JSON",
			},
		},
	})

	// Register Telegram channel
	RegisterChannelDefinition(Definition{
		Type:        channelTelegram,
		Name:        "Telegram bot",
		Description: "Configure Telegram bot notifications.",
		Fields: []Field{
			{
				Key:         KeyURL,
				Label:       "API base URL (optional)",
				Type:        TypeText,
				Required:    false,
				Placeholder: "https://api.telegram.org",
				Description: "HTTPS API base URL; defaults to https://api.telegram.org",
			},
			{
				Key:         KeyToken,
				Label:       "Bot token",
				Type:        TypePassword,
				Required:    true,
				Placeholder: "Enter the Telegram bot token",
				Description: "Bot access token issued by BotFather",
			},
			{
				Key:         KeyOther,
				Label:       "Default chat ID (optional)",
				Type:        TypeText,
				Required:    false,
				Placeholder: "For example -100123456789 or @channel_name",
				Description: "Default recipient chat ID, used when the notification event has no targets",
			},
		},
	})

	// Register Email channel
	RegisterChannelDefinition(Definition{
		Type:        channelEmail,
		Name:        "Email notification channel",
		Description: "Email notifications use the global SMTP settings; no server configuration is required here.",
		Fields:      []Field{},
	})
}
