// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDingTalkPusher(t *testing.T) {
	pusher, err := GetPusher("dingtalk")
	if err != nil {
		t.Fatalf("failed to get dingtalk pusher: %v", err)
	}

	err = pusher.ValidateConfig(Config{URL: "https://oapi.dingtalk.com/robot/send?access_token=test"})
	if err != nil {
		t.Errorf("ValidateConfig failed: %v", err)
	}

	err = pusher.ValidateConfig(Config{})
	if err == nil {
		t.Errorf("expected error for empty config, got nil")
	}
}

func TestBarkPusher(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":200,"message":"success"}`))
	}))
	defer server.Close()

	pusher, err := GetPusher("bark")
	if err != nil {
		t.Fatalf("failed to get bark pusher: %v", err)
	}

	err = pusher.ValidateConfig(Config{Key: "device_key_123"})
	if err != nil {
		t.Errorf("ValidateConfig failed: %v", err)
	}

	_, err = pusher.Send(context.Background(), Config{
		URL: server.URL,
		Key: "device_key_123",
	}, "", map[string]any{
		"title":   "Alert",
		"content": "Bark notification",
	}, "", nil)
	if err != nil {
		t.Errorf("Send failed: %v", err)
	}
}

func TestDiscordPusher(t *testing.T) {
	pusher, err := GetPusher("discord")
	if err != nil {
		t.Fatalf("failed to get discord pusher: %v", err)
	}

	err = pusher.ValidateConfig(Config{Key: "bot_token_123"})
	if err != nil {
		t.Errorf("ValidateConfig failed: %v", err)
	}

	err = pusher.ValidateConfig(Config{})
	if err == nil {
		t.Errorf("expected error for empty config, got nil")
	}
}

func TestSlackPusher(t *testing.T) {
	pusher, err := GetPusher("slack")
	if err != nil {
		t.Fatalf("failed to get slack pusher: %v", err)
	}

	err = pusher.ValidateConfig(Config{Key: "xoxb-123456"})
	if err != nil {
		t.Errorf("ValidateConfig failed: %v", err)
	}

	err = pusher.ValidateConfig(Config{})
	if err == nil {
		t.Errorf("expected error for empty config, got nil")
	}
}

func TestPushoverPusher(t *testing.T) {
	pusher, err := GetPusher("pushover")
	if err != nil {
		t.Fatalf("failed to get pushover pusher: %v", err)
	}

	err = pusher.ValidateConfig(Config{Key: "app_token_123"})
	if err != nil {
		t.Errorf("ValidateConfig failed: %v", err)
	}

	err = pusher.ValidateConfig(Config{})
	if err == nil {
		t.Errorf("expected error for empty config, got nil")
	}
}

func TestLarkPusher(t *testing.T) {
	pusher, err := GetPusher("lark")
	if err != nil {
		t.Fatalf("failed to get lark pusher: %v", err)
	}

	err = pusher.ValidateConfig(Config{URL: "https://open.feishu.cn/open-apis/bot/v2/hook/xxx"})
	if err != nil {
		t.Errorf("ValidateConfig failed: %v", err)
	}

	err = pusher.ValidateConfig(Config{})
	if err == nil {
		t.Errorf("expected error for empty config, got nil")
	}
}
