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
