// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package migrator

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

func TestCloudflareGroupTTLMigrationsMatch(t *testing.T) {
	postgresSQL, err := fs.ReadFile(migrationFS, "goose/postgres/202611080001_add_cloudflare_group_ttl.sql")
	if err != nil {
		t.Fatalf("read PostgreSQL TTL migration: %v", err)
	}
	sqliteSQL, err := fs.ReadFile(migrationFS, "goose/sqlite/202611080001_add_cloudflare_group_ttl.sql")
	if err != nil {
		t.Fatalf("read SQLite TTL migration: %v", err)
	}
	if !bytes.Equal(postgresSQL, sqliteSQL) {
		t.Error("Cloudflare TTL migrations differ, want identical portable ADD/DROP COLUMN statements")
	}
}

func TestCloudflareGroupTTLMigrationSQLiteUpDownUp(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "cloudflare-ttl.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open(sqlite) error = %v", err)
	}
	sqlDB, err := conn.DB()
	if err != nil {
		t.Fatalf("conn.DB() error = %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("sqlDB.Close() error = %v", err)
		}
	})
	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect(dialectSqlite); err != nil {
		t.Fatalf("goose.SetDialect(sqlite) error = %v", err)
	}
	const previousVersion = int64(202610260002)
	const ttlVersion = int64(202611080001)
	if err := goose.UpTo(sqlDB, "goose/sqlite", previousVersion); err != nil {
		t.Fatalf("goose.UpTo(previous) error = %v", err)
	}
	if err := conn.Exec("INSERT INTO of_cf_pointing_groups (name, primary_node_id, active_node_id) VALUES ('legacy', 0, 0)").Error; err != nil {
		t.Fatalf("insert legacy group error = %v", err)
	}
	for cycle := range 2 {
		if err := goose.UpTo(sqlDB, "goose/sqlite", ttlVersion); err != nil {
			t.Fatalf("goose.UpTo(ttl), cycle %d error = %v", cycle, err)
		}
		var ttl int
		if err := conn.Raw("SELECT ttl FROM of_cf_pointing_groups WHERE name = 'legacy'").Scan(&ttl).Error; err != nil {
			t.Fatalf("read migrated ttl, cycle %d error = %v", cycle, err)
		}
		if ttl != 0 {
			t.Errorf("migrated legacy ttl, cycle %d = %d, want compatibility sentinel 0", cycle, ttl)
		}
		if conn.Migrator().HasColumn("of_cf_pointing_members", "ttl") {
			t.Error("TTL migration added member-level ttl, want group-level only")
		}
		if err := conn.Exec("UPDATE of_cf_pointing_groups SET ttl = 120 WHERE name = 'legacy'").Error; err != nil {
			t.Fatalf("write manual ttl error = %v", err)
		}
		if err := goose.DownTo(sqlDB, "goose/sqlite", previousVersion); err != nil {
			t.Fatalf("goose.DownTo(previous), cycle %d error = %v", cycle, err)
		}
		if conn.Migrator().HasColumn("of_cf_pointing_groups", "ttl") {
			t.Errorf("goose.DownTo(previous), cycle %d left ttl column", cycle)
		}
		var count int64
		if err := conn.Table("of_cf_pointing_groups").Where("name = ?", "legacy").Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("legacy groups after rollback = %d, error = %v, want 1", count, err)
		}
	}
}
