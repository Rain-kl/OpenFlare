// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package smartdns

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/qualityapi"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type mockQualityProvider struct{}

func (mockQualityProvider) Name() string       { return "mock_oracle" }
func (mockQualityProvider) Generation() uint64 { return 7 }
func (m mockQualityProvider) EvaluateNode(_ context.Context, nodeID string, client qualityapi.ClientProfile) qualityapi.NodeLinkQuality {
	if nodeID == "edge-fast" {
		return qualityapi.NodeLinkQuality{
			NodeID: nodeID, EffectiveRTTMs: 14, RTTP95Ms: 20, Confidence: 0.95, QualityScore: 93,
			SourceTier: qualityapi.SourceTierMeasuredRegion,
		}
	}
	return qualityapi.NodeLinkQuality{
		NodeID: nodeID, EffectiveRTTMs: 95, RTTP95Ms: 120, LossRate: 0.02, Confidence: 0.8, QualityScore: 62,
		SourceTier: qualityapi.SourceTierMeasuredCountry,
	}
}
func (m mockQualityProvider) EvaluateNodes(ctx context.Context, nodeIDs []string, client qualityapi.ClientProfile) map[string]qualityapi.NodeLinkQuality {
	out := make(map[string]qualityapi.NodeLinkQuality, len(nodeIDs))
	for _, id := range nodeIDs {
		out[id] = m.EvaluateNode(ctx, id, client)
	}
	return out
}
func (mockQualityProvider) ExportSnapshot(_ context.Context, _ []string) (*qualityapi.CompactQualitySnapshot, error) {
	return &qualityapi.CompactQualitySnapshot{
		SchemaVersion: qualityapi.CompactSnapshotSchemaVersion,
		Generation:    7,
		GeneratedAt:   time.Now().UTC(),
		ProviderName:  "mock_oracle",
		Entries: []qualityapi.CompactQualityEntry{
			{NodeID: "edge-fast", ClientASN: 4134, ClientCountry: "CN", IPFamily: "ipv4", Level: "country", EffectiveRTTMs: 14, RTTP95Ms: 20, Confidence: 0.95, SourceTier: qualityapi.SourceTierMeasuredCountry},
		},
		NodeDefaults: map[string]qualityapi.CompactNodePrior{},
	}, nil
}

func setupSmartDNSTestDB(t *testing.T) func() {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(
		&model.SystemConfig{},
		&model.OpenFlareNode{},
		&model.OpenFlareNodeTag{},
		&model.OpenFlareNodeTagBinding{},
		&model.OpenFlareMetricSnapshot{},
		&model.OpenFlareDNSInstance{},
		&model.OpenFlareDNSPolicy{},
		&model.CFPointingGroup{},
		&model.CFPointingMember{},
		&model.ZoneDomain{},
	))
	db.SetDB(gdb)
	testhelper.SetupLogStoresForTest(t)
	qualityapi.RegisterDefault(mockQualityProvider{})
	defaultSnapshotManager = NewSnapshotManager()

	return func() {
		qualityapi.RegisterDefault(nil)
		defaultSnapshotManager = NewSnapshotManager()
		db.SetDB(nil)
	}
}

func TestSimulatorSettingsInstancesAndOverviewLifecycle(t *testing.T) {
	cleanup := setupSmartDNSTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Seed two nodes + default policy.
	now := time.Now().UTC()
	require.NoError(t, repository.CreateOpenFlareNode(ctx, &model.OpenFlareNode{
		NodeID: "edge-fast", Name: "HK Fast", IP: "203.0.113.10", Status: "online", NodeType: "edge_node", LastSeenAt: &now, CapacityWeight: 120,
	}))
	require.NoError(t, repository.CreateOpenFlareNode(ctx, &model.OpenFlareNode{
		NodeID: "edge-slow", Name: "US Slow", IP: "198.51.100.20", Status: "online", NodeType: "edge_node", LastSeenAt: &now, CapacityWeight: 100,
	}))

	del := "edge.example.com"
	savedSettings, err := SaveSettings(ctx, SettingsInput{
		DelegationDomain: &del,
		NSHosts:          []string{"ns1.edge.example.com", "ns2.edge.example.com"},
	})
	require.NoError(t, err)
	assert.Equal(t, "edge.example.com", savedSettings.DelegationDomain)
	assert.NotEmpty(t, savedSettings.DiscoveryToken)

	// Delegation lookup verification using injected resolver mock.
	origLookup := lookupNSRecords
	lookupNSRecords = func(_ context.Context, domain string) ([]string, error) {
		assert.Equal(t, "edge.example.com", domain)
		return []string{"ns1.edge.example.com", "ns2.edge.example.com"}, nil
	}
	t.Cleanup(func() { lookupNSRecords = origLookup })

	verifyRes, err := VerifyDelegation(ctx, "")
	require.NoError(t, err)
	assert.True(t, verifyRes.Verified)
	assert.Empty(t, verifyRes.MissingNSHosts)

	// Policy creation + default toggle + simulation.
	p, err := CreatePolicy(ctx, PolicyInput{
		Name:             "Smart Default",
		StrategyType:     model.DNSPolicyStrategySmart,
		ScopeType:        model.DNSPolicyScopeAll,
		FallbackStrategy: model.DNSPolicyFallbackLoadBalance,
		TTL:              60,
		MaxAnswers:       1,
		IsDefault:        boolPtr(true),
		Config:           defaultPolicyConfig(model.DNSPolicyStrategySmart),
	})
	require.NoError(t, err)
	assert.True(t, p.IsDefault)

	// Force snapshot rebuild and run simulator.
	_, err = DefaultSnapshotManager().Build(ctx)
	require.NoError(t, err)

	sim, err := SimulateResolution(ctx, SimulateInput{
		Domain:   "web.example.com",
		QType:    "A",
		ClientIP: "113.108.10.5",
	})
	require.NoError(t, err)
	require.Len(t, sim.Selection.Answers, 1)
	assert.Equal(t, "edge-fast", sim.Selection.Answers[0].NodeID)
	assert.Equal(t, "113.108.10.0/24", sim.Client.ClientPrefix)

	// Standalone instance CRUD + heartbeat + overview metrics.
	inst, err := CreateInstance(ctx, InstanceInput{
		Name:       "HK Standalone DNS",
		PublicAddr: "203.0.113.53",
		ListenAddr: ":53",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, inst.AccessToken)
	assert.Contains(t, inst.InstallCommand, inst.InstanceID)

	rawInst, err := repository.GetDNSInstanceByID(ctx, inst.ID)
	require.NoError(t, err)
	hbResp, err := RecordDNSHeartbeat(ctx, rawInst, protocol.DNSHeartbeatRequest{
		Version:         "0.2.0",
		SnapshotVersion: 1,
		QPS:             1500,
		P50Ms:           0.3,
		P95Ms:           0.9,
		P99Ms:           1.8,
		QueriesTotal:    15000,
		QueriesSuccess:  14980,
		CacheHitRatio:   0.94,
	})
	require.NoError(t, err)
	assert.True(t, hbResp.OK)

	overview, err := GetOverview(ctx, "", "1h")
	require.NoError(t, err)
	assert.Equal(t, 1, overview.OnlineInstances)
	assert.InDelta(t, 1500.0, overview.TotalQPS, 0.1)
	assert.Equal(t, 1, overview.ActivePolicies)

	// Verify metric row landed in test logstore.
	var metricsCount int64
	require.NoError(t, db.DB(ctx).Model(&analyticsmodel.DNSMetricSnapshot{}).Count(&metricsCount).Error)
	assert.EqualValues(t, 1, metricsCount)

	// Quarantine & rotate token & delete.
	qInst, err := QuarantineInstance(ctx, inst.ID, "maintenance")
	require.NoError(t, err)
	assert.Equal(t, model.DNSInstanceStatusQuarantined, qInst.Status)
	uInst, err := UnquarantineInstance(ctx, inst.ID)
	require.NoError(t, err)
	assert.Equal(t, model.DNSInstanceStatusOffline, uInst.Status)

	require.NoError(t, DeleteInstance(ctx, inst.ID))
}

func TestInstanceRemoteUpdateBatchAndHeartbeatReceipts(t *testing.T) {
	t.Cleanup(setupSmartDNSTestDB(t))
	ctx := t.Context()
	first, err := CreateInstance(ctx, InstanceInput{Name: "first"})
	require.NoError(t, err)
	second, err := CreateInstance(ctx, InstanceInput{Name: "second"})
	require.NoError(t, err)
	heartbeat := func(id uint, req protocol.DNSHeartbeatRequest) *protocol.DNSHeartbeatResponse {
		row, loadErr := repository.GetDNSInstanceByID(ctx, id)
		require.NoError(t, loadErr)
		resp, reportErr := RecordDNSHeartbeat(ctx, row, req)
		require.NoError(t, reportErr)
		return resp
	}
	capable := protocol.DNSHeartbeatRequest{RemoteUpdateSupported: true}
	heartbeat(first.ID, capable)
	input := InstanceUpdateInput{InstanceIDs: []uint{first.ID, second.ID}, Channel: "canary"}
	_, err = RequestInstanceUpdates(ctx, input)
	var conflict *logicError
	require.ErrorAs(t, err, &conflict)
	assert.Equal(t, http.StatusConflict, conflict.status)
	assert.Nil(t, heartbeat(first.ID, capable).Update, "rejected batch must leave both nodes unchanged")

	heartbeat(second.ID, capable)
	result, err := RequestInstanceUpdates(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Queued)
	firstCommand := heartbeat(first.ID, capable).Update
	require.NotNil(t, firstCommand)
	assert.Equal(t, "canary", firstCommand.Channel)
	require.NotNil(t, heartbeat(second.ID, capable).Update)
	_, err = RequestInstanceUpdates(ctx, input)
	require.ErrorAs(t, err, &conflict)

	capable.UpdateReceipt = &protocol.DNSUpdateReceipt{ID: firstCommand.ID, Status: model.DNSUpdateRunning}
	assert.Nil(t, heartbeat(first.ID, capable).Update)
	capable.UpdateReceipt.Status = model.DNSUpdateSucceeded
	heartbeat(first.ID, capable)
	capable.UpdateReceipt.Status = model.DNSUpdateFailed
	heartbeat(first.ID, capable)
	instance, err := repository.GetDNSInstanceByID(ctx, first.ID)
	require.NoError(t, err)
	assert.Equal(t, model.DNSUpdateSucceeded, instance.UpdateStatus, "a terminal receipt is immutable")
}
