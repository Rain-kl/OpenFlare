package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/pkg/geoip"
	"github.com/Rain-kl/Wavelet/pkg/geoip/iputil"
	"github.com/Rain-kl/Wavelet/pkg/utils"
)

const (
	defaultMainConfigRelativePath          = "etc/nginx/nginx.conf"
	defaultRouteConfigRelativePath         = "etc/nginx/conf.d/openflare_routes.conf"
	defaultCertDirRelativePath             = "etc/nginx/certs"
	defaultLuaDirRelativePath              = "etc/nginx/lua"
	defaultRuntimeConfigDirRelativePath    = "etc/openflare"
	defaultPagesDirRelativePath            = "var/lib/openflare/pages"
	defaultMMDBRelativePath                = "etc/openflare/GeoLite2-Country.mmdb"
	defaultAccessLogRelativePath           = "var/log/openflare/access.log"
	defaultStateRelativePath               = "var/lib/openflare/agent-state.json"
	defaultObservabilityBufferRelativePath = "var/lib/openflare/observability-buffer.json"
	defaultOpenRestyObservabilityPort      = 18081
	defaultObservabilityReplayMinutes      = 15
	defaultMMDBUpdateInterval              = 24 * time.Hour
	defaultMMDBDownloadURL                 = "https://raw.githubusercontent.com/Loyalsoldier/geoip/release/GeoLite2-Country.mmdb"
)

var (
	lookupOutboundIP = geoip.GetOutboundIP
	lookupLocalIP    = detectLocalNodeIP
)

type Config struct {
	ServerURL                  string              `json:"server_url"`
	AccessToken                string              `json:"agent_token"`
	DiscoveryToken             string              `json:"discovery_token"`
	NodeName                   string              `json:"node_name"`
	NodeIP                     string              `json:"node_ip"`
	Version                    string              `json:"-"`
	ExtVersion                 string              `json:"-"`
	OpenrestyPath              string              `json:"openresty_path"`
	OpenrestyResolvers         []string            `json:"openresty_resolvers,omitempty"`
	DataDir                    string              `json:"data_dir"`
	MainConfigPath             string              `json:"main_config_path"`
	RouteConfigPath            string              `json:"route_config_path"`
	AccessLogPath              string              `json:"access_log_path"`
	CertDir                    string              `json:"cert_dir"`
	OpenrestyCertDir           string              `json:"openresty_cert_dir"`
	LuaDir                     string              `json:"lua_dir"`
	OpenrestyLuaDir            string              `json:"openresty_lua_dir"`
	RuntimeConfigDir           string              `json:"runtime_config_dir"`
	PagesDir                   string              `json:"pages_dir"`
	MMDBPath                   string              `json:"mmdb_path"`
	MMDBUpdateInterval         MillisecondDuration `json:"mmdb_update_interval"`
	MMDBDownloadURL            string              `json:"mmdb_download_url"`
	OpenrestyObservabilityPort int                 `json:"openresty_observability_port"`
	ObservabilityBufferPath    string              `json:"observability_buffer_path"`
	ObservabilityReplayMinutes int                 `json:"observability_replay_minutes"`
	StatePath                  string              `json:"state_path"`
	HeartbeatInterval          MillisecondDuration `json:"heartbeat_interval"`
	RequestTimeout             MillisecondDuration `json:"request_timeout"`
	configPath                 string
}

type configFile struct {
	ServerURL                  string              `json:"server_url"`
	AccessToken                string              `json:"agent_token"`
	DiscoveryToken             string              `json:"discovery_token"`
	NodeName                   string              `json:"node_name"`
	NodeIP                     string              `json:"node_ip"`
	OpenrestyPath              string              `json:"openresty_path"`
	OpenrestyResolvers         []string            `json:"openresty_resolvers"`
	DataDir                    string              `json:"data_dir"`
	MainConfigPath             string              `json:"main_config_path"`
	RouteConfigPath            string              `json:"route_config_path"`
	AccessLogPath              string              `json:"access_log_path"`
	CertDir                    string              `json:"cert_dir"`
	OpenrestyCertDir           string              `json:"openresty_cert_dir"`
	LuaDir                     string              `json:"lua_dir"`
	OpenrestyLuaDir            string              `json:"openresty_lua_dir"`
	RuntimeConfigDir           string              `json:"runtime_config_dir"`
	PagesDir                   string              `json:"pages_dir"`
	MMDBPath                   string              `json:"mmdb_path"`
	MMDBUpdateInterval         MillisecondDuration `json:"mmdb_update_interval"`
	MMDBDownloadURL            string              `json:"mmdb_download_url"`
	OpenrestyObservabilityPort int                 `json:"openresty_observability_port"`
	ObservabilityBufferPath    string              `json:"observability_buffer_path"`
	ObservabilityReplayMinutes int                 `json:"observability_replay_minutes"`
	StatePath                  string              `json:"state_path"`
	HeartbeatInterval          MillisecondDuration `json:"heartbeat_interval"`
	RequestTimeout             MillisecondDuration `json:"request_timeout"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	file := &configFile{}
	if err == nil {
		if err = json.Unmarshal(data, file); err != nil {
			return nil, err
		}
	}
	if err != nil && !hasEnvConfig() {
		return nil, err
	}
	cfg := &Config{
		ServerURL:                  file.ServerURL,
		AccessToken:                file.AccessToken,
		DiscoveryToken:             file.DiscoveryToken,
		NodeName:                   file.NodeName,
		NodeIP:                     file.NodeIP,
		OpenrestyPath:              file.OpenrestyPath,
		OpenrestyResolvers:         append([]string{}, file.OpenrestyResolvers...),
		DataDir:                    file.DataDir,
		MainConfigPath:             file.MainConfigPath,
		RouteConfigPath:            file.RouteConfigPath,
		AccessLogPath:              file.AccessLogPath,
		CertDir:                    file.CertDir,
		OpenrestyCertDir:           file.OpenrestyCertDir,
		LuaDir:                     file.LuaDir,
		OpenrestyLuaDir:            file.OpenrestyLuaDir,
		RuntimeConfigDir:           file.RuntimeConfigDir,
		PagesDir:                   file.PagesDir,
		MMDBPath:                   file.MMDBPath,
		MMDBUpdateInterval:         file.MMDBUpdateInterval,
		MMDBDownloadURL:            file.MMDBDownloadURL,
		OpenrestyObservabilityPort: file.OpenrestyObservabilityPort,
		ObservabilityBufferPath:    file.ObservabilityBufferPath,
		ObservabilityReplayMinutes: file.ObservabilityReplayMinutes,
		StatePath:                  file.StatePath,
		HeartbeatInterval:          file.HeartbeatInterval,
		RequestTimeout:             file.RequestTimeout,
	}
	cfg.configPath = path
	applyEnvOverrides(cfg)
	applyDefaults(cfg, filepath.Dir(path))
	if err = validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func applyDefaults(cfg *Config, baseDir string) {
	baseDir = filepath.Clean(baseDir)
	cfg.Version = Version
	cfg.OpenrestyResolvers = utils.UniqueAndCleanStringSlice(cfg.OpenrestyResolvers)
	if cfg.OpenrestyPath == "" {
		cfg.OpenrestyPath = "openresty"
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
	if cfg.MainConfigPath == "" {
		cfg.MainConfigPath = joinManagedPath(cfg.DataDir, defaultMainConfigRelativePath)
	}
	if cfg.RouteConfigPath == "" {
		cfg.RouteConfigPath = joinManagedPath(cfg.DataDir, defaultRouteConfigRelativePath)
	}
	if cfg.AccessLogPath == "" {
		cfg.AccessLogPath = joinManagedPath(cfg.DataDir, defaultAccessLogRelativePath)
	}
	if cfg.StatePath == "" {
		cfg.StatePath = joinManagedPath(cfg.DataDir, defaultStateRelativePath)
	}
	if cfg.CertDir == "" {
		cfg.CertDir = joinManagedPath(cfg.DataDir, defaultCertDirRelativePath)
	}
	if cfg.OpenrestyCertDir == "" {
		cfg.OpenrestyCertDir = cfg.CertDir
	}
	if cfg.LuaDir == "" {
		cfg.LuaDir = joinManagedPath(cfg.DataDir, defaultLuaDirRelativePath)
	}
	if cfg.OpenrestyLuaDir == "" {
		cfg.OpenrestyLuaDir = cfg.LuaDir
	}
	if cfg.RuntimeConfigDir == "" {
		cfg.RuntimeConfigDir = joinManagedPath(cfg.DataDir, defaultRuntimeConfigDirRelativePath)
	}
	if cfg.PagesDir == "" {
		cfg.PagesDir = joinManagedPath(cfg.DataDir, defaultPagesDirRelativePath)
	}
	if cfg.MMDBPath == "" {
		cfg.MMDBPath = joinManagedPath(cfg.DataDir, defaultMMDBRelativePath)
	}
	if cfg.MMDBUpdateInterval <= 0 {
		cfg.MMDBUpdateInterval = MillisecondDuration(defaultMMDBUpdateInterval)
	}
	if cfg.MMDBDownloadURL == "" {
		cfg.MMDBDownloadURL = defaultMMDBDownloadURL
	}
	if cfg.OpenrestyObservabilityPort <= 0 {
		cfg.OpenrestyObservabilityPort = defaultOpenRestyObservabilityPort
	}
	if cfg.ObservabilityBufferPath == "" {
		cfg.ObservabilityBufferPath = joinManagedPath(cfg.DataDir, defaultObservabilityBufferRelativePath)
	}
	if cfg.ObservabilityReplayMinutes <= 0 {
		cfg.ObservabilityReplayMinutes = defaultObservabilityReplayMinutes
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = MillisecondDuration(10 * time.Second)
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
	paths := []*string{
		&cfg.DataDir,
		&cfg.MainConfigPath,
		&cfg.RouteConfigPath,
		&cfg.AccessLogPath,
		&cfg.CertDir,
		&cfg.OpenrestyCertDir,
		&cfg.LuaDir,
		&cfg.OpenrestyLuaDir,
		&cfg.RuntimeConfigDir,
		&cfg.PagesDir,
		&cfg.StatePath,
		&cfg.ObservabilityBufferPath,
		&cfg.MMDBPath,
	}
	for _, p := range paths {
		if usesSlashPath(*p) {
			*p = filepath.ToSlash(*p)
		}
	}
}

func hasEnvConfig() bool {
	for _, key := range []string{
		"OPENFLARE_SERVER_URL",
		"OPENFLARE_AGENT_TOKEN",
		"OPENFLARE_DISCOVERY_TOKEN",
		"OPENFLARE_NODE_NAME",
		"OPENFLARE_NODE_IP",
		"OPENFLARE_DATA_DIR",
		"OPENFLARE_OPENRESTY_PATH",
		"OPENFLARE_PAGES_DIR",
		"OPENFLARE_HEARTBEAT_INTERVAL",
		"OPENFLARE_REQUEST_TIMEOUT",
		"OPENFLARE_OPENRESTY_OBSERVABILITY_PORT",
		"OPENFLARE_MMDB_PATH",
		"OPENFLARE_MMDB_UPDATE_INTERVAL",
		"OPENFLARE_MMDB_DOWNLOAD_URL",
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
	overrideString("OPENFLARE_AGENT_TOKEN", &cfg.AccessToken)
	overrideString("OPENFLARE_DISCOVERY_TOKEN", &cfg.DiscoveryToken)
	overrideString("OPENFLARE_NODE_NAME", &cfg.NodeName)
	overrideString("OPENFLARE_NODE_IP", &cfg.NodeIP)
	overrideString("OPENFLARE_DATA_DIR", &cfg.DataDir)
	overrideString("OPENFLARE_OPENRESTY_PATH", &cfg.OpenrestyPath)
	overrideString("OPENFLARE_PAGES_DIR", &cfg.PagesDir)
	overrideString("OPENFLARE_MMDB_PATH", &cfg.MMDBPath)
	overrideString("OPENFLARE_MMDB_DOWNLOAD_URL", &cfg.MMDBDownloadURL)
	if value := strings.TrimSpace(os.Getenv("OPENFLARE_HEARTBEAT_INTERVAL")); value != "" {
		if duration, err := parseDurationValue(value); err == nil {
			cfg.HeartbeatInterval = duration
		}
	}
	if value := strings.TrimSpace(os.Getenv("OPENFLARE_REQUEST_TIMEOUT")); value != "" {
		if duration, err := parseDurationValue(value); err == nil {
			cfg.RequestTimeout = duration
		}
	}
	if value := strings.TrimSpace(os.Getenv("OPENFLARE_MMDB_UPDATE_INTERVAL")); value != "" {
		if duration, err := parseDurationValue(value); err == nil {
			cfg.MMDBUpdateInterval = duration
		}
	}
	if value := strings.TrimSpace(os.Getenv("OPENFLARE_OPENRESTY_OBSERVABILITY_PORT")); value != "" {
		var port int
		if _, err := fmt.Sscanf(value, "%d", &port); err == nil {
			cfg.OpenrestyObservabilityPort = port
		}
	}
}

func parseDurationValue(value string) (MillisecondDuration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}
	if parsed, err := time.ParseDuration(trimmed); err == nil {
		return MillisecondDuration(parsed), nil
	}
	ms, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, err
	}
	return MillisecondDuration(time.Duration(ms) * time.Millisecond), nil
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
	if strings.TrimSpace(cfg.AccessToken) == "" && strings.TrimSpace(cfg.DiscoveryToken) == "" {
		return errors.New("agent_token 和 discovery_token 不能同时为空")
	}
	if cfg.NodeName == "" {
		return errors.New("node_name 不能为空")
	}
	if cfg.NodeIP == "" {
		return errors.New("node_ip 不能为空")
	}
	if cfg.OpenrestyObservabilityPort <= 0 || cfg.OpenrestyObservabilityPort > 65535 {
		return errors.New("openresty_observability_port 必须在 1-65535 之间")
	}
	if cfg.ObservabilityReplayMinutes <= 0 {
		return errors.New("observability_replay_minutes 必须大于 0")
	}
	if cfg.MMDBUpdateInterval <= 0 {
		return errors.New("mmdb_update_interval 必须大于 0")
	}
	return nil
}

func (cfg *Config) InitialAuthToken() string {
	if cfg == nil {
		return ""
	}
	if token := strings.TrimSpace(cfg.AccessToken); token != "" {
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
	if ip := detectOutboundNodeIP(); ip != "" {
		return ip
	}
	return lookupLocalIP()
}

func detectOutboundNodeIP() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ip, err := lookupOutboundIP(ctx)
	if err != nil || ip == nil {
		return ""
	}
	return ip.String()
}

func detectLocalNodeIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	bestIP := ""
	bestPriority := -1
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
			ipv4 := normalizeIPv4(ipNet.IP)
			priority := nodeIPPriority(ipv4)
			if priority > bestPriority {
				bestIP = ipv4.String()
				bestPriority = priority
			}
			if bestPriority == 2 {
				return bestIP
			}
		}
	}
	return bestIP
}

func normalizeIPv4(ip net.IP) net.IP {
	if ip == nil {
		return nil
	}
	return ip.To4()
}

func nodeIPPriority(ip net.IP) int {
	return iputil.Score(ip)
}
