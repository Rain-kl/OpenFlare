// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

func TestIndependentPoWPolicySnapshot(t *testing.T) {
	t.Cleanup(setupConfigVersionTestDB(t))
	ctx := context.Background()
	route := &model.ProxyRoute{SiteName: "standalone", OriginURL: "http://127.0.0.1:8080", Upstreams: `["http://127.0.0.1:8080"]`, Enabled: true}
	if err := repository.CreateProxyRouteRecord(ctx, route); err != nil {
		t.Fatal(err)
	}
	createSnapshotZoneDomains(t, ctx, route, "pow.example.com")
	config := edgeconfig.DefaultPoWConfig()
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveOpenFlarePoWPolicy(ctx, model.OpenFlarePoWPolicy{RouteID: route.ID, Enabled: true, Config: string(raw)}); err != nil {
		t.Fatal(err)
	}
	bundle, err := buildCurrentConfigBundle(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.SnapshotRoutes) != 1 || !bundle.SnapshotRoutes[0].PoWEnabled || bundle.SnapshotRoutes[0].PoWConfig == nil {
		t.Fatalf("buildCurrentConfigBundle(independent PoW) routes = %+v, want published standalone policy", bundle.SnapshotRoutes)
	}
	if !strings.Contains(bundle.RouteConfig, `"pow_enabled":true`) {
		t.Error("buildCurrentConfigBundle(independent PoW) omitted challenge endpoints")
	}
	left := bundle.SnapshotRoutes[0]
	right := left
	changedConfig := *left.PoWConfig
	changedConfig.PathRules = &edgeconfig.PoWPathRules{Mode: "blacklist", Blacklist: []edgeconfig.PoWPathMatch{{Type: "exact", Value: "/login"}}}
	right.PoWConfig = &changedConfig
	if snapshotRouteConfigEqual(left, right) {
		t.Error("snapshotRouteConfigEqual(changed PoW paths) = true, want false")
	}
	right = left
	right.PoWEnabled = false
	if snapshotRouteConfigEqual(left, right) {
		t.Error("snapshotRouteConfigEqual(disabled PoW) = true, want false")
	}
}
