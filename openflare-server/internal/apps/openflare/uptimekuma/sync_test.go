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
	require.NoError(t, sqliteDB.AutoMigrate(&model.ProxyRoute{}))

	db.SetDB(sqliteDB)
	return func() {
		db.SetDB(nil)
	}
}

func backupUptimeKumaConfig() func() {
	oldEnabled := model.UptimeKumaEnabled
	oldURL := model.UptimeKumaUrl
	oldUsername := model.UptimeKumaUsername
	oldPassword := model.UptimeKumaPassword
	oldScope := model.UptimeKumaMonitorScope
	oldSelected := model.UptimeKumaSelectedSites
	oldInterval := model.UptimeKumaInterval
	oldRetry := model.UptimeKumaRetry
	oldRetryInterval := model.UptimeKumaRetryInterval
	oldTimeout := model.UptimeKumaTimeout

	return func() {
		model.UptimeKumaEnabled = oldEnabled
		model.UptimeKumaUrl = oldURL
		model.UptimeKumaUsername = oldUsername
		model.UptimeKumaPassword = oldPassword
		model.UptimeKumaMonitorScope = oldScope
		model.UptimeKumaSelectedSites = oldSelected
		model.UptimeKumaInterval = oldInterval
		model.UptimeKumaRetry = oldRetry
		model.UptimeKumaRetryInterval = oldRetryInterval
		model.UptimeKumaTimeout = oldTimeout
	}
}

func TestSyncToUptimeKumaDisabled(t *testing.T) {
	cleanup := setupSyncTestDB(t)
	defer cleanup()
	restore := backupUptimeKumaConfig()
	defer restore()

	model.UptimeKumaEnabled = false

	err := SyncToUptimeKuma(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestSyncToUptimeKumaSuccess(t *testing.T) {
	cleanup := setupSyncTestDB(t)
	defer cleanup()
	restore := backupUptimeKumaConfig()
	defer restore()
	ctx := context.Background()

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

	model.UptimeKumaEnabled = true
	model.UptimeKumaUrl = server.URL
	model.UptimeKumaUsername = "admin"
	model.UptimeKumaPassword = "password"
	model.UptimeKumaMonitorScope = "all"
	model.UptimeKumaInterval = 60
	model.UptimeKumaRetry = 0
	model.UptimeKumaRetryInterval = 60
	model.UptimeKumaTimeout = 48

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
	restore := backupUptimeKumaConfig()
	defer restore()
	ctx := context.Background()

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

	mockSrv := newMockKumaServer(`{}`)
	server := httptest.NewServer(mockSrv)
	defer server.Close()

	model.UptimeKumaEnabled = true
	model.UptimeKumaUrl = server.URL
	model.UptimeKumaUsername = "admin"
	model.UptimeKumaPassword = "password"
	model.UptimeKumaMonitorScope = "selected"
	model.UptimeKumaSelectedSites = "site-a"

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
