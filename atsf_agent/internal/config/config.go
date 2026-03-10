package config

import (
	"encoding/json"
	"errors"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultDockerRouteConfigRelativePath = "etc/nginx/conf.d/atsflare_routes.conf"
	defaultCertDirRelativePath           = "etc/nginx/certs"
	defaultDockerStateRelativePath       = "var/lib/atsflare/agent-state.json"
	defaultDockerNginxCertDir            = "/etc/nginx/atsflare-certs"
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
	DataDir            string        `json:"data_dir"`
	RouteConfigPath    string        `json:"route_config_path"`
	CertDir            string        `json:"cert_dir"`
	NginxCertDir       string        `json:"nginx_cert_dir"`
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
	applyDefaults(cfg, filepath.Dir(path))
	if err = validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func applyDefaults(cfg *Config, baseDir string) {
	baseDir = filepath.Clean(baseDir)
	if cfg.AgentVersion == "" {
		cfg.AgentVersion = "dev"
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
	if cfg.DataDir == "" {
		cfg.DataDir = filepath.Join(baseDir, "data")
	}
	if cfg.NginxPath == "" {
		cfg.RouteConfigPath = joinManagedPath(cfg.DataDir, defaultDockerRouteConfigRelativePath)
		cfg.StatePath = joinManagedPath(cfg.DataDir, defaultDockerStateRelativePath)
	} else {
		if cfg.RouteConfigPath == "" {
			cfg.RouteConfigPath = joinManagedPath(cfg.DataDir, defaultDockerRouteConfigRelativePath)
		}
		if cfg.StatePath == "" {
			cfg.StatePath = joinManagedPath(cfg.DataDir, defaultDockerStateRelativePath)
		}
	}
	if cfg.CertDir == "" {
		cfg.CertDir = joinManagedPath(cfg.DataDir, defaultCertDirRelativePath)
	}
	if cfg.NginxCertDir == "" {
		if cfg.NginxPath != "" {
			cfg.NginxCertDir = cfg.CertDir
		} else {
			cfg.NginxCertDir = defaultDockerNginxCertDir
		}
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
	normalizeManagedPaths(cfg)
}

func normalizeManagedPaths(cfg *Config) {
	if cfg == nil {
		return
	}
	if usesSlashPath(cfg.DataDir) {
		cfg.DataDir = filepath.ToSlash(cfg.DataDir)
	}
	if usesSlashPath(cfg.RouteConfigPath) {
		cfg.RouteConfigPath = filepath.ToSlash(cfg.RouteConfigPath)
	}
	if usesSlashPath(cfg.CertDir) {
		cfg.CertDir = filepath.ToSlash(cfg.CertDir)
	}
	if usesSlashPath(cfg.StatePath) {
		cfg.StatePath = filepath.ToSlash(cfg.StatePath)
	}
}

func usesSlashPath(path string) bool {
	return strings.HasPrefix(path, "/")
}

func joinManagedPath(base string, relative string) string {
	if usesSlashPath(base) {
		return pathpkg.Join(filepath.ToSlash(base), relative)
	}
	return filepath.Join(base, relative)
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
