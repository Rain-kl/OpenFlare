// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package migrator

import (
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"gorm.io/gorm"
)

func TestMigrateUpgradesLegacyDatabaseAndPreservesData(t *testing.T) {
	// 1. Initialize an empty in-memory SQLite DB
	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("gorm.Open(sqlite) error = %v", err)
	}

	rawDB, err := sqliteDB.DB()
	if err != nil {
		t.Fatalf("sqliteDB.DB() error = %v", err)
	}

	// 2. Set up the legacy schema manually
	setupLegacySchema := []string{
		`CREATE TABLE users (
			id integer(64) NOT NULL,
			username text,
			password text NOT NULL,
			display_name text,
			role integer(64),
			status integer(64),
			token text,
			email text,
			github_id text,
			wechat_id text,
			CONSTRAINT users_pkey PRIMARY KEY (id)
		);`,
		`CREATE TABLE options (
			key text NOT NULL,
			value text,
			CONSTRAINT options_pkey PRIMARY KEY (key)
		);`,
		`CREATE TABLE origins (
			id integer(64) NOT NULL,
			name text(255) NOT NULL,
			address text(255) NOT NULL,
			remark text(255),
			created_at text(6),
			updated_at text(6),
			CONSTRAINT origins_pkey PRIMARY KEY (id)
		);`,
		`CREATE TABLE apply_logs (
			id integer(64) NOT NULL,
			node_id text(64) NOT NULL,
			version text(32) NOT NULL,
			result text(32) NOT NULL,
			message text,
			checksum text(64) NOT NULL,
			main_config_checksum text(64) NOT NULL,
			route_config_checksum text(64) NOT NULL,
			support_file_count integer(64) NOT NULL,
			created_at text(6),
			CONSTRAINT apply_logs_pkey PRIMARY KEY (id)
		);`,
		// Legacy log tables (which should be dropped)
		`CREATE TABLE node_access_logs_00 (
			id integer(64) NOT NULL,
			node_id text(64) NOT NULL,
			logged_at text(6) NOT NULL
		);`,
		`CREATE TABLE node_system_profiles (
			id integer(64) NOT NULL,
			node_id text(64) NOT NULL,
			hostname text(255)
		);`,
	}

	for _, query := range setupLegacySchema {
		if _, err := rawDB.Exec(query); err != nil {
			t.Fatalf("Exec setup query failed: %v", err)
		}
	}

	// 3. Populate with legacy mock data
	insertMockData := []string{
		// Legacy admin user
		`INSERT INTO users (id, username, password, display_name, role, status, email)
		 VALUES (1, 'ryan', 'hashed_pass_123', 'Root User ryan', 100, 1, 'ryan@example.com');`,
		// Legacy normal user
		`INSERT INTO users (id, username, password, display_name, role, status, email)
		 VALUES (2, 'jack', 'hashed_pass_456', 'Normal User jack', 10, 1, 'jack@example.com');`,
		// Legacy config options（GeoIPProvider 会被迁移到 w_system_configs.geoip_provider）
		`INSERT INTO options (key, value) VALUES ('GeoIPProvider', 'ip-api');`,
		// Legacy origin
		`INSERT INTO origins (id, name, address, remark, created_at, updated_at)
		 VALUES (10, 'my_origin', '127.0.0.1:8080', 'original origin', '2026-06-01 12:00:00', '2026-06-01 12:00:00');`,
		// Legacy apply log
		`INSERT INTO apply_logs (id, node_id, version, result, message, checksum, main_config_checksum, route_config_checksum, support_file_count, created_at)
		 VALUES (100, 'node_1', 'v1.0.0', 'success', 'applied successfully', 'hash1', 'hash2', 'hash3', 2, '2026-06-01 12:00:00');`,
	}

	for _, query := range insertMockData {
		if _, err := rawDB.Exec(query); err != nil {
			t.Fatalf("Exec mock data query failed: %v", err)
		}
	}

	// 4. Set up mock services (Redis & Config)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
		MaintNotificationsConfig: &maintnotifications.Config{
			Mode: maintnotifications.ModeDisabled,
		},
	})

	previousDBEnabled := config.Config.Database.Enabled
	previousRedis := db.Redis
	config.Config.Database.Enabled = false
	db.SetDB(sqliteDB)
	db.Redis = redisClient

	t.Cleanup(func() {
		config.Config.Database.Enabled = previousDBEnabled
		db.SetDB(nil)
		db.Redis = previousRedis
		_ = redisClient.Close()
		mr.Close()
	})

	// 5. Run the Goose migrations
	Migrate()

	// 6. Assertions on migrated data schema & records

	// Check that the w_users table exists and contains correct columns/records
	var usersCount int64
	if err := sqliteDB.Table("w_users").Count(&usersCount).Error; err != nil {
		t.Fatalf("count w_users error: %v", err)
	}
	if usersCount < 2 {
		t.Errorf("expected at least 2 users, got %d", usersCount)
	}

	// Verify user 1 (ryan) details
	type userStruct struct {
		ID       int64
		Username string
		Nickname string
		Email    string
		IsActive bool
		IsAdmin  bool
	}
	var ryanUser userStruct
	if err := sqliteDB.Table("w_users").Where("id = ?", 1).First(&ryanUser).Error; err != nil {
		t.Fatalf("query ryan user failed: %v", err)
	}
	if ryanUser.Username != "ryan" || ryanUser.Nickname != "Root User ryan" || !ryanUser.IsActive || !ryanUser.IsAdmin {
		t.Errorf("ryan user migrated incorrectly: %+v", ryanUser)
	}

	// Verify user 2 (jack) details
	var jackUser userStruct
	if err := sqliteDB.Table("w_users").Where("id = ?", 2).First(&jackUser).Error; err != nil {
		t.Fatalf("query jack user failed: %v", err)
	}
	if jackUser.Username != "jack" || jackUser.Nickname != "Normal User jack" || !jackUser.IsActive || jackUser.IsAdmin {
		t.Errorf("jack user migrated incorrectly: %+v", jackUser)
	}

	// Verify legacy option migrated all the way to w_system_configs（of_options 已被删除）
	var geoIPProviderVal string
	if err := sqliteDB.Table("w_system_configs").Where("key = ?", "geoip_provider").Select("value").Scan(&geoIPProviderVal).Error; err != nil {
		t.Fatalf("query geoip_provider failed: %v", err)
	}
	if geoIPProviderVal != "ip-api" {
		t.Errorf("geoip_provider value incorrect: got %s, want ip-api", geoIPProviderVal)
	}

	// Verify origin migrated to of_origins
	type originStruct struct {
		ID      int64
		Name    string
		Address string
		Remark  string
	}
	var testOrigin originStruct
	if err := sqliteDB.Table("of_origins").Where("id = ?", 10).First(&testOrigin).Error; err != nil {
		t.Fatalf("query my_origin failed: %v", err)
	}
	if testOrigin.Name != "my_origin" || testOrigin.Address != "127.0.0.1:8080" || testOrigin.Remark != "original origin" {
		t.Errorf("origin migrated incorrectly: %+v", testOrigin)
	}

	// Verify apply log migrated to of_apply_logs
	type applyLogStruct struct {
		ID        int64
		NodeID    string
		Version   string
		Result    string
		Message   string
		CreatedAt time.Time
	}
	var testApplyLog applyLogStruct
	if err := sqliteDB.Table("of_apply_logs").Where("id = ?", 100).First(&testApplyLog).Error; err != nil {
		t.Fatalf("query apply_logs failed: %v", err)
	}
	if testApplyLog.NodeID != "node_1" || testApplyLog.Version != "v1.0.0" || testApplyLog.Result != "success" || testApplyLog.Message != "applied successfully" {
		t.Errorf("apply log migrated incorrectly: %+v", testApplyLog)
	}

	// 7. Verify that all legacy_ prefix tables have been dropped
	legacyTablesToCheck := []string{
		"legacy_users",
		"legacy_options",
		"legacy_origins",
		"legacy_apply_logs",
		"legacy_proxy_routes",
		"legacy_nodes",
		"legacy_waf_rule_groups",
		"legacy_waf_rule_group_bindings",
		"legacy_waf_ip_groups",
		"legacy_tls_certificates",
		"legacy_managed_domains",
		"legacy_dns_accounts",
		"legacy_acme_accounts",
		"legacy_config_versions",
		"legacy_pages_projects",
		"legacy_pages_deployments",
		"legacy_pages_deployment_files",
		"users",
		"options",
		"origins",
		"apply_logs",
		"node_access_logs_00",
		"node_system_profiles",
	}

	for _, table := range legacyTablesToCheck {
		var count int
		if err := sqliteDB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count).Error; err != nil {
			t.Fatalf("check table %s existence failed: %v", table, err)
		}
		if count > 0 {
			t.Errorf("expected table %s to be dropped, but it still exists", table)
		}
	}
}
