// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"encoding/json"
	"testing"

	"Wavelet/openflare/plugins/server/kernel/model"
	"Wavelet/openflare/plugins/server/kernel/repository"
	"Wavelet/openflare/plugins/server/kernel/testhelper"
	"Wavelet/pkg/cache/ram"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupOriginErrorPageSnapshotDB(t *testing.T) func() {
	t.Helper()

	// repository 读配置会写进程级 RAM 缓存（跨测试存活），换 DB 前后必须
	// 重置，否则 shuffle 下先跑的用例会污染后跑的用例。
	ram.ResetForTest()

	_, _, cleanup := testhelper.SetupTestEnvironment(t)

	return func() {
		cleanup()
		ram.ResetForTest()
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

	require.NoError(t, repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyOriginErrorPageEnabled, "false"))
	require.NoError(t, repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyOriginErrorPageStatusCodes, `["522","500-502"]`))
	require.NoError(t, repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyOriginErrorPageHTML, "<h1>{{status}}</h1>"))

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
	assert.Empty(t, keys["OriginErrorPageHTML"].PreviousValue)
	assert.Equal(t, "<p>x</p>", keys["OriginErrorPageHTML"].CurrentValue)
}

func TestDiffOpenRestyOptionDetailsSWOfflineDomains(t *testing.T) {
	t.Parallel()

	left := openRestyConfigSnapshot{
		SWOfflineDomains: []string{"a.com,b.com"},
	}
	right := openRestyConfigSnapshot{
		SWOfflineDomains: []string{"a.com", "b.com"},
	}
	details := diffOpenRestyOptionDetails(left, right)
	keys := make(map[string]ConfigOptionDiffItem, len(details))
	for _, item := range details {
		keys[item.Key] = item
	}
	assert.Equal(t, `["a.com,b.com"]`, keys["SWOfflineDomains"].PreviousValue)
	assert.Equal(t, `["a.com","b.com"]`, keys["SWOfflineDomains"].CurrentValue)
}
