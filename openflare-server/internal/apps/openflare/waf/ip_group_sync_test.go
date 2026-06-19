// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupIPGroupSyncTestDB(t *testing.T) func() {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(
		&model.OpenFlareWAFRuleGroup{},
		&model.OpenFlareWAFIPGroup{},
		&model.OpenFlareAccessLog{},
	))

	db.SetDB(sqliteDB)
	return func() {
		db.SetDB(nil)
	}
}

func TestParseIPGroupSubscriptionParsers(t *testing.T) {
	textItems, err := parseIPGroupSubscription([]byte("# comment\n203.0.113.10\n\n198.51.100.0/24\n"), "text", "")
	require.NoError(t, err)
	require.Len(t, textItems, 2)
	assert.Equal(t, "198.51.100.0/24", textItems[0])
	assert.Equal(t, "203.0.113.10", textItems[1])

	jsonItems, err := parseIPGroupSubscription([]byte(`{"data":{"items":[{"ip":"203.0.113.11"},{"ip":"203.0.113.12"}]}}`), "json", "data.items[].ip")
	require.NoError(t, err)
	require.Len(t, jsonItems, 2)
	assert.Equal(t, "203.0.113.11", jsonItems[0])
	assert.Equal(t, "203.0.113.12", jsonItems[1])
}

func TestSyncIPGroupDownloadsSubscription(t *testing.T) {
	cleanup := setupIPGroupSyncTestDB(t)
	defer cleanup()
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("203.0.113.20\n"))
	}))
	defer server.Close()

	group, err := CreateIPGroup(ctx, IPGroupInput{
		Name:                "subscription",
		Type:                wafIPGroupTypeSubscription,
		Enabled:             true,
		SubscriptionURL:     server.URL,
		SubscriptionFormat:  wafIPGroupSubscriptionFormatText,
		SyncIntervalMinutes: 10,
	})
	require.NoError(t, err)

	result, err := SyncIPGroup(ctx, group.ID)
	require.NoError(t, err)
	require.Equal(t, 1, result.IPCount)
	assert.Equal(t, "203.0.113.20", result.Group.IPList[0])
	assert.Equal(t, "success", result.Status)
}

func TestSyncIPGroupAutomaticExprRules(t *testing.T) {
	cleanup := setupIPGroupSyncTestDB(t)
	defer cleanup()
	ctx := context.Background()

	now := time.Now().UTC()
	seedWAFAccessLogs(t, ctx, now, "203.0.113.10", "app.example.com", 101, 81)
	seedWAFAccessLogs(t, ctx, now, "203.0.113.11", "198.51.100.10", 60, 0)
	seedWAFAccessLogs(t, ctx, now, "203.0.113.12", "app.example.com", 120, 10)

	group, err := CreateIPGroup(ctx, IPGroupInput{
		Name:    "auto blacklist",
		Type:    wafIPGroupTypeAutomatic,
		Enabled: true,
		AutoConfig: json.RawMessage(`{
			"lookback_minutes": 60,
			"rules": [
				{"name":"单 IP 404 高频扫描","expr":"request_count > 100 && StatusRatio(404) >= 0.8"},
				{"name":"单 IP 直连访问异常","expr":"ip_host_count > 50 && ip_host_ratio > 0.5"}
			]
		}`),
	})
	require.NoError(t, err)

	result, err := SyncIPGroup(ctx, group.ID)
	require.NoError(t, err)
	require.Equal(t, 2, result.IPCount)

	want := map[string]bool{"203.0.113.10": true, "203.0.113.11": true}
	for _, item := range result.Group.IPList {
		assert.True(t, want[item], "unexpected matched IP %s", item)
		delete(want, item)
	}
	assert.Empty(t, want)
}

func TestTestIPGroupAutoConfigReturnsMatchedIPs(t *testing.T) {
	cleanup := setupIPGroupSyncTestDB(t)
	defer cleanup()
	ctx := context.Background()

	now := time.Now().UTC()
	seedWAFAccessLogs(t, ctx, now, "203.0.113.10", "app.example.com", 101, 81)
	seedWAFAccessLogs(t, ctx, now, "203.0.113.11", "198.51.100.10", 60, 0)
	seedWAFAccessLogs(t, ctx, now, "203.0.113.12", "app.example.com", 120, 10)

	result, err := TestIPGroupAutoConfig(ctx, IPGroupAutoTestInput{
		AutoConfig: json.RawMessage(`{
			"lookback_minutes": 60,
			"rules": [
				{"name":"单 IP 404 高频扫描","expr":"request_count > 100 && StatusRatio(404) >= 0.8"},
				{"name":"单 IP 直连访问异常","expr":"ip_host_count > 50 && ip_host_ratio > 0.5"}
			]
		}`),
	})
	require.NoError(t, err)
	assert.Equal(t, 2, result.MatchedCount)
	assert.Equal(t, 2, result.RuleCount)
	assert.Equal(t, 60, result.LookbackMinutes)

	want := map[string]bool{"203.0.113.10": true, "203.0.113.11": true}
	for _, item := range result.MatchedIPs {
		assert.True(t, want[item], "unexpected matched IP %s", item)
		delete(want, item)
	}
	assert.Empty(t, want)
}

func TestListDueOpenFlareWAFIPGroups(t *testing.T) {
	cleanup := setupIPGroupSyncTestDB(t)
	defer cleanup()
	ctx := context.Background()

	past := time.Now().UTC().Add(-time.Hour)
	future := time.Now().UTC().Add(time.Hour)

	dueAuto := &model.OpenFlareWAFIPGroup{
		Name: "due auto", Type: wafIPGroupTypeAutomatic, Enabled: true,
		IPList: "[]", AutoConfig: "{}", ExtIPs: "[]", NextSyncAt: &past,
	}
	require.NoError(t, model.CreateOpenFlareWAFIPGroup(ctx, dueAuto))

	futureAuto := &model.OpenFlareWAFIPGroup{
		Name: "future auto", Type: wafIPGroupTypeAutomatic, Enabled: true,
		IPList: "[]", AutoConfig: "{}", ExtIPs: "[]", NextSyncAt: &future,
	}
	require.NoError(t, model.CreateOpenFlareWAFIPGroup(ctx, futureAuto))

	dueSub := &model.OpenFlareWAFIPGroup{
		Name: "due sub", Type: wafIPGroupTypeSubscription, Enabled: true,
		IPList: "[]", AutoConfig: "{}", ExtIPs: "[]",
		SubscriptionURL: "https://example.com/list", NextSyncAt: &past,
	}
	require.NoError(t, model.CreateOpenFlareWAFIPGroup(ctx, dueSub))

	manual := &model.OpenFlareWAFIPGroup{
		Name: "manual", Type: wafIPGroupTypeManual, Enabled: true,
		IPList: "[]", AutoConfig: "{}", ExtIPs: "[]", NextSyncAt: &past,
	}
	require.NoError(t, model.CreateOpenFlareWAFIPGroup(ctx, manual))

	groups, err := model.ListDueOpenFlareWAFIPGroups(ctx, time.Now().UTC())
	require.NoError(t, err)
	require.Len(t, groups, 2)
	ids := []uint{groups[0].ID, groups[1].ID}
	assert.Contains(t, ids, dueAuto.ID)
	assert.Contains(t, ids, dueSub.ID)
}

func seedWAFAccessLogs(t *testing.T, ctx context.Context, loggedAt time.Time, remoteAddr string, host string, total int, notFound int) {
	t.Helper()
	for i := 0; i < total; i++ {
		statusCode := http.StatusOK
		if i < notFound {
			statusCode = http.StatusNotFound
		}
		require.NoError(t, db.DB(ctx).Create(&model.OpenFlareAccessLog{
			NodeID:     "node-waf-auto",
			LoggedAt:   loggedAt.Add(-time.Duration(i%30) * time.Second),
			RemoteAddr: remoteAddr,
			Host:       host,
			Path:       "/probe",
			StatusCode: statusCode,
		}).Error)
	}
}
