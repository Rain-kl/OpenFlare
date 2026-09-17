// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cloudflare

import (
	"context"
	"errors"
	"testing"

	"Wavelet/openflare/plugins/server/kernel/credential"
	"Wavelet/openflare/plugins/server/kernel/model"
	"Wavelet/openflare/plugins/server/kernel/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type fakeClient struct {
	records      []DNSRecord
	created      *RecordInput
	updated      *RecordInput
	deleted      []string
	deleteErrors map[string]error
}

func (client *fakeClient) VerifyToken(context.Context) error { return nil }
func (client *fakeClient) FindZone(context.Context, string) (*Zone, error) {
	return &Zone{ID: "zone-1", Name: "example.com"}, nil
}
func (client *fakeClient) GetRecord(context.Context, string, string) (*DNSRecord, error) {
	return nil, errors.New("not found")
}
func (client *fakeClient) ListARecords(context.Context, string, string) ([]DNSRecord, error) {
	return client.records, nil
}
func (client *fakeClient) CreateARecord(_ context.Context, _ string, input RecordInput) (*DNSRecord, error) {
	client.created = &input
	return &DNSRecord{ID: "record-created", Name: input.Name, Content: input.Content, Proxied: input.Proxied}, nil
}
func (client *fakeClient) UpdateARecord(_ context.Context, _, id string, input RecordInput) (*DNSRecord, error) {
	client.updated = &input
	return &DNSRecord{ID: id, Name: input.Name, Content: input.Content, Proxied: input.Proxied}, nil
}
func (client *fakeClient) DeleteRecord(_ context.Context, _, recordID string) error {
	client.deleted = append(client.deleted, recordID)
	return client.deleteErrors[recordID]
}

func setupCloudflareLogicDB(t *testing.T) (context.Context, uint) {
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
	repository.SetDBForTest(conn)
	t.Cleanup(func() { repository.SetDBForTest(nil) })
	ctx := context.Background()
	sealed, err := credential.Seal(`{"api_token":"test-token"}`)
	if err != nil {
		t.Fatalf("credential.Seal() error = %v", err)
	}
	if err := repository.UpsertCFConnection(ctx, &model.CFConnection{Source: model.CFConnectionSourceStandalone, Authorization: sealed, Status: model.CFConnectionStatusReady}); err != nil {
		t.Fatalf("UpsertCFConnection() error = %v", err)
	}
	zone := model.Zone{Domain: "example.com"}
	node := model.OpenFlareNode{Name: "edge", NodeID: "node-1", NodeType: "edge_node", IP: "203.0.113.10"}
	if err := conn.Create(&zone).Error; err != nil {
		t.Fatalf("Create(zone) error = %v", err)
	}
	if err := conn.Create(&node).Error; err != nil {
		t.Fatalf("Create(node) error = %v", err)
	}
	domain := model.ZoneDomain{ZoneID: zone.ID, Domain: "api.example.com"}
	if err := conn.Create(&domain).Error; err != nil {
		t.Fatalf("Create(domain) error = %v", err)
	}
	group := model.CFPointingGroup{Name: "primary", PrimaryNodeID: node.ID, ActiveNodeID: node.ID, DefaultProxied: true, Enabled: true}
	if err := conn.Create(&group).Error; err != nil {
		t.Fatalf("Create(group) error = %v", err)
	}
	member := model.CFPointingMember{GroupID: group.ID, ZoneDomainID: domain.ID, Proxied: true, SyncStatus: model.CFMemberSyncPending}
	if err := conn.Create(&member).Error; err != nil {
		t.Fatalf("Create(member) error = %v", err)
	}
	return ctx, member.ID
}

func TestReconcileMemberCreatesMissingARecord(t *testing.T) {
	ctx, memberID := setupCloudflareLogicDB(t)
	fake := &fakeClient{}
	restore := SetClientFactoryForTest(func(string) Client { return fake })
	t.Cleanup(restore)

	if err := ReconcileMember(ctx, memberID); err != nil {
		t.Fatalf("ReconcileMember() error = %v", err)
	}
	if fake.created == nil || fake.created.Content != "203.0.113.10" || !fake.created.Proxied || fake.created.TTL != 1 {
		t.Errorf("CreateARecord input = %+v", fake.created)
	}
	member, err := repository.GetCFPointingMemberByID(ctx, memberID)
	if err != nil {
		t.Fatalf("GetCFPointingMemberByID() error = %v", err)
	}
	if member.SyncStatus != model.CFMemberSyncOK || member.CFRecordID != "record-created" || member.DesiredIP != "203.0.113.10" {
		t.Errorf("reconciled member = %+v", member)
	}
}

func TestReconcileMemberRejectsMultipleSameNameARecords(t *testing.T) {
	ctx, memberID := setupCloudflareLogicDB(t)
	fake := &fakeClient{records: []DNSRecord{{ID: "one"}, {ID: "two"}}}
	restore := SetClientFactoryForTest(func(string) Client { return fake })
	t.Cleanup(restore)

	if err := ReconcileMember(ctx, memberID); err == nil {
		t.Fatal("ReconcileMember() error = nil, want duplicate record error")
	}
	member, err := repository.GetCFPointingMemberByID(ctx, memberID)
	if err != nil {
		t.Fatalf("GetCFPointingMemberByID() error = %v", err)
	}
	if member.SyncStatus != model.CFMemberSyncError || member.LastError == "" {
		t.Errorf("failed member = %+v", member)
	}
}

func TestCreateMemberCopiesGroupDefaultProxied(t *testing.T) {
	ctx, existingMemberID := setupCloudflareLogicDB(t)
	existing, err := repository.GetCFPointingMemberByID(ctx, existingMemberID)
	if err != nil {
		t.Fatalf("GetCFPointingMemberByID() error = %v", err)
	}
	group, err := repository.GetCFPointingGroup(ctx, existing.GroupID)
	if err != nil {
		t.Fatalf("GetCFPointingGroup() error = %v", err)
	}
	zone := model.Zone{Domain: "example.net"}
	if err := repository.DB(ctx).Create(&zone).Error; err != nil {
		t.Fatalf("Create(zone) error = %v", err)
	}
	domain := model.ZoneDomain{ZoneID: zone.ID, Domain: "www.example.net"}
	if err := repository.DB(ctx).Create(&domain).Error; err != nil {
		t.Fatalf("Create(domain) error = %v", err)
	}
	restore := SetDispatchTaskForTest(func(context.Context, string, []byte, string) (string, error) { return "task-1", nil })
	t.Cleanup(restore)

	member, err := CreateMember(ctx, group.ID, MemberCreateInput{ZoneDomainID: domain.ID})
	if err != nil {
		t.Fatalf("CreateMember() error = %v", err)
	}
	if !member.Proxied {
		t.Error("CreateMember() proxied = false, want group default true")
	}
}

func TestDeleteManagedRecordFallsBackWhenCachedRecordIsStale(t *testing.T) {
	ctx, memberID := setupCloudflareLogicDB(t)
	if err := repository.UpdateCFPointingMemberColumns(ctx, memberID, map[string]any{
		"cf_zone_id":   "zone-1",
		"cf_record_id": "stale-record",
	}); err != nil {
		t.Fatalf("UpdateCFPointingMemberColumns() error = %v", err)
	}
	fake := &fakeClient{
		records:      []DNSRecord{{ID: "actual-record"}},
		deleteErrors: map[string]error{"stale-record": errors.New("not found")},
	}
	restore := SetClientFactoryForTest(func(string) Client { return fake })
	t.Cleanup(restore)

	if err := DeleteManagedRecord(ctx, memberID); err != nil {
		t.Fatalf("DeleteManagedRecord() error = %v", err)
	}
	if len(fake.deleted) != 2 || fake.deleted[0] != "stale-record" || fake.deleted[1] != "actual-record" {
		t.Errorf("deleted record IDs = %v, want [stale-record actual-record]", fake.deleted)
	}
}

func TestUpdateMemberDoesNotDispatchWhenGroupIsDisabled(t *testing.T) {
	ctx, memberID := setupCloudflareLogicDB(t)
	member, err := repository.GetCFPointingMemberByID(ctx, memberID)
	if err != nil {
		t.Fatalf("GetCFPointingMemberByID() error = %v", err)
	}
	group, err := repository.GetCFPointingGroup(ctx, member.GroupID)
	if err != nil {
		t.Fatalf("GetCFPointingGroup() error = %v", err)
	}
	group.Enabled = false
	if err = repository.SaveCFPointingGroup(ctx, group); err != nil {
		t.Fatalf("SaveCFPointingGroup() error = %v", err)
	}
	dispatchCount := 0
	restore := SetDispatchTaskForTest(func(context.Context, string, []byte, string) (string, error) {
		dispatchCount++
		return "task-1", nil
	})
	t.Cleanup(restore)

	updated, err := UpdateMember(ctx, group.ID, memberID, MemberUpdateInput{Proxied: false})
	if err != nil {
		t.Fatalf("UpdateMember() error = %v", err)
	}
	if updated.SyncStatus != model.CFMemberSyncPending {
		t.Errorf("UpdateMember() sync status = %q, want %q", updated.SyncStatus, model.CFMemberSyncPending)
	}
	if dispatchCount != 0 {
		t.Errorf("dispatch count = %d, want 0", dispatchCount)
	}
}
