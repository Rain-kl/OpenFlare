// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/repository"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/protocol"
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
		&model.ConfigVersion{},
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

	require.NoError(t, db.DB(ctx).Create(&model.ConfigVersion{
		Version:      "20260618-001",
		SnapshotJSON: string(snapshotJSON),
		Checksum:     "test-checksum",
		IsActive:     true,
	}).Error)
}

func seedActiveConfigWithWAFGraphIPGroup(t *testing.T, ctx context.Context, ipGroupID uint) {
	t.Helper()

	snapshot := map[string]any{
		"routes": []any{},
		"waf": map[string]any{
			"rule_groups": []map[string]any{
				{
					"id":      1,
					"name":    "graph refs",
					"enabled": true,
					"graph": map[string]any{
						"entry": "start",
						"nodes": map[string]any{
							"start": map[string]any{
								"type":   "start",
								"config": map[string]any{},
								"next":   map[string]string{"next": "match"},
							},
							"match": map[string]any{
								"type": "ip_match",
								"config": map[string]any{
									"ip_group_ids": []uint{ipGroupID},
								},
								"next": map[string]string{"true": "allow", "false": "allow"},
							},
							"allow": map[string]any{
								"type":   "allow",
								"config": map[string]any{},
							},
						},
					},
				},
			},
			"bindings": []any{},
		},
	}
	snapshotJSON, err := json.Marshal(snapshot)
	require.NoError(t, err)

	require.NoError(t, db.DB(ctx).Create(&model.ConfigVersion{
		Version:      "20260713-graph-001",
		SnapshotJSON: string(snapshotJSON),
		Checksum:     "graph-test-checksum",
		IsActive:     true,
	}).Error)
}

func TestChangedWAFIPGroupsForAgentDiscoversGraphReferences(t *testing.T) {
	cleanup := setupWAFIPGroupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	ipGroup := &model.OpenFlareWAFIPGroup{
		Name:    "graph runtime group",
		Type:    "manual",
		Enabled: true,
		IPList:  `["192.0.2.88"]`,
	}
	require.NoError(t, repository.CreateOpenFlareWAFIPGroup(ctx, ipGroup))
	seedActiveConfigWithWAFGraphIPGroup(t, ctx, ipGroup.ID)

	groups, err := ChangedWAFIPGroupsForAgent(ctx, nil, nil)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, ipGroup.ID, groups[0].ID)
	assert.Equal(t, []string{"192.0.2.88"}, groups[0].IPList)
}

func TestChangedWAFIPGroupsForAgentRejectsMalformedIPMatchConfig(t *testing.T) {
	cleanup := setupWAFIPGroupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	require.NoError(t, db.DB(ctx).Create(&model.ConfigVersion{
		Version: "20260713-malformed-001",
		SnapshotJSON: `{"waf":{"rule_groups":[{"id":7,"graph":{"entry":"match","nodes":{` +
			`"match":{"type":"ip_match","config":{"ip_group_ids":"not-an-array"}}}}}],"bindings":[]}}`,
		Checksum: "malformed-test-checksum",
		IsActive: true,
	}).Error)

	_, err := ChangedWAFIPGroupsForAgent(ctx, nil, nil)
	require.ErrorContains(t, err, "规则 7 节点 match")
	require.ErrorContains(t, err, "IP 匹配配置无效")
}

func TestChangedWAFIPGroupsForAgentRejectsOversizedSnapshotBeforeChecksumDelta(t *testing.T) {
	cleanup := setupWAFIPGroupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	ipGroup := &model.OpenFlareWAFIPGroup{
		Name:    strings.Repeat("x", protocol.MaxWAFIPGroupSnapshotBytes),
		Type:    "manual",
		Enabled: true,
		IPList:  `[]`,
	}
	require.NoError(t, repository.CreateOpenFlareWAFIPGroup(ctx, ipGroup))
	agentGroup, err := buildAgentWAFIPGroup(ipGroup)
	require.NoError(t, err)

	_, err = ChangedWAFIPGroupsForAgent(ctx, []uint{ipGroup.ID}, map[string]string{
		strconv.FormatUint(uint64(ipGroup.ID), 10): agentGroup.Checksum,
	})
	require.ErrorContains(t, err, "WAF IP 组快照大小")
	require.ErrorContains(t, err, "超过上限")
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
	require.NoError(t, repository.CreateOpenFlareWAFIPGroup(ctx, ipGroup))
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
	require.NoError(t, repository.UpdateOpenFlareWAFIPGroup(ctx, ipGroup))

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
	require.NoError(t, repository.CreateOpenFlareWAFIPGroup(ctx, ipGroup))
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
	require.NoError(t, repository.CreateOpenFlareWAFIPGroup(ctx, ipGroup))
	ipGroup.Enabled = false
	require.NoError(t, repository.UpdateOpenFlareWAFIPGroup(ctx, ipGroup))
	seedActiveConfigWithWAFIPGroup(t, ctx, ipGroup.ID)

	groups, err := ChangedWAFIPGroupsForAgent(ctx, nil, nil)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.False(t, groups[0].Enabled)
	assert.Empty(t, groups[0].IPList)
	assert.NotEmpty(t, groups[0].Checksum)
}
