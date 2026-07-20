// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import "time"

// 配置键常量 - 所有系统配置的 key 定义
const (
	ConfigKeyUploadAllowedExtensions          = "upload_allowed_extensions"           // 允许上传的文件扩展名，逗号分隔
	ConfigKeySiteName                         = "site_name"                           // 站点名称
	ConfigKeyPasswordLoginEnabled             = "password_login_enabled"              // 是否允许密码登录
	ConfigKeyRegistrationEnabled              = "registration_enabled"                // 是否允许注册
	ConfigKeyPasswordRegisterEnabled          = "password_register_enabled"           // 是否允许密码注册
	ConfigKeyOIDCLoginEnabled                 = "oidc_login_enabled"                  // 是否允许 OIDC 登录
	ConfigKeyMaxAPIKeysPerUser                = "max_api_keys_per_user"               //nolint:gosec // false positive: config key name. 每个用户最大 API Key 数量
	ConfigKeyCapLoginEnabled                  = "cap_login_enabled"                   // 是否启用登录人机验证
	ConfigKeyCapAutoSolve                     = "cap_auto_solve"                      // 打开页面后是否自动开始计算（false 则需用户手动点击）
	ConfigKeyCapChallengeCount                = "cap_challenge_count"                 // 客户端需求解的 PoW 难题总数，默认 1，推荐 1～5
	ConfigKeyCapChallengeSize                 = "cap_challenge_size"                  // 人机验证盐值长度
	ConfigKeyCapChallengeDifficulty           = "cap_challenge_difficulty"            // 人机验证 PoW 难度（目标前缀长度）
	ConfigKeyCapChallengeTTL                  = "cap_challenge_ttl_seconds"           // 人机验证难题有效时间（秒）
	ConfigKeyCapTokenTTL                      = "cap_token_ttl_seconds"               //nolint:gosec // false positive: config key name. 人机验证兑换凭证有效时间（秒）
	ConfigKeyServerAddress                    = "server_address"                      // 服务器地址
	ConfigKeySMTPHost                         = "smtp_host"                           // SMTP 服务器地址
	ConfigKeySMTPPort                         = "smtp_port"                           // SMTP 端口
	ConfigKeySMTPUsername                     = "smtp_username"                       // SMTP 账户
	ConfigKeySMTPPassword                     = "smtp_password"                       // SMTP 访问凭证
	ConfigKeyEmailLoginVerificationEnabled    = "email_login_verification_enabled"    // 是否启用邮箱登录验证
	ConfigKeyEmailRegisterVerificationEnabled = "email_register_verification_enabled" // 是否启用邮箱注册验证
	ConfigKeyMenuDisplayConfig                = "menu_display_config"                 // 目录显示配置 (JSON 字符串)
	ConfigKeySearchEngineIndexingEnabled      = "search_engine_indexing_enabled"      // 是否允许搜索引擎检索
	ConfigKeyFileAccessWhitelist              = "file_access_whitelist"               // 免登录访问的文件业务类型白名单 (JSON 数组格式)
	ConfigKeyDiskCacheMaxSizeMB               = "disk_cache_max_size_mb"              // 磁盘缓存最大空间大小 (MB)
	ConfigKeyDiskCacheTTLMinutes              = "disk_cache_ttl_minutes"              // 磁盘缓存默认有效期 (分钟)
	ConfigKeyDiskCacheLRUEnabled              = "disk_cache_lru_enabled"              // 是否启用 LRU 淘汰机制
	ConfigKeyLoginSessionTTLHours             = "login_session_ttl_hours"             // 登录会话过期时间 (小时，0表示浏览器关闭后自动退出登录，-1表示永不过期)
	ConfigKeyUpdateUpstreamRepository         = "update_upstream_repository"          // GitHub Actions Release 上游仓库
	ConfigKeyStorageConfig                    = "storage_config"                      // 文件存储配置 (JSON)
	ConfigKeyRelayFRPSWebUIEnabled            = "relay_frps_web_ui_enabled"           // 是否启用 FRPS 内置 Web 界面
	ConfigKeyRelayFRPSWebUIPort               = "relay_frps_web_ui_port"              // FRPS 内置 Web 界面端口

	// OpenFlare 业务配置（从 of_options 迁移）
	ConfigKeyAgentDiscoveryToken              = "agent_discovery_token"                //nolint:gosec // false positive: config key name. Agent 发现令牌
	ConfigKeyAgentHeartbeatInterval           = "agent_heartbeat_interval"             // Agent 心跳间隔（毫秒）
	ConfigKeyAgentWebsocketUpgradeEnabled     = "agent_websocket_upgrade_enabled"      // Agent WebSocket 升级开关
	ConfigKeyNodeOfflineThreshold             = "node_offline_threshold"               // 节点离线阈值（毫秒）
	ConfigKeyAgentUpdateRepo                  = "agent_update_repo"                    // Agent 更新仓库
	ConfigKeyGeoIPProvider                    = "geoip_provider"                       // GeoIP 服务商
	ConfigKeyDatabaseAutoCleanupEnabled       = "database_auto_cleanup_enabled"        // 数据库自动清理开关
	ConfigKeyDatabaseAutoCleanupRetentionDays = "database_auto_cleanup_retention_days" // 数据库保留天数

	// Pages 静态托管配置
	ConfigKeyPagesMaxPackageSizeMB = "pages_max_package_size_mb" // Pages 部署包上传大小上限（MiB）
	ConfigKeyPagesMaxHistoryCount  = "pages_max_history_count"   // Pages 每个项目最大历史部署保留数（0 表示不限制）

	// UptimeKuma 集成配置
	ConfigKeyUptimeKumaEnabled       = "uptime_kuma_enabled"        // UptimeKuma 集成开关
	ConfigKeyUptimeKumaURL           = "uptime_kuma_url"            // UptimeKuma URL
	ConfigKeyUptimeKumaUsername      = "uptime_kuma_username"       // UptimeKuma 用户名
	ConfigKeyUptimeKumaPassword      = "uptime_kuma_password"       //nolint:gosec // false positive: config key name. UptimeKuma 密码
	ConfigKeyUptimeKumaMonitorScope  = "uptime_kuma_monitor_scope"  // UptimeKuma 监控范围
	ConfigKeyUptimeKumaSelectedSites = "uptime_kuma_selected_sites" // UptimeKuma 选定站点
	ConfigKeyUptimeKumaSyncInterval  = "uptime_kuma_sync_interval"  // UptimeKuma 同步间隔（分钟）
	ConfigKeyUptimeKumaInterval      = "uptime_kuma_interval"       // UptimeKuma 监控间隔（秒）
	ConfigKeyUptimeKumaRetry         = "uptime_kuma_retry"          // UptimeKuma 重试次数
	ConfigKeyUptimeKumaRetryInterval = "uptime_kuma_retry_interval" // UptimeKuma 重试间隔（秒）
	ConfigKeyUptimeKumaTimeout       = "uptime_kuma_timeout"        // UptimeKuma 超时（秒）

	// OpenResty 配置
	ConfigKeyOpenRestyDefaultServerReturnStatus    = "openresty_default_server_return_status"    // 默认服务器返回状态码
	ConfigKeyOpenRestyWorkerProcesses              = "openresty_worker_processes"                // Worker 进程数
	ConfigKeyOpenRestyWorkerConnections            = "openresty_worker_connections"              // Worker 连接数
	ConfigKeyOpenRestyWorkerRlimitNofile           = "openresty_worker_rlimit_nofile"            // Worker 文件描述符限制
	ConfigKeyOpenRestyEventsUse                    = "openresty_events_use"                      // 事件模型
	ConfigKeyOpenRestyEventsMultiAcceptEnabled     = "openresty_events_multi_accept_enabled"     // 多路接受开关
	ConfigKeyOpenRestyKeepaliveTimeout             = "openresty_keepalive_timeout"               // Keepalive 超时（秒）
	ConfigKeyOpenRestyKeepaliveRequests            = "openresty_keepalive_requests"              // Keepalive 请求数
	ConfigKeyOpenRestyClientHeaderTimeout          = "openresty_client_header_timeout"           // 客户端头超时（秒）
	ConfigKeyOpenRestyClientBodyTimeout            = "openresty_client_body_timeout"             // 客户端体超时（秒）
	ConfigKeyOpenRestyClientMaxBodySize            = "openresty_client_max_body_size"            // 客户端最大体大小
	ConfigKeyOpenRestyLargeClientHeaderBuffers     = "openresty_large_client_header_buffers"     // 大客户端头缓冲区
	ConfigKeyOpenRestySendTimeout                  = "openresty_send_timeout"                    // 发送超时（秒）
	ConfigKeyOpenRestyResolvers                    = "openresty_resolvers"                       // DNS 解析器
	ConfigKeyOpenRestyProxyConnectTimeout          = "openresty_proxy_connect_timeout"           // 代理连接超时（秒）
	ConfigKeyOpenRestyProxySendTimeout             = "openresty_proxy_send_timeout"              // 代理发送超时（秒）
	ConfigKeyOpenRestyProxyReadTimeout             = "openresty_proxy_read_timeout"              // 代理读取超时（秒）
	ConfigKeyOpenRestyWebsocketEnabled             = "openresty_websocket_enabled"               // WebSocket 支持开关
	ConfigKeyOpenRestyHTTP3Enabled                 = "openresty_http3_enabled"                   // HTTP/3 支持开关
	ConfigKeyOpenRestyProxyRequestBufferingEnabled = "openresty_proxy_request_buffering_enabled" // 代理请求缓冲开关
	ConfigKeyOpenRestyProxyBufferingEnabled        = "openresty_proxy_buffering_enabled"         // 代理响应缓冲开关
	ConfigKeyOpenRestyProxyBuffers                 = "openresty_proxy_buffers"                   // 代理缓冲区
	ConfigKeyOpenRestyProxyBufferSize              = "openresty_proxy_buffer_size"               // 代理缓冲区大小
	ConfigKeyOpenRestyProxyBusyBuffersSize         = "openresty_proxy_busy_buffers_size"         // 代理繁忙缓冲区大小
	ConfigKeyOpenRestyGzipEnabled                  = "openresty_gzip_enabled"                    // Gzip 压缩开关
	ConfigKeyOpenRestyGzipMinLength                = "openresty_gzip_min_length"                 // Gzip 最小长度
	ConfigKeyOpenRestyGzipCompLevel                = "openresty_gzip_comp_level"                 // Gzip 压缩级别
	ConfigKeyOpenRestyCacheEnabled                 = "openresty_cache_enabled"                   // 缓存开关
	ConfigKeyOpenRestyCachePath                    = "openresty_cache_path"                      // 缓存路径
	ConfigKeyOpenRestyCacheLevels                  = "openresty_cache_levels"                    // 缓存层级
	ConfigKeyOpenRestyCacheInactive                = "openresty_cache_inactive"                  // 缓存不活跃时间
	ConfigKeyOpenRestyCacheMaxSize                 = "openresty_cache_max_size"                  // 缓存最大大小
	ConfigKeyOpenRestyCacheKeyTemplate             = "openresty_cache_key_template"              // 缓存键模板
	ConfigKeyOpenRestyCacheLockEnabled             = "openresty_cache_lock_enabled"              // 缓存锁开关
	ConfigKeyOpenRestyCacheLockTimeout             = "openresty_cache_lock_timeout"              // 缓存锁超时
	ConfigKeyOpenRestyCacheUseStale                = "openresty_cache_use_stale"                 // 缓存失效策略
	ConfigKeyOpenRestyMainConfigTemplate           = "openresty_main_config_template"            // 主配置模板
	ConfigKeyOpenRestyDefaultLimitConnPerServer    = "openresty_default_limit_conn_per_server"   // 默认站点并发连接
	ConfigKeyOpenRestyDefaultLimitConnPerIP        = "openresty_default_limit_conn_per_ip"       // 默认单 IP 并发连接
	ConfigKeyOpenRestyDefaultLimitRate             = "openresty_default_limit_rate"              // 默认单请求带宽
	ConfigKeyOpenRestyDefaultLimitReqPerIP         = "openresty_default_limit_req_per_ip"        // 默认单 IP 请求频率限制
)

const (
	// ConfigVisibilityHidden 表示配置不通过公共配置接口暴露
	ConfigVisibilityHidden = 0
	// ConfigVisibilityVisible 表示配置通过公共配置接口暴露
	ConfigVisibilityVisible = 1
)

// SystemConfig 系统配置实体
type SystemConfig struct {
	Key         string    `json:"key" gorm:"primaryKey;size:64;not null"`
	Value       string    `json:"value" gorm:"type:text;not null"`
	Type        string    `json:"type" gorm:"size:32;not null;default:'system'"`
	Visibility  int       `json:"visibility" gorm:"not null;default:0"`
	Description string    `json:"description" gorm:"size:255"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName 表名
func (SystemConfig) TableName() string {
	return "w_system_configs"
}
