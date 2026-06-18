// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package testhelper 提供测试辅助工具
package testhelper

import (
	"context"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"gorm.io/gorm"
)

const (
	configTypeSystem = "system"
	configValueTrue  = "true"
	configValueFalse = "false"
)

// SetupTestEnvironment initializes an in-memory SQLite DB, seeds default configurations,
// starts miniredis, and overrides the global db/Redis clients. It returns a cleanup function.
func SetupTestEnvironment(t *testing.T) (*gorm.DB, *miniredis.Miniredis, func()) {
	// Initialize GORM in-memory SQLite
	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("failed to open in-memory SQLite db: %v", err)
	}

	// AutoMigrate all tables
	err = sqliteDB.AutoMigrate(
		&model.User{},
		&model.AuthSource{},
		&model.ExternalAccount{},
		&model.SystemConfig{},
		&model.Upload{},
		&model.UploadStat{},
		&model.TaskExecution{},
		&model.Template{},
		&model.AccessToken{},
		&model.Schedule{},
	)
	if err != nil {
		t.Fatalf("failed to auto migrate tables: %v", err)
	}

	// Set global db
	db.SetDB(sqliteDB)

	// Start miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	// Hook up Redis Client to miniredis
	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
		MaintNotificationsConfig: &maintnotifications.Config{
			Mode: maintnotifications.ModeDisabled,
		},
	})
	db.Redis = redisClient

	// Seed default configurations
	seedDefaultConfigs(t, sqliteDB)

	// Cleanup function
	cleanup := func() {
		runExtraCleanups()
		repository.ResetSystemConfigRAMCacheForTest()
		_ = redisClient.Close()
		mr.Close()
		// Reset database and Redis references
		db.SetDB(nil)
		db.Redis = nil
	}

	return sqliteDB, mr, cleanup
}

func getSeedConfigsPart1() []model.SystemConfig {
	return []model.SystemConfig{
		{
			Key:         model.ConfigKeyUploadAllowedExtensions,
			Value:       "jpg,png,webp",
			Type:        configTypeSystem,
			Description: "允许上传的图片扩展名（逗号分隔）",
		},
		{
			Key:         model.ConfigKeySiteName,
			Value:       "OpenFlare",
			Type:        configTypeSystem,
			Description: "系统平台的展示名称",
		},
		{
			Key:         model.ConfigKeyPasswordLoginEnabled,
			Value:       configValueTrue,
			Type:        configTypeSystem,
			Description: "是否允许使用账号密码登录",
		},
		{
			Key:         model.ConfigKeyRegistrationEnabled,
			Value:       configValueFalse,
			Type:        configTypeSystem,
			Description: "控制普通用户是否可以自主注册（true/false）",
		},
		{
			Key:         model.ConfigKeyPasswordRegisterEnabled,
			Value:       configValueFalse,
			Type:        configTypeSystem,
			Description: "是否允许通过密码创建本地账号",
		},
		{
			Key:         model.ConfigKeyOIDCLoginEnabled,
			Value:       configValueTrue,
			Type:        configTypeSystem,
			Description: "是否允许使用第三方 OIDC 认证源登录",
		},
		{
			Key:         model.ConfigKeyMaxAPIKeysPerUser,
			Value:       "5",
			Type:        "business",
			Description: "限制每个普通用户可以创建的 API Key 最大数量",
		},
		{
			Key:         model.ConfigKeyCapLoginEnabled,
			Value:       configValueTrue,
			Type:        configTypeSystem,
			Description: "是否启用登录人机验证（true/false）",
		},
		{
			Key:         model.ConfigKeyCapAutoSolve,
			Value:       configValueTrue,
			Type:        configTypeSystem,
			Description: "打开页面后是否自动开始计算，关闭则需用户手动点击触发",
		},
		{
			Key:         model.ConfigKeyCapChallengeCount,
			Value:       "1",
			Type:        configTypeSystem,
			Description: "客户端需求解的 PoW 难题总数，默认 1，推荐 1～5",
		},
		{
			Key:         model.ConfigKeyCapChallengeSize,
			Value:       "32",
			Type:        configTypeSystem,
			Description: "人机验证盐值长度",
		},
		{
			Key:         model.ConfigKeyCapChallengeDifficulty,
			Value:       "4",
			Type:        configTypeSystem,
			Description: "人机验证 PoW 难度（目标前缀长度）",
		},
		{
			Key:         model.ConfigKeyCapChallengeTTL,
			Value:       "600",
			Type:        configTypeSystem,
			Description: "人机验证难题有效时间（秒）",
		},
		{
			Key:         model.ConfigKeyCapTokenTTL,
			Value:       "1200",
			Type:        configTypeSystem,
			Description: "人机验证兑换凭证有效时间（秒）",
		},
	}
}

func getSeedConfigsPart2() []model.SystemConfig {
	return []model.SystemConfig{
		{
			Key:         model.ConfigKeyServerAddress,
			Value:       "",
			Type:        configTypeSystem,
			Description: "服务器地址（用于跨域源控制，不设定则允许任意源）",
		},
		{
			Key:         model.ConfigKeySMTPHost,
			Value:       "",
			Type:        configTypeSystem,
			Description: "SMTP 服务器地址（例如 smtp.example.com）",
		},
		{
			Key:         model.ConfigKeySMTPPort,
			Value:       "587",
			Type:        configTypeSystem,
			Description: "SMTP 端口（例如 587 或 465）",
		},
		{
			Key:         model.ConfigKeySMTPUsername,
			Value:       "",
			Type:        configTypeSystem,
			Description: "SMTP 账户（如 sender@example.com）",
		},
		{
			Key:         model.ConfigKeySMTPPassword,
			Value:       "",
			Type:        configTypeSystem,
			Description: "SMTP 访问凭证（授权码/密码）",
		},
		{
			Key:         model.ConfigKeyEmailLoginVerificationEnabled,
			Value:       configValueFalse,
			Type:        configTypeSystem,
			Description: "是否开启邮箱登录验证（true/false）",
		},
		{
			Key:         model.ConfigKeyEmailRegisterVerificationEnabled,
			Value:       configValueFalse,
			Type:        configTypeSystem,
			Description: "是否开启邮箱注册验证（true/false）",
		},
		{
			Key:         model.ConfigKeyMenuDisplayConfig,
			Value:       "{}",
			Type:        configTypeSystem,
			Description: "目录显示配置（JSON 字符串，格式为 {url: enabled}）",
		},
		{
			Key:         model.ConfigKeySearchEngineIndexingEnabled,
			Value:       configValueFalse,
			Type:        configTypeSystem,
			Description: "是否允许搜索引擎检索",
		},
		{
			Key:         model.ConfigKeyFileAccessWhitelist,
			Value:       `["avatar"]`,
			Type:        configTypeSystem,
			Description: "免登录访问的文件业务类型白名单",
		},
		{
			Key:         model.ConfigKeyDiskCacheMaxSizeMB,
			Value:       "100",
			Type:        configTypeSystem,
			Description: "磁盘缓存最大空间大小 (MB)",
		},
		{
			Key:         model.ConfigKeyDiskCacheTTLMinutes,
			Value:       "60",
			Type:        configTypeSystem,
			Description: "磁盘缓存默认有效期 (分钟)",
		},
		{
			Key:         model.ConfigKeyDiskCacheLRUEnabled,
			Value:       configValueTrue,
			Type:        configTypeSystem,
			Description: "是否启用 LRU 淘汰机制",
		},
		{
			Key:         model.ConfigKeyLoginSessionTTLHours,
			Value:       "0",
			Type:        configTypeSystem,
			Description: "登录会话过期时间 (小时，0表示浏览器关闭后自动退出，-1表示永不过期)",
		},
		{
			Key:         model.ConfigKeyUpdateUpstreamRepository,
			Value:       "Rain-kl/OpenFlare",
			Type:        configTypeSystem,
			Description: "GitHub Actions Release 上游仓库（owner/repo 或 GitHub 仓库地址）",
		},
		{
			Key:         model.ConfigKeyStorageConfig,
			Value:       `{"driver":"local","local":{"root":"."},"s3":{"region":"us-east-1"},"r2":{"region":"auto"},"minio":{"region":"us-east-1","path_style":true},"oss":{},"webdav":{}}`,
			Type:        configTypeSystem,
			Description: "文件存储驱动及连接配置（JSON）",
		},
	}
}

func seedDefaultConfigs(t *testing.T, tx *gorm.DB) {
	defaultConfigs := append(getSeedConfigsPart1(), getSeedConfigsPart2()...)

	if err := tx.Create(&defaultConfigs).Error; err != nil {
		t.Fatalf("failed to seed default system configs: %v", err)
	}

	publicKeys := map[string]struct{}{
		model.ConfigKeyUploadAllowedExtensions:          {},
		model.ConfigKeySiteName:                         {},
		model.ConfigKeyPasswordLoginEnabled:             {},
		model.ConfigKeyRegistrationEnabled:              {},
		model.ConfigKeyPasswordRegisterEnabled:          {},
		model.ConfigKeyOIDCLoginEnabled:                 {},
		model.ConfigKeyMaxAPIKeysPerUser:                {},
		model.ConfigKeyCapLoginEnabled:                  {},
		model.ConfigKeyCapAutoSolve:                     {},
		model.ConfigKeyEmailLoginVerificationEnabled:    {},
		model.ConfigKeyEmailRegisterVerificationEnabled: {},
		model.ConfigKeyMenuDisplayConfig:                {},
		model.ConfigKeySearchEngineIndexingEnabled:      {},
		model.ConfigKeyFileAccessWhitelist:              {},
	}
	keys := make([]string, 0, len(publicKeys))
	for key := range publicKeys {
		keys = append(keys, key)
	}
	if err := tx.Model(&model.SystemConfig{}).
		Where("key IN ?", keys).
		Update("visibility", model.ConfigVisibilityVisible).Error; err != nil {
		t.Fatalf("failed to seed public system config visibility: %v", err)
	}

	// Also seed these in miniredis context if required, but they are stored in postgres first.
	// We'll write configs to miniredis in actual handlers.
	for _, config := range defaultConfigs {
		if _, ok := publicKeys[config.Key]; ok {
			config.Visibility = model.ConfigVisibilityVisible
		}
		_ = db.HSetJSON(context.Background(), repository.SystemConfigRedisHashKey, config.Key, &config)
	}
}
