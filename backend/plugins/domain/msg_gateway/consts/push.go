// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package consts

// Push notification channel type constants.
const (
	TypeCustom      = "custom"
	TypeEmail       = "email"
	TypeTelegram    = "telegram"
	ChannelCustom   = "custom"
	ChannelEmail    = "email"
	ChannelLark     = "lark"
	ChannelDingTalk = "dingtalk"
	ChannelTelegram = "telegram"
	ChannelBark     = "bark"
	ChannelDiscord  = "discord"
	ChannelSlack    = "slack"
	ChannelPushover = "pushover"
)

// Push message template and payload keys.
const (
	DefaultLevelInfo = "INFO"
	KeyTitle         = "title"
	KeyContent       = "content"
	KeyLevel         = "level"

	KeyURL   = "url"
	KeyToken = "token"
	KeyOther = "other"

	TypeText     = "text"
	TypePassword = "password"
	TypeTextarea = "textarea"
)

// Push task identifier constants.
const (
	TaskPushNotification     = "msg_gateway:push_notification"
	SendNotificationTask     = "push:send"
	TaskTypeSendNotification = "send_notification"
)
