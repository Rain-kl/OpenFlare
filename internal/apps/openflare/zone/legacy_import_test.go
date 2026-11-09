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

// TestImportLegacyTxUnicodeDomains canonicalizes historical names and preserves bindings on upgrade.
func TestImportLegacyTxUnicodeDomains(t *testing.T) {
	for _, source := range []string{"route", "managed"} {
		t.Run(source, func(t *testing.T) {
			sqlDB, cleanup := setupLegacyImportDB(t)
			t.Cleanup(cleanup)
			ctx := context.Background()
			if _, err := sqlDB.Exec(`INSERT INTO of_tls_certificates (id, name) VALUES (7, 'cert')`); err != nil {
				t.Fatal(err)
			}
			var err error
			if source == "route" {
				_, err = sqlDB.Exec(`INSERT INTO of_proxy_routes (id, domain, domains, domain_cert_ids) VALUES (3, 'api.bücher.de', '["api.bücher.de"]', '[7]')`)
			} else {
				_, err = sqlDB.Exec(`INSERT INTO of_managed_domains (domain, cert_id) VALUES ('api.bücher.de', 7)`)
			}
			if err != nil {
				t.Fatal(err)
			}
			for attempt := range 2 {
				tx, err := sqlDB.Begin()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = tx.Rollback() })
				report, err := ImportLegacyTx(ctx, tx, false)
				if err != nil {
					t.Fatalf("ImportLegacyTx(%s, attempt=%d) = %+v, %v, want no conflicts", source, attempt, report, err)
				}
				if err := tx.Commit(); err != nil {
					t.Fatal(err)
				}
				want := 1 - attempt
				if report.Zones != want || report.Domains != want {
					t.Errorf("ImportLegacyTx(%s, attempt=%d) = %+v, want %d zones/domains", source, attempt, report, want)
				}
			}
			var root, domain string
			var certID uint
			var routeID sql.NullInt64
			if err := sqlDB.QueryRow(`SELECT z.domain, zd.domain, zd.cert_id, zd.proxy_route_id FROM of_zone_domains zd JOIN of_zones z ON z.id = zd.zone_id`).Scan(&root, &domain, &certID, &routeID); err != nil {
				t.Fatal(err)
			}
			if root != "xn--bcher-kva.de" || domain != "api.xn--bcher-kva.de" || certID != 7 {
				t.Errorf("imported %s = (%q, %q, cert=%d), want (xn--bcher-kva.de, api.xn--bcher-kva.de, cert=7)", source, root, domain, certID)
			}
			if (source == "route" && (!routeID.Valid || routeID.Int64 != 3)) || (source == "managed" && routeID.Valid) {
				t.Errorf("imported %s route binding = %v", source, routeID)
			}

			var zoneID uint
			if err := sqlDB.QueryRow(`SELECT id FROM of_zones`).Scan(&zoneID); err != nil {
				t.Fatal(err)
			}
			if _, err := Create(ctx, Input{Domain: "xn--bcher-kva.de"}); err == nil {
				t.Error("Create accepted an equivalent imported IDN zone")
			}
			if _, err := CreateDomain(ctx, zoneID, DomainInput{Domain: "api.xn--bcher-kva.de"}); err == nil {
				t.Error("CreateDomain accepted an equivalent imported IDN host")
			}
			if _, err := CreateDomain(ctx, zoneID, DomainInput{Domain: "www.xn--bcher-kva.de"}); err != nil {
				t.Errorf("CreateDomain in imported IDN zone: %v", err)
			}
			if _, err := normalizeDomain("api.bücher.de"); err == nil {
				t.Error("normalizeDomain(api.bücher.de) accepted a new Unicode domain")
			}
		})
	}
}

// TestImportLegacyTxIDNBindingConflict rejects equivalent names bound to different routes.
func TestImportLegacyTxIDNBindingConflict(t *testing.T) {
	sqlDB, cleanup := setupLegacyImportDB(t)
	t.Cleanup(cleanup)
	if _, err := sqlDB.Exec(`INSERT INTO of_proxy_routes (id, domain, domains) VALUES (3, 'api.bücher.de', '["api.bücher.de"]'), (4, 'api.xn--bcher-kva.de', '["api.xn--bcher-kva.de"]')`); err != nil {
		t.Fatal(err)
	}
	tx, err := sqlDB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	report, err := ImportLegacyTx(context.Background(), tx, false)
	if err == nil || len(report.Conflicts) != 1 {
		t.Errorf("ImportLegacyTx(equivalent IDN with different bindings) = %+v, %v; want one conflict", report, err)
	}
}

// TestNormalizeLegacyIDNRoot resolves Unicode public suffixes through their ASCII identity.
func TestNormalizeLegacyIDNRoot(t *testing.T) {
	domain, err := normalizeLegacyDomain("api.example.公司.cn")
	if err != nil {
		t.Fatal(err)
	}
	root, err := zoneRoot(domain)
	if err != nil || root != "example.xn--55qx5d.cn" {
		t.Errorf("zoneRoot(%q) = %q, %v; want example.xn--55qx5d.cn", domain, root, err)
	}
}

// TestNormalizeLegacyIDNCanonicalForms keeps Unicode normalization and wildcard identity consistent.
func TestNormalizeLegacyIDNCanonicalForms(t *testing.T) {
	for _, raw := range []string{"api.bücher.de", "api.bu\u0308cher.de", "API.XN--BCHER-KVA.DE"} {
		domain, err := normalizeLegacyDomain(raw)
		if err != nil || domain != "api.xn--bcher-kva.de" {
			t.Errorf("normalizeLegacyDomain(%q) = %q, %v; want api.xn--bcher-kva.de", raw, domain, err)
		}
	}
	domain, err := normalizeLegacyDomain("*.bücher.de")
	if err != nil || domain != "*.xn--bcher-kva.de" {
		t.Errorf("normalizeLegacyDomain(*.bücher.de) = %q, %v; want *.xn--bcher-kva.de", domain, err)
	}
}
