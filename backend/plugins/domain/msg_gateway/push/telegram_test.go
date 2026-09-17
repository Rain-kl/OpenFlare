// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"context"
	"testing"
)

func TestTelegramPusherValidation(t *testing.T) {
	pusher := &TelegramPusher{}

	err := pusher.ValidateConfig(Config{Secret: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"})
	if err != nil {
		t.Errorf("ValidateConfig failed: %v", err)
	}

	err = pusher.ValidateConfig(Config{})
	if err == nil {
		t.Errorf("expected error for empty config, got nil")
	}

	_, err = pusher.Send(context.Background(), Config{Secret: "123:token"}, "not-a-number", map[string]any{"title": "test"}, "", nil)
	if err == nil {
		t.Errorf("expected error for invalid chat_id, got nil")
	}
}
