// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package option

import (
	"context"
	"testing"

	"Wavelet/openflare/plugins/server/kernel/model"
	"Wavelet/openflare/plugins/server/kernel/repository"
	"Wavelet/openflare/plugins/server/kernel/testhelper"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOptionTestDB(t *testing.T) func() {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(&model.SystemConfig{}))

	repository.SetDBForTest(sqliteDB)
	repository.SetSystemConfigService(testhelper.NewMockSystemConfigService(sqliteDB))

	// 预填充一些业务配置用于测试
	seedConfigs := []model.SystemConfig{
		{Key: "geoip_provider", Value: "ipinfo", Type: "business", Visibility: 0},
		{Key: "uptime_kuma_password", Value: "secret-pwd", Type: "business", Visibility: 0},
	}
	for _, cfg := range seedConfigs {
		require.NoError(t, sqliteDB.Create(&cfg).Error)
	}

	return func() {
		repository.SetSystemConfigService(nil)
		repository.SetDBForTest(nil)
	}
}

// setTestConfig 设置测试配置的辅助函数
func setTestConfig(t *testing.T, ctx context.Context, key, value string) {
	t.Helper()
	require.NoError(t, repository.DB(ctx).Model(&model.SystemConfig{}).Where("key = ?", key).Update("value", value).Error)
}

func TestListOptionsFiltersSecretKeys(t *testing.T) {
	cleanup := setupOptionTestDB(t)
	defer cleanup()
	ctx := context.Background()

	options, err := listOptions(ctx)
	require.NoError(t, err)

	keys := make(map[string]string, len(options))
	for _, option := range options {
		keys[option.Key] = option.Value
	}

	// geoip_provider 应该出现在列表中
	assert.Equal(t, "ipinfo", keys["geoip_provider"])
	// 敏感配置（密码）应该被过滤掉
	assert.NotContains(t, keys, "uptime_kuma_password")
}

func TestUpdateOptionPersistsToSystemConfig(t *testing.T) {
	cleanup := setupOptionTestDB(t)
	defer cleanup()
	ctx := context.Background()

	err := updateOption(ctx, model.OpenFlareOption{
		Key:   model.ConfigKeyGeoIPProvider,
		Value: "mmdb",
	})
	require.NoError(t, err)

	// 验证配置已写入 SystemConfig
	config, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyGeoIPProvider)
	require.NoError(t, err)
	assert.Equal(t, "mmdb", config.Value)
}

func TestUpdateOpenRestyOptionPersistsToSystemConfig(t *testing.T) {
	cleanup := setupOptionTestDB(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, repository.DB(ctx).Create(&model.SystemConfig{
		Key:        model.ConfigKeyOpenRestyEventsUse,
		Value:      "epoll",
		Type:       "business",
		Visibility: 0,
	}).Error)

	err := updateOption(ctx, model.OpenFlareOption{
		Key:   model.ConfigKeyOpenRestyEventsUse,
		Value: "kqueue",
	})
	require.NoError(t, err)

	config, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyOpenRestyEventsUse)
	require.NoError(t, err)
	assert.Equal(t, "kqueue", config.Value)
}

func TestLookupGeoIPDisabledProvider(t *testing.T) {
	cleanup := setupOptionTestDB(t)
	defer cleanup()
	ctx := context.Background()

	view, err := lookupGeoIP(ctx, "disabled", "8.8.8.8")
	require.NoError(t, err)
	assert.Equal(t, "disabled", view.Provider)
	assert.Equal(t, "8.8.8.8", view.IP)
}
