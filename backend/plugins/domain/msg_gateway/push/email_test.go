// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"context"
	"testing"
)

func TestEmailPusherValidateConfig(t *testing.T) {
	pusher := &EmailPusher{}

	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "empty url",
			cfg:     Config{URL: "", Key: "user", Secret: "pass"},
			wantErr: true,
		},
		{
			name:    "empty key",
			cfg:     Config{URL: "smtp.example.com:587", Key: "", Secret: "pass"},
			wantErr: true,
		},
		{
			name:    "empty secret",
			cfg:     Config{URL: "smtp.example.com:587", Key: "user", Secret: ""},
			wantErr: true,
		},
		{
			name:    "valid config",
			cfg:     Config{URL: "smtp.example.com:587", Key: "user", Secret: "pass"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pusher.ValidateConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEmailPusherSendValidation(t *testing.T) {
	pusher := &EmailPusher{}

	// Missing target
	_, err := pusher.Send(context.Background(), Config{URL: "127.0.0.1:25", Key: "u", Secret: "p"}, "", map[string]any{"title": "hi"}, "", nil)
	if err == nil {
		t.Errorf("expected error for empty target, got nil")
	}

	// Missing config
	_, err = pusher.Send(context.Background(), Config{}, "test@example.com", map[string]any{"title": "hi"}, "", nil)
	if err == nil {
		t.Errorf("expected error for empty config, got nil")
	}
}
