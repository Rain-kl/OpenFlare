// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package uptimekuma

import (
	"context"
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
