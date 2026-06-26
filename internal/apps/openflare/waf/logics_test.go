// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"context"
	"testing"

	ofagent "github.com/Rain-kl/Wavelet/internal/apps/openflare/agent"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupWAFTestDB(t *testing.T) func() {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(
		&model.OpenFlareWAFRuleGroup{},
		&model.OpenFlareWAFIPGroup{},
		&model.OpenFlareWAFRuleGroupBinding{},
	))

	db.SetDB(sqliteDB)
	return func() {
		db.SetDB(nil)
	}
}

func TestCreateRuleGroup(t *testing.T) {
	cleanup := setupWAFTestDB(t)
	defer cleanup()
	ctx := context.Background()

	group, err := CreateRuleGroup(ctx, RuleGroupInput{
		Name:             "edge guard",
		Enabled:          true,
		BlockStatusCode:  451,
		IPWhitelist:      []string{" 192.0.2.1 ", "192.0.2.1", "198.51.100.0/24"},
		IPBlacklist:      []string{"203.0.113.10"},
		CountryBlacklist: []string{" cn ", "CN", "us"},
	})
	require.NoError(t, err)
	assert.NotZero(t, group.ID)
	assert.False(t, group.IsGlobal)
	assert.Equal(t, "edge guard", group.Name)
	require.Len(t, group.IPWhitelist, 2)
	assert.Equal(t, "192.0.2.1", group.IPWhitelist[0])
	assert.Equal(t, "198.51.100.0/24", group.IPWhitelist[1])
	require.Len(t, group.CountryBlacklist, 2)
	assert.Equal(t, "CN", group.CountryBlacklist[0])
	assert.Equal(t, "US", group.CountryBlacklist[1])

	_, err = CreateRuleGroup(ctx, RuleGroupInput{
		Name:        "bad ip",
		Enabled:     true,
		IPBlacklist: []string{"not-an-ip"},
	})
	require.Error(t, err)
}

func TestPruneIPGroupExtIPs(t *testing.T) {
	group := &model.OpenFlareWAFIPGroup{
		ExtIPs: `[{"ip":"203.0.113.10","captured_at":"2026-06-18T10:00:00Z"},{"ip":"203.0.113.11","captured_at":"2026-06-18T11:00:00Z"}]`,
	}
	err := pruneIPGroupExtIPs(group, []string{"203.0.113.10"})
	require.NoError(t, err)
	assert.JSONEq(t, `[{"ip":"203.0.113.10","captured_at":"2026-06-18T10:00:00Z"}]`, group.ExtIPs)
}

func TestUpdateIPGroupPrunesAutomaticExtIPs(t *testing.T) {
	cleanup := setupWAFTestDB(t)
	defer cleanup()
	ctx := context.Background()

	created, err := CreateIPGroup(ctx, IPGroupInput{
		Name:       "auto group",
		Type:       wafIPGroupTypeAutomatic,
		Enabled:    true,
		AutoConfig: []byte(`{"lookback_minutes":60,"ttl":-1,"rules":[{"name":"scan","expr":"request_count > 1"}]}`),
	})
	require.NoError(t, err)

	group, err := model.GetOpenFlareWAFIPGroupByID(ctx, created.ID)
	require.NoError(t, err)
	group.IPList = `["203.0.113.10","203.0.113.11"]`
	group.ExtIPs = `[{"ip":"203.0.113.10","captured_at":"2026-06-18T10:00:00Z"},{"ip":"203.0.113.11","captured_at":"2026-06-18T11:00:00Z"}]`
	require.NoError(t, model.UpdateOpenFlareWAFIPGroup(ctx, group))

	updated, err := UpdateIPGroup(ctx, created.ID, IPGroupInput{
		Name:       created.Name,
		Type:       created.Type,
		Enabled:    created.Enabled,
		IPList:     []string{"203.0.113.10"},
		AutoConfig: created.AutoConfig,
		Remark:     created.Remark,
	})
	require.NoError(t, err)
	require.Len(t, updated.IPList, 1)
	assert.Equal(t, "203.0.113.10", updated.IPList[0])
	require.Len(t, updated.ExtIPs, 1)
	assert.Equal(t, "203.0.113.10", updated.ExtIPs[0].IP)
}

func TestCreateIPGroupBroadcastsSavedMembers(t *testing.T) {
	cleanup := setupWAFTestDB(t)
	defer cleanup()
	ctx := context.Background()

	originalBroadcast := broadcastWAFIPGroups
	defer func() { broadcastWAFIPGroups = originalBroadcast }()

	var payloads [][]ofagent.WAFIPGroup
	broadcastWAFIPGroups = func(payload any) int {
		groups, ok := payload.([]ofagent.WAFIPGroup)
		require.True(t, ok)
		payloads = append(payloads, append([]ofagent.WAFIPGroup(nil), groups...))
		return 1
	}

	group, err := CreateIPGroup(ctx, IPGroupInput{
		Name:    "manual blacklist",
		Type:    wafIPGroupTypeManual,
		Enabled: true,
		IPList:  []string{"203.0.113.10", "198.51.100.0/24"},
	})
	require.NoError(t, err)
	require.NotZero(t, group.ID)
	require.Len(t, payloads, 1)
	require.Len(t, payloads[0], 1)
	assert.Equal(t, group.ID, payloads[0][0].ID)
	assert.Equal(t, []string{"198.51.100.0/24", "203.0.113.10"}, payloads[0][0].IPList)
	assert.NotEmpty(t, payloads[0][0].Checksum)
}

func TestUpdateIPGroupBroadcastsUpdatedMembersWithoutRemovedCIDR(t *testing.T) {
	cleanup := setupWAFTestDB(t)
	defer cleanup()
	ctx := context.Background()

	group, err := CreateIPGroup(ctx, IPGroupInput{
		Name:    "manual blacklist",
		Type:    wafIPGroupTypeManual,
		Enabled: true,
		IPList:  []string{"203.0.113.10", "198.51.100.0/24"},
	})
	require.NoError(t, err)

	originalBroadcast := broadcastWAFIPGroups
	defer func() { broadcastWAFIPGroups = originalBroadcast }()

	var payloads [][]ofagent.WAFIPGroup
	broadcastWAFIPGroups = func(payload any) int {
		groups, ok := payload.([]ofagent.WAFIPGroup)
		require.True(t, ok)
		payloads = append(payloads, append([]ofagent.WAFIPGroup(nil), groups...))
		return 1
	}

	updated, err := UpdateIPGroup(ctx, group.ID, IPGroupInput{
		Name:    group.Name,
		Type:    group.Type,
		Enabled: group.Enabled,
		IPList:  []string{"198.51.100.0/24"},
	})
	require.NoError(t, err)
	require.Len(t, updated.IPList, 1)
	require.Len(t, payloads, 1)
	require.Len(t, payloads[0], 1)
	assert.Equal(t, []string{"198.51.100.0/24"}, payloads[0][0].IPList)
	assert.NotContains(t, payloads[0][0].IPList, "203.0.113.10")
}
