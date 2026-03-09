package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	ServerURL          string        `json:"server_url"`
	AgentToken         string        `json:"agent_token"`
	NodeName           string        `json:"node_name"`
	NodeIP             string        `json:"node_ip"`
	AgentVersion       string        `json:"agent_version"`
	NginxVersion       string        `json:"nginx_version"`
	NginxPath          string        `json:"nginx_path"`
	NginxContainerName string        `json:"nginx_container_name"`
	NginxDockerImage   string        `json:"nginx_docker_image"`
	DockerBinary       string        `json:"docker_binary"`
	RouteConfigPath    string        `json:"route_config_path"`
	StatePath          string        `json:"state_path"`
	HeartbeatInterval  time.Duration `json:"heartbeat_interval"`
	SyncInterval       time.Duration `json:"sync_interval"`
	RequestTimeout     time.Duration `json:"request_timeout"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err = json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	applyDefaults(cfg)
	if err = validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.AgentVersion == "" {
		cfg.AgentVersion = "dev"
	}
	if cfg.RouteConfigPath == "" {
		cfg.RouteConfigPath = filepath.Clean("./atsflare_routes.conf")
	}
	if cfg.StatePath == "" {
		cfg.StatePath = filepath.Clean("./atsf_agent_state.json")
	}
	if cfg.NginxContainerName == "" {
		cfg.NginxContainerName = "atsflare-nginx"
	}
	if cfg.NginxDockerImage == "" {
		cfg.NginxDockerImage = "nginx:stable-alpine"
	}
	if cfg.DockerBinary == "" {
		cfg.DockerBinary = "docker"
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = 30 * time.Second
	}
	if cfg.SyncInterval <= 0 {
		cfg.SyncInterval = 30 * time.Second
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 10 * time.Second
	}
}

func validate(cfg *Config) error {
	if cfg.ServerURL == "" {
		return errors.New("server_url 不能为空")
	}
	if cfg.AgentToken == "" {
		return errors.New("agent_token 不能为空")
	}
	if cfg.NodeName == "" {
		return errors.New("node_name 不能为空")
	}
	if cfg.NodeIP == "" {
		return errors.New("node_ip 不能为空")
	}
	return nil
}
