// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/waf"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	openrestyrender "github.com/Rain-kl/Wavelet/pkg/render/openresty"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupConfigVersionTestDB(t *testing.T) func() {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(
		&model.ProxyRoute{},
		&model.Zone{},
		&model.ZoneDomain{},
		&model.ConfigVersion{},
		&model.OpenFlareWAFRuleGroup{},
		&model.OpenFlareWAFRuleGroupBinding{},
		&model.OpenFlareWAFIPGroup{},
		&model.SystemConfig{},
	))

	db.SetDB(sqliteDB)
	return func() {
		db.SetDB(nil)
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

func TestListConfigVersionsOrdersByCreatedAtDesc(t *testing.T) {
	cleanup := setupConfigVersionTestDB(t)
	defer cleanup()
	ctx := context.Background()
	conn := db.DB(ctx)
	require.NotNil(t, conn)

	newer := &model.ConfigVersion{
		Version:        "20260102-001",
		SnapshotJSON:   "{}",
		RenderedConfig: "route {}",
		Checksum:       "checksum-newer",
		CreatedBy:      "tester",
		CreatedAt:      time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC),
	}
	older := &model.ConfigVersion{
		Version:        "20260101-001",
		SnapshotJSON:   "{}",
		RenderedConfig: "route {}",
		Checksum:       "checksum-older",
		CreatedBy:      "tester",
		CreatedAt:      time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
	}
	require.NoError(t, conn.Create(newer).Error)
	require.NoError(t, conn.Create(older).Error)

	versions, err := ListConfigVersions(ctx)
	require.NoError(t, err)
	require.Len(t, versions, 2)
	assert.Equal(t, newer.Version, versions[0].Version)
	assert.Equal(t, older.Version, versions[1].Version)
}

func TestPublishConfigVersionCreatesVersion(t *testing.T) {
	cleanup := setupConfigVersionTestDB(t)
	defer cleanup()
	ctx := context.Background()

	route := &model.ProxyRoute{
		SiteName:  "publish-site",
		OriginURL: "http://origin.publish.example.com:8080",
		Upstreams: `["http://origin.publish.example.com:8080"]`,
		Enabled:   true,
	}
	require.NoError(t, model.CreateProxyRouteRecord(ctx, route))
	createSnapshotZoneDomains(t, ctx, route, "publish.example.com")

	version, err := PublishConfigVersion(ctx, "tester", false)
	require.NoError(t, err)
	require.NotNil(t, version)
	assert.NotEmpty(t, version.ID)
	assert.True(t, version.IsActive)
	assert.Equal(t, "tester", version.CreatedBy)
	assert.NotEmpty(t, version.Version)
	assert.NotEmpty(t, version.Checksum)
	assert.NotEmpty(t, version.SnapshotJSON)
	assert.NotEmpty(t, version.RenderedConfig)

	var snapshot snapshotDocument
	require.NoError(t, json.Unmarshal([]byte(version.SnapshotJSON), &snapshot))
	require.Len(t, snapshot.Routes, 1)
	assert.Equal(t, "publish-site", snapshot.Routes[0].SiteName)
	assert.Equal(t, []string{"publish.example.com"}, snapshot.Routes[0].Domains)

	active, err := GetActiveConfigVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, version.ID, active.ID)

	_, err = PublishConfigVersion(ctx, "tester", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), errNoChangesToPublish)

	forced, err := PublishConfigVersion(ctx, "tester", true)
	require.NoError(t, err)
	assert.NotEqual(t, version.ID, forced.ID)
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
	require.NoError(t, model.CreateProxyRouteRecord(ctx, route))
	createSnapshotZoneDomains(t, ctx, route, "example.com", "www.example.com")

	require.NoError(t, waf.EnsureDefaultRuleGroup(ctx))
	globalGroup, err := model.GetGlobalOpenFlareWAFRuleGroup(ctx)
	require.NoError(t, err)

	customGroup := createSnapshotRule(t, ctx, "pow-group", waf.DefaultRuleGraph())
	require.NoError(t, model.ReplaceOpenFlareWAFRuleGroupBindings(ctx, customGroup.ID, []uint{route.ID}))

	bundle, err := buildCurrentConfigBundle(ctx, true)
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

	var wafRuntime openrestyrender.WAFDocument
	foundWAFConfig := false
	for _, file := range bundle.SupportFiles {
		if file.Path != "waf_config.json" {
			continue
		}
		foundWAFConfig = true
		require.NoError(t, json.Unmarshal([]byte(file.Content), &wafRuntime))
	}
	require.True(t, foundWAFConfig, "expected rendered WAF support file")
	require.NotEmpty(t, wafRuntime.RuleGroups)
	assert.Equal(t, globalGroup.ID, wafRuntime.RuleGroups[0].ID)
	assert.True(t, wafRuntime.RuleGroups[0].IsGlobal)
	require.Len(t, wafRuntime.Bindings, 1)
	assert.Equal(t, route.ID, wafRuntime.Bindings[0].RouteID)
	assert.Equal(t, "example.com", wafRuntime.Bindings[0].SiteName)
	assert.Equal(t, []uint{customGroup.ID}, wafRuntime.Bindings[0].RuleGroupIDs)
	assert.Contains(t, bundle.RouteConfig, `set $openflare_waf_site "example.com"`)
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
	require.NoError(t, model.CreateProxyRouteRecord(ctx, route))
	createSnapshotZoneDomains(t, ctx, route, "pow-global.example.com")

	require.NoError(t, waf.EnsureDefaultRuleGroup(ctx))
	globalGroup, err := model.GetGlobalOpenFlareWAFRuleGroup(ctx)
	require.NoError(t, err)
	graphJSON, err := json.Marshal(snapshotPoWGraph())
	require.NoError(t, err)
	globalGroup.Graph = string(graphJSON)
	require.NoError(t, db.DB(ctx).Model(globalGroup).Update("graph", globalGroup.Graph).Error)

	bundle, err := buildCurrentConfigBundle(ctx, true)
	require.NoError(t, err)
	var wafRuntime openrestyrender.WAFDocument
	foundWAFConfig := false
	for _, file := range bundle.SupportFiles {
		if file.Path != "waf_config.json" {
			continue
		}
		foundWAFConfig = true
		assert.Contains(t, file.Content, `"rule_group_ids":[]`)
		assert.NotContains(t, file.Content, `"rule_group_ids":null`)
		require.NoError(t, json.Unmarshal([]byte(file.Content), &wafRuntime))
	}
	require.True(t, foundWAFConfig, "expected rendered WAF support file")
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
