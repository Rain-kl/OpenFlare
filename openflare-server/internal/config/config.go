// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package config 负责应用配置的加载、解析与环境变量覆盖。
package config

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// 默认队列优先级
const (
	webhookQueuePriority   = 10
	whitelistQueuePriority = 5
	defaultQueuePriority   = 3
)

// Config 全局配置单例，初始化后不可变
var Config *configModel

// findConfigPath searches upward for the config file to handle tests running in subdirectories.
func findConfigPath(configPath string) string {
	if _, err := os.Stat(configPath); err == nil {
		return configPath
	}
	dir := "."
	for i := 0; i < 5; i++ {
		dir += "/.."
		path := dir + "/" + configPath
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return configPath
}

// isTest checks if the current execution context is within 'go test'.
func isTest() bool {
	if flag.Lookup("test.v") != nil {
		return true
	}
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") || strings.HasSuffix(arg, ".test") {
			return true
		}
	}
	return false
}

func init() {
	// 加载配置文件路径
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = findConfigPath("config.yaml")
	}

	// 设置配置文件
	viper.SetConfigFile(configPath)
	viper.AutomaticEnv()

	// 读取配置文件（可选：找不到文件时使用空默认值 + 环境变量）
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// 文件存在但读取/解析失败
			if _, statErr := os.Stat(configPath); statErr == nil { //nolint:gosec // configPath is loaded from CONFIG_PATH environment variable
				log.Fatalf("[Config] read config failed: %v\n", err)
			}
		}
		log.Println("[Config] no config file found, using environment variables only")
		viper.SetConfigType("yaml")
		if err := viper.ReadConfig(strings.NewReader("")); err != nil {
			log.Fatalf("[Config] failed to init empty config: %v\n", err)
		}
	}

	// 解析配置到结构体
	var c configModel
	if err := viper.Unmarshal(&c); err != nil {
		log.Fatalf("[Config] parse config failed: %v\n", err)
	}

	applyDefaults(&c)

	// 环境变量覆盖（优先级高于 config.yaml）
	applyEnvOverrides(&c)
	applyDefaults(&c)

	// Disable standard DB/Redis initializations during tests to prevent connection attempts.
	if isTest() {
		c.Database.Enabled = false
		c.Database.SQLitePath = ":memory:"
		c.Redis.Enabled = false
		c.ClickHouse.Enabled = false
	}

	// 设置全局配置
	Config = &c

	// 打印配置
	printConfig(&c)
}

func applyDefaults(c *configModel) {
	if c.App.SessionAge <= 0 {
		c.App.SessionAge = 86400
	}
	if c.Otel.TracerName == "" {
		c.Otel.TracerName = "github.com/Rain-kl/OpenFlare"
	}
}

// ─── 环境变量覆盖层 ────────────────────────────────────────────────────────────
// 环境变量优先级高于 config.yaml，未设置则保留 yaml 中的值。

func envStr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envInt64(key string, fallback int64) int64 {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}

func envFloat64(key string, fallback float64) float64 {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

// applyEnvOverrides 将环境变量值覆盖到配置结构体上（仅当环境变量已设置时生效）
func applyEnvOverrides(c *configModel) {
	// ─── App ───
	c.App.AppName = envStr("APP_NAME", c.App.AppName)
	c.App.Env = envStr("APP_ENV", c.App.Env)
	c.App.Addr = envStr("APP_ADDR", c.App.Addr)
	c.App.NodeID = envInt64("APP_NODE_ID", c.App.NodeID)
	c.App.APIPrefix = envStr("APP_API_PREFIX", c.App.APIPrefix)
	c.App.GracefulShutdownTimeout = envInt("APP_GRACEFUL_SHUTDOWN_TIMEOUT", c.App.GracefulShutdownTimeout)
	c.App.SessionCookieName = envStr("APP_SESSION_COOKIE_NAME", c.App.SessionCookieName)
	c.App.SessionSecret = envStr("APP_SESSION_SECRET", c.App.SessionSecret)
	c.App.SessionDomain = envStr("APP_SESSION_DOMAIN", c.App.SessionDomain)
	c.App.SessionAge = envInt("APP_SESSION_AGE", c.App.SessionAge)
	c.App.SessionHTTPOnly = envBool("APP_SESSION_HTTP_ONLY", c.App.SessionHTTPOnly)
	c.App.SessionSecure = envBool("APP_SESSION_SECURE", c.App.SessionSecure)

	// ─── Database ───
	c.Database.Host = envStr("DB_HOST", c.Database.Host)
	c.Database.Port = envInt("DB_PORT", c.Database.Port)
	c.Database.Username = envStr("DB_USERNAME", c.Database.Username)
	c.Database.Password = envStr("DB_PASSWORD", c.Database.Password)
	c.Database.Database = envStr("DB_NAME", c.Database.Database)
	c.Database.SSLMode = envStr("DB_SSL_MODE", c.Database.SSLMode)
	c.Database.TimeZone = envStr("DB_TIMEZONE", c.Database.TimeZone)
	c.Database.LogLevel = envStr("DB_LOG_LEVEL", c.Database.LogLevel)
	c.Database.MaxIdleConn = envInt("DB_MAX_IDLE_CONN", c.Database.MaxIdleConn)
	c.Database.MaxOpenConn = envInt("DB_MAX_OPEN_CONN", c.Database.MaxOpenConn)
	// 当 DB_HOST 环境变量已设置时自动启用数据库
	if _, ok := os.LookupEnv("DB_HOST"); ok {
		c.Database.Enabled = true
	}
	c.Database.Enabled = envBool("DB_ENABLED", c.Database.Enabled)
	c.Database.SQLitePath = envStr("SQLITE_PATH", c.Database.SQLitePath)

	// ─── Redis ───
	if v, ok := os.LookupEnv("REDIS_ADDR"); ok {
		c.Redis.Addrs = []string{v}
		c.Redis.Enabled = true // 当 REDIS_ADDR 已设置时自动启用
	}
	c.Redis.Enabled = envBool("REDIS_ENABLED", c.Redis.Enabled)
	c.Redis.Username = envStr("REDIS_USERNAME", c.Redis.Username)
	c.Redis.Password = envStr("REDIS_PASSWORD", c.Redis.Password)
	c.Redis.DB = envInt("REDIS_DB", c.Redis.DB)
	c.Redis.KeyPrefix = envStr("REDIS_KEY_PREFIX", c.Redis.KeyPrefix)
	c.Redis.PoolSize = envInt("REDIS_POOL_SIZE", c.Redis.PoolSize)

	// ─── ClickHouse ───
	if v, ok := os.LookupEnv("CLICKHOUSE_HOST"); ok {
		c.ClickHouse.Hosts = []string{v}
		c.ClickHouse.Enabled = true
	}
	c.ClickHouse.Enabled = envBool("CLICKHOUSE_ENABLED", c.ClickHouse.Enabled)
	c.ClickHouse.Username = envStr("CLICKHOUSE_USERNAME", c.ClickHouse.Username)
	c.ClickHouse.Password = envStr("CLICKHOUSE_PASSWORD", c.ClickHouse.Password)
	c.ClickHouse.Database = envStr("CLICKHOUSE_NAME", c.ClickHouse.Database)

	// ─── Log ───
	c.Log.Level = envStr("LOG_LEVEL", c.Log.Level)
	c.Log.Format = envStr("LOG_FORMAT", c.Log.Format)
	c.Log.Output = envStr("LOG_OUTPUT", c.Log.Output)

	// ─── OTel ───
	c.Otel.SamplingRate = envFloat64("OTEL_SAMPLING_RATE", c.Otel.SamplingRate)
	c.Otel.TracerName = envStr("OTEL_TRACER_NAME", c.Otel.TracerName)

	// ─── Worker ───
	c.Worker.Concurrency = envInt("WORKER_CONCURRENCY", c.Worker.Concurrency)
	c.Worker.StrictPriority = envBool("WORKER_STRICT_PRIORITY", c.Worker.StrictPriority)

	// 无 yaml 且无环境变量时，使用代码级默认队列
	if len(c.Worker.Queues) == 0 {
		c.Worker.Queues = []QueueConfig{
			{Name: "webhook", Priority: webhookQueuePriority},
			{Name: "whitelist_only", Priority: whitelistQueuePriority},
			{Name: "default", Priority: defaultQueuePriority},
		}
	}
}

// printConfig 打印配置内容
func printConfig(c *configModel) {
	configJSON, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		log.Printf("[Config] failed to marshal config: %v\n", err)
		return
	}
	log.Printf("[Config] loaded configuration:\n%s\n", string(configJSON))
}
