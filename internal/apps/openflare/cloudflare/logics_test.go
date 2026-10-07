// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cloudflare

import (
	"context"
	"errors"
	"testing"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

func TestCheckNodeFailoverContinuesAfterDispatchError(t *testing.T) {
	ctx, memberID := setupCloudflareLogicDB(t)
	member, err := repository.GetCFPointingMemberByID(ctx, memberID)
	if err != nil {
		t.Fatalf("GetCFPointingMemberByID() error = %v", err)
	}
	group, err := repository.GetCFPointingGroup(ctx, member.GroupID)
	if err != nil {
		t.Fatalf("GetCFPointingGroup() error = %v", err)
	}

	lastSeen := time.Now()
	backup := model.OpenFlareNode{Name: "backup", NodeID: "node-backup", NodeType: "edge_node", IP: "203.0.113.11", Status: "online", OpenrestyStatus: "healthy", LastSeenAt: &lastSeen}
	if err := db.DB(ctx).Create(&backup).Error; err != nil {
		t.Fatalf("Create(backup) error = %v", err)
	}
	if err := db.DB(ctx).Model(group).Update("backup_node_id", backup.ID).Error; err != nil {
		t.Fatalf("Update(group backup) error = %v", err)
	}

	zoneDomain := model.ZoneDomain{ZoneID: 1, Domain: "second.example.com"}
	if err := db.DB(ctx).Create(&zoneDomain).Error; err != nil {
		t.Fatalf("Create(zoneDomain) error = %v", err)
	}
	secondGroup := model.CFPointingGroup{Name: "secondary", PrimaryNodeID: group.PrimaryNodeID, BackupNodeID: &backup.ID, ActiveNodeID: group.PrimaryNodeID, Enabled: true}
	if err := db.DB(ctx).Create(&secondGroup).Error; err != nil {
		t.Fatalf("Create(secondGroup) error = %v", err)
	}
	secondMember := model.CFPointingMember{GroupID: secondGroup.ID, ZoneDomainID: zoneDomain.ID, SyncStatus: model.CFMemberSyncPending}
	if err := db.DB(ctx).Create(&secondMember).Error; err != nil {
		t.Fatalf("Create(secondMember) error = %v", err)
	}

	dispatchErr := errors.New("dispatch unavailable")
	dispatchCalls := 0
	restoreDispatch := SetDispatchTaskForTest(func(context.Context, string, []byte, string) (string, error) {
		dispatchCalls++
		if dispatchCalls == 1 {
			return "", dispatchErr
		}
		return "task-id", nil
	})
	t.Cleanup(restoreDispatch)

	changed, err := CheckNodeFailover(ctx)
	if !errors.Is(err, dispatchErr) {
		t.Fatalf("CheckNodeFailover() error = %v, want dispatch error", err)
	}
	if changed != 2 {
		t.Errorf("CheckNodeFailover() changed = %d, want 2", changed)
	}
	if dispatchCalls != 2 {
		t.Errorf("CheckNodeFailover() dispatch calls = %d, want 2", dispatchCalls)
	}
	for _, groupID := range []uint{group.ID, secondGroup.ID} {
		updated, err := repository.GetCFPointingGroup(ctx, groupID)
		if err != nil {
			t.Fatalf("GetCFPointingGroup(%d) error = %v", groupID, err)
		}
		if updated.ActiveNodeID != backup.ID {
			t.Errorf("group %d active node = %d, want %d", groupID, updated.ActiveNodeID, backup.ID)
		}
	}
}

func TestBatchEnableProxyContinuesAfterDispatchError(t *testing.T) {
	ctx, firstMemberID := setupCloudflareLogicDB(t)
	firstMember, err := repository.GetCFPointingMemberByID(ctx, firstMemberID)
	if err != nil {
		t.Fatalf("GetCFPointingMemberByID() error = %v", err)
	}
	zoneDomain := model.ZoneDomain{ZoneID: 1, Domain: "second.example.com"}
	if err := db.DB(ctx).Create(&zoneDomain).Error; err != nil {
		t.Fatalf("Create(zoneDomain) error = %v", err)
	}
	secondMember := model.CFPointingMember{GroupID: firstMember.GroupID, ZoneDomainID: zoneDomain.ID}
	if err := db.DB(ctx).Create(&secondMember).Error; err != nil {
		t.Fatalf("Create(secondMember) error = %v", err)
	}

	dispatchErr := errors.New("dispatch unavailable")
	dispatchCalls := 0
	restoreDispatch := SetDispatchTaskForTest(func(context.Context, string, []byte, string) (string, error) {
		dispatchCalls++
		if dispatchCalls == 1 {
			return "", dispatchErr
		}
		return "task-id", nil
	})
	t.Cleanup(restoreDispatch)

	err = BatchEnableProxy(ctx, firstMember.GroupID, MemberBatchProxyInput{MemberIDs: []uint{firstMember.ID, secondMember.ID}})
	if !errors.Is(err, dispatchErr) {
		t.Fatalf("BatchEnableProxy() error = %v, want dispatch error", err)
	}
	if dispatchCalls != 2 {
		t.Errorf("BatchEnableProxy() dispatch calls = %d, want 2", dispatchCalls)
	}
	for _, memberID := range []uint{firstMember.ID, secondMember.ID} {
		updated, err := repository.GetCFPointingMemberByID(ctx, memberID)
		if err != nil {
			t.Fatalf("GetCFPointingMemberByID(%d) error = %v", memberID, err)
		}
		if !updated.Proxied {
			t.Errorf("member %d proxied = false, want true", memberID)
		}
	}
}

func TestBatchEnableProxyContinuesAfterMemberLookupError(t *testing.T) {
	ctx, memberID := setupCloudflareLogicDB(t)
	member, err := repository.GetCFPointingMemberByID(ctx, memberID)
	if err != nil {
		t.Fatalf("GetCFPointingMemberByID() error = %v", err)
	}
	dispatchCalls := 0
	restoreDispatch := SetDispatchTaskForTest(func(context.Context, string, []byte, string) (string, error) {
		dispatchCalls++
		return "task-id", nil
	})
	t.Cleanup(restoreDispatch)

	err = BatchEnableProxy(ctx, member.GroupID, MemberBatchProxyInput{MemberIDs: []uint{99999, member.ID}})
	if err == nil {
		t.Fatal("BatchEnableProxy() error = nil, want member lookup error")
	}
	if dispatchCalls != 1 {
		t.Errorf("BatchEnableProxy() dispatch calls = %d, want 1", dispatchCalls)
	}
	updated, err := repository.GetCFPointingMemberByID(ctx, member.ID)
	if err != nil {
		t.Fatalf("GetCFPointingMemberByID() error = %v", err)
	}
	if !updated.Proxied {
		t.Errorf("member %d proxied = false, want true", member.ID)
	}
}

func TestMoveMemberAndBatchOperations(t *testing.T) {
	ctx, member1ID := setupCloudflareLogicDB(t)

	var dispatchedTasks []string
	restoreDispatch := SetDispatchTaskForTest(func(ctx context.Context, taskType string, payload []byte, triggeredBy string) (string, error) {
		dispatchedTasks = append(dispatchedTasks, triggeredBy)
		return "mock-task-id", nil
	})
	t.Cleanup(restoreDispatch)

	fake := &fakeClient{}
	restoreClient := SetClientFactoryForTest(func(string) Client { return fake })
	t.Cleanup(restoreClient)

	member1, err := repository.GetCFPointingMemberByID(ctx, member1ID)
	if err != nil {
		t.Fatalf("GetCFPointingMemberByID() error = %v", err)
	}
	sourceGroupID := member1.GroupID

	// Create target group
	targetGroup := model.CFPointingGroup{
		Name:           "secondary",
		PrimaryNodeID:  1,
		ActiveNodeID:   1,
		DefaultProxied: true,
		Enabled:        true,
	}
	if err := db.DB(ctx).Create(&targetGroup).Error; err != nil {
		t.Fatalf("Create(targetGroup) error = %v", err)
	}

	// 1. Target group equals source group -> errTargetGroupSame
	t.Run("MoveMember target equals source", func(t *testing.T) {
		_, err := MoveMember(ctx, sourceGroupID, member1.ID, sourceGroupID)
		if err == nil || err.Error() != errTargetGroupSame {
			t.Fatalf("MoveMember() error = %v, want %s", err, errTargetGroupSame)
		}
	})

	// 2. Target group does not exist -> errTargetGroupInvalid
	t.Run("MoveMember target group invalid", func(t *testing.T) {
		_, err := MoveMember(ctx, sourceGroupID, member1.ID, 99999)
		if err == nil || err.Error() != errTargetGroupInvalid {
			t.Fatalf("MoveMember() error = %v, want %s", err, errTargetGroupInvalid)
		}
	})

	// 3. Successfully move member to target group
	t.Run("MoveMember success", func(t *testing.T) {
		item, err := MoveMember(ctx, sourceGroupID, member1.ID, targetGroup.ID)
		if err != nil {
			t.Fatalf("MoveMember() error = %v", err)
		}
		if item == nil || item.GroupID != targetGroup.ID {
			t.Fatalf("MoveMember() returned item group ID = %v, want %d", item, targetGroup.ID)
		}

		updated, err := repository.GetCFPointingMemberByID(ctx, member1.ID)
		if err != nil {
			t.Fatalf("GetCFPointingMemberByID() error = %v", err)
		}
		if updated.GroupID != targetGroup.ID {
			t.Errorf("member GroupID = %d, want %d", updated.GroupID, targetGroup.ID)
		}
		if updated.SyncStatus != model.CFMemberSyncPending {
			t.Errorf("member SyncStatus = %s, want %s", updated.SyncStatus, model.CFMemberSyncPending)
		}
		if updated.LastError != "" {
			t.Errorf("member LastError = %q, want empty", updated.LastError)
		}
	})

	// 4. Batch move multiple members
	t.Run("BatchMoveMembers", func(t *testing.T) {
		// Test empty members error
		emptyErr := BatchMoveMembers(ctx, sourceGroupID, MemberBatchMoveInput{MemberIDs: nil, TargetGroupID: targetGroup.ID})
		if emptyErr == nil || emptyErr.Error() != errNoMembersSelected {
			t.Fatalf("BatchMoveMembers() empty error = %v, want %s", emptyErr, errNoMembersSelected)
		}

		// Test target equals source error
		sameErr := BatchMoveMembers(ctx, sourceGroupID, MemberBatchMoveInput{MemberIDs: []uint{1}, TargetGroupID: sourceGroupID})
		if sameErr == nil || sameErr.Error() != errTargetGroupSame {
			t.Fatalf("BatchMoveMembers() same group error = %v, want %s", sameErr, errTargetGroupSame)
		}

		// Test invalid target group error
		invalidErr := BatchMoveMembers(ctx, sourceGroupID, MemberBatchMoveInput{MemberIDs: []uint{1}, TargetGroupID: 99999})
		if invalidErr == nil || invalidErr.Error() != errTargetGroupInvalid {
			t.Fatalf("BatchMoveMembers() invalid group error = %v, want %s", invalidErr, errTargetGroupInvalid)
		}

		// Create 2 additional members in sourceGroup
		domain2 := model.ZoneDomain{ZoneID: 1, Domain: "test2.example.com"}
		domain3 := model.ZoneDomain{ZoneID: 1, Domain: "test3.example.com"}
		if err := db.DB(ctx).Create(&domain2).Error; err != nil {
			t.Fatalf("Create(domain2) error = %v", err)
		}
		if err := db.DB(ctx).Create(&domain3).Error; err != nil {
			t.Fatalf("Create(domain3) error = %v", err)
		}
		member2 := model.CFPointingMember{GroupID: sourceGroupID, ZoneDomainID: domain2.ID, Proxied: false, SyncStatus: model.CFMemberSyncOK}
		member3 := model.CFPointingMember{GroupID: sourceGroupID, ZoneDomainID: domain3.ID, Proxied: true, SyncStatus: model.CFMemberSyncOK}
		if err := db.DB(ctx).Create(&member2).Error; err != nil {
			t.Fatalf("Create(member2) error = %v", err)
		}
		if err := db.DB(ctx).Create(&member3).Error; err != nil {
			t.Fatalf("Create(member3) error = %v", err)
		}

		// Perform batch move
		if err := BatchMoveMembers(ctx, sourceGroupID, MemberBatchMoveInput{
			MemberIDs:     []uint{member2.ID, member3.ID},
			TargetGroupID: targetGroup.ID,
		}); err != nil {
			t.Fatalf("BatchMoveMembers() error = %v", err)
		}

		// Verify updated in DB
		for _, mid := range []uint{member2.ID, member3.ID} {
			m, err := repository.GetCFPointingMemberByID(ctx, mid)
			if err != nil {
				t.Fatalf("GetCFPointingMemberByID(%d) error = %v", mid, err)
			}
			if m.GroupID != targetGroup.ID {
				t.Errorf("member %d GroupID = %d, want %d", mid, m.GroupID, targetGroup.ID)
			}
			if m.SyncStatus != model.CFMemberSyncPending {
				t.Errorf("member %d SyncStatus = %s, want %s", mid, m.SyncStatus, model.CFMemberSyncPending)
			}
			if m.LastError != "" {
				t.Errorf("member %d LastError = %q, want empty", mid, m.LastError)
			}
		}
	})

	// 5. Batch remove members
	t.Run("BatchRemoveMembers", func(t *testing.T) {
		// Test empty members error
		emptyErr := BatchRemoveMembers(ctx, targetGroup.ID, MemberBatchRemoveInput{MemberIDs: nil})
		if emptyErr == nil || emptyErr.Error() != errNoMembersSelected {
			t.Fatalf("BatchRemoveMembers() empty error = %v, want %s", emptyErr, errNoMembersSelected)
		}

		// Create members to remove
		domain4 := model.ZoneDomain{ZoneID: 1, Domain: "test4.example.com"}
		domain5 := model.ZoneDomain{ZoneID: 1, Domain: "test5.example.com"}
		if err := db.DB(ctx).Create(&domain4).Error; err != nil {
			t.Fatalf("Create(domain4) error = %v", err)
		}
		if err := db.DB(ctx).Create(&domain5).Error; err != nil {
			t.Fatalf("Create(domain5) error = %v", err)
		}
		member4 := model.CFPointingMember{GroupID: targetGroup.ID, ZoneDomainID: domain4.ID, Proxied: false, SyncStatus: model.CFMemberSyncOK}
		member5 := model.CFPointingMember{GroupID: targetGroup.ID, ZoneDomainID: domain5.ID, Proxied: true, SyncStatus: model.CFMemberSyncOK}
		if err := db.DB(ctx).Create(&member4).Error; err != nil {
			t.Fatalf("Create(member4) error = %v", err)
		}
		if err := db.DB(ctx).Create(&member5).Error; err != nil {
			t.Fatalf("Create(member5) error = %v", err)
		}

		// Perform batch remove
		if err := BatchRemoveMembers(ctx, targetGroup.ID, MemberBatchRemoveInput{
			MemberIDs: []uint{member4.ID, member5.ID},
		}); err != nil {
			t.Fatalf("BatchRemoveMembers() error = %v", err)
		}

		// Verify deleted from DB
		for _, mid := range []uint{member4.ID, member5.ID} {
			_, err := repository.GetCFPointingMemberByID(ctx, mid)
			if err == nil {
				t.Errorf("GetCFPointingMemberByID(%d) should be deleted, but found", mid)
			}
		}
	})
}
