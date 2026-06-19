// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package push 提供解耦的、无外部业务依赖 of 通知推送底层实现
package push

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	defaultTitle             = "系统通知"
	levelInfo                = "INFO"
	defaultHTTPClientTimeout = 10 * time.Second
)

// Config 基础通知渠道配置
type Config struct {
	Channel string         `json:"channel"`          // 渠道名称，例如 "lark", "custom", "email" 等，唯一标识
	URL     string         `json:"url,omitempty"`    // Webhook 地址或 SMTP 地址
	Secret  string         `json:"secret,omitempty"` // 签名密钥或 SMTP 密码/Token
	Key     string         `json:"key,omitempty"`    // AppID 或 SMTP 用户名
	Ext     map[string]any `json:"ext,omitempty"`    // 预留拓展 JSON 配置
}

// Pusher 通知推送渠道接口
type Pusher interface {
	// Send 发送通知消息
	// target: 发送目标 (如邮箱地址或特定用户标识；若为 bot 机器人此项为空)
	// body: 消息体数据 (含默认字段如 title, content, level)
	// template: 消息卡片/模板 JSON (可选)
	// ext: 预留的单次发送拓展数据
	Send(ctx context.Context, cfg Config, target string, body map[string]any, template string, ext map[string]any) error

	// ValidateConfig 校验渠道配置合法性
	ValidateConfig(cfg Config) error
}

var (
	pushersMu sync.RWMutex
	pushers   = make(map[string]Pusher)
)

// Register 注册一个推送渠道实现
func Register(channelType string, pusher Pusher) {
	pushersMu.Lock()
	defer pushersMu.Unlock()
	if pusher == nil {
		panic("push: Register pusher is nil")
	}
	pushers[channelType] = pusher
}

// GetPusher 获取指定类型的推送渠道实现
func GetPusher(channelType string) (Pusher, error) {
	pushersMu.RLock()
	defer pushersMu.RUnlock()
	pusher, ok := pushers[channelType]
	if !ok {
		return nil, fmt.Errorf("push: unknown channel type %q", channelType)
	}
	return pusher, nil
}
