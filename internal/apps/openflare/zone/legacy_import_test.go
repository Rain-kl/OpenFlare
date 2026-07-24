// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package zone

import (
	"context"
	"database/sql"
	"testing"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLegacyImportDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := gormDB.DB()
	require.NoError(t, err)

	// Pre-phase-2 schema: legacy route columns + managed domains + zone tables.
	stmts := []string{
		`CREATE TABLE of_zones (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			domain TEXT NOT NULL UNIQUE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE of_zone_domains (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			zone_id INTEGER NOT NULL,
			proxy_route_id INTEGER,
			domain TEXT NOT NULL UNIQUE,
			cert_id INTEGER,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE of_proxy_routes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site_name TEXT NOT NULL DEFAULT '',
			domain TEXT NOT NULL DEFAULT '',
			domains TEXT NOT NULL DEFAULT '[]',
			domain_cert_ids TEXT NOT NULL DEFAULT '[]',
			origin_url TEXT NOT NULL DEFAULT '',
			remark TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE of_tls_certificates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE of_managed_domains (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			domain TEXT NOT NULL,
			cert_id INTEGER,
			remark TEXT NOT NULL DEFAULT ''
		)`,
	}
	for _, stmt := range stmts {
		_, err := sqlDB.Exec(stmt)
		require.NoError(t, err)
	}

	previous := db.DB(context.Background())
	db.SetDB(gormDB)
	return sqlDB, func() {
		db.SetDB(previous)
		_ = sqlDB.Close()
	}
}

func TestImportLegacyTxBindsRouteDomains(t *testing.T) {
	sqlDB, cleanup := setupLegacyImportDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := sqlDB.Exec(`INSERT INTO of_tls_certificates (id, name) VALUES (7, 'cert')`)
	require.NoError(t, err)
	_, err = sqlDB.Exec(`
		INSERT INTO of_proxy_routes (id, site_name, domain, domains, domain_cert_ids, origin_url, remark)
		VALUES (3, 'api', 'api.example.com', '["api.example.com","www.example.com"]', '[7,7]', 'http://origin', 'r')
	`)
	require.NoError(t, err)

	tx, err := sqlDB.Begin()
	require.NoError(t, err)
	report, err := ImportLegacyTx(ctx, tx, false)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	assert.Equal(t, 1, report.Zones)
	assert.Equal(t, 2, report.Domains)

	var zoneDomain string
	require.NoError(t, sqlDB.QueryRow(`SELECT domain FROM of_zones`).Scan(&zoneDomain))
	assert.Equal(t, "example.com", zoneDomain)

	var count int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM of_zone_domains WHERE proxy_route_id = 3`).Scan(&count))
	assert.Equal(t, 2, count)

	// Idempotent re-run
	tx, err = sqlDB.Begin()
	require.NoError(t, err)
	report2, err := ImportLegacyTx(ctx, tx, false)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	assert.Equal(t, 0, report2.Domains)
}

func TestImportLegacyTxNoOpWithoutLegacyColumns(t *testing.T) {
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := gormDB.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	_, err = sqlDB.Exec(`
		CREATE TABLE of_zones (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			domain TEXT NOT NULL UNIQUE,
			created_at DATETIME, updated_at DATETIME
		);
		CREATE TABLE of_zone_domains (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			zone_id INTEGER NOT NULL,
			proxy_route_id INTEGER,
			domain TEXT NOT NULL UNIQUE,
			cert_id INTEGER,
			created_at DATETIME, updated_at DATETIME
		);
		CREATE TABLE of_proxy_routes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site_name TEXT NOT NULL DEFAULT '',
			origin_url TEXT NOT NULL DEFAULT ''
		);
	`)
	require.NoError(t, err)

	tx, err := sqlDB.Begin()
	require.NoError(t, err)
	report, err := ImportLegacyTx(context.Background(), tx, false)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	assert.Equal(t, 0, report.Zones)
	assert.Equal(t, 0, report.Domains)
}
