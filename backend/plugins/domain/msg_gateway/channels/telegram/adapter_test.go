// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package telegram

import (
	"Wavelet/plugins/domain/msg_gateway/model/do"
	"context"
	"testing"
	"time"

	tele "gopkg.in/telebot.v4"
)

// TestBuildTeleSettingsLongPollWindow 回归：LongPoller.Timeout 是 time.Duration，
// telebot 以 int(timeout/time.Second) 下发给 getUpdates。写成裸整数会被解释为
// 纳秒，令 timeout=0，长轮询退化为对 Bot API 的空转轮询。
func TestBuildTeleSettingsLongPollWindow(t *testing.T) {
	pref := buildTeleSettings(do.ChannelConfig{
		Credentials: map[string]string{"bot_token": "token"},
		Extra:       map[string]string{"base_url": "https://tg.example.com/api/"},
	})

	poller, ok := pref.Poller.(*tele.LongPoller)
	if !ok {
		t.Fatalf("expected *tele.LongPoller, got %T", pref.Poller)
	}
	if got := int(poller.Timeout / time.Second); got != 10 {
		t.Errorf("getUpdates would receive timeout=%d seconds, want 10", got)
	}
	if pref.URL != "https://tg.example.com/api" {
		t.Errorf("base_url trailing slash should be trimmed, got %q", pref.URL)
	}
}

func TestHandleUpdate_DropsGroups(t *testing.T) {
	var got int
	a := &Adapter{onInbound: func(_ context.Context, _ do.InboundMessage) error {
		got++
		return nil
	}}
	a.handleTeleMessage(context.Background(), &tele.Message{
		ID:     1,
		Text:   "hi",
		Chat:   &tele.Chat{ID: -100, Type: tele.ChatGroup},
		Sender: &tele.User{ID: 1},
	})
	if got != 0 {
		t.Fatalf("group must be ignored")
	}
}

func TestHandleUpdate_PrivateText(t *testing.T) {
	var got do.InboundMessage
	a := &Adapter{
		cfg: do.ChannelConfig{ID: 7, Type: "telegram"},
		onInbound: func(_ context.Context, msg do.InboundMessage) error {
			got = msg
			return nil
		},
	}
	a.handleTeleMessage(context.Background(), &tele.Message{
		ID:     9,
		Text:   "hi",
		Chat:   &tele.Chat{ID: 42, Type: tele.ChatPrivate},
		Sender: &tele.User{ID: 42},
	})
	if got.Text != "hi" || got.PlatformUserID != "42" || got.ChannelID != 7 {
		t.Fatalf("%+v", got)
	}
}

func TestNew_RequiresToken(t *testing.T) {
	_, err := New(do.ChannelConfig{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
