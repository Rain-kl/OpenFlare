// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupWAFBindingsTestDB(t *testing.T) func() {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(&OpenFlareWAFRuleGroupBinding{}))

	db.SetDB(sqliteDB)
	return func() {
		db.SetDB(nil)
	}
}

func TestReplaceOpenFlareWAFRuleGroupBindingsAfterExplicitHighID(t *testing.T) {
	cleanup := setupWAFBindingsTestDB(t)
	defer cleanup()
	ctx := context.Background()

	conn := db.DB(ctx)
	require.NotNil(t, conn)
	require.NoError(t, conn.Create(&OpenFlareWAFRuleGroupBinding{
		ID:           50,
		RuleGroupID:  1,
		ProxyRouteID: 1,
	}).Error)

	require.NoError(t, ReplaceOpenFlareWAFRuleGroupBindings(ctx, 2, []uint{2, 3}))

	var bindings []OpenFlareWAFRuleGroupBinding
	require.NoError(t, conn.Where("rule_group_id = ?", 2).Order("proxy_route_id asc").Find(&bindings).Error)
	require.Len(t, bindings, 2)
	assert.Equal(t, uint(2), bindings[0].ProxyRouteID)
	assert.Equal(t, uint(3), bindings[1].ProxyRouteID)
	assert.Greater(t, bindings[0].ID, uint(50))
	assert.Greater(t, bindings[1].ID, bindings[0].ID)
}
