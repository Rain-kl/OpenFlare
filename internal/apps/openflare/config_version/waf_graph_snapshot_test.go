// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/waf"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSnapshotRejectsOversizedAggregateWAFIPGroups(t *testing.T) {
	cleanup := setupConfigVersionTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Each group remains below the existing 2 MiB per-subscription ceiling,
	// while the complete Agent runtime document exceeds the aggregate limit.
	ipList, err := json.Marshal(strings.Fields(strings.Repeat("192.0.2.1 ", 165000)))
	require.NoError(t, err)
	require.Less(t, len(ipList), 2<<20)

	groupIDs := make([]uint, 0, 12)
	for index := 0; index < 12; index++ {
		group := &model.OpenFlareWAFIPGroup{
			Name:    "aggregate-" + strings.Repeat("x", index),
			Type:    "manual",
			Enabled: true,
			IPList:  string(ipList),
		}
		require.NoError(t, db.DB(ctx).Create(group).Error)
		groupIDs = append(groupIDs, group.ID)
	}
	createSnapshotRule(t, ctx, "oversized-aggregate", snapshotIPMatchGraphForGroups(groupIDs))

	_, err = buildSnapshotWAFDocument(ctx, nil)
	require.ErrorContains(t, err, "WAF IP 组快照大小")
	require.ErrorContains(t, err, "超过上限")
}

func TestWAFGraphSnapshotPreservesOrderAndGraphReferences(t *testing.T) {
	cleanup := setupConfigVersionTestDB(t)
	defer cleanup()
	ctx := context.Background()

	route := &model.ProxyRoute{SiteName: "ordered.example.com", OriginURL: "http://origin:8080", Upstreams: `["http://origin:8080"]`, Enabled: true}
	require.NoError(t, model.CreateProxyRouteRecord(ctx, route))
	createSnapshotZoneDomains(t, ctx, route, route.SiteName)

	referenced := &model.OpenFlareWAFIPGroup{Name: "referenced", Type: "manual", Enabled: true, IPList: `["192.0.2.1"]`}
	unused := &model.OpenFlareWAFIPGroup{Name: "unused", Type: "manual", Enabled: true, IPList: `["198.51.100.1"]`}
	require.NoError(t, db.DB(ctx).Create(referenced).Error)
	require.NoError(t, db.DB(ctx).Create(unused).Error)

	customA := createSnapshotRule(t, ctx, "custom-a", waf.DefaultRuleGraph())
	customB := createSnapshotRule(t, ctx, "custom-b", snapshotIPMatchGraph(referenced.ID))
	require.NoError(t, model.ReplaceOpenFlareWAFSiteRuleGroupBindings(ctx, route.ID, []uint{customB.ID, customA.ID}))

	snapshot, err := buildSnapshotWAFDocument(ctx, []*model.ProxyRoute{route})
	require.NoError(t, err)
	require.Len(t, snapshot.Bindings, 1)
	assert.Equal(t, []uint{customB.ID, customA.ID}, snapshot.Bindings[0].RuleGroupIDs)
	require.Len(t, snapshot.IPGroups, 1)
	assert.Equal(t, referenced.ID, snapshot.IPGroups[0].ID)

	var customBSnapshot *snapshotWAFRuleGroup
	for index := range snapshot.RuleGroups {
		if snapshot.RuleGroups[index].ID == customB.ID {
			customBSnapshot = &snapshot.RuleGroups[index]
		}
	}
	require.NotNil(t, customBSnapshot)
	assert.Equal(t, "start", customBSnapshot.Graph.Entry)
	assert.Equal(t, waf.RuleNodeIPMatch, customBSnapshot.Graph.Nodes["match"].Type)
	raw, err := json.Marshal(customBSnapshot)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "position")
	assert.NotContains(t, string(raw), "ip_whitelist")
}

func TestWAFGraphSnapshotEncodesEmptyBindingsAsArrays(t *testing.T) {
	cleanup := setupConfigVersionTestDB(t)
	defer cleanup()
	ctx := context.Background()

	route := &model.ProxyRoute{SiteName: "empty-binding.example.com", OriginURL: "http://origin:8080", Upstreams: `["http://origin:8080"]`, Enabled: true}
	require.NoError(t, model.CreateProxyRouteRecord(ctx, route))
	createSnapshotZoneDomains(t, ctx, route, route.SiteName)

	snapshot, err := buildSnapshotWAFDocument(ctx, []*model.ProxyRoute{route})
	require.NoError(t, err)
	require.Len(t, snapshot.Bindings, 1)
	require.NotNil(t, snapshot.Bindings[0].RuleGroupIDs)

	raw, err := json.Marshal(snapshot)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"rule_group_ids":[]`)
	assert.NotContains(t, string(raw), `"rule_group_ids":null`)
}

func TestBuildSnapshotRejectsInvalidWAFGraph(t *testing.T) {
	cleanup := setupConfigVersionTestDB(t)
	defer cleanup()
	ctx := context.Background()

	invalid := &model.OpenFlareWAFRuleGroup{Name: "invalid", Enabled: true, Graph: `{"schema_version":1,"nodes":[],"edges":[]}`, Revision: 1}
	require.NoError(t, db.DB(ctx).Create(invalid).Error)
	_, err := buildSnapshotWAFDocument(ctx, nil)
	require.ErrorContains(t, err, "invalid")
}

func createSnapshotRule(t *testing.T, ctx context.Context, name string, graph waf.RuleGraph) *model.OpenFlareWAFRuleGroup {
	t.Helper()
	raw, err := json.Marshal(graph)
	require.NoError(t, err)
	rule := &model.OpenFlareWAFRuleGroup{Name: name, Enabled: true, Graph: string(raw), Revision: 1}
	require.NoError(t, db.DB(ctx).Create(rule).Error)
	return rule
}

func snapshotIPMatchGraph(ipGroupID uint) waf.RuleGraph {
	return snapshotIPMatchGraphForGroups([]uint{ipGroupID})
}

func snapshotIPMatchGraphForGroups(ipGroupIDs []uint) waf.RuleGraph {
	config, _ := json.Marshal(waf.IPMatchConfig{IPGroupIDs: ipGroupIDs})
	return waf.RuleGraph{SchemaVersion: waf.RuleGraphSchemaVersion, Nodes: []waf.RuleNode{
		{ID: "start", Type: waf.RuleNodeStart, Position: waf.RulePosition{X: 1, Y: 2}, Config: json.RawMessage(`{}`)},
		{ID: "match", Type: waf.RuleNodeIPMatch, Position: waf.RulePosition{X: 3, Y: 4}, Config: config},
		{ID: "allow", Type: waf.RuleNodeAllow, Position: waf.RulePosition{X: 5, Y: 6}, Config: json.RawMessage(`{}`)},
	}, Edges: []waf.RuleEdge{
		{ID: "e1", Source: "start", SourceHandle: "next", Target: "match"},
		{ID: "e2", Source: "match", SourceHandle: "true", Target: "allow"},
		{ID: "e3", Source: "match", SourceHandle: "false", Target: "allow"},
	}}
}

func snapshotPoWGraph() waf.RuleGraph {
	config, _ := json.Marshal(waf.PoWNodeConfig{Algorithm: "fast", Difficulty: 4, SessionTTL: 600, ChallengeTTL: 300})
	return waf.RuleGraph{SchemaVersion: waf.RuleGraphSchemaVersion, Nodes: []waf.RuleNode{
		{ID: "start", Type: waf.RuleNodeStart, Config: json.RawMessage(`{}`)},
		{ID: "pow", Type: waf.RuleNodePoW, Config: config},
		{ID: "allow", Type: waf.RuleNodeAllow, Config: json.RawMessage(`{}`)},
	}, Edges: []waf.RuleEdge{
		{ID: "e1", Source: "start", SourceHandle: "next", Target: "pow"},
		{ID: "e2", Source: "pow", SourceHandle: "next", Target: "allow"},
	}}
}
