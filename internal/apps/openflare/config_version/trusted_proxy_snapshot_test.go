// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/cache/ram"
	openrestyrender "github.com/Rain-kl/Wavelet/pkg/render/openresty"
)

func TestTrustedProxyCIDRsSurviveSnapshotRendering(t *testing.T) {
	snapshot := snapshotDocument{
		OpenRestyConfig: openRestyConfigSnapshot{
			MainConfigTemplate: model.DefaultOpenRestyMainConfigTemplate,
			TrustedProxyCIDRs:  parseTrustedProxyCIDRs(`["173.245.48.0/20","2001:db8:1234::/48"]`),
		},
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := renderSnapshotConfig(string(data), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"real_ip_header CF-Connecting-IP;", "set_real_ip_from 173.245.48.0/20;", "set_real_ip_from 2001:db8:1234::/48;"} {
		if !strings.Contains(rendered.MainConfig, expected) {
			t.Errorf("rendered snapshot missing %q", expected)
		}
	}
}

func TestTrustedProxyCIDRsSnapshotEmptyAndLegacyDefaults(t *testing.T) {
	for _, testCase := range []struct {
		name string
		json string
		want int
	}{
		{name: "explicit empty list remains disabled", json: `{"openresty_config":{"trusted_proxy_cidrs":[]}}`, want: 0},
		{name: "legacy snapshot defaults to Cloudflare ranges", json: `{"openresty_config":{}}`, want: 22},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			rendered, err := renderSnapshotConfig(testCase.json, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Count(rendered.MainConfig, "set_real_ip_from "); got != testCase.want {
				t.Fatalf("rendered trusted proxy range count = %d, want %d", got, testCase.want)
			}
		})
	}
}

func TestTrustedProxyCIDRConfigMissingAndExplicitEmpty(t *testing.T) {
	cleanup := setupOriginErrorPageSnapshotDB(t)
	defer cleanup()
	ctx := context.Background()

	missing := buildOpenRestyConfigSnapshot(ctx).TrustedProxyCIDRs
	if len(missing) != 22 || !slices.Equal(missing, openrestyrender.DefaultTrustedProxyCIDRs()) {
		t.Fatalf("missing database key defaults to %#v, want the 22 Cloudflare ranges", missing)
	}

	if err := db.DB(ctx).Create(&model.SystemConfig{Key: model.ConfigKeyOpenRestyTrustedProxyCIDRs, Value: `[]`, Type: "business"}).Error; err != nil {
		t.Fatal(err)
	}
	explicitEmpty := buildOpenRestyConfigSnapshot(ctx).TrustedProxyCIDRs
	if explicitEmpty == nil || len(explicitEmpty) != 0 {
		t.Fatalf("explicit [] must remain a non-nil empty list, got %#v", explicitEmpty)
	}
	payload, err := json.Marshal(snapshotDocument{OpenRestyConfig: buildOpenRestyConfigSnapshot(ctx)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"trusted_proxy_cidrs":[]`) {
		t.Fatalf("explicit empty list was omitted from snapshot: %s", payload)
	}
	rendered, err := renderSnapshotConfig(string(payload), nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rendered.MainConfig, "set_real_ip_from ") {
		t.Fatal("explicit empty list still trusts proxy ranges")
	}

	if err := db.DB(ctx).Model(&model.SystemConfig{}).Where("key = ?", model.ConfigKeyOpenRestyTrustedProxyCIDRs).Update("value", "invalid").Error; err != nil {
		t.Fatal(err)
	}
	ram.ResetForTest()
	malformed := buildOpenRestyConfigSnapshot(ctx).TrustedProxyCIDRs
	if malformed == nil || len(malformed) != 0 {
		t.Fatalf("malformed configuration must fail closed, got %#v", malformed)
	}
}

func TestTrustedProxyCIDRDiffTreatsLegacyNilAsCloudflareDefaults(t *testing.T) {
	diffs := diffOpenRestyOptionDetails(openRestyConfigSnapshot{}, openRestyConfigSnapshot{TrustedProxyCIDRs: []string{}})
	for _, diff := range diffs {
		if diff.Key == "OpenRestyTrustedProxyCIDRs" {
			if diff.PreviousValue == diff.CurrentValue {
				t.Fatal("legacy nil and explicit empty list should have different effective values")
			}
			return
		}
	}
	t.Fatal("trusted proxy CIDR change was not included in config diff")
}
