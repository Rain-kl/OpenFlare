// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"context"
	"testing"

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
		&model.OriginProxyRoute{},
	))

	db.SetDB(sqliteDB)
	return func() {
		db.SetDB(nil)
	}
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
	})
	require.NoError(t, err)
	require.Len(t, updated.IPList, 1)
	assert.Equal(t, "203.0.113.10", updated.IPList[0])
	require.Len(t, updated.ExtIPs, 1)
	assert.Equal(t, "203.0.113.10", updated.ExtIPs[0].IP)
}
