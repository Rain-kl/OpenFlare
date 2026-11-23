// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/repository"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/waf"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/Rain-kl/Wavelet/pkg/cache/ram"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupConfigVersionTestDB(t *testing.T) func() {
	t.Helper()

	// 同 origin_error_page_snapshot_test.go：换 DB 前后重置进程级 RAM 配置缓存。
	ram.ResetForTest()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(
		&model.ProxyRoute{},
		&model.OpenFlarePoWPolicy{},
		&model.Zone{},
		&model.ZoneDomain{},
		&model.ConfigVersion{},
		&model.EdgeSnapshotState{},
		&model.OpenFlareWAFRuleGroup{},
		&model.OpenFlareAttackGroup{},
		&model.OpenFlareWAFWhitelistEntry{},
		&model.OpenFlareWAFRuleGroupBinding{},
		&model.OpenFlareWAFIPGroup{},
		&model.SystemConfig{},
	))

	require.NoError(t, sqliteDB.Create(&model.EdgeSnapshotState{ID: 1, SourceRevision: 1}).Error)
	db.SetDB(sqliteDB)
	return func() {
		db.SetDB(nil)
		ram.ResetForTest()
	}
}

func createSnapshotZoneDomains(t *testing.T, ctx context.Context, route *model.ProxyRoute, domains ...string) {
	t.Helper()
	zone := &model.Zone{Domain: fmt.Sprintf("zone-%d.example", route.ID)}
	require.NoError(t, db.DB(ctx).Create(zone).Error)
	for _, domain := range domains {
		require.NoError(t, db.DB(ctx).Create(&model.ZoneDomain{
			ZoneID:       zone.ID,
			ProxyRouteID: &route.ID,
			Domain:       domain,
		}).Error)
	}
}

func TestBuildSnapshotWAFDocumentUsesNormalizedSiteNames(t *testing.T) {
	cleanup := setupConfigVersionTestDB(t)
	defer cleanup()
	ctx := context.Background()

	route := &model.ProxyRoute{
		SiteName:  "example.com",
		OriginURL: "http://origin.example.com:8080",
		Upstreams: `["http://origin.example.com:8080"]`,
		Enabled:   true,
	}
	require.NoError(t, repository.CreateProxyRouteRecord(ctx, route))
	createSnapshotZoneDomains(t, ctx, route, "example.com", "www.example.com")

	require.NoError(t, waf.EnsureDefaultRuleGroup(ctx))
	globalGroup, err := repository.GetGlobalOpenFlareWAFRuleGroup(ctx)
	require.NoError(t, err)

	customGroup := createSnapshotRule(t, ctx, "pow-group", waf.DefaultRuleGraph())
	require.NoError(t, repository.ReplaceOpenFlareWAFRuleGroupBindings(ctx, customGroup.ID, []uint{route.ID}))

	bundle, err := buildCurrentConfigBundle(ctx)
	require.NoError(t, err)
	require.Len(t, bundle.SnapshotRoutes, 1)
	assert.Equal(t, "example.com", bundle.SnapshotRoutes[0].SiteName)

	require.NotEmpty(t, bundle.WAFSnapshot.Bindings)
	found := false
	for _, binding := range bundle.WAFSnapshot.Bindings {
		if binding.RouteID != route.ID {
			continue
		}
		found = true
		assert.Equal(t, "example.com", binding.SiteName)
		assert.Contains(t, binding.RuleGroupIDs, customGroup.ID)
	}
	assert.True(t, found, "expected WAF binding for enabled route")

	var document edgeconfig.Document
	require.NoError(t, json.Unmarshal([]byte(bundle.SnapshotJSON), &document))
	wafRuntime := document.WAF
	require.NotEmpty(t, wafRuntime.RuleGroups)
	assert.Equal(t, globalGroup.ID, wafRuntime.RuleGroups[0].ID)
	assert.True(t, wafRuntime.RuleGroups[0].IsGlobal)
	require.Len(t, wafRuntime.Bindings, 1)
	assert.Equal(t, route.ID, wafRuntime.Bindings[0].RouteID)
	assert.Equal(t, "example.com", wafRuntime.Bindings[0].SiteName)
	assert.Equal(t, []uint{customGroup.ID}, wafRuntime.Bindings[0].RuleGroupIDs)
	assert.Contains(t, bundle.SnapshotJSON, `"site_name":"example.com"`)
}

func TestBuildCurrentConfigBundleEnablesGlobalPoWWithoutExplicitBinding(t *testing.T) {
	cleanup := setupConfigVersionTestDB(t)
	defer cleanup()
	ctx := context.Background()

	route := &model.ProxyRoute{
		SiteName:  "pow-global.example.com",
		OriginURL: "http://origin.example.com:8080",
		Upstreams: `["http://origin.example.com:8080"]`,
		Enabled:   true,
	}
	require.NoError(t, repository.CreateProxyRouteRecord(ctx, route))
	createSnapshotZoneDomains(t, ctx, route, "pow-global.example.com")

	require.NoError(t, waf.EnsureDefaultRuleGroup(ctx))
	globalGroup, err := repository.GetGlobalOpenFlareWAFRuleGroup(ctx)
	require.NoError(t, err)
	graphJSON, err := json.Marshal(snapshotPoWGraph())
	require.NoError(t, err)
	globalGroup.Graph = string(graphJSON)
	require.NoError(t, db.DB(ctx).Model(globalGroup).Update("graph", globalGroup.Graph).Error)

	bundle, err := buildCurrentConfigBundle(ctx)
	require.NoError(t, err)
	var document edgeconfig.Document
	require.NoError(t, json.Unmarshal([]byte(bundle.SnapshotJSON), &document))
	wafRuntime := document.WAF
	require.NotEmpty(t, wafRuntime.RuleGroups)
	assert.Equal(t, globalGroup.ID, wafRuntime.RuleGroups[0].ID)
	assert.True(t, wafRuntime.RuleGroups[0].IsGlobal)
	assert.Equal(t, string(waf.RuleNodePoW), wafRuntime.RuleGroups[0].Graph.Nodes["pow"].Type)
	require.Len(t, wafRuntime.Bindings, 1)
	assert.Equal(t, "pow-global.example.com", wafRuntime.Bindings[0].SiteName)
	assert.Empty(t, wafRuntime.Bindings[0].RuleGroupIDs)
	require.NotEmpty(t, bundle.WAFSnapshot.RuleGroups)
	assert.Equal(t, waf.RuleNodePoW, bundle.WAFSnapshot.RuleGroups[0].Graph.Nodes["pow"].Type)
}
