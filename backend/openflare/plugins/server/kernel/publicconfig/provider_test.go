// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package publicconfig

import (
	"context"
	"testing"

	"Wavelet/openflare/plugins/server/kernel/model"
	"Wavelet/openflare/plugins/server/kernel/repository"
	"Wavelet/openflare/plugins/server/kernel/testhelper"
)

func TestPublicConfigSeesSaveOrUpdateThroughAdminCache(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	provider := New(nil)

	first, err := provider.PublicConfig(ctx)
	if err != nil {
		t.Fatalf("PublicConfig() warm error = %v", err)
	}
	if got := first[model.ConfigKeySiteName]; got != "OpenFlare" {
		t.Fatalf("PublicConfig()[%q] = %q, want %q", model.ConfigKeySiteName, got, "OpenFlare")
	}

	if err := repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeySiteName, "Updated"); err != nil {
		t.Fatalf("SaveOrUpdateSystemConfig(%q) error = %v", model.ConfigKeySiteName, err)
	}

	got, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeySiteName)
	if err != nil {
		t.Fatalf("GetSystemConfigByKey(%q) error = %v", model.ConfigKeySiteName, err)
	}
	if got.Value != "Updated" {
		t.Fatalf("GetSystemConfigByKey(%q).Value = %q, want %q", model.ConfigKeySiteName, got.Value, "Updated")
	}

	second, err := provider.PublicConfig(ctx)
	if err != nil {
		t.Fatalf("PublicConfig() after save error = %v", err)
	}
	if got := second[model.ConfigKeySiteName]; got == "OpenFlare" {
		t.Fatalf("PublicConfig() after save [%q] stayed stale at %q", model.ConfigKeySiteName, got)
	}
	if got := second[model.ConfigKeySiteName]; got != "Updated" {
		t.Fatalf("PublicConfig() after save [%q] = %q, want %q", model.ConfigKeySiteName, got, "Updated")
	}
}
