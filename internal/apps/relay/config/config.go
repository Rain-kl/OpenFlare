package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	edgeconfig "github.com/Rain-kl/Wavelet/internal/apps/edge/config"
	"github.com/Rain-kl/Wavelet/internal/apps/edge/nodeip"
)

type MillisecondDuration = edgeconfig.MillisecondDuration

type Config struct {
	ServerURL         string              `json:"server_url"`
	AgentToken        string              `json:"agent_token"`
	DiscoveryToken    string              `json:"discovery_token"`
	NodeName          string              `json:"node_name"`
	NodeIP            string              `json:"node_ip"`
	FrpsPath          string              `json:"frps_path"`
	DataDir           string              `json:"data_dir"`
	StatePath         string              `json:"state_path"`
	HeartbeatInterval MillisecondDuration `json:"heartbeat_interval"`
	RequestTimeout    MillisecondDuration `json:"request_timeout"`
	configPath        string
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
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
		"OPENFLARE_AGENT_TOKEN",
		"OPENFLARE_DISCOVERY_TOKEN",
		"OPENFLARE_NODE_NAME",
		"OPENFLARE_NODE_IP",
		"OPENFLARE_DATA_DIR",
		"OPENFLARE_FRPS_PATH",
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
	overrideString("OPENFLARE_AGENT_TOKEN", &cfg.AgentToken)
	overrideString("OPENFLARE_DISCOVERY_TOKEN", &cfg.DiscoveryToken)
	overrideString("OPENFLARE_NODE_NAME", &cfg.NodeName)
	overrideString("OPENFLARE_NODE_IP", &cfg.NodeIP)
	overrideString("OPENFLARE_DATA_DIR", &cfg.DataDir)
	overrideString("OPENFLARE_FRPS_PATH", &cfg.FrpsPath)
}

func applyDefaults(cfg *Config, baseDir string) {
	baseDir = filepath.Clean(baseDir)
	if cfg.FrpsPath == "" {
		cfg.FrpsPath = "frps" // rely on PATH
	}
	if cfg.DataDir == "" {
		cfg.DataDir = filepath.Join(baseDir, "data")
	}
	if cfg.NodeName == "" {
		host, _ := os.Hostname()
		cfg.NodeName = strings.TrimSpace(host)
	}
	if cfg.NodeIP == "" {
		cfg.NodeIP = nodeip.Detect()
	}
	if cfg.StatePath == "" {
		cfg.StatePath = filepath.Join(cfg.DataDir, "relay-state.json")
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = MillisecondDuration(10 * time.Second)
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = MillisecondDuration(10 * time.Second)
	}
}

func validate(cfg *Config) error {
	if cfg.ServerURL == "" {
		return errors.New("server_url 不能为空")
	}
	if strings.TrimSpace(cfg.AgentToken) == "" && strings.TrimSpace(cfg.DiscoveryToken) == "" {
		return errors.New("agent_token 和 discovery_token 不能同时为空")
	}
	if cfg.NodeName == "" {
		return errors.New("node_name 不能为空")
	}
	return nil
}

func (cfg *Config) InitialAuthToken() string {
	if cfg == nil {
		return ""
	}
	if token := strings.TrimSpace(cfg.AgentToken); token != "" {
		return token
	}
	return strings.TrimSpace(cfg.DiscoveryToken)
}

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
	return os.WriteFile(cfg.configPath, data, 0o644)
}


