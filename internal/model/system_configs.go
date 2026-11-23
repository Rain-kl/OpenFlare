// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import "time"

// 配置键常量 - 所有系统配置的 key 定义
const (
	ConfigKeyUploadAllowedExtensions          = "upload_allowed_extensions" // 允许上传的文件扩展名，逗号分隔
	ConfigKeySiteName                         = "site_name"                 // 站点名称
	ConfigKeyPasswordLoginEnabled             = "password_login_enabled"    // 是否允许密码登录
	ConfigKeyRegistrationEnabled              = "registration_enabled"      // 是否允许注册
	ConfigKeyPasswordRegisterEnabled          = "password_register_enabled" // 是否允许密码注册
	ConfigKeyOIDCLoginEnabled                 = "oidc_login_enabled"        // 是否允许 OIDC 登录
	ConfigKeyMaxAPIKeysPerUser                = "max_api_keys_per_user"     //nolint:gosec // false positive: config key name. 每个用户最大 API Key 数量
	ConfigKeyCapLoginEnabled                  = "cap_login_enabled"         // 是否启用登录人机验证
	ConfigKeyCapAutoSolve                     = "cap_auto_solve"            // 打开页面后是否自动开始计算（false 则需用户手动点击）
	ConfigKeyCapChallengeCount                = "cap_challenge_count"       // 客户端需求解的 PoW 难题总数，默认 1，推荐 1～5
	ConfigKeyCapChallengeSize                 = "cap_challenge_size"        // 人机验证盐值长度
	ConfigKeyCapChallengeDifficulty           = "cap_challenge_difficulty"  // 人机验证 PoW 难度（目标前缀长度）
	ConfigKeyCapChallengeTTL                  = "cap_challenge_ttl_seconds" // 人机验证难题有效时间（秒）
	ConfigKeyCapTokenTTL                      = "cap_token_ttl_seconds"     //nolint:gosec // false positive: config key name. 人机验证兑换凭证有效时间（秒）
	ConfigKeyServerAddress                    = "server_address"            // 服务器地址
	ConfigKeyAutomationPublicURL              = "automation_public_url"
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

	// 隧道与中继配置
	ConfigKeyRelayDiscoveryToken     = "relay_discovery_token"     //nolint:gosec // false positive: config key name. 中继节点自动发现注册令牌
	ConfigKeyTunnelUpdateRepo        = "tunnel_update_repo"        // 隧道组件（openflare-relay / openflared）更新仓库
	ConfigKeyTunnelHeartbeatInterval = "tunnel_heartbeat_interval" // 隧道心跳间隔（毫秒）
	ConfigKeyTunnelLeaseTTLSeconds   = "tunnel_lease_ttl_seconds"  // 中继连接租约有效时间（秒）
	ConfigKeyTunnelDefaultHARelays   = "tunnel_default_ha_relays"  // 默认并行挂载的冗余中继节点数

	// OpenFlare 业务配置（从 of_options 迁移）
	ConfigKeyAgentDiscoveryToken          = "agent_discovery_token"           //nolint:gosec // false positive: config key name. Agent 发现令牌
	ConfigKeyAgentHeartbeatInterval       = "agent_heartbeat_interval"        // Agent 心跳间隔（毫秒）
	ConfigKeyAgentWebsocketUpgradeEnabled = "agent_websocket_upgrade_enabled" // Agent WebSocket 升级开关
	ConfigKeyNodeOfflineThreshold         = "node_offline_threshold"          // 节点离线阈值（毫秒）
	ConfigKeyAgentUpdateRepo              = "agent_update_repo"               // Agent 更新仓库

	ConfigKeyGeoIPProvider = "geoip_provider" // GeoIP 服务商

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
	ConfigKeyEdgeTrustedProxyCIDRs         = "edge_trusted_proxy_cidrs"           // 可信代理 CIDR JSON 数组
	ConfigKeyEdgeDefaultLimitConnPerServer = "edge_default_limit_conn_per_server" // 默认站点并发连接
	ConfigKeyEdgeDefaultLimitConnPerIP     = "edge_default_limit_conn_per_ip"     // 默认单 IP 并发连接
	ConfigKeyEdgeDefaultLimitRate          = "edge_default_limit_rate"            // 默认单请求带宽
	ConfigKeyEdgeDefaultLimitReqPerIP      = "edge_default_limit_req_per_ip"      // 默认单 IP 请求频率限制

	// 源站错误页
	ConfigKeyOriginErrorPageEnabled     = "origin_error_page_enabled"      // 是否启用源站错误页
	ConfigKeyOriginErrorPageStatusCodes = "origin_error_page_status_codes" // 源站错误页触发状态码标签 JSON 数组
	ConfigKeyOriginErrorPageHTML        = "origin_error_page_html"         // 源站错误页自定义 HTML（空则内置默认）
	ConfigKeyPoWPage                    = "pow_page"                       // PoW 页面模板、组件样式与自定义 HTML
	ConfigKeySiteAuthHTML               = "site_auth_html"                 // 站点身份认证页 HTML
	ConfigKeyOriginErrorPageGetOnly     = "origin_error_page_get_only"     // 是否仅对 GET 请求返回自定义错误页

	// Service Worker 离线兜底
	ConfigKeySWOfflineEnabled = "sw_offline_enabled" // 是否启用 Service Worker 离线兜底
	ConfigKeySWOfflineHTML    = "sw_offline_html"    // 离线联系页自定义 HTML（空则内置默认）
	ConfigKeySWOfflineDomains = "sw_offline_domains" // 离线兜底生效域名列表（JSON 数组，空则仅总开关无效）
)

// 日志数据库解耦
const (
	ConfigKeyLogDatabase                = "log_database"                  // 当前日志主库：postgres|sqlite|clickhouse（仅迁移任务写入）
	ConfigKeyLogDBMigration             = "log_db_migration"              // 迁移冻结标记："migrating" 或空
	ConfigKeyLogRetentionDaysPostgres   = "log_retention_days_postgres"   // PostgreSQL 日志保留天数
	ConfigKeyLogRetentionDaysSQLite     = "log_retention_days_sqlite"     // SQLite 日志保留天数
	ConfigKeyLogRetentionDaysClickHouse = "log_retention_days_clickhouse" // ClickHouse 日志保留天数
	// ConfigKeyMetricRetentionDays 性能指标（CPU/内存/磁盘/网络）保留天数，三库共用；
	// 性能数据价值衰减快，默认短留存（3 天），不随访问日志保留配置。
	ConfigKeyMetricRetentionDays = "metric_retention_days"
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
