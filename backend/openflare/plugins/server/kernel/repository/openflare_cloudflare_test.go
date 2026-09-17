// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"testing"

	"Wavelet/openflare/plugins/server/kernel/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupCloudflareRepositoryDB(t *testing.T) *gorm.DB {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := conn.AutoMigrate(
		&model.CFConnection{}, &model.CFPointingGroup{}, &model.CFPointingMember{},
		&model.Zone{}, &model.ZoneDomain{}, &model.OpenFlareNode{}, &model.DNSAccount{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	SetDBForTest(conn)
	t.Cleanup(func() { SetDBForTest(nil) })
	return conn
}

func TestUpsertCFConnectionKeepsSingleRow(t *testing.T) {
	setupCloudflareRepositoryDB(t)
	ctx := context.Background()

	first := &model.CFConnection{Source: model.CFConnectionSourceStandalone, Authorization: "one"}
	if err := UpsertCFConnection(ctx, first); err != nil {
		t.Fatalf("UpsertCFConnection(first) error = %v", err)
	}
	accountID := uint(9)
	second := &model.CFConnection{Source: model.CFConnectionSourceDNSAccount, DNSAccountID: &accountID}
	if err := UpsertCFConnection(ctx, second); err != nil {
		t.Fatalf("UpsertCFConnection(second) error = %v", err)
	}

	got, err := GetCFConnection(ctx)
	if err != nil {
		t.Fatalf("GetCFConnection() error = %v", err)
	}
	if got.ID != first.ID || got.Source != model.CFConnectionSourceDNSAccount {
		t.Errorf("GetCFConnection() = %+v, want same row with dns_account source", got)
	}
}

func TestListAvailableCFZoneDomainsExcludesMembers(t *testing.T) {
	conn := setupCloudflareRepositoryDB(t)
	ctx := context.Background()
	zone := model.Zone{Domain: "example.com"}
	if err := conn.Create(&zone).Error; err != nil {
		t.Fatalf("Create(zone) error = %v", err)
	}
	domains := []model.ZoneDomain{
		{ZoneID: zone.ID, Domain: "api.example.com"},
		{ZoneID: zone.ID, Domain: "www.example.com"},
	}
	if err := conn.Create(&domains).Error; err != nil {
		t.Fatalf("Create(domains) error = %v", err)
	}
	group := model.CFPointingGroup{Name: "edge", PrimaryNodeID: 1, ActiveNodeID: 1, Enabled: true}
	if err := CreateCFPointingGroup(ctx, &group); err != nil {
		t.Fatalf("CreateCFPointingGroup() error = %v", err)
	}
	member := model.CFPointingMember{GroupID: group.ID, ZoneDomainID: domains[0].ID}
	if err := CreateCFPointingMember(ctx, &member); err != nil {
		t.Fatalf("CreateCFPointingMember() error = %v", err)
	}

	got, err := ListAvailableCFZoneDomains(ctx)
	if err != nil {
		t.Fatalf("ListAvailableCFZoneDomains() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != domains[1].ID {
		t.Errorf("ListAvailableCFZoneDomains() = %+v, want only %d", got, domains[1].ID)
	}
}
