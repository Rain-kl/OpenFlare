package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDockerModeUsesManagedPaths(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "agent.json")
	payload := map[string]any{
		"server_url":    "http://127.0.0.1:3000",
		"agent_token":   "token",
		"node_name":     "edge-01",
		"node_ip":       "10.0.0.8",
		"agent_version": "0.1.0",
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
	if cfg.NginxCertDir != defaultDockerNginxCertDir {
		t.Fatalf("unexpected nginx cert dir: %s", cfg.NginxCertDir)
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
		"nginx_path":        "/opt/nginx/sbin/nginx",
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
	if cfg.NginxCertDir != cfg.CertDir {
		t.Fatalf("expected path mode nginx cert dir to equal cert dir, got %s / %s", cfg.NginxCertDir, cfg.CertDir)
	}
}

func TestLoadUsesCustomDataDirForGeneratedFiles(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "agent.json")
	payload := map[string]any{
		"server_url":    "http://127.0.0.1:3000",
		"agent_token":   "token",
		"node_name":     "edge-01",
		"node_ip":       "10.0.0.8",
		"data_dir":      "/srv/atsflare",
		"agent_version": "0.1.0",
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
