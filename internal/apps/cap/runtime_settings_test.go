// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cap

import (
	"context"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
)

func TestCurrentSettingsLoadsSnapshotOnce(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	ResetRuntimeSettingsForTest()
	repository.ResetSystemConfigRAMCacheForTest()

	first, err := CurrentSettings(ctx)
	if err != nil {
		t.Fatalf("CurrentSettings() first error = %v", err)
	}
	if first.ChallengeCount != 1 {
		t.Fatalf("CurrentSettings().ChallengeCount = %d, want %d", first.ChallengeCount, 1)
	}

	if err := db.DB(ctx).Model(&model.SystemConfig{}).
		Where("key = ?", model.ConfigKeyCapChallengeCount).
		Update("value", "4").Error; err != nil {
		t.Fatalf("Update(cap_challenge_count) error = %v", err)
	}
	if err := repository.InvalidateSystemConfigCache(ctx, model.ConfigKeyCapChallengeCount); err != nil {
		t.Fatalf("InvalidateSystemConfigCache() error = %v", err)
	}
	InvalidateRuntimeSettings()

	second, err := CurrentSettings(ctx)
	if err != nil {
		t.Fatalf("CurrentSettings() second error = %v", err)
	}
	if second.ChallengeCount != 4 {
		t.Fatalf("CurrentSettings().ChallengeCount = %d, want %d", second.ChallengeCount, 4)
	}
}

func TestProtectionEnabledReflectsLoginSwitch(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	ResetRuntimeSettingsForTest()

	if ProtectionEnabled(ctx) {
		t.Fatal("ProtectionEnabled() = true, want false from seed defaults")
	}

	if err := db.DB(ctx).Model(&model.SystemConfig{}).
		Where("key = ?", model.ConfigKeyCapLoginEnabled).
		Update("value", "true").Error; err != nil {
		t.Fatalf("Update(cap_login_enabled) error = %v", err)
	}
	if err := repository.InvalidateSystemConfigCache(ctx, model.ConfigKeyCapLoginEnabled); err != nil {
		t.Fatalf("InvalidateSystemConfigCache() error = %v", err)
	}
	InvalidateRuntimeSettings()

	if !ProtectionEnabled(ctx) {
		t.Fatal("ProtectionEnabled() = false, want true after config update")
	}
}

func TestParseRuntimeSettingsUsesDefaultsForMissingKeys(t *testing.T) {
	settings := parseRuntimeSettings(map[string]model.SystemConfig{})

	if settings.ChallengeCount != defaultChallengeCount {
		t.Fatalf("ChallengeCount = %d, want %d", settings.ChallengeCount, defaultChallengeCount)
	}
	if settings.ChallengeTTL != defaultChallengeTTL {
		t.Fatalf("ChallengeTTL = %s, want %s", settings.ChallengeTTL, defaultChallengeTTL)
	}
	if settings.TokenTTL != defaultTokenTTL {
		t.Fatalf("TokenTTL = %s, want %s", settings.TokenTTL, defaultTokenTTL)
	}
}

func TestIsRuntimeConfigKey(t *testing.T) {
	if !IsRuntimeConfigKey(model.ConfigKeyCapChallengeCount) {
		t.Fatalf("IsRuntimeConfigKey(%s) = false, want true", model.ConfigKeyCapChallengeCount)
	}
	if IsRuntimeConfigKey(model.ConfigKeySiteName) {
		t.Fatalf("IsRuntimeConfigKey(%s) = true, want false", model.ConfigKeySiteName)
	}
}

func TestInstallTestRuntimeSettings(t *testing.T) {
	cleanup := InstallTestRuntimeSettings(RuntimeSettings{
		LoginEnabled:   true,
		ChallengeCount: 2,
		TokenTTL:       30 * time.Minute,
	})
	defer cleanup()

	settings, err := CurrentSettings(context.Background())
	if err != nil {
		t.Fatalf("CurrentSettings() error = %v", err)
	}
	if !settings.LoginEnabled {
		t.Fatal("LoginEnabled = false, want true")
	}
	if settings.ChallengeCount != 2 {
		t.Fatalf("ChallengeCount = %d, want %d", settings.ChallengeCount, 2)
	}
}
