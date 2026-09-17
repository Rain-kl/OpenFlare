// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"testing"

	"Wavelet/openflare/plugins/server/kernel/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const defaultWAFRuleGraph = `{"schema_version":1,"nodes":[{"id":"start","type":"start","position":{"x":0,"y":0},"config":{}},{"id":"allow","type":"allow","position":{"x":320,"y":0},"config":{}}],"edges":[{"id":"start-allow","source":"start","source_handle":"next","target":"allow"}]}`

func TestOpenFlareWAFGraphOptimisticUpdate(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, conn.AutoMigrate(&model.OpenFlareWAFRuleGroup{}))
	SetDBForTest(conn)
	t.Cleanup(func() { SetDBForTest(nil) })

	group := model.OpenFlareWAFRuleGroup{Name: "rule", Graph: defaultWAFRuleGraph, Revision: 1}
	require.NoError(t, conn.Create(&group).Error)

	nextRevision, err := UpdateOpenFlareWAFRuleGraph(context.Background(), group.ID, 1, `{"schema_version":1}`)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), nextRevision)

	_, err = UpdateOpenFlareWAFRuleGraph(context.Background(), group.ID, 1, defaultWAFRuleGraph)
	assert.ErrorIs(t, err, model.ErrWAFRuleRevisionConflict)
}

func TestReplaceOpenFlareWAFRuleGroupBindingsPreservesInputOrder(t *testing.T) {
	cleanup := setupWAFBindingsTestDB(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, ReplaceOpenFlareWAFSiteRuleGroupBindings(ctx, 7, []uint{30, 10, 20}))
	bindings, err := ListOpenFlareWAFRuleGroupBindingsByRouteID(ctx, 7)
	require.NoError(t, err)
	require.Len(t, bindings, 3)
	assert.Equal(t, []uint{30, 10, 20}, []uint{bindings[0].RuleGroupID, bindings[1].RuleGroupID, bindings[2].RuleGroupID})
	assert.Equal(t, []int{0, 1, 2}, []int{bindings[0].Sequence, bindings[1].Sequence, bindings[2].Sequence})
}

func TestLegacyWAFColumnsRemoved(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, conn.AutoMigrate(&model.OpenFlareWAFRuleGroup{}))
	legacy := []string{"block_status_code", "block_response_body", "ip_whitelist", "ip_blacklist", "ip_whitelist_groups", "ip_blacklist_groups", "country_whitelist", "country_blacklist", "region_whitelist", "region_blacklist", "pow_enabled", "pow_config"}
	for _, column := range legacy {
		if conn.Migrator().HasColumn(&model.OpenFlareWAFRuleGroup{}, column) {
			t.Fatalf("legacy WAF column %s still exists", column)
		}
	}
}
