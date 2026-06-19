// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config

import "time"

type configModel struct {
	App        appConfig        `mapstructure:"app"`
	Database   databaseConfig   `mapstructure:"database"`
	Redis      redisConfig      `mapstructure:"redis"`
	Log        logConfig        `mapstructure:"log"`
	Scheduler  schedulerConfig  `mapstructure:"scheduler"`
	Worker     workerConfig     `mapstructure:"worker"`
	ClickHouse clickHouseConfig `mapstructure:"clickhouse"`
	Otel       otelConfig       `mapstructure:"otel"`
}

// appConfig 应用基本配置
type appConfig struct {
	AppName                 string `mapstructure:"app_name"`
	Env                     string `mapstructure:"env"`
	Addr                    string `mapstructure:"addr"`
	NodeID                  int64  `mapstructure:"node_id"`
	APIPrefix               string `mapstructure:"api_prefix"`
	GracefulShutdownTimeout int    `mapstructure:"graceful_shutdown_timeout"`
	SessionCookieName       string `mapstructure:"session_cookie_name"`
	SessionSecret           string `mapstructure:"session_secret"`
	SessionDomain           string `mapstructure:"session_domain"`
	SessionAge              int    `mapstructure:"session_age"`
	SessionHTTPOnly         bool   `mapstructure:"session_http_only"`
	SessionSecure           bool   `mapstructure:"session_secure"`
}

// IsProduction 检查当前环境是否为生产环境
func (a *appConfig) IsProduction() bool {
	return a.Env == "production"
}

// databaseConfig 数据库配置
type databaseConfig struct {
	Enabled                bool                    `mapstructure:"enabled"`
	SQLitePath             string                  `mapstructure:"sqlite_path"` // PostgreSQL 禁用时的 SQLite 文件路径
	Host                   string                  `mapstructure:"host"`
	Port                   int                     `mapstructure:"port"`
	Username               string                  `mapstructure:"username"`
	Password               string                  `mapstructure:"password"`
	Database               string                  `mapstructure:"database"`
	MaxIdleConn            int                     `mapstructure:"max_idle_conn"`
	MaxOpenConn            int                     `mapstructure:"max_open_conn"`
	ConnMaxLifetime        int                     `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime        int                     `mapstructure:"conn_max_idle_time"`
	LogLevel               string                  `mapstructure:"log_level"`
	SSLMode                string                  `mapstructure:"ssl_mode"`
	TimeZone               string                  `mapstructure:"time_zone"`
	ApplicationName        string                  `mapstructure:"application_name"`
	SearchPath             string                  `mapstructure:"search_path"`
	PreferSimpleProtocol   bool                    `mapstructure:"prefer_simple_protocol"`
	StatementCacheCapacity int                     `mapstructure:"statement_cache_capacity"`
	DefaultQueryExecMode   string                  `mapstructure:"default_query_exec_mode"`
	Replicas               []databaseReplicaConfig `mapstructure:"replicas"`
	SlowThreshold          time.Duration           `mapstructure:"slow_threshold"`
}

// databaseReplicaConfig 只读副本配置
type databaseReplicaConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// clickhouse 配置
type clickHouseConfig struct {
	Enabled         bool     `mapstructure:"enabled"`
	Hosts           []string `mapstructure:"hosts"`
	Username        string   `mapstructure:"username"`
	Password        string   `mapstructure:"password"`
	Database        string   `mapstructure:"database"`
	MaxIdleConn     int      `mapstructure:"max_idle_conn"`
	MaxOpenConn     int      `mapstructure:"max_open_conn"`
	ConnMaxLifetime int      `mapstructure:"conn_max_lifetime"`
	DialTimeout     int      `mapstructure:"dial_timeout"`
	BlockBufferSize uint8    `mapstructure:"block_buffer_size"`
}

// redisConfig Redis配置
type redisConfig struct {
	Enabled         bool     `mapstructure:"enabled"`
	Addrs           []string `mapstructure:"addrs"`
	Username        string   `mapstructure:"username"`
	Password        string   `mapstructure:"password"`
	DB              int      `mapstructure:"db"`
	ClusterMode     bool     `mapstructure:"cluster_mode"`
	MasterName      string   `mapstructure:"master_name"`
	KeyPrefix       string   `mapstructure:"key_prefix"`
	PoolSize        int      `mapstructure:"pool_size"`
	MinIdleConn     int      `mapstructure:"min_idle_conn"`
	DialTimeout     int      `mapstructure:"dial_timeout"`
	ReadTimeout     int      `mapstructure:"read_timeout"`
	WriteTimeout    int      `mapstructure:"write_timeout"`
	MaxRetries      int      `mapstructure:"max_retries"`
	PoolTimeout     int      `mapstructure:"pool_timeout"`
	ConnMaxIdleTime int      `mapstructure:"conn_max_idle_time"`
}

// logConfig 日志配置
type logConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	FilePath   string `mapstructure:"file_path"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxAge     int    `mapstructure:"max_age"`
	MaxBackups int    `mapstructure:"max_backups"`
	Compress   bool   `mapstructure:"compress"`
}

// schedulerConfig 定时任务配置
type schedulerConfig struct {
}

// workerConfig 工作配置
type workerConfig struct {
	Concurrency    int           `mapstructure:"concurrency"`
	StrictPriority bool          `mapstructure:"strict_priority"`
	Queues         []QueueConfig `mapstructure:"queues"`
}

// QueueConfig 队列配置
type QueueConfig struct {
	Name     string `mapstructure:"name"`
	Priority int    `mapstructure:"priority"`
}

// otelConfig OpenTelemetry 配置
type otelConfig struct {
	SamplingRate float64 `mapstructure:"sampling_rate"`
	TracerName   string  `mapstructure:"tracer_name"`
}
