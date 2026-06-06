package controller

import (
	"testing"
)

func TestValidateOpenRestyOption(t *testing.T) {
	testCases := []struct {
		name    string
		key     string
		value   string
		wantErr bool
	}{
		{name: "default server status valid 421", key: "OpenRestyDefaultServerReturnStatus", value: "421"},
		{name: "default server status valid 200", key: "OpenRestyDefaultServerReturnStatus", value: "200"},
		{name: "default server status invalid 99", key: "OpenRestyDefaultServerReturnStatus", value: "99", wantErr: true},
		{name: "default server status invalid 1000", key: "OpenRestyDefaultServerReturnStatus", value: "1000", wantErr: true},
		{name: "default server status invalid abc", key: "OpenRestyDefaultServerReturnStatus", value: "abc", wantErr: true},
		{name: "worker processes auto", key: "OpenRestyWorkerProcesses", value: "auto"},
		{name: "worker processes number", key: "OpenRestyWorkerProcesses", value: "8"},
		{name: "worker processes invalid", key: "OpenRestyWorkerProcesses", value: "0", wantErr: true},
		{name: "events use empty", key: "OpenRestyEventsUse", value: ""},
		{name: "events use invalid", key: "OpenRestyEventsUse", value: "io_uring", wantErr: true},
		{name: "resolvers valid", key: "OpenRestyResolvers", value: "1.1.1.1 8.8.8.8"},
		{name: "resolvers invalid", key: "OpenRestyResolvers", value: "1.1.1.1; 8.8.8.8", wantErr: true},
		{name: "proxy buffers valid", key: "OpenRestyProxyBuffers", value: "16 16k"},
		{name: "proxy buffers invalid", key: "OpenRestyProxyBuffers", value: "16x16k", wantErr: true},
		{name: "cache max size valid", key: "OpenRestyCacheMaxSize", value: "2g"},
		{name: "cache max size invalid", key: "OpenRestyCacheMaxSize", value: "2gb", wantErr: true},
		{name: "client max body size valid", key: "OpenRestyClientMaxBodySize", value: "64m"},
		{name: "client max body size invalid", key: "OpenRestyClientMaxBodySize", value: "64mb", wantErr: true},
		{name: "large client header buffers valid", key: "OpenRestyLargeClientHeaderBuffers", value: "4 16k"},
		{name: "large client header buffers invalid", key: "OpenRestyLargeClientHeaderBuffers", value: "4x16k", wantErr: true},
		{name: "proxy request buffering valid", key: "OpenRestyProxyRequestBufferingEnabled", value: "true"},
		{name: "proxy request buffering invalid", key: "OpenRestyProxyRequestBufferingEnabled", value: "on", wantErr: true},
		{name: "websocket valid", key: "OpenRestyWebsocketEnabled", value: "false"},
		{name: "websocket invalid", key: "OpenRestyWebsocketEnabled", value: "off", wantErr: true},
		{name: "cache inactive valid", key: "OpenRestyCacheInactive", value: "30m"},
		{name: "cache inactive invalid", key: "OpenRestyCacheInactive", value: "30", wantErr: true},
		{name: "cache use stale valid", key: "OpenRestyCacheUseStale", value: "error timeout http_500"},
		{name: "cache use stale invalid", key: "OpenRestyCacheUseStale", value: "error whatever", wantErr: true},
		{name: "gzip level valid", key: "OpenRestyGzipCompLevel", value: "9"},
		{name: "gzip level invalid", key: "OpenRestyGzipCompLevel", value: "10", wantErr: true},
	}

	for _, testCase := range testCases {
		err := validateOpenRestyOption(testCase.key, testCase.value)
		if testCase.wantErr && err == nil {
			t.Fatalf("%s: expected error", testCase.name)
		}
		if !testCase.wantErr && err != nil {
			t.Fatalf("%s: unexpected error: %v", testCase.name, err)
		}
	}
}

func TestValidateAgentOption(t *testing.T) {
	if err := validateAgentOption("AgentWebsocketUpgradeEnabled", "true"); err != nil {
		t.Fatalf("expected websocket upgrade option to accept true: %v", err)
	}
	if err := validateAgentOption("AgentWebsocketUpgradeEnabled", "false"); err != nil {
		t.Fatalf("expected websocket upgrade option to accept false: %v", err)
	}
	if err := validateAgentOption("AgentWebsocketUpgradeEnabled", "on"); err == nil {
		t.Fatal("expected websocket upgrade option to reject non-boolean value")
	}
}

func TestValidateUptimeKumaOption(t *testing.T) {
	state := map[string]string{
		"UptimeKumaUrl":      "http://localhost:3001",
		"UptimeKumaUsername": "admin",
		"UptimeKumaPassword": "password",
	}

	testCases := []struct {
		name    string
		key     string
		value   string
		wantErr bool
	}{
		{name: "enabled true", key: "UptimeKumaEnabled", value: "true"},
		{name: "enabled false", key: "UptimeKumaEnabled", value: "false"},
		{name: "enabled invalid", key: "UptimeKumaEnabled", value: "on", wantErr: true},
		{name: "url http valid", key: "UptimeKumaUrl", value: "http://192.168.1.100:3001"},
		{name: "url https valid", key: "UptimeKumaUrl", value: "https://kuma.example.com"},
		{name: "url invalid", key: "UptimeKumaUrl", value: "kuma.example.com", wantErr: true},
		{name: "scope all", key: "UptimeKumaMonitorScope", value: "all"},
		{name: "scope selected", key: "UptimeKumaMonitorScope", value: "selected"},
		{name: "scope invalid", key: "UptimeKumaMonitorScope", value: "none", wantErr: true},
		{name: "sync interval valid", key: "UptimeKumaSyncInterval", value: "5"},
		{name: "sync interval invalid", key: "UptimeKumaSyncInterval", value: "0", wantErr: true},
		{name: "interval valid", key: "UptimeKumaInterval", value: "60"},
		{name: "interval invalid", key: "UptimeKumaInterval", value: "-60", wantErr: true},
		{name: "retry valid", key: "UptimeKumaRetry", value: "0"},
		{name: "retry positive valid", key: "UptimeKumaRetry", value: "3"},
		{name: "retry invalid", key: "UptimeKumaRetry", value: "-1", wantErr: true},
	}

	for _, tc := range testCases {
		err := validateUptimeKumaOption(tc.key, tc.value, state)
		if tc.wantErr && err == nil {
			t.Fatalf("%s: expected error", tc.name)
		}
		if !tc.wantErr && err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
	}

	// Test enabling Uptime Kuma when URL or credentials are empty in state
	stateEmpty := map[string]string{
		"UptimeKumaUrl":      "",
		"UptimeKumaUsername": "",
		"UptimeKumaPassword": "",
	}
	if err := validateUptimeKumaOption("UptimeKumaEnabled", "true", stateEmpty); err == nil {
		t.Fatal("expected error when enabling Uptime Kuma with empty URL/credentials in state")
	}
}
