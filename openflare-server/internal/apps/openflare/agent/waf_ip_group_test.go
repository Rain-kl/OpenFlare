// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupWAFIPGroupTestDB(t *testing.T) func() {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(
		&model.OpenFlareWAFIPGroup{},
		&configVersionRecord{},
	))

	db.SetDB(sqliteDB)
	return func() {
		db.SetDB(nil)
	}
}

func seedActiveConfigWithWAFIPGroup(t *testing.T, ctx context.Context, ipGroupID uint) {
	t.Helper()

	snapshot := map[string]any{
		"routes": []any{},
		"waf": map[string]any{
			"rule_groups": []map[string]any{
				{
					"id":                     1,
					"name":                   "agent refs",
					"enabled":                true,
					"ip_blacklist_group_ids": []uint{ipGroupID},
				},
			},
			"bindings": []any{},
		},
	}
	snapshotJSON, err := json.Marshal(snapshot)
	require.NoError(t, err)

	require.NoError(t, db.DB(ctx).Create(&configVersionRecord{
		Version:      "20260618-001",
		SnapshotJSON: string(snapshotJSON),
		Checksum:     "test-checksum",
		IsActive:     true,
	}).Error)
}

func TestChangedWAFIPGroupsForAgentReturnsChecksumDelta(t *testing.T) {
	cleanup := setupWAFIPGroupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	ipGroup := &model.OpenFlareWAFIPGroup{
		Name:    "agent runtime group",
		Type:    "manual",
		Enabled: true,
		IPList:  `["203.0.113.44"]`,
	}
	require.NoError(t, model.CreateOpenFlareWAFIPGroup(ctx, ipGroup))
	seedActiveConfigWithWAFIPGroup(t, ctx, ipGroup.ID)

	groups, err := ChangedWAFIPGroupsForAgent(ctx, nil, nil)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, ipGroup.ID, groups[0].ID)
	assert.Equal(t, "203.0.113.44", groups[0].IPList[0])
	assert.NotEmpty(t, groups[0].Checksum)

	groupKey := strconv.FormatUint(uint64(ipGroup.ID), 10)
	same, err := ChangedWAFIPGroupsForAgent(ctx, nil, map[string]string{groupKey: groups[0].Checksum})
	require.NoError(t, err)
	assert.Empty(t, same)

	ipGroup.IPList = `["203.0.113.45"]`
	require.NoError(t, model.UpdateOpenFlareWAFIPGroup(ctx, ipGroup))

	delta, err := ChangedWAFIPGroupsForAgent(ctx, nil, map[string]string{groupKey: groups[0].Checksum})
	require.NoError(t, err)
	require.Len(t, delta, 1)
	assert.Equal(t, ipGroup.ID, delta[0].ID)
	assert.Equal(t, "203.0.113.45", delta[0].IPList[0])
	assert.NotEqual(t, groups[0].Checksum, delta[0].Checksum)
}

func TestSyncWAFIPGroupsReturnsChangedGroups(t *testing.T) {
	cleanup := setupWAFIPGroupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	ipGroup := &model.OpenFlareWAFIPGroup{
		Name:    "sync group",
		Type:    "manual",
		Enabled: true,
		IPList:  `["198.51.100.10"]`,
	}
	require.NoError(t, model.CreateOpenFlareWAFIPGroup(ctx, ipGroup))
	seedActiveConfigWithWAFIPGroup(t, ctx, ipGroup.ID)

	result, err := SyncWAFIPGroups(ctx, WAFIPGroupSyncInput{
		IDs:       []uint{ipGroup.ID},
		Checksums: map[string]string{},
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, ipGroup.ID, result.Groups[0].ID)
	assert.Equal(t, "198.51.100.10", result.Groups[0].IPList[0])

	result, err = SyncWAFIPGroups(ctx, WAFIPGroupSyncInput{
		IDs: []uint{ipGroup.ID},
		Checksums: map[string]string{
			strconv.FormatUint(uint64(ipGroup.ID), 10): result.Groups[0].Checksum,
		},
	})
	require.NoError(t, err)
	assert.Empty(t, result.Groups)
}

func TestChangedWAFIPGroupsForAgentDisabledGroupClearsIPList(t *testing.T) {
	cleanup := setupWAFIPGroupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	ipGroup := &model.OpenFlareWAFIPGroup{
		Name:    "disabled group",
		Type:    "manual",
		Enabled: true,
		IPList:  `["203.0.113.10"]`,
	}
	require.NoError(t, model.CreateOpenFlareWAFIPGroup(ctx, ipGroup))
	ipGroup.Enabled = false
	require.NoError(t, model.UpdateOpenFlareWAFIPGroup(ctx, ipGroup))
	seedActiveConfigWithWAFIPGroup(t, ctx, ipGroup.ID)

	groups, err := ChangedWAFIPGroupsForAgent(ctx, nil, nil)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.False(t, groups[0].Enabled)
	assert.Empty(t, groups[0].IPList)
	assert.NotEmpty(t, groups[0].Checksum)
}
