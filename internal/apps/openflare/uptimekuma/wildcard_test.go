// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package uptimekuma

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

func TestRouteMonitorURLWildcard(t *testing.T) {
	t.Cleanup(setupSyncTestDB(t))
	ctx := context.Background()
	route := &model.ProxyRoute{SiteName: "wildcard-site", EnableHTTPS: true}
	if err := repository.CreateProxyRouteRecord(ctx, route); err != nil {
		t.Fatal(err)
	}
	domain := &model.ZoneDomain{Domain: "*.example.com", ProxyRouteID: &route.ID}
	if err := repository.CreateZoneDomain(ctx, domain); err != nil {
		t.Fatal(err)
	}
	if got, err := routeMonitorURL(ctx, route); got != "" || err == nil || !strings.Contains(err.Error(), "wildcard") {
		t.Fatalf("routeMonitorURL(wildcard only) = (%q, %v), want empty URL and wildcard error", got, err)
	}
	// 无可探测地址时不能调用客户端，也不能将已有监控标记为过期。
	expected := syncRouteMonitors(ctx, nil, []*model.ProxyRoute{route}, nil, 0, nil)
	if !expected[route.SiteName] {
		t.Error("syncRouteMonitors(wildcard only) removed route from expected sites")
	}
	if err := repository.CreateZoneDomain(ctx, &model.ZoneDomain{Domain: "www.example.com", ProxyRouteID: &route.ID}); err != nil {
		t.Fatal(err)
	}
	if got, err := routeMonitorURL(ctx, route); err != nil || got != "https://www.example.com" {
		t.Errorf("routeMonitorURL(mixed HTTPS) = (%q, %v), want https://www.example.com", got, err)
	}
	route.EnableHTTPS = false
	if got, err := routeMonitorURL(ctx, route); err != nil || got != "http://www.example.com" {
		t.Errorf("routeMonitorURL(mixed HTTP) = (%q, %v), want http://www.example.com", got, err)
	}
}

// TestSyncWildcardPreservesExistingMonitor exercises cleanup through a full sync.
func TestSyncWildcardPreservesExistingMonitor(t *testing.T) {
	t.Cleanup(setupSyncTestDB(t))
	ctx := context.Background()
	route := &model.ProxyRoute{SiteName: "wildcard-site", Enabled: true}
	if err := repository.CreateProxyRouteRecord(ctx, route); err != nil {
		t.Fatal(err)
	}
	createRouteZoneDomain(t, ctx, route, "*.example.com")
	mockSrv := newMockKumaServer(`{"98":{"id":98,"name":"wildcard-site","url":"https://old.example.com","tags":[{"id":10,"name":"OpenFlare"}]},"99":{"id":99,"name":"stale-site","tags":[{"id":10,"name":"OpenFlare"}]}}`)
	server := httptest.NewServer(mockSrv)
	t.Cleanup(server.Close)
	setTestConfig(ctx, model.ConfigKeyUptimeKumaEnabled, "true")
	setTestConfig(ctx, model.ConfigKeyUptimeKumaURL, server.URL)
	setTestConfig(ctx, model.ConfigKeyUptimeKumaUsername, "admin")
	setTestConfig(ctx, model.ConfigKeyUptimeKumaPassword, "password")
	setTestConfig(ctx, model.ConfigKeyUptimeKumaMonitorScope, "all")
	if err := SyncToUptimeKuma(ctx); err != nil {
		t.Fatal(err)
	}
	mockSrv.mu.Lock()
	posts := append([]string(nil), mockSrv.postsReceived...)
	mockSrv.mu.Unlock()
	deletedStale := false
	for _, body := range posts {
		if strings.Contains(body, `"editMonitor"`) || strings.Contains(body, `"add"`) || (strings.Contains(body, `"deleteMonitor"`) && strings.Contains(body, "98")) {
			t.Errorf("SyncToUptimeKuma(wildcard only) sent monitor mutation %q", body)
		}
		if strings.Contains(body, `"deleteMonitor"`) && strings.Contains(body, "99") {
			deletedStale = true
		}
	}
	if !deletedStale {
		t.Error("SyncToUptimeKuma did not delete stale monitor 99; cleanup must run while preserving wildcard monitor 98")
	}
}
