// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"encoding/json"
	"testing"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOriginErrorPageSnapshotDB(t *testing.T) func() {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(&model.SystemConfig{}))
	db.SetDB(sqliteDB)

	return func() {
		db.SetDB(nil)
	}
}

func TestBuildOpenRestyConfigSnapshotOriginErrorPageDefaults(t *testing.T) {
	cleanup := setupOriginErrorPageSnapshotDB(t)
	defer cleanup()

	snapshot := buildOpenRestyConfigSnapshot(context.Background())
	assert.True(t, snapshot.OriginErrorPageEnabled)
	assert.Equal(t, []string{"500-599"}, snapshot.OriginErrorPageStatusCodes)
	assert.Empty(t, snapshot.OriginErrorPageHTML)

	payload, err := json.Marshal(snapshot)
	require.NoError(t, err)
	assert.Contains(t, string(payload), `"origin_error_page_enabled":true`)
	assert.Contains(t, string(payload), `"origin_error_page_status_codes":["500-599"]`)
}

func TestBuildOpenRestyConfigSnapshotOriginErrorPageCustom(t *testing.T) {
	cleanup := setupOriginErrorPageSnapshotDB(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, db.DB(ctx).Create(&model.SystemConfig{
		Key: model.ConfigKeyOriginErrorPageEnabled, Value: "false", Type: "business",
	}).Error)
	require.NoError(t, db.DB(ctx).Create(&model.SystemConfig{
		Key: model.ConfigKeyOriginErrorPageStatusCodes, Value: `["522","500-502"]`, Type: "business",
	}).Error)
	require.NoError(t, db.DB(ctx).Create(&model.SystemConfig{
		Key: model.ConfigKeyOriginErrorPageHTML, Value: "<h1>{{status}}</h1>", Type: "business",
	}).Error)

	snapshot := buildOpenRestyConfigSnapshot(ctx)
	assert.False(t, snapshot.OriginErrorPageEnabled)
	assert.Equal(t, []string{"522", "500-502"}, snapshot.OriginErrorPageStatusCodes)
	assert.Equal(t, "<h1>{{status}}</h1>", snapshot.OriginErrorPageHTML)
}

func TestParseOriginErrorPageStatusCodesFallback(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"500-599"}, parseOriginErrorPageStatusCodes(""))
	assert.Equal(t, []string{"500-599"}, parseOriginErrorPageStatusCodes("not-json"))
	assert.Equal(t, []string{"500-599"}, parseOriginErrorPageStatusCodes("[]"))
	assert.Equal(t, []string{"502"}, parseOriginErrorPageStatusCodes(`["502"]`))
}

func TestDiffOpenRestyOptionDetailsOriginErrorPage(t *testing.T) {
	t.Parallel()

	left := openRestyConfigSnapshot{
		OriginErrorPageEnabled:     true,
		OriginErrorPageStatusCodes: []string{"500-599"},
		OriginErrorPageHTML:        "",
	}
	right := openRestyConfigSnapshot{
		OriginErrorPageEnabled:     false,
		OriginErrorPageStatusCodes: []string{"522"},
		OriginErrorPageHTML:        "<p>x</p>",
	}
	details := diffOpenRestyOptionDetails(left, right)
	keys := make(map[string]ConfigOptionDiffItem, len(details))
	for _, item := range details {
		keys[item.Key] = item
	}
	assert.Equal(t, "true", keys["OriginErrorPageEnabled"].PreviousValue)
	assert.Equal(t, "false", keys["OriginErrorPageEnabled"].CurrentValue)
	assert.Equal(t, `["500-599"]`, keys["OriginErrorPageStatusCodes"].PreviousValue)
	assert.Equal(t, `["522"]`, keys["OriginErrorPageStatusCodes"].CurrentValue)
	assert.Equal(t, "", keys["OriginErrorPageHTML"].PreviousValue)
	assert.Equal(t, "<p>x</p>", keys["OriginErrorPageHTML"].CurrentValue)
}
