// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package do

import (
	"Wavelet/plugins/domain/msg_gateway/consts"
	pkgpush "Wavelet/plugins/domain/msg_gateway/push"
	"time"
)

// SMTPConfig mirrors the system SMTP settings consumed by the push service.
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
}

// PushField represents a form field configuration for a channel.
type PushField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Placeholder string `json:"placeholder"`
	Description string `json:"description"`
}

// PushDefinition represents the metadata and form schema for a notification channel.
type PushDefinition struct {
	Type        string      `json:"type"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Fields      []PushField `json:"fields"`
}

// CreatePushChannelRequest is the create channel request payload.
type CreatePushChannelRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Type        string `json:"type" binding:"required"`
	Token       string `json:"token"`
	URL         string `json:"url"`
	Other       string `json:"other"`
	Enabled     bool   `json:"enabled"`
}

// UpdatePushChannelRequest is the update channel request payload.
type UpdatePushChannelRequest struct {
	Description string `json:"description"`
	Type        string `json:"type" binding:"required"`
	Token       string `json:"token"`
	URL         string `json:"url"`
	Other       string `json:"other"`
	Enabled     bool   `json:"enabled"`
}

// TestPushChannelRequest is the test channel request payload.
type TestPushChannelRequest struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Token  string `json:"token"`
	URL    string `json:"url"`
	Other  string `json:"other"`
	Target string `json:"target"`
}

// CustomPushRequest contains custom webhook parameters.
type CustomPushRequest struct {
	Title       string `json:"title" form:"title"`
	Description string `json:"description" form:"description"`
	Content     string `json:"content" form:"content"`
	URL         string `json:"url" form:"url"`
	To          string `json:"to" form:"to"`
	Token       string `json:"token" form:"token"`
}

// CreatePushEventRequest is the request body for creating a push event.
type CreatePushEventRequest struct {
	EventKey string   `json:"event_key"`
	TaskType string   `json:"task_type"`
	Channels []string `json:"channels"`
	Targets  []string `json:"targets"`
	Template string   `json:"template"`
	Enabled  bool     `json:"enabled"`
}

// UpdatePushEventRequest is the request body for updating a push event.
type UpdatePushEventRequest struct {
	Channels []string `json:"channels"`
	Targets  []string `json:"targets"`
	Template string   `json:"template" binding:"required"`
	Enabled  bool     `json:"enabled"`
}

// TestPushRequest is the request body for testing push config.
type TestPushRequest struct {
	Config pkgpush.Config `json:"config" binding:"required"`
	Target string         `json:"target"`
}

// NotificationMessage represents the structured notification message payload.
type NotificationMessage struct {
	Title   string         `json:"title"`
	Content string         `json:"content"`
	Level   string         `json:"level"`
	Ext     map[string]any `json:"ext,omitempty"`
}

// Flatten converts the structured NotificationMessage back to a flat map.
func (m NotificationMessage) Flatten() map[string]any {
	res := map[string]any{
		consts.KeyTitle:   m.Title,
		consts.KeyContent: m.Content,
		consts.KeyLevel:   m.Level,
	}
	for k, v := range m.Ext {
		res[k] = v
	}
	return res
}

// EventMetadata represents the metadata of a push notification event.
type EventMetadata struct {
	Key             string              `json:"key"`
	Name            string              `json:"name"`
	DefaultTemplate NotificationMessage `json:"default_template"`
	Description     string              `json:"description"`
}

// SendPayload is the async push dispatch payload consumed by the notification worker.
type SendPayload struct {
	EventKey string              `json:"event_key"`
	Config   pkgpush.Config      `json:"config"`
	Target   string              `json:"target"`
	Body     NotificationMessage `json:"body"`
	Template string              `json:"template"`
}

// PushHistoryListFilter filters push history pagination queries.
type PushHistoryListFilter struct {
	EventKey  string
	Channel   string
	Status    string
	StartTime *time.Time
	EndTime   *time.Time
	Page      int
	PageSize  int
}

// PushNotificationEvent defines the payload for eventbus notification trigger.
type PushNotificationEvent struct {
	UserID   uint64         `json:"user_id"`
	Channel  string         `json:"channel"`
	Title    string         `json:"title"`
	Content  string         `json:"content"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

//nolint:goconst,dupl // Static push channel form definitions table
var defaultPushDefinitions = []PushDefinition{
	{
		Type:        consts.ChannelCustom,
		Name:        "自定义消息通道",
		Description: "使用自定义 HTTP POST 请求向外部 Webhook 发送数据。",
		Fields: []PushField{
			{
				Key:         consts.KeyURL,
				Label:       "请求地址",
				Type:        consts.TypeText,
				Required:    true,
				Placeholder: "在此填写完整的请求地址，必须使用 HTTPS 协议",
				Description: "接口请求的完整 HTTPS URL，例如 https://api.example.com/webhook",
			},
			{
				Key:         consts.KeyOther,
				Label:       "请求体 (JSON)",
				Type:        consts.TypeTextarea,
				Required:    true,
				Placeholder: "在此输入请求体，支持模板变量，必须为合法的 JSON 格式",
				Description: "可使用的变量：$title, $description, $content, $url, $to。例如 {\"text\": \"$content\"}",
			},
		},
	},
	{
		Type:        consts.ChannelLark,
		Name:        "飞书群机器人",
		Description: "配置飞书群自定义机器人的 Webhook 接口投递。",
		Fields: []PushField{
			{
				Key:         consts.KeyURL,
				Label:       "Webhook 地址",
				Type:        consts.TypeText,
				Required:    true,
				Placeholder: "https://open.feishu.cn/open-apis/bot/v2/hook/YOUR_TOKEN",
				Description: "从飞书群机器人设置中复制的 Webhook URL",
			},
			{
				Key:         consts.KeyToken,
				Label:       "签名校验密钥 (Secret) (可选)",
				Type:        consts.TypeText,
				Required:    false,
				Placeholder: "可选，若机器人启用了安全设置中的签名校验，请在此输入",
				Description: "飞书群机器人安全设置中的签名校验 Key",
			},
			{
				Key:         consts.KeyOther,
				Label:       "自定义卡片 JSON 模版 (可选)",
				Type:        consts.TypeTextarea,
				Required:    false,
				Placeholder: "可选，留空则默认使用系统内置的精美互动卡片",
				Description: "若填写，必须是合法的飞书卡片 JSON 格式",
			},
		},
	},
	{
		Type:        consts.ChannelDingTalk,
		Name:        "钉钉群机器人",
		Description: "配置钉钉群自定义机器人的 Webhook 接口投递。",
		Fields: []PushField{
			{
				Key:         consts.KeyURL,
				Label:       "Webhook 地址",
				Type:        consts.TypeText,
				Required:    true,
				Placeholder: "https://oapi.dingtalk.com/robot/send?access_token=YOUR_TOKEN",
				Description: "从钉钉群机器人设置中获取的完整 Webhook URL",
			},
			{
				Key:         consts.KeyToken,
				Label:       "加签密钥 (Secret) (可选)",
				Type:        consts.TypeText,
				Required:    false,
				Placeholder: "可选，若机器人启用了安全设置中的加签校验，请在此输入 SEC 开头的密钥",
				Description: "钉钉群机器人安全设置中的加签 Secret",
			},
		},
	},
	{
		Type:        consts.ChannelTelegram,
		Name:        "Telegram 机器人",
		Description: "配置 Telegram 机器人推送消息。",
		Fields: []PushField{
			{
				Key:         consts.KeyURL,
				Label:       "API 基础地址 (可选)",
				Type:        consts.TypeText,
				Required:    false,
				Placeholder: "https://api.telegram.org",
				Description: "接口请求的 HTTPS 基础地址，留空默认为 https://api.telegram.org",
			},
			{
				Key:         consts.KeyToken,
				Label:       "机器人 Token (Bot Token)",
				Type:        consts.TypePassword,
				Required:    true,
				Placeholder: "在此输入 Telegram 机器人的 Bot Token",
				Description: "通过 BotFather 申请到的机器人 Access Token",
			},
			{
				Key:         consts.KeyOther,
				Label:       "默认会话 ID (Chat ID) (可选)",
				Type:        consts.TypeText,
				Required:    false,
				Placeholder: "例如 -100123456789 或 @channel_name",
				Description: "默认的消息接收 Chat ID。如果通知事件中未配置 targets，将推送到此 ID",
			},
		},
	},
	{
		Type:        consts.ChannelBark,
		Name:        "Bark (iOS 推送)",
		Description: "配置 Bark 推送通知至 iPhone / iPad 客户端。",
		Fields: []PushField{
			{
				Key:         consts.KeyToken,
				Label:       "设备 Key (Device Key)",
				Type:        consts.TypeText,
				Required:    true,
				Placeholder: "Bark App 首页显示的 Device Key",
				Description: "从 Bark App 复制的设备专属 Key",
			},
			{
				Key:         consts.KeyURL,
				Label:       "Bark 服务器地址 (可选)",
				Type:        consts.TypeText,
				Required:    false,
				Placeholder: "https://api.day.app",
				Description: "Bark 服务器地址，留空默认使用官方公共服务器 https://api.day.app",
			},
			{
				Key:         consts.KeyOther,
				Label:       "额外配置 JSON (可选)",
				Type:        consts.TypeTextarea,
				Required:    false,
				Placeholder: "{\"group\": \"Wavelet\", \"sound\": \"minuet\", \"icon\": \"https://...\"}",
				Description: "可选的 JSON 配置，支持 group (分组)、sound (铃声)、icon (自定义图标)",
			},
		},
	},
	{
		Type:        consts.ChannelDiscord,
		Name:        "Discord 频道",
		Description: "配置 Discord 频道的 Incoming Webhook 消息推送。",
		Fields: []PushField{
			{
				Key:         consts.KeyURL,
				Label:       "Webhook 地址",
				Type:        consts.TypeText,
				Required:    true,
				Placeholder: "https://discord.com/api/webhooks/...",
				Description: "从 Discord 频道集成设置中复制的 Webhook URL",
			},
		},
	},
	{
		Type:        consts.ChannelSlack,
		Name:        "Slack 频道",
		Description: "配置 Slack 频道的 Incoming Webhook 消息推送。",
		Fields: []PushField{
			{
				Key:         consts.KeyURL,
				Label:       "Webhook 地址",
				Type:        consts.TypeText,
				Required:    true,
				Placeholder: "https://hooks.slack.com/services/...",
				Description: "从 Slack 应用配置中复制的 Incoming Webhook URL",
			},
		},
	},
	{
		Type:        consts.ChannelPushover,
		Name:        "Pushover 推送",
		Description: "配置 Pushover 即时推送到手机/桌面客户端。",
		Fields: []PushField{
			{
				Key:         consts.KeyToken,
				Label:       "应用 Token (App Token)",
				Type:        consts.TypePassword,
				Required:    true,
				Placeholder: "Pushover 创建应用生成的 API Token / Key",
				Description: "从 Pushover 控制台创建的 Application API Token",
			},
			{
				Key:         consts.KeyURL,
				Label:       "用户 Key (User Key)",
				Type:        consts.TypeText,
				Required:    true,
				Placeholder: "Pushover 账号主页的 User Key",
				Description: "Pushover 个人账号的 User Key",
			},
		},
	},
	{
		Type:        consts.ChannelEmail,
		Name:        "邮件推送通道",
		Description: "邮件推送通道直接使用系统全局 SMTP 设置进行发送，无需在此填写服务器配置。",
		Fields:      []PushField{},
	},
}

// ListPushDefinitions returns all registered channel definitions.
func ListPushDefinitions() []PushDefinition {
	res := make([]PushDefinition, len(defaultPushDefinitions))
	copy(res, defaultPushDefinitions)
	return res
}
