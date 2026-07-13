// Package config loads and persists agent daemon configuration.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/edge/nodeip"
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
	defaultCityMMDBRelativePath            = "etc/openflare/GeoLite2-City.mmdb"
	defaultAccessLogRelativePath           = "var/log/openflare/access.log"
	defaultStateRelativePath               = "var/lib/openflare/agent-state.json"
	defaultObservabilityBufferRelativePath = "var/lib/openflare/observability-buffer.json"
	defaultOpenRestyObservabilityPort      = 18081
	defaultObservabilityReplayMinutes      = 15
	defaultMMDBUpdateInterval              = 24 * time.Hour
	defaultMMDBDownloadURL                 = "https://github.com/FyraLabs/geolite2/releases/latest/download/GeoLite2-Country.mmdb"
	defaultCityMMDBDownloadURL             = "https://github.com/FyraLabs/geolite2/releases/latest/download/GeoLite2-City.mmdb"
	defaultHeartbeatInterval               = 10 * time.Second
	defaultRequestTimeout                  = 10 * time.Second
	configFilePerm                         = 0o600
)

// Config holds the full runtime configuration for the OpenFlare agent.
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
	CityMMDBPath               string              `json:"city_mmdb_path"`
	MMDBUpdateInterval         MillisecondDuration `json:"mmdb_update_interval"`
	MMDBDownloadURL            string              `json:"mmdb_download_url"`
	CityMMDBDownloadURL        string              `json:"city_mmdb_download_url"`
	OpenrestyObservabilityPort int                 `json:"openresty_observability_port"`
	ObservabilityBufferPath    string              `json:"observability_buffer_path"`
	ObservabilityReplayMinutes int                 `json:"observability_replay_minutes"`
	StatePath                  string              `json:"state_path"`
	HeartbeatInterval          MillisecondDuration `json:"heartbeat_interval"`
	RequestTimeout             MillisecondDuration `json:"request_timeout"`
	configPath                 string              `json:"-"`
	NodeIPConfigured           bool                `json:"-"`
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
	CityMMDBPath               string              `json:"city_mmdb_path"`
	MMDBUpdateInterval         MillisecondDuration `json:"mmdb_update_interval"`
	MMDBDownloadURL            string              `json:"mmdb_download_url"`
	CityMMDBDownloadURL        string              `json:"city_mmdb_download_url"`
	OpenrestyObservabilityPort int                 `json:"openresty_observability_port"`
	ObservabilityBufferPath    string              `json:"observability_buffer_path"`
	ObservabilityReplayMinutes int                 `json:"observability_replay_minutes"`
	StatePath                  string              `json:"state_path"`
	HeartbeatInterval          MillisecondDuration `json:"heartbeat_interval"`
	RequestTimeout             MillisecondDuration `json:"request_timeout"`
}

func transferPersistedConfig(dst, src any) error {
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

// Load reads and parses the agent configuration file at the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is the configured agent config location
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
	cfg := &Config{}
	if err == nil {
		if err = transferPersistedConfig(cfg, file); err != nil {
			return nil, err
		}
	}
	cfg.configPath = path
	applyEnvOverrides(cfg)
	cfg.NodeIPConfigured = cfg.NodeIP != ""
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
	applyAgentIdentityDefaults(cfg)
	applyAgentPathDefaults(cfg, baseDir)
	applyAgentTimingDefaults(cfg)
	normalizeManagedPaths(cfg)
}

func applyAgentIdentityDefaults(cfg *Config) {
	if cfg.OpenrestyPath == "" {
		cfg.OpenrestyPath = "openresty"
	}
	if cfg.NodeName == "" {
		cfg.NodeName = detectHostname()
	}
	if cfg.NodeIP == "" {
		cfg.NodeIP = nodeip.Detect()
	}
}

func applyAgentPathDefaults(cfg *Config, baseDir string) {
	if cfg.DataDir == "" {
		cfg.DataDir = filepath.Join(baseDir, "data")
	}
	type managedPathDefault struct {
		target   *string
		relative string
	}
	pathDefaults := []managedPathDefault{
		{&cfg.MainConfigPath, defaultMainConfigRelativePath},
		{&cfg.RouteConfigPath, defaultRouteConfigRelativePath},
		{&cfg.AccessLogPath, defaultAccessLogRelativePath},
		{&cfg.StatePath, defaultStateRelativePath},
		{&cfg.CertDir, defaultCertDirRelativePath},
		{&cfg.LuaDir, defaultLuaDirRelativePath},
		{&cfg.RuntimeConfigDir, defaultRuntimeConfigDirRelativePath},
		{&cfg.PagesDir, defaultPagesDirRelativePath},
		{&cfg.MMDBPath, defaultMMDBRelativePath},
		{&cfg.CityMMDBPath, defaultCityMMDBRelativePath},
		{&cfg.ObservabilityBufferPath, defaultObservabilityBufferRelativePath},
	}
	for _, item := range pathDefaults {
		if strings.TrimSpace(*item.target) == "" {
			*item.target = joinManagedPath(cfg.DataDir, item.relative)
		}
	}
	if cfg.OpenrestyCertDir == "" {
		cfg.OpenrestyCertDir = cfg.CertDir
	}
	if cfg.OpenrestyLuaDir == "" {
		cfg.OpenrestyLuaDir = cfg.LuaDir
	}
}

func applyAgentTimingDefaults(cfg *Config) {
	if cfg.MMDBUpdateInterval <= 0 {
		cfg.MMDBUpdateInterval = MillisecondDuration(defaultMMDBUpdateInterval)
	}
	if cfg.MMDBDownloadURL == "" {
		cfg.MMDBDownloadURL = defaultMMDBDownloadURL
	}
	if cfg.CityMMDBDownloadURL == "" {
		cfg.CityMMDBDownloadURL = defaultCityMMDBDownloadURL
	}
	if cfg.OpenrestyObservabilityPort <= 0 {
		cfg.OpenrestyObservabilityPort = defaultOpenRestyObservabilityPort
	}
	if cfg.ObservabilityReplayMinutes <= 0 {
		cfg.ObservabilityReplayMinutes = defaultObservabilityReplayMinutes
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = MillisecondDuration(defaultHeartbeatInterval)
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = MillisecondDuration(defaultRequestTimeout)
	}
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
		&cfg.CityMMDBPath,
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
		"OPENFLARE_CITY_MMDB_PATH",
		"OPENFLARE_CITY_MMDB_DOWNLOAD_URL",
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
	overrideString("OPENFLARE_CITY_MMDB_PATH", &cfg.CityMMDBPath)
	overrideString("OPENFLARE_CITY_MMDB_DOWNLOAD_URL", &cfg.CityMMDBDownloadURL)
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

// InitialAuthToken returns the agent access token, falling back to the discovery token if absent.
func (cfg *Config) InitialAuthToken() string {
	if cfg == nil {
		return ""
	}
	if token := strings.TrimSpace(cfg.AccessToken); token != "" {
		return token
	}
	return strings.TrimSpace(cfg.DiscoveryToken)
}

func (cfg *Config) toConfigFile() configFile {
	var file configFile
	if err := transferPersistedConfig(&file, cfg); err != nil {
		return configFile{}
	}
	return file
}

// Save persists the current configuration back to its original file path.
func (cfg *Config) Save() error {
	if cfg == nil {
		return errors.New("config 不能为空")
	}
	if cfg.configPath == "" {
		return errors.New("config path 未初始化")
	}
	data, err := json.MarshalIndent(cfg.toConfigFile(), "", "  ") //nolint:gosec // agent token must be persisted in local config file
	if err != nil {
		return err
	}
	return os.WriteFile(cfg.configPath, data, configFilePerm)
}

func detectHostname() string {
	host, err := os.Hostname()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(host)
}
