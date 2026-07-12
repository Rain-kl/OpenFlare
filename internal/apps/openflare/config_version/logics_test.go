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
		Domain:    "publish.example.com",
		Domains:   `["publish.example.com"]`,
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
		Domain:    "Example.COM",
		Domains:   `["example.com","www.example.com"]`,
		OriginURL: "http://origin.example.com:8080",
		Upstreams: `["http://origin.example.com:8080"]`,
		Enabled:   true,
	}
	require.NoError(t, model.CreateProxyRouteRecord(ctx, route))
	createSnapshotZoneDomains(t, ctx, route, "example.com", "www.example.com")

	require.NoError(t, waf.EnsureDefaultRuleGroup(ctx))
	globalGroup, err := model.GetGlobalOpenFlareWAFRuleGroup(ctx)
	require.NoError(t, err)

	customGroup := &model.OpenFlareWAFRuleGroup{
		Name:       "pow-group",
		Enabled:    true,
		PoWEnabled: true,
		PoWConfig:  `{"difficulty":4,"algorithm":"fast","session_ttl":600,"challenge_ttl":300}`,
	}
	require.NoError(t, model.CreateOpenFlareWAFRuleGroup(ctx, customGroup))
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

	var wafRuntime struct {
		SiteRuleGroups map[string][]uint `json:"site_rule_groups"`
	}
	for _, file := range bundle.SupportFiles {
		if file.Path != "waf_config.json" {
			continue
		}
		require.NoError(t, json.Unmarshal([]byte(file.Content), &wafRuntime))
	}
	require.Contains(t, wafRuntime.SiteRuleGroups, "example.com")
	require.Contains(t, wafRuntime.SiteRuleGroups["example.com"], customGroup.ID)
	require.Contains(t, wafRuntime.SiteRuleGroups["example.com"], globalGroup.ID)
	assert.Contains(t, bundle.RouteConfig, `set $openflare_waf_site "example.com"`)
	assert.Contains(t, bundle.RouteConfig, `require("pow.runtime").check()`)
}

func TestBuildCurrentConfigBundleEnablesGlobalPoWWithoutExplicitBinding(t *testing.T) {
	cleanup := setupConfigVersionTestDB(t)
	defer cleanup()
	ctx := context.Background()

	route := &model.ProxyRoute{
		SiteName:  "pow-global.example.com",
		Domain:    "pow-global.example.com",
		Domains:   `["pow-global.example.com"]`,
		OriginURL: "http://origin.example.com:8080",
		Upstreams: `["http://origin.example.com:8080"]`,
		Enabled:   true,
	}
	require.NoError(t, model.CreateProxyRouteRecord(ctx, route))
	createSnapshotZoneDomains(t, ctx, route, "pow-global.example.com")

	require.NoError(t, waf.EnsureDefaultRuleGroup(ctx))
	globalGroup, err := model.GetGlobalOpenFlareWAFRuleGroup(ctx)
	require.NoError(t, err)
	globalGroup.PoWEnabled = true
	globalGroup.PoWConfig = `{"difficulty":4,"algorithm":"fast","session_ttl":600,"challenge_ttl":300}`
	require.NoError(t, model.UpdateOpenFlareWAFRuleGroup(ctx, globalGroup))

	bundle, err := buildCurrentConfigBundle(ctx, true)
	require.NoError(t, err)
	assert.Contains(t, bundle.RouteConfig, `require("pow.runtime").check()`)

	var wafRuntime struct {
		RuleGroups []struct {
			ID         uint `json:"id"`
			PoWEnabled bool `json:"pow_enabled"`
			PoWConfig  *struct {
				Difficulty int `json:"difficulty"`
			} `json:"pow_config"`
		} `json:"rule_groups"`
		SiteRuleGroups map[string][]uint `json:"site_rule_groups"`
	}
	for _, file := range bundle.SupportFiles {
		if file.Path != "waf_config.json" {
			continue
		}
		require.NoError(t, json.Unmarshal([]byte(file.Content), &wafRuntime))
	}
	require.Contains(t, wafRuntime.SiteRuleGroups, "pow-global.example.com")
	require.Contains(t, wafRuntime.SiteRuleGroups["pow-global.example.com"], globalGroup.ID)
	require.NotEmpty(t, wafRuntime.RuleGroups)
	assert.True(t, wafRuntime.RuleGroups[0].PoWEnabled)
	require.NotNil(t, wafRuntime.RuleGroups[0].PoWConfig)
	assert.Equal(t, 4, wafRuntime.RuleGroups[0].PoWConfig.Difficulty)
}
