// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package config loads and persists flared daemon configuration.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	edgeconfig "github.com/Rain-kl/Wavelet/internal/apps/edge/config"
)

const (
	defaultHeartbeatInterval = 10 * time.Second
	defaultSyncInterval      = 30 * time.Second
	defaultRequestTimeout    = 10 * time.Second
	configFilePerm           = 0o644
)

// MillisecondDuration is a JSON-friendly duration type shared with edge config.
type MillisecondDuration = edgeconfig.MillisecondDuration

// Config holds flared daemon settings loaded from file and environment.
type Config struct {
	ServerURL         string              `json:"server_url"`
	TunnelToken       string              `json:"tunnel_token"`
	FrpcPath          string              `json:"frpc_path"`
	DataDir           string              `json:"data_dir"`
	StatePath         string              `json:"state_path"`
	HeartbeatInterval MillisecondDuration `json:"heartbeat_interval"`
	SyncInterval      MillisecondDuration `json:"sync_interval"`
	RequestTimeout    MillisecondDuration `json:"request_timeout"`
	configPath        string
}

// Load reads configuration from path, applying environment overrides and defaults.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is the flared config file location from startup configuration
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	cfg := &Config{}
	if err == nil {
		if err = json.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}
	if err != nil && !hasEnvConfig() {
		return nil, err
	}
	cfg.configPath = path
	applyEnvOverrides(cfg)
	applyDefaults(cfg, filepath.Dir(path))
	if err = validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func hasEnvConfig() bool {
	for _, key := range []string{
		"OPENFLARE_SERVER_URL",
		"OPENFLARE_TUNNEL_TOKEN",
		"OPENFLARE_DATA_DIR",
		"OPENFLARE_FRPC_PATH",
	} {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return true
		}
	}
	return false
}

func applyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}
	overrideString := func(key string, target *string) {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			*target = value
		}
	}
	overrideString("OPENFLARE_SERVER_URL", &cfg.ServerURL)
	overrideString("OPENFLARE_TUNNEL_TOKEN", &cfg.TunnelToken)
	overrideString("OPENFLARE_DATA_DIR", &cfg.DataDir)
	overrideString("OPENFLARE_FRPC_PATH", &cfg.FrpcPath)
}

func applyDefaults(cfg *Config, baseDir string) {
	baseDir = filepath.Clean(baseDir)
	if cfg.FrpcPath == "" {
		cfg.FrpcPath = "frpc" // rely on PATH
	}
	if cfg.DataDir == "" {
		cfg.DataDir = filepath.Join(baseDir, "data")
	}
	if cfg.StatePath == "" {
		cfg.StatePath = filepath.Join(cfg.DataDir, "flared-state.json")
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = MillisecondDuration(defaultHeartbeatInterval)
	}
	if cfg.SyncInterval <= 0 {
		cfg.SyncInterval = MillisecondDuration(defaultSyncInterval)
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = MillisecondDuration(defaultRequestTimeout)
	}
}

func validate(cfg *Config) error {
	if cfg.ServerURL == "" {
		return errors.New("server_url 不能为空")
	}
	if strings.TrimSpace(cfg.TunnelToken) == "" {
		return errors.New("tunnel_token 不能为空")
	}
	return nil
}

// InitialAuthToken returns the tunnel token used for initial authentication.
func (cfg *Config) InitialAuthToken() string {
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.TunnelToken)
}

// Save writes the current configuration back to the loaded config path.
func (cfg *Config) Save() error {
	if cfg == nil {
		return errors.New("config 不能为空")
	}
	if cfg.configPath == "" {
		return errors.New("config path 未初始化")
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cfg.configPath, data, configFilePerm)
}
