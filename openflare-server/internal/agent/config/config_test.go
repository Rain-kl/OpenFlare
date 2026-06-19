package config

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/pkg/geoip"
)

func TestLoadDefaultsToManagedBinaryPaths(t *testing.T) {
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
	if cfg.OpenrestyPath != "openresty" {
		t.Fatalf("unexpected openresty path: %s", cfg.OpenrestyPath)
	}
	if cfg.MainConfigPath != filepath.Join(dir, "data", defaultMainConfigRelativePath) {
		t.Fatalf("unexpected main config path: %s", cfg.MainConfigPath)
	}
	if cfg.RouteConfigPath != filepath.Join(dir, "data", defaultRouteConfigRelativePath) {
		t.Fatalf("unexpected route config path: %s", cfg.RouteConfigPath)
	}
	if cfg.AccessLogPath != filepath.Join(dir, "data", defaultAccessLogRelativePath) {
		t.Fatalf("unexpected access log path: %s", cfg.AccessLogPath)
	}
	if cfg.CertDir != filepath.Join(dir, "data", defaultCertDirRelativePath) {
		t.Fatalf("unexpected cert dir: %s", cfg.CertDir)
	}
	if cfg.LuaDir != filepath.Join(dir, "data", defaultLuaDirRelativePath) {
		t.Fatalf("unexpected lua dir: %s", cfg.LuaDir)
	}
	if cfg.RuntimeConfigDir != filepath.Join(dir, "data", defaultRuntimeConfigDirRelativePath) {
		t.Fatalf("unexpected runtime config dir: %s", cfg.RuntimeConfigDir)
	}
	if cfg.OpenrestyCertDir != cfg.CertDir {
		t.Fatalf("unexpected openresty cert dir: %s", cfg.OpenrestyCertDir)
	}
	if cfg.OpenrestyLuaDir != cfg.LuaDir {
		t.Fatalf("unexpected openresty lua dir: %s", cfg.OpenrestyLuaDir)
	}
	if cfg.StatePath != filepath.Join(dir, "data", defaultStateRelativePath) {
		t.Fatalf("unexpected state path: %s", cfg.StatePath)
	}
	if cfg.ObservabilityBufferPath != filepath.Join(dir, "data", defaultObservabilityBufferRelativePath) {
		t.Fatalf("unexpected observability buffer path: %s", cfg.ObservabilityBufferPath)
	}
	if cfg.OpenrestyObservabilityPort != defaultOpenRestyObservabilityPort {
		t.Fatalf("unexpected openresty observability port: %d", cfg.OpenrestyObservabilityPort)
	}
	if cfg.ObservabilityReplayMinutes != defaultObservabilityReplayMinutes {
		t.Fatalf("unexpected observability replay minutes: %d", cfg.ObservabilityReplayMinutes)
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
		"main_config_path":  "/tmp/nginx.conf",
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

	if cfg.MainConfigPath != "/tmp/nginx.conf" {
		t.Fatalf("unexpected main config path: %s", cfg.MainConfigPath)
	}
	if cfg.RouteConfigPath != "/tmp/routes.conf" {
		t.Fatalf("unexpected route config path: %s", cfg.RouteConfigPath)
	}
	if cfg.StatePath != "/tmp/agent-state.json" {
		t.Fatalf("unexpected state path: %s", cfg.StatePath)
	}
	if cfg.ObservabilityBufferPath != filepath.Join(dir, "data", defaultObservabilityBufferRelativePath) {
		t.Fatalf("unexpected observability buffer path: %s", cfg.ObservabilityBufferPath)
	}
	if cfg.OpenrestyCertDir != cfg.CertDir {
		t.Fatalf("expected path mode openresty cert dir to equal cert dir, got %s / %s", cfg.OpenrestyCertDir, cfg.CertDir)
	}
	if cfg.OpenrestyLuaDir != cfg.LuaDir {
		t.Fatalf("expected path mode openresty lua dir to equal lua dir, got %s / %s", cfg.OpenrestyLuaDir, cfg.LuaDir)
	}
	if cfg.OpenrestyObservabilityPort != defaultOpenRestyObservabilityPort {
		t.Fatalf("unexpected path mode openresty observability port: %d", cfg.OpenrestyObservabilityPort)
	}
}

func TestLoadNormalizesExplicitResolvers(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "agent.json")
	payload := map[string]any{
		"server_url":          "http://127.0.0.1:3000",
		"agent_token":         "token",
		"node_name":           "edge-01",
		"node_ip":             "10.0.0.8",
		"openresty_resolvers": []string{" 10.0.0.2 ", "10.0.0.2", "", "1.1.1.1"},
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

	expected := []string{"10.0.0.2", "1.1.1.1"}
	if len(cfg.OpenrestyResolvers) != len(expected) {
		t.Fatalf("unexpected resolver count: %#v", cfg.OpenrestyResolvers)
	}
	for index, value := range expected {
		if cfg.OpenrestyResolvers[index] != value {
			t.Fatalf("unexpected resolver at %d: got %q want %q", index, cfg.OpenrestyResolvers[index], value)
		}
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
		"data_dir":    "/srv/openflare",
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

	if cfg.RouteConfigPath != "/srv/openflare/"+defaultRouteConfigRelativePath {
		t.Fatalf("unexpected route config path: %s", cfg.RouteConfigPath)
	}
	if cfg.MainConfigPath != "/srv/openflare/"+defaultMainConfigRelativePath {
		t.Fatalf("unexpected main config path: %s", cfg.MainConfigPath)
	}
	if cfg.AccessLogPath != "/srv/openflare/"+defaultAccessLogRelativePath {
		t.Fatalf("unexpected access log path: %s", cfg.AccessLogPath)
	}
	if cfg.StatePath != "/srv/openflare/"+defaultStateRelativePath {
		t.Fatalf("unexpected state path: %s", cfg.StatePath)
	}
	if cfg.ObservabilityBufferPath != "/srv/openflare/"+defaultObservabilityBufferRelativePath {
		t.Fatalf("unexpected observability buffer path: %s", cfg.ObservabilityBufferPath)
	}
	if cfg.CertDir != "/srv/openflare/"+defaultCertDirRelativePath {
		t.Fatalf("unexpected cert dir: %s", cfg.CertDir)
	}
	if cfg.LuaDir != "/srv/openflare/"+defaultLuaDirRelativePath {
		t.Fatalf("unexpected lua dir: %s", cfg.LuaDir)
	}
	if cfg.RuntimeConfigDir != "/srv/openflare/"+defaultRuntimeConfigDirRelativePath {
		t.Fatalf("unexpected runtime config dir: %s", cfg.RuntimeConfigDir)
	}
}

func TestLoadUsesEnvConfigWhenFileIsMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENFLARE_SERVER_URL", "http://127.0.0.1:3000")
	t.Setenv("OPENFLARE_AGENT_TOKEN", "token")
	t.Setenv("OPENFLARE_NODE_NAME", "edge-env")
	t.Setenv("OPENFLARE_NODE_IP", "10.0.0.9")
	t.Setenv("OPENFLARE_DATA_DIR", "/srv/openflare-env")
	t.Setenv("OPENFLARE_OPENRESTY_PATH", "/usr/bin/openresty")
	t.Setenv("OPENFLARE_HEARTBEAT_INTERVAL", "45s")
	t.Setenv("OPENFLARE_REQUEST_TIMEOUT", "2500")
	t.Setenv("OPENFLARE_OPENRESTY_OBSERVABILITY_PORT", "19091")

	cfg, err := Load(filepath.Join(dir, "missing-agent.json"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.ServerURL != "http://127.0.0.1:3000" || cfg.AccessToken != "token" {
		t.Fatalf("unexpected env auth config: %#v", cfg)
	}
	if cfg.OpenrestyPath != "/usr/bin/openresty" {
		t.Fatalf("unexpected openresty path: %s", cfg.OpenrestyPath)
	}
	if cfg.DataDir != "/srv/openflare-env" {
		t.Fatalf("unexpected data dir: %s", cfg.DataDir)
	}
	if cfg.HeartbeatInterval.Duration() != 45*time.Second {
		t.Fatalf("unexpected heartbeat interval: %s", cfg.HeartbeatInterval)
	}
	if cfg.RequestTimeout.Duration() != 2500*time.Millisecond {
		t.Fatalf("unexpected request timeout: %s", cfg.RequestTimeout)
	}
	if cfg.OpenrestyObservabilityPort != 19091 {
		t.Fatalf("unexpected observability port: %d", cfg.OpenrestyObservabilityPort)
	}
}

func TestLoadDetectsOutboundIPWhenNodeIPMissing(t *testing.T) {
	previousLookup := lookupOutboundIP
	lookupOutboundIP = func(ctx context.Context, strategies ...geoip.OutboundIPStrategy) (net.IP, error) {
		return net.ParseIP("8.8.8.8"), nil
	}
	defer func() {
		lookupOutboundIP = previousLookup
	}()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "agent.json")
	payload := map[string]any{
		"server_url":  "http://127.0.0.1:3000",
		"agent_token": "token",
		"node_name":   "edge-01",
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
	if cfg.NodeIP != "8.8.8.8" {
		t.Fatalf("expected outbound IP, got %s", cfg.NodeIP)
	}
}

func TestLoadFallsBackToLocalIPWhenOutboundLookupFails(t *testing.T) {
	previousOutboundLookup := lookupOutboundIP
	previousLocalLookup := lookupLocalIP
	lookupOutboundIP = func(ctx context.Context, strategies ...geoip.OutboundIPStrategy) (net.IP, error) {
		return nil, errors.New("realip.cc unavailable")
	}
	lookupLocalIP = func() string {
		return "9.9.9.9"
	}
	defer func() {
		lookupOutboundIP = previousOutboundLookup
		lookupLocalIP = previousLocalLookup
	}()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "agent.json")
	payload := map[string]any{
		"server_url":  "http://127.0.0.1:3000",
		"agent_token": "token",
		"node_name":   "edge-01",
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
	if cfg.NodeIP != "9.9.9.9" {
		t.Fatalf("expected local fallback IP, got %s", cfg.NodeIP)
	}
}

func TestLoadEnvOverridesConfigFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "agent.json")
	if err := os.WriteFile(configPath, []byte(`{"server_url":"http://old:3000","agent_token":"old","node_name":"edge-01","node_ip":"10.0.0.8","openresty_path":"/old/openresty"}`), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	t.Setenv("OPENFLARE_SERVER_URL", "http://new:3000")
	t.Setenv("OPENFLARE_AGENT_TOKEN", "new-token")
	t.Setenv("OPENFLARE_OPENRESTY_PATH", "/new/openresty")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.ServerURL != "http://new:3000" {
		t.Fatalf("expected server url from env, got %s", cfg.ServerURL)
	}
	if cfg.AccessToken != "new-token" {
		t.Fatalf("expected token from env, got %s", cfg.AccessToken)
	}
	if cfg.OpenrestyPath != "/new/openresty" {
		t.Fatalf("expected openresty path from env, got %s", cfg.OpenrestyPath)
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
	cfg.ExtVersion = "1.27.1.2"
	cfg.HeartbeatInterval = MillisecondDuration(5 * time.Second)
	cfg.RequestTimeout = MillisecondDuration(7 * time.Second)
	cfg.OpenrestyResolvers = []string{"10.0.0.2", "1.1.1.1"}

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
	if decoded["request_timeout"] != float64(7000) {
		t.Fatalf("unexpected request timeout: %#v", decoded["request_timeout"])
	}
	resolvers, ok := decoded["openresty_resolvers"].([]any)
	if !ok || len(resolvers) != 2 || resolvers[0] != "10.0.0.2" || resolvers[1] != "1.1.1.1" {
		t.Fatalf("unexpected resolvers: %#v", decoded["openresty_resolvers"])
	}
	if decoded["openresty_observability_port"] != float64(defaultOpenRestyObservabilityPort) {
		t.Fatalf("unexpected observability port: %#v", decoded["openresty_observability_port"])
	}
	if decoded["observability_replay_minutes"] != float64(defaultObservabilityReplayMinutes) {
		t.Fatalf("unexpected observability replay minutes: %#v", decoded["observability_replay_minutes"])
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
					AccessToken:    tt.agentToken,
					DiscoveryToken: tt.discoveryToken,
				}
			}
			if token := cfg.InitialAuthToken(); token != tt.expected {
				t.Fatalf("unexpected initial auth token: %q", token)
			}
		})
	}
}

func TestNodeIPPriority(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected int
	}{
		{
			name:     "public ipv4 preferred",
			ip:       "8.8.8.8",
			expected: 2,
		},
		{
			name:     "private ipv4 fallback",
			ip:       "10.0.0.8",
			expected: 1,
		},
		{
			name:     "link local ignored",
			ip:       "169.254.1.10",
			expected: -1,
		},
		{
			name:     "loopback ignored",
			ip:       "127.0.0.1",
			expected: -1,
		},
		{
			name:     "nil ignored",
			ip:       "",
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var parsed net.IP
			if tt.ip != "" {
				parsed = net.ParseIP(tt.ip)
			}
			if got := nodeIPPriority(parsed); got != tt.expected {
				t.Fatalf("unexpected priority for %q: got %d want %d", tt.ip, got, tt.expected)
			}
		})
	}
}
