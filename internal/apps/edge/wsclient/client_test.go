package wsclient

import (
	"testing"
	"time"
)

func TestPresetConfig(t *testing.T) {
	tests := []struct {
		preset    Preset
		headerKey string
		wsPath    string
	}{
		{preset: PresetAgent, headerKey: "X-Agent-Token", wsPath: "/api/v1/agent/ws"},
		{preset: PresetRelay, headerKey: "X-Agent-Token", wsPath: "/api/v1/relay/ws"},
		{preset: PresetFlared, headerKey: "X-Tunnel-Token", wsPath: "/api/v1/tunnel/ws"},
	}

	for _, tt := range tests {
		t.Run(tt.wsPath, func(t *testing.T) {
			if got := PresetHeaderKey(tt.preset); got != tt.headerKey {
				t.Fatalf("PresetHeaderKey() = %q, want %q", got, tt.headerKey)
			}
			if got := PresetWSPath(tt.preset); got != tt.wsPath {
				t.Fatalf("PresetWSPath() = %q, want %q", got, tt.wsPath)
			}
		})
	}
}

func TestClientURL(t *testing.T) {
	tests := []struct {
		name    string
		preset  Preset
		baseURL string
		want    string
	}{
		{
			name:    "agent https",
			preset:  PresetAgent,
			baseURL: "https://example.com",
			want:    "wss://example.com/api/v1/agent/ws",
		},
		{
			name:    "relay http with path prefix",
			preset:  PresetRelay,
			baseURL: "http://example.com/api",
			want:    "ws://example.com/api/api/v1/relay/ws",
		},
		{
			name:    "flared wss",
			preset:  PresetFlared,
			baseURL: "wss://edge.example.com",
			want:    "wss://edge.example.com/api/v1/tunnel/ws",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := New(tt.preset, tt.baseURL, "token", time.Second)
			if got := client.URL(); got != tt.want {
				t.Fatalf("URL() = %q, want %q", got, tt.want)
			}
		})
	}
}
