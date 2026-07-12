// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package uptimekuma

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type mockKumaServer struct {
	mu             sync.Mutex
	postsReceived  []string
	pendingPackets chan string
	monitorList    string
}

func newMockKumaServer(monitorList string) *mockKumaServer {
	return &mockKumaServer{
		pendingPackets: make(chan string, 100),
		monitorList:    monitorList,
	}
}

func (s *mockKumaServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	transport := r.URL.Query().Get("transport")
	sid := r.URL.Query().Get("sid")

	if r.Method == http.MethodGet {
		if transport == "polling" && sid == "" {
			w.Header().Set("Content-Type", "text/plain;charset=UTF-8")
			_, _ = w.Write([]byte(`0{"sid":"mock-sid"}`))
			return
		}

		if transport == "polling" && sid == "mock-sid" {
			w.Header().Set("Content-Type", "text/plain;charset=UTF-8")
			select {
			case pkt := <-s.pendingPackets:
				_, _ = w.Write([]byte(pkt))
			case <-time.After(100 * time.Millisecond):
				_, _ = w.Write([]byte(""))
			}
			return
		}
	} else if r.Method == http.MethodPost {
		bodyBytes, _ := io.ReadAll(r.Body)
		bodyStr := string(bodyBytes)
		s.postsReceived = append(s.postsReceived, bodyStr)

		w.Header().Set("Content-Type", "text/plain;charset=UTF-8")
		w.WriteHeader(http.StatusOK)

		if bodyStr == "40" {
			s.pendingPackets <- fmt.Sprintf(`42["monitorList",%s]`, s.monitorList)
			return
		}

		if strings.HasPrefix(bodyStr, "42") {
			payload := bodyStr[2:]
			digitsEnd := 0
			for digitsEnd < len(payload) && payload[digitsEnd] >= '0' && payload[digitsEnd] <= '9' {
				digitsEnd++
			}
			if digitsEnd == 0 {
				return
			}
			ackIDStr := payload[:digitsEnd]
			jsonArrayStr := payload[digitsEnd:]

			var arr []json.RawMessage
			if err := json.Unmarshal([]byte(jsonArrayStr), &arr); err != nil || len(arr) == 0 {
				return
			}

			var eventName string
			_ = json.Unmarshal(arr[0], &eventName)

			switch eventName {
			case "login", "loginByToken":
				s.pendingPackets <- fmt.Sprintf("43%s[{\"ok\":true}]", ackIDStr)
			case "getTags":
				s.pendingPackets <- fmt.Sprintf("43%s[{\"ok\":true,\"tags\":[{\"id\":10,\"name\":\"OpenFlare\",\"color\":\"#4f46e5\"}]}]", ackIDStr)
			case "addTag":
				s.pendingPackets <- fmt.Sprintf("43%s[{\"ok\":true,\"tag\":{\"id\":10}}]", ackIDStr)
			case "add":
				s.pendingPackets <- fmt.Sprintf("43%s[{\"ok\":true,\"monitorID\":100}]", ackIDStr)
			case "addMonitorTag":
				s.pendingPackets <- fmt.Sprintf("43%s[{\"ok\":true}]", ackIDStr)
			case "editMonitor":
				s.pendingPackets <- fmt.Sprintf("43%s[{\"ok\":true}]", ackIDStr)
			case "deleteMonitor":
				s.pendingPackets <- fmt.Sprintf("43%s[{\"ok\":true}]", ackIDStr)
			}
		}
	}
}

func setupSyncTestDB(t *testing.T) func() {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(&model.ProxyRoute{}, &model.Zone{}, &model.ZoneDomain{}, &model.SystemConfig{}))

	db.SetDB(sqliteDB)
	return func() {
		db.SetDB(nil)
	}
}

func createRouteZoneDomain(t *testing.T, ctx context.Context, route *model.ProxyRoute, domain string) {
	t.Helper()
	zone := &model.Zone{Domain: fmt.Sprintf("zone-%d.example", route.ID)}
	require.NoError(t, db.DB(ctx).Create(zone).Error)
	require.NoError(t, db.DB(ctx).Create(&model.ZoneDomain{
		ZoneID:       zone.ID,
		ProxyRouteID: &route.ID,
		Domain:       domain,
	}).Error)
}

func backupUptimeKumaConfig(ctx context.Context) func() {
	// 备份所有 UptimeKuma 相关配置
	configs := []string{
		model.ConfigKeyUptimeKumaEnabled,
		model.ConfigKeyUptimeKumaURL,
		model.ConfigKeyUptimeKumaUsername,
		model.ConfigKeyUptimeKumaPassword,
		model.ConfigKeyUptimeKumaMonitorScope,
		model.ConfigKeyUptimeKumaSelectedSites,
		model.ConfigKeyUptimeKumaInterval,
		model.ConfigKeyUptimeKumaRetry,
		model.ConfigKeyUptimeKumaRetryInterval,
		model.ConfigKeyUptimeKumaTimeout,
	}

	oldValues := make(map[string]string)
	for _, key := range configs {
		config, _ := repository.GetSystemConfigByKey(ctx, key)
		oldValues[key] = config.Value
	}

	return func() {
		// 恢复所有配置
		for key, value := range oldValues {
			_ = db.DB(ctx).Model(&model.SystemConfig{}).Where("key = ?", key).Update("value", value).Error
		}
	}
}

// setTestConfig 设置测试配置的辅助函数（不存在则创建）
func setTestConfig(ctx context.Context, key, value string) {
	_ = repository.SaveOrUpdateSystemConfig(ctx, key, value)
}

func TestSyncToUptimeKumaDisabled(t *testing.T) {
	cleanup := setupSyncTestDB(t)
	defer cleanup()
	ctx := context.Background()
	restore := backupUptimeKumaConfig(ctx)
	defer restore()

	setTestConfig(ctx, model.ConfigKeyUptimeKumaEnabled, "false")

	err := SyncToUptimeKuma(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestSyncToUptimeKumaSuccess(t *testing.T) {
	cleanup := setupSyncTestDB(t)
	defer cleanup()
	ctx := context.Background()
	restore := backupUptimeKumaConfig(ctx)
	defer restore()

	require.NoError(t, db.DB(ctx).Where("1 = 1").Delete(&model.ProxyRoute{}).Error)

	routeA := &model.ProxyRoute{
		SiteName:    "site-a",
		Domain:      "site-a.com",
		Domains:     `["site-a.com"]`,
		OriginURL:   "http://10.0.0.1",
		Enabled:     true,
		EnableHTTPS: false,
	}
	routeB := &model.ProxyRoute{
		SiteName:    "site-b",
		Domain:      "site-b.com",
		Domains:     `["site-b.com"]`,
		OriginURL:   "https://10.0.0.2",
		Enabled:     true,
		EnableHTTPS: true,
	}
	routeC := &model.ProxyRoute{
		SiteName:    "site-c",
		Domain:      "site-c.com",
		Domains:     `["site-c.com"]`,
		OriginURL:   "http://10.0.0.3",
		Enabled:     false,
		EnableHTTPS: false,
	}

	require.NoError(t, model.CreateProxyRouteRecord(ctx, routeA))
	require.NoError(t, model.CreateProxyRouteRecord(ctx, routeB))
	require.NoError(t, model.CreateProxyRouteRecord(ctx, routeC))
	createRouteZoneDomain(t, ctx, routeA, "site-a.com")
	createRouteZoneDomain(t, ctx, routeB, "site-b.com")
	createRouteZoneDomain(t, ctx, routeC, "site-c.com")

	monitorListJSON := `{
		"99": {
			"id": 99,
			"name": "site-old",
			"url": "http://site-old.com",
			"interval": 60,
			"tags": [{"tag_id": 10, "name": "OpenFlare"}]
		},
		"98": {
			"id": 98,
			"name": "site-a",
			"url": "http://site-a.com",
			"interval": 30,
			"tags": [{"tag_id": 10, "name": "OpenFlare"}]
		}
	}`

	mockSrv := newMockKumaServer(monitorListJSON)
	server := httptest.NewServer(mockSrv)
	defer server.Close()

	// 设置测试配置
	setTestConfig(ctx, model.ConfigKeyUptimeKumaEnabled, "true")
	setTestConfig(ctx, model.ConfigKeyUptimeKumaURL, server.URL)
	setTestConfig(ctx, model.ConfigKeyUptimeKumaUsername, "admin")
	setTestConfig(ctx, model.ConfigKeyUptimeKumaPassword, "password")
	setTestConfig(ctx, model.ConfigKeyUptimeKumaMonitorScope, "all")
	setTestConfig(ctx, model.ConfigKeyUptimeKumaInterval, "60")
	setTestConfig(ctx, model.ConfigKeyUptimeKumaRetry, "0")
	setTestConfig(ctx, model.ConfigKeyUptimeKumaRetryInterval, "60")
	setTestConfig(ctx, model.ConfigKeyUptimeKumaTimeout, "48")

	require.NoError(t, SyncToUptimeKuma(ctx))

	mockSrv.mu.Lock()
	posts := mockSrv.postsReceived
	mockSrv.mu.Unlock()

	hasLogin := false
	hasGetTags := false
	hasAddSiteB := false
	hasTagSiteB := false
	hasEditSiteA := false
	hasDeleteOld := false

	for _, body := range posts {
		if strings.Contains(body, `"login"`) && strings.Contains(body, `"admin"`) && strings.Contains(body, `"password"`) {
			hasLogin = true
		}
		if strings.Contains(body, `"getTags"`) {
			hasGetTags = true
		}
		if strings.Contains(body, `"add"`) && strings.Contains(body, `"site-b"`) && strings.Contains(body, `"https://site-b.com"`) {
			hasAddSiteB = true
		}
		if strings.Contains(body, `"addMonitorTag"`) && strings.Contains(body, `10`) && strings.Contains(body, `100`) {
			hasTagSiteB = true
		}
		if strings.Contains(body, `"editMonitor"`) && strings.Contains(body, `98`) && strings.Contains(body, `"site-a"`) && strings.Contains(body, `"interval":60`) {
			hasEditSiteA = true
		}
		if strings.Contains(body, `"deleteMonitor"`) && strings.Contains(body, `99`) {
			hasDeleteOld = true
		}
	}

	assert.True(t, hasLogin, "expected login event to be called")
	assert.True(t, hasGetTags, "expected getTags event to be called")
	assert.True(t, hasAddSiteB, "expected site-b to be added")
	assert.True(t, hasTagSiteB, "expected site-b to be tagged")
	assert.True(t, hasEditSiteA, "expected site-a to be edited/updated")
	assert.True(t, hasDeleteOld, "expected site-old to be deleted")
}

func TestSyncToUptimeKumaSelectedScope(t *testing.T) {
	cleanup := setupSyncTestDB(t)
	defer cleanup()
	ctx := context.Background()
	restore := backupUptimeKumaConfig(ctx)
	defer restore()

	require.NoError(t, db.DB(ctx).Where("1 = 1").Delete(&model.ProxyRoute{}).Error)

	routeA := &model.ProxyRoute{
		SiteName:    "site-a",
		Domain:      "site-a.com",
		Domains:     `["site-a.com"]`,
		OriginURL:   "http://10.0.0.1",
		Enabled:     true,
		EnableHTTPS: false,
	}
	routeB := &model.ProxyRoute{
		SiteName:    "site-b",
		Domain:      "site-b.com",
		Domains:     `["site-b.com"]`,
		OriginURL:   "http://10.0.0.2",
		Enabled:     true,
		EnableHTTPS: false,
	}

	require.NoError(t, model.CreateProxyRouteRecord(ctx, routeA))
	require.NoError(t, model.CreateProxyRouteRecord(ctx, routeB))
	createRouteZoneDomain(t, ctx, routeA, "site-a.com")
	createRouteZoneDomain(t, ctx, routeB, "site-b.com")

	mockSrv := newMockKumaServer(`{}`)
	server := httptest.NewServer(mockSrv)
	defer server.Close()

	// 设置测试配置
	setTestConfig(ctx, model.ConfigKeyUptimeKumaEnabled, "true")
	setTestConfig(ctx, model.ConfigKeyUptimeKumaURL, server.URL)
	setTestConfig(ctx, model.ConfigKeyUptimeKumaUsername, "admin")
	setTestConfig(ctx, model.ConfigKeyUptimeKumaPassword, "password")
	setTestConfig(ctx, model.ConfigKeyUptimeKumaMonitorScope, "selected")
	setTestConfig(ctx, model.ConfigKeyUptimeKumaSelectedSites, "site-a")

	require.NoError(t, SyncToUptimeKuma(ctx))

	mockSrv.mu.Lock()
	posts := mockSrv.postsReceived
	mockSrv.mu.Unlock()

	hasLogin := false
	hasAddSiteA := false
	hasAddSiteB := false

	for _, body := range posts {
		if strings.Contains(body, `"login"`) && strings.Contains(body, `"admin"`) && strings.Contains(body, `"password"`) {
			hasLogin = true
		}
		if strings.Contains(body, `"add"`) && strings.Contains(body, `"site-a"`) {
			hasAddSiteA = true
		}
		if strings.Contains(body, `"add"`) && strings.Contains(body, `"site-b"`) {
			hasAddSiteB = true
		}
	}

	assert.True(t, hasLogin, "expected login event to be called")
	assert.True(t, hasAddSiteA, "expected site-a to be added")
	assert.False(t, hasAddSiteB, "expected site-b NOT to be added (not in selected scope)")
}
