// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package zone

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/testhelper"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupZoneDB(t *testing.T) context.Context {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	require.NoError(t, err)
	require.NoError(t, conn.AutoMigrate(&model.Zone{}, &model.ZoneDomain{}, &model.TLSCertificate{}, &model.CFPointingGroup{}, &model.CFPointingMember{}))
	db.SetDB(conn)
	t.Cleanup(func() { db.SetDB(nil) })
	return context.Background()
}

func TestNormalizeDomainWildcard(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: " *.Example.COM ", want: "*.example.com"},
		{input: "*.s3.example.com", want: "*.s3.example.com"},
		{input: "*.example.co.uk", want: "*.example.co.uk"},
		{input: "API.Example.com", want: "api.example.com"},
		{input: "example.com", want: "example.com"},
		{input: "*"}, {input: "*.com"}, {input: "*.co.uk"},
		{input: "foo*.example.com"}, {input: "a.*.example.com"},
		{input: "*.*.example.com"}, {input: "**.example.com"},
		{input: "*.example.com."}, {input: "*..example.com"},
		{input: "*.-s3.example.com"}, {input: "*.s3-.example.com"},
		{input: "*.s_3.example.com"}, {input: "*.example.com:443"},
		{input: "*.example.com;"}, {input: "*.a b.example.com"},
		{input: "*.a\nb.example.com"}, {input: "https://*.example.com"},
		{input: "*.example.com/path"}, {input: "*.example.com?query"},
		{input: "*.example.com#fragment"}, {input: "*@example.com"},
		{input: "*." + strings.Repeat("a", 64) + ".example.com"},
		{input: "*." + strings.Repeat(strings.Repeat("a", 63)+".", 4) + "com"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := normalizeDomain(tt.input)
			if got != tt.want || (err != nil) != (tt.want == "") {
				t.Errorf("normalizeDomain(%q) = (%q, %v), want %q with error=%t", tt.input, got, err, tt.want, tt.want == "")
			}
		})
	}
}

func TestCreateAndUpdateWildcardDomain(t *testing.T) {
	ctx := setupZoneDB(t)
	zone, err := Create(ctx, Input{Domain: "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := CreateDomain(ctx, zone.ID, DomainInput{Domain: " *.Example.COM "})
	if err != nil {
		t.Fatalf("CreateDomain(wildcard) error = %v", err)
	}
	if item.Domain != "*.example.com" {
		t.Errorf("CreateDomain(wildcard).Domain = %q, want *.example.com", item.Domain)
	}
	if _, err := CreateDomain(ctx, zone.ID, DomainInput{Domain: "*.example.com"}); err == nil {
		t.Error("CreateDomain(duplicate wildcard) error = nil, want duplicate error")
	}
	// 精确域名可以和通配符共存，路由匹配优先级交给 OpenResty。
	if _, err := CreateDomain(ctx, zone.ID, DomainInput{Domain: "api.example.com"}); err != nil {
		t.Fatalf("CreateDomain(exact alongside wildcard) error = %v", err)
	}
	for _, domain := range []string{"*.s3.example.com", "www.example.com", "*.example.com"} {
		updated, err := UpdateDomain(ctx, zone.ID, item.ID, DomainInput{Domain: domain})
		if err != nil {
			t.Fatalf("UpdateDomain(%q) error = %v", domain, err)
		}
		if updated.Domain != domain {
			t.Errorf("UpdateDomain(%q).Domain = %q", domain, updated.Domain)
		}
	}
	for _, domain := range []string{"*.other.com", "*.example.com.evil.net", "a.*.example.com"} {
		if _, err := CreateDomain(ctx, zone.ID, DomainInput{Domain: domain}); err == nil {
			t.Errorf("CreateDomain(%q) error = nil, want invalid/outside-zone error", domain)
		}
		if _, err := UpdateDomain(ctx, zone.ID, item.ID, DomainInput{Domain: domain}); err == nil {
			t.Errorf("UpdateDomain(%q) error = nil, want invalid/outside-zone error", domain)
		}
	}
	stored, err := repository.GetZoneDomainByZoneAndID(ctx, zone.ID, item.ID)
	if err != nil || stored.Domain != "*.example.com" {
		t.Fatalf("stored domain after rejected updates = %+v, error=%v, want *.example.com", stored, err)
	}
}

func TestZoneRootRejectsWildcard(t *testing.T) {
	ctx := setupZoneDB(t)
	if _, err := Create(ctx, Input{Domain: "*.example.com"}); err == nil {
		t.Error("Create(wildcard zone) error = nil, want invalid root error")
	}
	zone, err := Create(ctx, Input{Domain: "example.co.uk"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Update(ctx, zone.ID, Input{Domain: "*.example.co.uk"}); err == nil {
		t.Error("Update(wildcard zone) error = nil, want invalid root error")
	}
	if _, err := CreateDomain(ctx, zone.ID, DomainInput{Domain: "*.s3.example.co.uk"}); err != nil {
		t.Errorf("CreateDomain(wildcard under multi-label public suffix) error = %v", err)
	}
}

func TestDeleteDomainRejectsBoundRoute(t *testing.T) {
	ctx := setupZoneDB(t)
	zone, err := Create(ctx, Input{Domain: "example.com"})
	require.NoError(t, err)
	item, err := CreateDomain(ctx, zone.ID, DomainInput{Domain: "api.example.com"})
	require.NoError(t, err)
	routeID := uint(9)
	item.ProxyRouteID = &routeID
	require.NoError(t, repository.SaveZoneDomain(ctx, item))

	err = DeleteDomain(ctx, zone.ID, item.ID)
	require.EqualError(t, err, errDomainBoundToRoute)

	item.ProxyRouteID = nil
	require.NoError(t, repository.SaveZoneDomain(ctx, item))
	require.NoError(t, DeleteDomain(ctx, zone.ID, item.ID))
}

func TestDeleteDomainCleansUpCloudflareMember(t *testing.T) {
	ctx := setupZoneDB(t)
	zone, err := Create(ctx, Input{Domain: "example.com"})
	require.NoError(t, err)
	domain, err := CreateDomain(ctx, zone.ID, DomainInput{Domain: "api.example.com"})
	require.NoError(t, err)

	member := model.CFPointingMember{GroupID: 1, ZoneDomainID: domain.ID}
	require.NoError(t, repository.CreateCFPointingMember(ctx, &member))

	require.NoError(t, DeleteDomain(ctx, zone.ID, domain.ID))

	_, err = repository.GetCFPointingMemberByZoneDomainID(ctx, domain.ID)
	require.Error(t, err)
}

func TestLegacyImportUsesEffectiveTLDPlusOne(t *testing.T) {
	root, err := zoneRoot("api.example.co.uk")
	require.NoError(t, err)
	require.Equal(t, "example.co.uk", root)
}

func TestGetStatsAggregatesZoneHosts(t *testing.T) {
	ctx := setupZoneDB(t)
	testhelper.SetupLogStoresForTest(t)

	zone, err := Create(ctx, Input{Domain: "example.com"})
	require.NoError(t, err)
	_, err = CreateDomain(ctx, zone.ID, DomainInput{Domain: "api.example.com"})
	require.NoError(t, err)
	_, err = CreateDomain(ctx, zone.ID, DomainInput{Domain: "www.example.com"})
	require.NoError(t, err)

	now := time.Now().UTC()
	require.NoError(t, repository.InsertOpenFlareAccessLogsBatch(ctx, []*model.OpenFlareAccessLog{
		{NodeID: "n1", LoggedAt: now.Add(-1 * time.Hour), RemoteAddr: "1.1.1.1", Host: "api.example.com", Path: "/", StatusCode: 200, BytesSent: 1000},
		{NodeID: "n1", LoggedAt: now.Add(-2 * time.Hour), RemoteAddr: "1.1.1.1", Host: "www.example.com", Path: "/", StatusCode: 200, BytesSent: 500},
		{NodeID: "n1", LoggedAt: now.Add(-3 * time.Hour), RemoteAddr: "2.2.2.2", Host: "api.example.com", Path: "/x", StatusCode: 404, BytesSent: 200},
		{NodeID: "n1", LoggedAt: now.Add(-3 * time.Hour), RemoteAddr: "3.3.3.3", Host: "other.com", Path: "/", StatusCode: 200, BytesSent: 100},
		{NodeID: "n1", LoggedAt: now.Add(-48 * time.Hour), RemoteAddr: "4.4.4.4", Host: "api.example.com", Path: "/", StatusCode: 200, BytesSent: 800},
	}))

	stats, err := GetStats(ctx, zone.ID, "24h")
	require.NoError(t, err)
	require.Equal(t, StatsRange24h, stats.Range)
	require.Equal(t, int64(3), stats.RequestCount)
	require.Equal(t, int64(2), stats.UniqueVisitors)
	require.Equal(t, int64(1700), stats.BytesSent)
	require.Equal(t, 2, stats.DomainCount)
	require.True(t, stats.Available)
	require.NotEmpty(t, stats.Series)
	require.Equal(t, 60, stats.BucketMinutes)
	var seriesRequests int64
	var seriesBytes int64
	for _, point := range stats.Series {
		seriesRequests += point.RequestCount
		seriesBytes += point.BytesSent
	}
	require.Equal(t, int64(3), seriesRequests)
	require.Equal(t, int64(1700), seriesBytes)

	stats7d, err := GetStats(ctx, zone.ID, "7d")
	require.NoError(t, err)
	require.Equal(t, int64(4), stats7d.RequestCount)
	require.Equal(t, int64(3), stats7d.UniqueVisitors)
	require.Equal(t, int64(2500), stats7d.BytesSent)
	require.NotEmpty(t, stats7d.Series)

	_, err = GetStats(ctx, zone.ID, "1h")
	require.EqualError(t, err, errStatsRangeInvalid)
}
