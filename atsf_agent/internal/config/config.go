package config

import (
	"encoding/json"
	"errors"
	"net"
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
	defaultDockerOpenRestyCertDir        = "/etc/nginx/atsflare-certs"
)

type Config struct {
	ServerURL          string              `json:"server_url"`
	AgentToken         string              `json:"agent_token"`
	DiscoveryToken     string              `json:"discovery_token"`
	NodeName           string              `json:"node_name"`
	NodeIP             string              `json:"node_ip"`
	AgentVersion       string              `json:"-"`
	NginxVersion       string              `json:"-"`
	OpenrestyPath      string              `json:"openresty_path"`
	OpenrestyContainerName string          `json:"openresty_container_name"`
	OpenrestyDockerImage   string          `json:"openresty_docker_image"`
	DockerBinary       string              `json:"docker_binary"`
	DataDir            string              `json:"data_dir"`
	RouteConfigPath    string              `json:"route_config_path"`
	CertDir            string              `json:"cert_dir"`
	OpenrestyCertDir   string              `json:"openresty_cert_dir"`
	StatePath          string              `json:"state_path"`
	HeartbeatInterval  MillisecondDuration `json:"heartbeat_interval"`
	SyncInterval       MillisecondDuration `json:"sync_interval"`
	RequestTimeout     MillisecondDuration `json:"request_timeout"`
	configPath         string
}

type configFile struct {
	ServerURL              string              `json:"server_url"`
	AgentToken             string              `json:"agent_token"`
	DiscoveryToken         string              `json:"discovery_token"`
	NodeName               string              `json:"node_name"`
	NodeIP                 string              `json:"node_ip"`
	OpenrestyPath          string              `json:"openresty_path"`
	OpenrestyContainerName string              `json:"openresty_container_name"`
	OpenrestyDockerImage   string              `json:"openresty_docker_image"`
	DockerBinary           string              `json:"docker_binary"`
	DataDir                string              `json:"data_dir"`
	RouteConfigPath        string              `json:"route_config_path"`
	CertDir                string              `json:"cert_dir"`
	OpenrestyCertDir       string              `json:"openresty_cert_dir"`
	StatePath              string              `json:"state_path"`
	HeartbeatInterval      MillisecondDuration `json:"heartbeat_interval"`
	SyncInterval           MillisecondDuration `json:"sync_interval"`
	RequestTimeout         MillisecondDuration `json:"request_timeout"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	file := &configFile{}
	if err = json.Unmarshal(data, file); err != nil {
		return nil, err
	}
	cfg := &Config{
		ServerURL:              file.ServerURL,
		AgentToken:             file.AgentToken,
		DiscoveryToken:         file.DiscoveryToken,
		NodeName:               file.NodeName,
		NodeIP:                 file.NodeIP,
		OpenrestyPath:          file.OpenrestyPath,
		OpenrestyContainerName: file.OpenrestyContainerName,
		OpenrestyDockerImage:   file.OpenrestyDockerImage,
		DockerBinary:           file.DockerBinary,
		DataDir:                file.DataDir,
		RouteConfigPath:        file.RouteConfigPath,
		CertDir:                file.CertDir,
		OpenrestyCertDir:       file.OpenrestyCertDir,
		StatePath:              file.StatePath,
		HeartbeatInterval:      file.HeartbeatInterval,
		SyncInterval:           file.SyncInterval,
		RequestTimeout:         file.RequestTimeout,
	}
	cfg.configPath = path
	applyDefaults(cfg, filepath.Dir(path))
	if err = validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func applyDefaults(cfg *Config, baseDir string) {
	baseDir = filepath.Clean(baseDir)
	cfg.AgentVersion = AgentVersion
	if cfg.OpenrestyContainerName == "" {
		cfg.OpenrestyContainerName = "atsflare-openresty"
	}
	if cfg.OpenrestyDockerImage == "" {
		cfg.OpenrestyDockerImage = "openresty/openresty:alpine"
	}
	if cfg.DockerBinary == "" {
		cfg.DockerBinary = "docker"
	}
	if cfg.DataDir == "" {
		cfg.DataDir = filepath.Join(baseDir, "data")
	}
	if cfg.NodeName == "" {
		cfg.NodeName = detectHostname()
	}
	if cfg.NodeIP == "" {
		cfg.NodeIP = detectNodeIP()
	}
	if cfg.OpenrestyPath == "" {
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
	if cfg.OpenrestyCertDir == "" {
		if cfg.OpenrestyPath != "" {
			cfg.OpenrestyCertDir = cfg.CertDir
		} else {
			cfg.OpenrestyCertDir = defaultDockerOpenRestyCertDir
		}
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = MillisecondDuration(30 * time.Second)
	}
	if cfg.SyncInterval <= 0 {
		cfg.SyncInterval = MillisecondDuration(30 * time.Second)
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = MillisecondDuration(10 * time.Second)
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
	if strings.TrimSpace(cfg.AgentToken) == "" && strings.TrimSpace(cfg.DiscoveryToken) == "" {
		return errors.New("agent_token 和 discovery_token 不能同时为空")
	}
	if cfg.NodeName == "" {
		return errors.New("node_name 不能为空")
	}
	if cfg.NodeIP == "" {
		return errors.New("node_ip 不能为空")
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

func detectHostname() string {
	host, err := os.Hostname()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(host)
}

func detectNodeIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP == nil || ipNet.IP.IsLoopback() {
				continue
			}
			ipv4 := ipNet.IP.To4()
			if ipv4 != nil {
				return ipv4.String()
			}
		}
	}
	return ""
}
