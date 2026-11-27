// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package smartdns

import (
	"fmt"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/qualityapi"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func f64Ptr(v float64) *float64 { return &v }

func TestSelectNodesSmartScoringOverloadHysteresisAndFallback(t *testing.T) {
	hkLat, hkLon := 22.3193, 114.1694
	sgLat, sgLon := 1.3521, 103.8198
	clientLat, clientLon := 23.1291, 113.2644

	nodes := []protocol.DNSSnapshotNode{
		{
			NodeID: "node-hk-opt", Name: "HK Optimized", IPv4: "203.0.113.1", Status: "online",
			GeoLatitude: &hkLat, GeoLongitude: &hkLon, CapacityWeight: 100,
			CPUUsagePercent: 20, MemoryUsagePercent: 30,
			Tags: []protocol.DNSSnapshotTag{{Slug: "optimized", EffectiveWeight: 1.3}},
		},
		{
			NodeID: "node-sg-highload", Name: "SG Overloaded", IPv4: "203.0.113.2", Status: "online",
			GeoLatitude: &sgLat, GeoLongitude: &sgLon, CapacityWeight: 100,
			CPUUsagePercent: 98, MemoryUsagePercent: 95,
			CapacityBandwidthMbps: 1000, BandwidthMbps: 950,
		},
		{
			NodeID: "node-sg-ok", Name: "SG Normal", IPv4: "203.0.113.3", Status: "online",
			GeoLatitude: &sgLat, GeoLongitude: &sgLon, CapacityWeight: 100,
			CPUUsagePercent: 25, MemoryUsagePercent: 35,
			Tags: []protocol.DNSSnapshotTag{{Slug: "international", EffectiveWeight: 1.0}},
		},
		{
			NodeID: "node-quar", Name: "Quarantined", IPv4: "203.0.113.4", Status: "quarantined", Quarantined: true,
		},
	}

	qual := map[string]qualityapi.NodeLinkQuality{
		"node-hk-opt":      {NodeID: "node-hk-opt", EffectiveRTTMs: 18, LossRate: 0.0, SourceTier: qualityapi.SourceTierMeasuredRegion},
		"node-sg-highload": {NodeID: "node-sg-highload", EffectiveRTTMs: 12, LossRate: 0.0, SourceTier: qualityapi.SourceTierMeasuredRegion},
		"node-sg-ok":       {NodeID: "node-sg-ok", EffectiveRTTMs: 45, LossRate: 0.01, SourceTier: qualityapi.SourceTierMeasuredCountry},
	}
	client := qualityapi.ClientProfile{
		CountryISO: "CN", RegionCode: "CN-GD", IPFamily: "ipv4",
		Latitude: &clientLat, Longitude: &clientLon,
	}
	policy := protocol.DNSSnapshotPolicy{
		ID: 1, Name: "Smart", StrategyType: model.DNSPolicyStrategySmart, TTL: 60, MaxAnswers: 1,
		FallbackStrat: model.DNSPolicyFallbackLoadBalance,
	}

	hyst := NewHysteresisTracker(128)
	now := time.Now().UTC()

	res1 := SelectNodes(SelectInput{
		Domain: "app.example.com", QType: "A", ClientKey: "113.108.10.0/24",
		Client: client, Nodes: nodes, Policy: policy, Quality: qual, Hysteresis: hyst, Now: now,
	})
	require.Len(t, res1.Answers, 1)
	assert.Equal(t, "node-hk-opt", res1.Answers[0].NodeID)
	assert.False(t, res1.FallbackUsed)

	// Verify overloaded node (>0.92) was excluded from primary pool.
	for _, c := range res1.Candidates {
		if c.NodeID == "node-sg-highload" {
			assert.True(t, c.Overloaded)
			assert.Equal(t, "load_above_0.92", c.ExclusionReason)
		}
		if c.NodeID == "node-quar" {
			assert.Equal(t, "quarantined", c.ExclusionReason)
		}
	}

	// Small (<5%) lead by node-sg-ok must stay sticky on node-hk-opt via 5% hysteresis.
	qualClose := map[string]qualityapi.NodeLinkQuality{
		"node-hk-opt": {NodeID: "node-hk-opt", EffectiveRTTMs: 42, LossRate: 0, SourceTier: qualityapi.SourceTierMeasuredRegion},
		"node-sg-ok":  {NodeID: "node-sg-ok", EffectiveRTTMs: 10, LossRate: 0, SourceTier: qualityapi.SourceTierMeasuredRegion},
	}
	nodesClose := []protocol.DNSSnapshotNode{
		{NodeID: "node-hk-opt", Name: "HK", IPv4: "203.0.113.1", Status: "online", CapacityWeight: 100, CPUUsagePercent: 20, MemoryUsagePercent: 20},
		{NodeID: "node-sg-ok", Name: "SG", IPv4: "203.0.113.3", Status: "online", CapacityWeight: 100, CPUUsagePercent: 20, MemoryUsagePercent: 20},
	}
	resStick := SelectNodes(SelectInput{
		Domain: "app.example.com", QType: "A", ClientKey: "198.51.100.0/24",
		Client: qualityapi.ClientProfile{CountryISO: "CN", IPFamily: "ipv4"},
		Nodes:  nodesClose, Policy: policy, Quality: map[string]qualityapi.NodeLinkQuality{
			"node-hk-opt": {NodeID: "node-hk-opt", EffectiveRTTMs: 25, LossRate: 0},
			"node-sg-ok":  {NodeID: "node-sg-ok", EffectiveRTTMs: 28, LossRate: 0},
		},
		Hysteresis: hyst, Now: now.Add(time.Second),
	})
	require.Equal(t, "node-hk-opt", resStick.Answers[0].NodeID)

	resSmallSwing := SelectNodes(SelectInput{
		Domain: "app.example.com", QType: "A", ClientKey: "198.51.100.0/24",
		Client: qualityapi.ClientProfile{CountryISO: "CN", IPFamily: "ipv4"},
		Nodes:  nodesClose, Policy: policy, Quality: map[string]qualityapi.NodeLinkQuality{
			"node-hk-opt": {NodeID: "node-hk-opt", EffectiveRTTMs: 26.5, LossRate: 0},
			"node-sg-ok":  {NodeID: "node-sg-ok", EffectiveRTTMs: 25.0, LossRate: 0},
		},
		Hysteresis: hyst, Now: now.Add(2 * time.Second),
	})
	require.Len(t, resSmallSwing.Answers, 1)
	assert.Equal(t, "node-hk-opt", resSmallSwing.Answers[0].NodeID)
	assert.True(t, resSmallSwing.HysteresisApplied)

	// Mandatory fallback when filter excludes every node.
	jdmStrictFilter, err := CompilePresetJDM(model.DNSPolicyStrategySmart, PolicyConfig{
		Mode:    "smart",
		Filters: PolicyFilters{Tags: []string{"nonexistent-tag"}},
	})
	require.NoError(t, err)
	policyStrict := policy
	policyStrict.JDMJSON = jdmStrictFilter
	policyStrict.MaxAnswers = 2
	resFallback := SelectNodes(SelectInput{
		Domain: "app.example.com", QType: "A", ClientKey: "198.51.100.0/24",
		Client: client, Nodes: nodes, Policy: policyStrict, Quality: qualClose, Now: now,
	})
	assert.True(t, resFallback.FallbackUsed)
	assert.NotEmpty(t, resFallback.Answers)
}

func TestSelectNodesLoadBalanceMultiAnswer(t *testing.T) {
	nodes := []protocol.DNSSnapshotNode{
		{NodeID: "n1", Name: "N1", IPv4: "203.0.113.10", Status: "online", CapacityWeight: 200, CPUUsagePercent: 10, MemoryUsagePercent: 10},
		{NodeID: "n2", Name: "N2", IPv4: "203.0.113.11", Status: "online", CapacityWeight: 100, CPUUsagePercent: 10, MemoryUsagePercent: 10},
		{NodeID: "n3", Name: "N3", IPv4: "203.0.113.12", Status: "online", CapacityWeight: 50, CPUUsagePercent: 10, MemoryUsagePercent: 10},
	}
	policy := protocol.DNSSnapshotPolicy{
		ID: 2, Name: "LB", StrategyType: model.DNSPolicyStrategyLoadBalance, TTL: 60, MaxAnswers: 2,
	}
	res := SelectNodes(SelectInput{
		Domain: "lb.example.com", QType: "A", ClientKey: "203.0.113.0/24",
		Nodes: nodes, Policy: policy,
	})
	require.Len(t, res.Answers, 2)
	assert.NotEqual(t, res.Answers[0].IP, res.Answers[1].IP)
	assert.Equal(t, "load_balance", res.Mode)
}

func TestSelectNodesDynamicP95TailPenaltyAndECSExploration(t *testing.T) {
	nodes := []protocol.DNSSnapshotNode{
		{NodeID: "hk-a", Name: "HK-A", IPv4: "203.0.113.1", Status: "online", CapacityWeight: 100, CPUUsagePercent: 20, MemoryUsagePercent: 20},
		{NodeID: "jp-b", Name: "JP-B", IPv4: "203.0.113.2", Status: "online", CapacityWeight: 100, CPUUsagePercent: 20, MemoryUsagePercent: 20},
		{NodeID: "us-d", Name: "US-D", IPv4: "203.0.113.4", Status: "online", CapacityWeight: 100, CPUUsagePercent: 20, MemoryUsagePercent: 20},
	}
	policy := protocol.DNSSnapshotPolicy{
		ID: 3, Name: "Smart", StrategyType: model.DNSPolicyStrategySmart, TTL: 60, MaxAnswers: 1,
	}
	now := time.Unix(1_700_000_000, 0).UTC()

	// 1. P95 tail penalty flips winner only when P95Weight > 0.
	qualLowWeight := map[string]qualityapi.NodeLinkQuality{
		"hk-a": {NodeID: "hk-a", EffectiveRTTMs: 30, RTTP95Ms: 220, P95Weight: 0.0, SourceTier: qualityapi.SourceTierMeasuredRegion},
		"jp-b": {NodeID: "jp-b", EffectiveRTTMs: 65, RTTP95Ms: 80, P95Weight: 0.35, SourceTier: qualityapi.SourceTierMeasuredRegion},
		"us-d": {NodeID: "us-d", EffectiveRTTMs: 220, RTTP95Ms: 250, P95Weight: 0.35, SourceTier: qualityapi.SourceTierMeasuredRegion},
	}
	resNoTail := SelectNodes(SelectInput{
		Domain: "app.example.com", QType: "A", ClientKey: "113.108.1.0/24",
		Client: qualityapi.ClientProfile{CountryISO: "CN", IPFamily: "ipv4"}, Nodes: nodes, Policy: policy, Quality: qualLowWeight, Now: now,
	})
	require.Equal(t, "hk-a", resNoTail.Answers[0].NodeID)

	qualWithTail := map[string]qualityapi.NodeLinkQuality{
		"hk-a": {NodeID: "hk-a", EffectiveRTTMs: 30, RTTP95Ms: 220, P95Weight: 0.35, SourceTier: qualityapi.SourceTierMeasuredRegion},
		"jp-b": {NodeID: "jp-b", EffectiveRTTMs: 65, RTTP95Ms: 80, P95Weight: 0.35, SourceTier: qualityapi.SourceTierMeasuredRegion},
		"us-d": {NodeID: "us-d", EffectiveRTTMs: 220, RTTP95Ms: 250, P95Weight: 0.35, SourceTier: qualityapi.SourceTierMeasuredRegion},
	}
	resWithTail := SelectNodes(SelectInput{
		Domain: "app.example.com", QType: "A", ClientKey: "113.108.1.0/24",
		Client: qualityapi.ClientProfile{CountryISO: "CN", IPFamily: "ipv4"}, Nodes: nodes, Policy: policy, Quality: qualWithTail, Now: now,
	})
	require.Equal(t, "jp-b", resWithTail.Answers[0].NodeID)

	// 2. Exploration MUST stay disabled when FromECS == false.
	qualExplore := map[string]qualityapi.NodeLinkQuality{
		"hk-a": {NodeID: "hk-a", EffectiveRTTMs: 180, PredictedRTTMs: 30, Confidence: 0.2, QualityState: qualityapi.QualityStateStale, ExplorePriority: 0.9, SourceTier: qualityapi.SourceTierMeasuredRegion},
		"jp-b": {NodeID: "jp-b", EffectiveRTTMs: 65, PredictedRTTMs: 65, Confidence: 0.9, QualityState: qualityapi.QualityStateHealthy, ExplorePriority: 0.0, SourceTier: qualityapi.SourceTierMeasuredRegion},
		"us-d": {NodeID: "us-d", EffectiveRTTMs: 220, PredictedRTTMs: 210, Confidence: 0.8, QualityState: qualityapi.QualityStateHealthy, ExplorePriority: 0.0, SourceTier: qualityapi.SourceTierMeasuredRegion},
	}
	for i := range 500 {
		prefix := fmt.Sprintf("113.%d.%d.0/24", (i/250)+1, i%250)
		res := SelectNodes(SelectInput{
			Domain: "app.example.com", QType: "A", ClientKey: prefix,
			Client: qualityapi.ClientProfile{CountryISO: "CN", IPFamily: "ipv4", FromECS: false},
			Nodes:  nodes, Policy: policy, Quality: qualExplore, Now: now,
		})
		require.False(t, res.ExplorationUsed)
		require.Equal(t, "jp-b", res.Answers[0].NodeID)
	}

	// 3. With FromECS == true, deterministic exploration selects hk-a for a small budget (~0.5%-3%),
	// clamps TTL <= 25s, never chooses zero-priority us-d, and never writes hysteresis.
	hyst := NewHysteresisTracker(1024)
	exploredCount := 0
	var exploredPrefix string
	for i := range 500 {
		prefix := fmt.Sprintf("113.%d.%d.0/24", (i/250)+1, i%250)
		res := SelectNodes(SelectInput{
			Domain: "app.example.com", QType: "A", ClientKey: prefix,
			Client: qualityapi.ClientProfile{CountryISO: "CN", IPFamily: "ipv4", FromECS: true},
			Nodes:  nodes, Policy: policy, Quality: qualExplore, Hysteresis: hyst, Now: now,
		})
		if res.ExplorationUsed {
			exploredCount++
			exploredPrefix = prefix
			require.Equal(t, "hk-a", res.ExploredNodeID)
			require.Equal(t, "hk-a", res.Answers[0].NodeID)
			require.LessOrEqual(t, res.TTL, explorationMaxTTL)
		} else {
			require.Equal(t, "jp-b", res.Answers[0].NodeID)
		}
	}
	require.Greater(t, exploredCount, 0)
	require.Less(t, exploredCount, 30)
	require.Empty(t, hyst.get(hysteresisKey(policy.ID, exploredPrefix, "A"), now))
}
