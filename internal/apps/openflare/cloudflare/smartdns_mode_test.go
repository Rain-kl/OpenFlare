// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cloudflare

import (
	"context"
	"testing"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/listener"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSmartDNSTargetModeForcedNonProxiedCNAMEAndReconcile(t *testing.T) {
	ctx, existingMemberID := setupCloudflareLogicDB(t)
	require.NoError(t, db.DB(ctx).AutoMigrate(&model.SystemConfig{}, &model.OpenFlareDNSPolicy{}))

	dirtyEvents := 0
	listener.OnSmartDNSSnapshotDirty(func(_ context.Context, _ listener.SmartDNSSnapshotDirty) {
		dirtyEvents++
	})

	restoreDispatch := SetDispatchTaskForTest(func(context.Context, string, []byte, string) (string, error) {
		return "task-ok", nil
	})
	t.Cleanup(restoreDispatch)

	policy := &model.OpenFlareDNSPolicy{
		Name:             "Smart Route",
		ScopeType:        model.DNSPolicyScopeAll,
		StrategyType:     model.DNSPolicyStrategySmart,
		Priority:         100,
		TTL:              60,
		MaxAnswers:       1,
		Enabled:          true,
		IsDefault:        true,
		FallbackStrategy: model.DNSPolicyFallbackSmart,
		ConfigJSON:       "{}",
		JDMJSON:          `{"nodes":[{"id":"in","name":"In","type":"inputNode"},{"id":"out","name":"Out","type":"outputNode"}],"edges":[{"id":"e1","sourceId":"in","targetId":"out"}]}`,
	}
	require.NoError(t, repository.CreateDNSPolicy(ctx, policy))

	// Fails without delegation domain.
	_, err := CreateGroup(ctx, GroupInput{
		Name:             "Web Group",
		TargetMode:       model.CFPointingTargetModeSmartDNS,
		SmartDNSPolicyID: &policy.ID,
		DefaultProxied:   true,
		Enabled:          true,
	})
	require.Error(t, err)
	assert.Equal(t, errSmartDNSDelegationUnset, err.Error())

	require.NoError(t, repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeySmartDNSDelegationDomain, "edge.example.com"))

	// Rejects target host outside delegation domain.
	_, err = CreateGroup(ctx, GroupInput{
		Name:               "Bad Host",
		TargetMode:         model.CFPointingTargetModeSmartDNS,
		SmartDNSPolicyID:   &policy.ID,
		SmartDNSTargetHost: "outside.other.com",
		Enabled:            true,
	})
	require.Error(t, err)
	assert.Equal(t, errSmartDNSHostInvalid, err.Error())

	// Valid creation forces CNAME + DefaultProxied=false even when DefaultProxied=true is supplied.
	created, err := CreateGroup(ctx, GroupInput{
		Name:               "Global Edge",
		TargetMode:         model.CFPointingTargetModeSmartDNS,
		SmartDNSPolicyID:   &policy.ID,
		SmartDNSTargetHost: "app.edge.example.com",
		DefaultProxied:     true,
		Enabled:            true,
	})
	require.NoError(t, err)
	assert.Equal(t, model.CFPointingTargetModeSmartDNS, created.TargetMode)
	assert.Equal(t, "CNAME", created.RecordType)
	assert.Equal(t, "app.edge.example.com", created.RecordContent)
	assert.Equal(t, "app.edge.example.com", created.SmartDNSTargetHost)
	assert.False(t, created.DefaultProxied)
	assert.GreaterOrEqual(t, dirtyEvents, 1)

	domain := model.ZoneDomain{ZoneID: 1, Domain: "www.example.com"}
	require.NoError(t, db.DB(ctx).Create(&domain).Error)

	// CreateMember forces proxied=false even if input asks for proxied=true.
	proxiedReq := true
	mem, err := CreateMember(ctx, created.ID, MemberCreateInput{
		ZoneDomainID: domain.ID,
		Proxied:      &proxiedReq,
	})
	require.NoError(t, err)
	assert.False(t, mem.Proxied)

	// Enabling proxy via UpdateMember or BatchEnableProxy is rejected.
	_, err = UpdateMember(ctx, created.ID, mem.ID, MemberUpdateInput{Proxied: true})
	require.Error(t, err)
	assert.Equal(t, errSmartDNSProxyUnsupported, err.Error())

	err = BatchEnableProxy(ctx, created.ID, MemberBatchProxyInput{MemberIDs: []uint{mem.ID}})
	require.Error(t, err)
	assert.Equal(t, errSmartDNSProxyUnsupported, err.Error())

	// Reconcile creates non-proxied CNAME pointing to SmartDNSTargetHost.
	fake := &fakeClient{}
	restoreClient := SetClientFactoryForTest(func(string) Client { return fake })
	t.Cleanup(restoreClient)

	require.NoError(t, ReconcileMember(ctx, mem.ID))
	require.NotNil(t, fake.created)
	assert.Equal(t, "CNAME", fake.created.Type)
	assert.Equal(t, "app.edge.example.com", fake.created.Content)
	assert.False(t, fake.created.Proxied)

	// Moving a proxied node-group member into a SmartDNS group clears proxied=false.
	existingMem, err := repository.GetCFPointingMemberByID(ctx, existingMemberID)
	require.NoError(t, err)
	moved, err := MoveMember(ctx, existingMem.GroupID, existingMem.ID, created.ID)
	require.NoError(t, err)
	assert.False(t, moved.Proxied)

	// DeleteManagedRecord deletes cached CFRecordID first ("record-created").
	require.NoError(t, DeleteManagedRecord(ctx, mem.ID))
	assert.Equal(t, []string{"record-created"}, fake.deleted)
}
