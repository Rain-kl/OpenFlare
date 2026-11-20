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
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/stretchr/testify/require"
)

func TestTrustedProxyCIDRsSurviveSnapshotEncoding(t *testing.T) {
	snapshot := snapshotDocument{
		EdgeSettings: edgeSettingsSnapshot{TrustedProxyCIDRs: parseTrustedProxyCIDRs(`["173.245.48.0/20","2001:db8:1234::/48"]`)},
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var document edgeconfig.Document
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(document.Settings.TrustedProxyCIDRs, snapshot.EdgeSettings.TrustedProxyCIDRs) {
		t.Fatalf("trusted proxies changed in snapshot: %v", document.Settings.TrustedProxyCIDRs)
	}
}

func TestTrustedProxyCIDRConfigMissingAndExplicitEmpty(t *testing.T) {
	cleanup := setupOriginErrorPageSnapshotDB(t)
	defer cleanup()
	ctx := context.Background()

	snapshot, err := buildEdgeSettingsSnapshot(ctx)
	require.NoError(t, err)
	missing := snapshot.TrustedProxyCIDRs
	if len(missing) != 22 || !slices.Equal(missing, edgeconfig.DefaultTrustedProxyCIDRs()) {
		t.Fatalf("missing database key defaults to %#v, want the 22 Cloudflare ranges", missing)
	}

	if err := db.DB(ctx).Create(&model.SystemConfig{Key: model.ConfigKeyEdgeTrustedProxyCIDRs, Value: `[]`, Type: "business"}).Error; err != nil {
		t.Fatal(err)
	}
	snapshot, err = buildEdgeSettingsSnapshot(ctx)
	require.NoError(t, err)
	explicitEmpty := snapshot.TrustedProxyCIDRs
	if explicitEmpty == nil || len(explicitEmpty) != 0 {
		t.Fatalf("explicit [] must remain a non-nil empty list, got %#v", explicitEmpty)
	}
	payload, err := json.Marshal(snapshotDocument{EdgeSettings: snapshot})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"trusted_proxy_cidrs":[]`) {
		t.Fatalf("explicit empty list was omitted from snapshot: %s", payload)
	}

	if err := db.DB(ctx).Model(&model.SystemConfig{}).Where("key = ?", model.ConfigKeyEdgeTrustedProxyCIDRs).Update("value", "invalid").Error; err != nil {
		t.Fatal(err)
	}
	snapshot, err = buildEdgeSettingsSnapshot(ctx)
	require.NoError(t, err)
	malformed := snapshot.TrustedProxyCIDRs
	if malformed == nil || len(malformed) != 0 {
		t.Fatalf("malformed configuration must fail closed, got %#v", malformed)
	}
}
