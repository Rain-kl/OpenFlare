// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/model"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupZoneTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(&model.Zone{}, &model.ZoneDomain{}))
	db.SetDB(sqliteDB)
	t.Cleanup(func() { db.SetDB(nil) })
	return sqliteDB
}

func TestReplaceZoneDomainRouteBindingsRejectsForeignDomain(t *testing.T) {
	conn := setupZoneTestDB(t)
	ctx := context.Background()

	zone := model.Zone{Domain: "example.com"}
	require.NoError(t, conn.Create(&zone).Error)
	foreignRouteID := uint(11)
	domain := model.ZoneDomain{
		ZoneID:       zone.ID,
		ProxyRouteID: &foreignRouteID,
		Domain:       "api.example.com",
	}
	require.NoError(t, conn.Create(&domain).Error)

	err := ReplaceZoneDomainRouteBindings(ctx, 12, []uint{domain.ID})
	require.Error(t, err)

	var got model.ZoneDomain
	require.NoError(t, conn.First(&got, domain.ID).Error)
	require.Equal(t, &foreignRouteID, got.ProxyRouteID)
}

func TestReplaceZoneDomainRouteBindingsReplacesCurrentRouteBindings(t *testing.T) {
	conn := setupZoneTestDB(t)
	ctx := context.Background()

	zone := model.Zone{Domain: "example.com"}
	require.NoError(t, conn.Create(&zone).Error)
	routeID := uint(21)
	boundDomain := model.ZoneDomain{ZoneID: zone.ID, ProxyRouteID: &routeID, Domain: "old.example.com"}
	requestedDomain := model.ZoneDomain{ZoneID: zone.ID, Domain: "new.example.com"}
	require.NoError(t, conn.Create(&boundDomain).Error)
	require.NoError(t, conn.Create(&requestedDomain).Error)

	require.NoError(t, ReplaceZoneDomainRouteBindings(ctx, routeID, []uint{requestedDomain.ID}))

	var domains []model.ZoneDomain
	require.NoError(t, conn.Order("id asc").Find(&domains).Error)
	require.Len(t, domains, 2)
	require.Nil(t, domains[0].ProxyRouteID)
	require.Equal(t, &routeID, domains[1].ProxyRouteID)
}

func TestListZoneDomainsByRouteID(t *testing.T) {
	conn := setupZoneTestDB(t)
	ctx := context.Background()

	zone := model.Zone{Domain: "example.com"}
	require.NoError(t, conn.Create(&zone).Error)
	routeID := uint(31)
	boundDomain := model.ZoneDomain{ZoneID: zone.ID, ProxyRouteID: &routeID, Domain: "api.example.com"}
	unboundDomain := model.ZoneDomain{ZoneID: zone.ID, Domain: "www.example.com"}
	require.NoError(t, conn.Create(&boundDomain).Error)
	require.NoError(t, conn.Create(&unboundDomain).Error)

	domains, err := ListZoneDomainsByRouteID(ctx, routeID)
	require.NoError(t, err)
	require.Len(t, domains, 1)
	require.Equal(t, boundDomain.ID, domains[0].ID)
}
