// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"encoding/json"
	"testing"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/cache/ram"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOriginErrorPageSnapshotDB(t *testing.T) func() {
	t.Helper()

	// repository 读配置会写进程级 RAM 缓存（跨测试存活），换 DB 前后必须
	// 重置，否则 shuffle 下先跑的用例会污染后跑的用例。
	ram.ResetForTest()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(&model.SystemConfig{}))
	db.SetDB(sqliteDB)

	return func() {
		db.SetDB(nil)
		ram.ResetForTest()
	}
}

func TestBuildEdgeSettingsSnapshotOriginErrorPageDefaults(t *testing.T) {
	cleanup := setupOriginErrorPageSnapshotDB(t)
	defer cleanup()

	snapshot, err := buildEdgeSettingsSnapshot(context.Background())
	require.NoError(t, err)
	assert.True(t, snapshot.OriginErrorPageEnabled)
	assert.Equal(t, []string{"500-599"}, snapshot.OriginErrorPageStatusCodes)
	assert.Empty(t, snapshot.OriginErrorPageHTML)

	payload, err := json.Marshal(snapshot)
	require.NoError(t, err)
	assert.Contains(t, string(payload), `"origin_error_page_enabled":true`)
	assert.Contains(t, string(payload), `"origin_error_page_status_codes":["500-599"]`)
}

func TestBuildEdgeSettingsSnapshotOriginErrorPageCustom(t *testing.T) {
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

	snapshot, err := buildEdgeSettingsSnapshot(ctx)
	require.NoError(t, err)
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
