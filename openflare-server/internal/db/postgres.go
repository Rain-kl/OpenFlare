// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"context"
	"log"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/glebarez/sqlite"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
	"gorm.io/plugin/opentelemetry/tracing"
)

var (
	db *gorm.DB
)

func init() {
	if !config.Config.Database.Enabled {
		// PostgreSQL 禁用，使用 SQLite
		initSQLite()
		return
	}

	initPostgres()
}

// initSQLite 初始化 SQLite 数据库（PostgreSQL 禁用时的后备方案）
func initSQLite() {
	sqlitePath := config.Config.Database.SQLitePath
	if sqlitePath == "" {
		sqlitePath = "./data/openflare.db"
	}

	var err error
	db, err = gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger: &gormZapLogger{
			logLevel:                  parseLogLevel(config.Config.Database.LogLevel),
			slowThreshold:             config.Config.Database.SlowThreshold,
			ignoreRecordNotFoundError: config.Config.App.IsProduction(),
		},
	})
	if err != nil {
		log.Fatalf("[SQLite] init connection failed: %v\n", err)
	}

	// Trace 注入
	if err = db.Use(
		tracing.NewPlugin(
			tracing.WithoutMetrics(),
			tracing.WithAttributes(
				attribute.String("db.instance", sqlitePath),
				attribute.String("db.system", "SQLite"),
			),
		),
	); err != nil {
		log.Fatalf("[SQLite] init trace failed: %v\n", err)
	}

	log.Printf("[SQLite] initialized (path: %s)\n", sqlitePath)
}

// initPostgres 初始化 PostgreSQL 数据库
func initPostgres() {
	var err error
	dbConfig := config.Config.Database

	// 构建主库 DSN 并连接
	primaryDSN := buildDSN(dbConfig.Host, dbConfig.Port, dbConfig.Username, dbConfig.Password)

	pgConfig := postgres.Config{
		DSN:                  primaryDSN,
		PreferSimpleProtocol: dbConfig.PreferSimpleProtocol,
	}

	db, err = gorm.Open(postgres.New(pgConfig), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger: &gormZapLogger{
			logLevel:                  parseLogLevel(config.Config.Database.LogLevel),
			slowThreshold:             config.Config.Database.SlowThreshold,
			ignoreRecordNotFoundError: config.Config.App.IsProduction(),
		},
	})
	if err != nil {
		log.Fatalf("[PostgreSQL] init connection failed: %v\n", err)
	}

	// Trace 注入
	if err = db.Use(
		tracing.NewPlugin(
			tracing.WithoutMetrics(),
			tracing.WithAttributes(
				attribute.String("db.instance", dbConfig.Database),
				attribute.String("db.ip", dbConfig.Host),
				attribute.String("server.address", net.JoinHostPort(dbConfig.Host, strconv.Itoa(dbConfig.Port))),
				attribute.String("db.system", "PostgreSQL"),
			),
		),
	); err != nil {
		log.Fatalf("[PostgreSQL] init trace failed: %v\n", err)
	}

	if len(dbConfig.Replicas) > 0 {
		var replicaDialectors []gorm.Dialector
		for _, replica := range dbConfig.Replicas {
			username := replica.Username
			if username == "" {
				username = dbConfig.Username
			}
			password := replica.Password
			if password == "" {
				password = dbConfig.Password
			}
			replicaDSN := buildDSN(replica.Host, replica.Port, username, password)
			replicaDialectors = append(replicaDialectors, postgres.New(postgres.Config{
				DSN:                  replicaDSN,
				PreferSimpleProtocol: dbConfig.PreferSimpleProtocol,
			}))
		}

		resolver := dbresolver.Register(dbresolver.Config{
			Replicas: replicaDialectors,
			Policy:   dbresolver.RandomPolicy{},
		})

		resolver.SetMaxIdleConns(dbConfig.MaxIdleConn).
			SetMaxOpenConns(dbConfig.MaxOpenConn).
			SetConnMaxLifetime(time.Duration(dbConfig.ConnMaxLifetime) * time.Second).
			SetConnMaxIdleTime(time.Duration(dbConfig.ConnMaxIdleTime) * time.Second)

		if err = db.Use(resolver); err != nil {
			log.Fatalf("[PostgreSQL] init dbresolver failed: %v\n", err)
		}
		log.Printf("[PostgreSQL] initialized in Primary-Replica mode (%d replicas)\n", len(dbConfig.Replicas))
	} else {
		log.Println("[PostgreSQL] initialized in Standalone mode")
	}

	// 获取通用数据库对象设置连接池
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("[PostgreSQL] load sql db failed: %v\n", err)
	}

	sqlDB.SetMaxIdleConns(dbConfig.MaxIdleConn)
	sqlDB.SetMaxOpenConns(dbConfig.MaxOpenConn)
	sqlDB.SetConnMaxLifetime(time.Duration(dbConfig.ConnMaxLifetime) * time.Second)
	sqlDB.SetConnMaxIdleTime(time.Duration(dbConfig.ConnMaxIdleTime) * time.Second)

}

// buildDSN 构建 PostgreSQL DSN
func buildDSN(host string, port int, username, password string) string {
	cfg := config.Config.Database
	pqURL := &url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
		Path:   cfg.Database,
	}
	if username != "" {
		pqURL.User = url.UserPassword(username, password)
	}

	query := pqURL.Query()
	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	query.Set("sslmode", sslMode)
	if cfg.ApplicationName != "" {
		query.Set("application_name", cfg.ApplicationName)
	}
	if cfg.SearchPath != "" {
		query.Set("search_path", cfg.SearchPath)
	}
	if cfg.DefaultQueryExecMode != "" {
		query.Set("default_query_exec_mode", cfg.DefaultQueryExecMode)
	}
	if cfg.StatementCacheCapacity > 0 {
		query.Set("statement_cache_capacity", strconv.Itoa(cfg.StatementCacheCapacity))
	}

	rawQuery := query.Encode()
	if cfg.TimeZone != "" {
		if rawQuery != "" {
			rawQuery += "&"
		}
		rawQuery += "TimeZone=" + cfg.TimeZone
	}
	pqURL.RawQuery = rawQuery

	return pqURL.String()
}

// DB 返回带上下文追踪的 GORM 数据库实例
func DB(ctx context.Context) *gorm.DB {
	if db == nil {
		return nil
	}
	return db.WithContext(ctx)
}

// SetDB sets the package-level database instance for testing.
func SetDB(d *gorm.DB) {
	db = d
}
