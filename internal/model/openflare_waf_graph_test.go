// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const defaultWAFRuleGraph = `{"schema_version":1,"nodes":[{"id":"start","type":"start","position":{"x":0,"y":0},"config":{}},{"id":"allow","type":"allow","position":{"x":320,"y":0},"config":{}}],"edges":[{"id":"start-allow","source":"start","source_handle":"next","target":"allow"}]}`

func wafMigrationFS(t *testing.T) fs.FS {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Join(filepath.Dir(filename), "..", "db", "migrator", "goose", "sqlite")
	migrations := fstest.MapFS{}
	for _, name := range []string{
		"202607150001_orchestrate_waf_rules.sql",
		"202607150002_reset_waf_rule_graphs.sql",
		"202607150003_drop_legacy_waf_rule_fields.sql",
	} {
		contents, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		migrations[name] = &fstest.MapFile{Data: contents}
	}
	return migrations
}

func TestOpenFlareWAFGraphMigrationResetsGraphsAndOrdersBindings(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := conn.DB()
	require.NoError(t, err)

	require.NoError(t, conn.Exec(`CREATE TABLE of_waf_rule_groups (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, block_status_code INTEGER NOT NULL DEFAULT 418, block_response_body TEXT NOT NULL DEFAULT '', ip_whitelist TEXT NOT NULL DEFAULT '[]', ip_blacklist TEXT NOT NULL DEFAULT '[]', ip_whitelist_groups TEXT NOT NULL DEFAULT '[]', ip_blacklist_groups TEXT NOT NULL DEFAULT '[]', country_whitelist TEXT NOT NULL DEFAULT '[]', country_blacklist TEXT NOT NULL DEFAULT '[]', region_whitelist TEXT NOT NULL DEFAULT '[]', region_blacklist TEXT NOT NULL DEFAULT '[]', pow_enabled BOOLEAN NOT NULL DEFAULT 0, pow_config TEXT NOT NULL DEFAULT '{}')`).Error)
	require.NoError(t, conn.Exec(`CREATE TABLE of_waf_rule_group_bindings (id INTEGER PRIMARY KEY AUTOINCREMENT, rule_group_id INTEGER NOT NULL, proxy_route_id INTEGER NOT NULL)`).Error)
	require.NoError(t, conn.Exec(`INSERT INTO of_waf_rule_groups (id, name) VALUES (1, 'one'), (2, 'two')`).Error)
	require.NoError(t, conn.Exec(`INSERT INTO of_waf_rule_group_bindings (id, rule_group_id, proxy_route_id) VALUES (20, 2, 7), (10, 1, 7)`).Error)

	goose.SetBaseFS(wafMigrationFS(t))
	require.NoError(t, goose.SetDialect("sqlite3"))
	require.NoError(t, goose.Up(sqlDB, "."))

	var groups []OpenFlareWAFRuleGroup
	require.NoError(t, conn.Order("id asc").Find(&groups).Error)
	require.Len(t, groups, 2)
	for _, group := range groups {
		require.JSONEq(t, defaultWAFRuleGraph, group.Graph)
		assert.Equal(t, uint64(1), group.Revision)
	}
	for _, column := range []string{"block_status_code", "ip_whitelist", "pow_enabled"} {
		assert.False(t, conn.Migrator().HasColumn("of_waf_rule_groups", column))
	}

	require.NoError(t, conn.Exec(`INSERT INTO of_waf_rule_groups (name) VALUES ('new')`).Error)
	var newGroup OpenFlareWAFRuleGroup
	require.NoError(t, conn.First(&newGroup, 3).Error)
	assert.Empty(t, newGroup.Graph)
	assert.Equal(t, uint64(1), newGroup.Revision)

	var bindings []OpenFlareWAFRuleGroupBinding
	require.NoError(t, conn.Where("proxy_route_id = ?", 7).Order("sequence asc").Order("id asc").Find(&bindings).Error)
	require.Len(t, bindings, 2)
	assert.Equal(t, []int{0, 1}, []int{bindings[0].Sequence, bindings[1].Sequence})
	assert.Equal(t, []uint{10, 20}, []uint{bindings[0].ID, bindings[1].ID})
}

func TestOpenFlareWAFGraphOptimisticUpdate(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, conn.AutoMigrate(&OpenFlareWAFRuleGroup{}))
	db.SetDB(conn)
	t.Cleanup(func() { db.SetDB(nil) })

	group := OpenFlareWAFRuleGroup{Name: "rule", Graph: defaultWAFRuleGraph, Revision: 1}
	require.NoError(t, conn.Create(&group).Error)

	nextRevision, err := UpdateOpenFlareWAFRuleGraph(context.Background(), group.ID, 1, `{"schema_version":1}`)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), nextRevision)

	_, err = UpdateOpenFlareWAFRuleGraph(context.Background(), group.ID, 1, defaultWAFRuleGraph)
	assert.ErrorIs(t, err, ErrWAFRuleRevisionConflict)
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
	require.NoError(t, conn.AutoMigrate(&OpenFlareWAFRuleGroup{}))
	legacy := []string{"block_status_code", "block_response_body", "ip_whitelist", "ip_blacklist", "ip_whitelist_groups", "ip_blacklist_groups", "country_whitelist", "country_blacklist", "region_whitelist", "region_blacklist", "pow_enabled", "pow_config"}
	for _, column := range legacy {
		if conn.Migrator().HasColumn(&OpenFlareWAFRuleGroup{}, column) {
			t.Fatalf("legacy WAF column %s still exists", column)
		}
	}
}
