// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"strings"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
)

func TestGitHubSourceIdentityLengthPrefixesFieldsAndResetsRuntime(t *testing.T) {
	firstInput := SourceUpdateInput{
		SourceType:      PagesSourceTypeGitHubRelease,
		RepositoryURL:   "https://github.com/OpenFlare/site",
		ReleaseSelector: githubReleaseSelectorTag,
		ReleaseTag:      "release|foo",
		AssetName:       "bar.zip",
	}
	secondInput := SourceUpdateInput{
		SourceType:      PagesSourceTypeGitHubRelease,
		RepositoryURL:   "https://github.com/OpenFlare/site",
		ReleaseSelector: githubReleaseSelectorTag,
		ReleaseTag:      "release",
		AssetName:       "foo|bar.zip",
	}
	firstConfig, err := buildGitHubSourceConfig(firstInput)
	if err != nil {
		t.Fatalf("buildGitHubSourceConfig(first) error = %v, want nil", err)
	}
	secondConfig, err := buildGitHubSourceConfig(secondInput)
	if err != nil {
		t.Fatalf("buildGitHubSourceConfig(second) error = %v, want nil", err)
	}
	legacyIdentityInput := func(config githubSourceConfig) string {
		return "github|" + config.Repository + "|" + config.Selector + "|" +
			config.Tag + "|" + config.AssetName
	}
	if firstLegacy, secondLegacy := legacyIdentityInput(firstConfig), legacyIdentityInput(secondConfig); firstLegacy != secondLegacy {
		t.Fatalf("legacy identity inputs differ: %q != %q; collision fixture is invalid", firstLegacy, secondLegacy)
	}
	if firstConfig.SourceIdentity == secondConfig.SourceIdentity {
		t.Fatalf("length-prefixed identities collide: %q", firstConfig.SourceIdentity)
	}

	ctx := setupPagesSourceTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "github-identity-collision")
	firstSource, _ := mustConfigureGitHubSourceWithoutDispatch(t, ctx, project.ID, firstInput)
	if got, want := firstSource.SourceIdentity, firstConfig.SourceIdentity; got != want {
		t.Fatalf("first source identity = %q, want %q", got, want)
	}

	checkedAt := time.Now().Add(-time.Minute)
	syncedAt := time.Now().Add(-30 * time.Second)
	nextCheckAt := time.Now().Add(time.Hour)
	leaseExpiresAt := time.Now().Add(time.Minute)
	if err := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ?", firstSource.ID).
		Updates(map[string]any{
			"etag":                  `"old-etag"`,
			"last_seen_revision":    strings.Repeat("a", 64),
			"last_seen_detail":      `{"provider":"github_release","tag":"release|foo"}`,
			"last_applied_revision": strings.Repeat("b", 64),
			"last_applied_detail":   `{"provider":"github_release","tag":"older"}`,
			"sync_status":           pagesSourceStatusSyncing,
			"last_error":            "old error",
			"last_checked_at":       &checkedAt,
			"last_synced_at":        &syncedAt,
			"next_check_at":         &nextCheckAt,
			"lease_expires_at":      &leaseExpiresAt,
			"lease_token":           "old-lease",
		}).Error; err != nil {
		t.Fatalf("seed runtime cursors error = %v, want nil", err)
	}

	secondSource, runtime := mustConfigureGitHubSourceWithoutDispatch(t, ctx, project.ID, secondInput)
	if secondSource.ID != firstSource.ID {
		t.Errorf("updated source ID = %d, want unchanged %d", secondSource.ID, firstSource.ID)
	}
	if got, want := secondSource.SourceIdentity, secondConfig.SourceIdentity; got != want {
		t.Errorf("updated source identity = %q, want %q", got, want)
	}
	if got, want := secondSource.ConfigVersion, firstSource.ConfigVersion+1; got != want {
		t.Errorf("updated source config version = %d, want %d", got, want)
	}
	if runtime.ETag != "" || runtime.LastSeenRevision != "" || runtime.LastSeenDetail != "" ||
		runtime.LastAppliedRevision != "" || runtime.LastAppliedDetail != "" {
		t.Errorf("identity change retained runtime cursors: %+v", runtime)
	}
	if runtime.LastCheckedAt != nil || runtime.LastSyncedAt != nil || runtime.NextCheckAt != nil {
		t.Errorf(
			"identity change retained runtime timestamps: checked=%v synced=%v next=%v",
			runtime.LastCheckedAt,
			runtime.LastSyncedAt,
			runtime.NextCheckAt,
		)
	}
	if runtime.SyncStatus != pagesSourceStatusIdle || runtime.LastError != "" ||
		runtime.LeaseToken != "" || runtime.LeaseExpiresAt != nil {
		t.Errorf(
			"identity change retained runtime state: status=%q error=%q lease=(%q, %v)",
			runtime.SyncStatus,
			runtime.LastError,
			runtime.LeaseToken,
			runtime.LeaseExpiresAt,
		)
	}
}
