package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDockerModeUsesManagedPaths(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "agent.json")
	payload := map[string]any{
		"server_url":  "http://127.0.0.1:3000",
		"agent_token": "token",
		"node_name":   "edge-01",
		"node_ip":     "10.0.0.8",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}
	if err = os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.DataDir != filepath.Join(dir, "data") {
		t.Fatalf("unexpected data dir: %s", cfg.DataDir)
	}
	if cfg.RouteConfigPath != filepath.Join(dir, "data", defaultDockerRouteConfigRelativePath) {
		t.Fatalf("unexpected route config path: %s", cfg.RouteConfigPath)
	}
	if cfg.CertDir != filepath.Join(dir, "data", defaultCertDirRelativePath) {
		t.Fatalf("unexpected cert dir: %s", cfg.CertDir)
	}
	if cfg.OpenrestyContainerName != "atsflare-openresty" {
		t.Fatalf("unexpected openresty container name: %s", cfg.OpenrestyContainerName)
	}
	if cfg.OpenrestyDockerImage != "openresty/openresty:alpine" {
		t.Fatalf("unexpected openresty image: %s", cfg.OpenrestyDockerImage)
	}
	if cfg.OpenrestyCertDir != defaultDockerOpenRestyCertDir {
		t.Fatalf("unexpected openresty cert dir: %s", cfg.OpenrestyCertDir)
	}
	if cfg.StatePath != filepath.Join(dir, "data", defaultDockerStateRelativePath) {
		t.Fatalf("unexpected state path: %s", cfg.StatePath)
	}
}

func TestLoadPathModeKeepsExplicitPaths(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "agent.json")
	payload := map[string]any{
		"server_url":        "http://127.0.0.1:3000",
		"agent_token":       "token",
		"node_name":         "edge-01",
		"node_ip":           "10.0.0.8",
		"openresty_path":    "/usr/local/openresty/nginx/sbin/openresty",
		"route_config_path": "/tmp/routes.conf",
		"state_path":        "/tmp/agent-state.json",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}
	if err = os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.RouteConfigPath != "/tmp/routes.conf" {
		t.Fatalf("unexpected route config path: %s", cfg.RouteConfigPath)
	}
	if cfg.StatePath != "/tmp/agent-state.json" {
		t.Fatalf("unexpected state path: %s", cfg.StatePath)
	}
	if cfg.OpenrestyCertDir != cfg.CertDir {
		t.Fatalf("expected path mode openresty cert dir to equal cert dir, got %s / %s", cfg.OpenrestyCertDir, cfg.CertDir)
	}
}

func TestLoadUsesCustomDataDirForGeneratedFiles(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "agent.json")
	payload := map[string]any{
		"server_url":  "http://127.0.0.1:3000",
		"agent_token": "token",
		"node_name":   "edge-01",
		"node_ip":     "10.0.0.8",
		"data_dir":    "/srv/atsflare",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}
	if err = os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.RouteConfigPath != "/srv/atsflare/"+defaultDockerRouteConfigRelativePath {
		t.Fatalf("unexpected route config path: %s", cfg.RouteConfigPath)
	}
	if cfg.StatePath != "/srv/atsflare/"+defaultDockerStateRelativePath {
		t.Fatalf("unexpected state path: %s", cfg.StatePath)
	}
	if cfg.CertDir != "/srv/atsflare/"+defaultCertDirRelativePath {
		t.Fatalf("unexpected cert dir: %s", cfg.CertDir)
	}
}

func TestLoadUsesMillisecondsForIntervals(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "agent.json")
	payload := map[string]any{
		"server_url":         "http://127.0.0.1:3000",
		"agent_token":        "token",
		"node_name":          "edge-01",
		"node_ip":            "10.0.0.8",
		"heartbeat_interval": 30000,
		"sync_interval":      45000,
		"request_timeout":    1500,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}
	if err = os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.HeartbeatInterval.Duration() != 30*time.Second {
		t.Fatalf("unexpected heartbeat interval: %s", cfg.HeartbeatInterval)
	}
	if cfg.SyncInterval.Duration() != 45*time.Second {
		t.Fatalf("unexpected sync interval: %s", cfg.SyncInterval)
	}
	if cfg.RequestTimeout.Duration() != 1500*time.Millisecond {
		t.Fatalf("unexpected request timeout: %s", cfg.RequestTimeout)
	}
}

func TestSavePersistsMillisecondsAndOmitsRuntimeVersions(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "agent.json")
	if err := os.WriteFile(configPath, []byte(`{"server_url":"http://127.0.0.1:3000","agent_token":"token","node_name":"edge-01","node_ip":"10.0.0.8"}`), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	cfg.NginxVersion = "1.27.1.2"
	cfg.HeartbeatInterval = MillisecondDuration(5 * time.Second)
	cfg.SyncInterval = MillisecondDuration(6 * time.Second)
	cfg.RequestTimeout = MillisecondDuration(7 * time.Second)

	if err = cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read saved config: %v", err)
	}
	var decoded map[string]any
	if err = json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to decode saved config: %v", err)
	}
	if _, ok := decoded["agent_version"]; ok {
		t.Fatal("agent_version should not be persisted")
	}
	if _, ok := decoded["nginx_version"]; ok {
		t.Fatal("nginx_version should not be persisted")
	}
	if decoded["heartbeat_interval"] != float64(5000) {
		t.Fatalf("unexpected heartbeat interval: %#v", decoded["heartbeat_interval"])
	}
	if decoded["sync_interval"] != float64(6000) {
		t.Fatalf("unexpected sync interval: %#v", decoded["sync_interval"])
	}
	if decoded["request_timeout"] != float64(7000) {
		t.Fatalf("unexpected request timeout: %#v", decoded["request_timeout"])
	}
	if _, ok := decoded["nginx_path"]; ok {
		t.Fatal("legacy nginx_path should not be persisted")
	}
}

func TestInitialAuthToken(t *testing.T) {
	tests := []struct {
		name           string
		agentToken     string
		discoveryToken string
		expected       string
	}{
		{
			name:           "prefer agent token",
			agentToken:     "agent-token",
			discoveryToken: "discovery-token",
			expected:       "agent-token",
		},
		{
			name:           "fallback to discovery token",
			agentToken:     "   ",
			discoveryToken: "discovery-token",
			expected:       "discovery-token",
		},
		{
			name:           "nil config returns empty string",
			agentToken:     "",
			discoveryToken: "",
			expected:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg *Config
			if tt.name != "nil config returns empty string" {
				cfg = &Config{
					AgentToken:     tt.agentToken,
					DiscoveryToken: tt.discoveryToken,
				}
			}
			if token := cfg.InitialAuthToken(); token != tt.expected {
				t.Fatalf("unexpected initial auth token: %q", token)
			}
		})
	}
}
