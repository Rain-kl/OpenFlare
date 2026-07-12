// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package zone

import (
	"context"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupZoneDB(t *testing.T) context.Context {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	require.NoError(t, err)
	require.NoError(t, conn.AutoMigrate(&model.Zone{}, &model.ZoneDomain{}, &model.TLSCertificate{}))
	db.SetDB(conn)
	t.Cleanup(func() { db.SetDB(nil) })
	return context.Background()
}

func TestCreateZoneDomainRejectsWildcard(t *testing.T) {
	ctx := setupZoneDB(t)
	zone, err := Create(ctx, Input{Domain: "example.com"})
	require.NoError(t, err)
	_, err = CreateDomain(ctx, zone.ID, DomainInput{Domain: "*.example.com"})
	require.EqualError(t, err, errDomainWildcardUnsupported)
}

func TestDeleteDomainRejectsBoundRoute(t *testing.T) {
	ctx := setupZoneDB(t)
	zone, err := Create(ctx, Input{Domain: "example.com"})
	require.NoError(t, err)
	item, err := CreateDomain(ctx, zone.ID, DomainInput{Domain: "api.example.com"})
	require.NoError(t, err)
	routeID := uint(9)
	item.ProxyRouteID = &routeID
	require.NoError(t, db.DB(ctx).Save(item).Error)

	err = DeleteDomain(ctx, zone.ID, item.ID)
	require.EqualError(t, err, errDomainBoundToRoute)

	item.ProxyRouteID = nil
	require.NoError(t, db.DB(ctx).Save(item).Error)
	require.NoError(t, DeleteDomain(ctx, zone.ID, item.ID))
}

func TestLegacyImportUsesEffectiveTLDPlusOne(t *testing.T) {
	root, err := zoneRoot("api.example.co.uk")
	require.NoError(t, err)
	require.Equal(t, "example.co.uk", root)
}

func TestGetStatsAggregatesZoneHosts(t *testing.T) {
	ctx := setupZoneDB(t)
	reset := model.SetAccessLogStoreForTest(model.NewMemoryAccessLogStore())
	t.Cleanup(reset)

	zone, err := Create(ctx, Input{Domain: "example.com"})
	require.NoError(t, err)
	_, err = CreateDomain(ctx, zone.ID, DomainInput{Domain: "api.example.com"})
	require.NoError(t, err)
	_, err = CreateDomain(ctx, zone.ID, DomainInput{Domain: "www.example.com"})
	require.NoError(t, err)

	now := time.Now().UTC()
	require.NoError(t, model.InsertOpenFlareAccessLogsBatch(ctx, []*model.OpenFlareAccessLog{
		{NodeID: "n1", LoggedAt: now.Add(-1 * time.Hour), RemoteAddr: "1.1.1.1", Host: "api.example.com", Path: "/", StatusCode: 200},
		{NodeID: "n1", LoggedAt: now.Add(-2 * time.Hour), RemoteAddr: "1.1.1.1", Host: "www.example.com", Path: "/", StatusCode: 200},
		{NodeID: "n1", LoggedAt: now.Add(-3 * time.Hour), RemoteAddr: "2.2.2.2", Host: "api.example.com", Path: "/x", StatusCode: 404},
		{NodeID: "n1", LoggedAt: now.Add(-3 * time.Hour), RemoteAddr: "3.3.3.3", Host: "other.com", Path: "/", StatusCode: 200},
		{NodeID: "n1", LoggedAt: now.Add(-48 * time.Hour), RemoteAddr: "4.4.4.4", Host: "api.example.com", Path: "/", StatusCode: 200},
	}))

	stats, err := GetStats(ctx, zone.ID, "24h")
	require.NoError(t, err)
	require.Equal(t, StatsRange24h, stats.Range)
	require.Equal(t, int64(3), stats.RequestCount)
	require.Equal(t, int64(2), stats.UniqueVisitors)
	require.Equal(t, 2, stats.DomainCount)
	require.True(t, stats.Available)
	require.NotEmpty(t, stats.Series)
	require.Equal(t, 60, stats.BucketMinutes)
	var seriesRequests int64
	for _, point := range stats.Series {
		seriesRequests += point.RequestCount
	}
	require.Equal(t, int64(3), seriesRequests)

	stats7d, err := GetStats(ctx, zone.ID, "7d")
	require.NoError(t, err)
	require.Equal(t, int64(4), stats7d.RequestCount)
	require.Equal(t, int64(3), stats7d.UniqueVisitors)
	require.NotEmpty(t, stats7d.Series)

	_, err = GetStats(ctx, zone.ID, "1h")
	require.EqualError(t, err, errStatsRangeInvalid)
}
