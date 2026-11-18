// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/zone"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/infra/persistence/migrator"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestZoneCreateRespectsImportedIDNIdentity(t *testing.T) {
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	require.NoError(t, err)
	require.NoError(t, gormDB.AutoMigrate(&model.Zone{}, &model.ZoneDomain{}, &model.TLSCertificate{}, &model.CFPointingGroup{}, &model.CFPointingMember{}))

	require.NoError(t, gormDB.Create(&model.TLSCertificate{
		ID:            7,
		Name:          "cert",
		PrimaryDomain: "api.xn--bcher-kva.de",
		CertPEM:       "dummy-cert",
		KeyPEM:        "dummy-key",
		Provider:      "manual",
	}).Error)

	sqlDB, err := gormDB.DB()
	require.NoError(t, err)
	_, err = sqlDB.Exec(`
		CREATE TABLE of_proxy_routes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site_name TEXT NOT NULL DEFAULT '',
			domain TEXT NOT NULL DEFAULT '',
			domains TEXT NOT NULL DEFAULT '[]',
			domain_cert_ids TEXT NOT NULL DEFAULT '[]',
			origin_url TEXT NOT NULL DEFAULT '',
			remark TEXT NOT NULL DEFAULT ''
		);
		INSERT INTO of_proxy_routes (id, domain, domains, domain_cert_ids) VALUES (3, 'api.bücher.de', '["api.bücher.de"]', '[7]');
	`)
	require.NoError(t, err)

	previous := db.DB(context.Background())
	db.SetDB(gormDB)
	t.Cleanup(func() {
		db.SetDB(previous)
		_ = sqlDB.Close()
	})

	ctx := context.Background()
	tx, err := sqlDB.Begin()
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })

	report, err := migrator.ImportZoneLegacyTx(ctx, tx, false)
	require.NoError(t, err)
	require.Equal(t, 1, report.Zones)
	require.Equal(t, 1, report.Domains)
	require.NoError(t, tx.Commit())

	var zoneID uint
	var routeID sql.NullInt64
	require.NoError(t, sqlDB.QueryRow(`SELECT z.id, zd.proxy_route_id FROM of_zone_domains zd JOIN of_zones z ON z.id = zd.zone_id`).Scan(&zoneID, &routeID))
	require.True(t, routeID.Valid && routeID.Int64 == 3)

	if _, err := zone.Create(ctx, zone.Input{Domain: "xn--bcher-kva.de"}); err == nil {
		t.Error("Create accepted an equivalent imported IDN zone")
	}
	if _, err := zone.CreateDomain(ctx, zoneID, zone.DomainInput{Domain: "api.xn--bcher-kva.de"}); err == nil {
		t.Error("CreateDomain accepted an equivalent imported IDN host")
	}
	if _, err := zone.CreateDomain(ctx, zoneID, zone.DomainInput{Domain: "www.xn--bcher-kva.de"}); err != nil {
		t.Errorf("CreateDomain in imported IDN zone: %v", err)
	}
}
