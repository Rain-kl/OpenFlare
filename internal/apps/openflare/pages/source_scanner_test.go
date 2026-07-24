// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/integration/githubrelease"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

type scannerDispatchedSync struct {
	SourceID uint
	Revision string
}

func TestGitHubLatestAutoConfigPreservesIdentityAndRuntimeCursor(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "scanner-auto-config")
	source, _ := mustConfigureGitHubSourceWithoutDispatch(t, ctx, project.ID, SourceUpdateInput{
		SourceType:           PagesSourceTypeGitHubRelease,
		RepositoryURL:        "https://github.com/scanner/auto-config",
		AutoUpdateEnabled:    false,
		CheckIntervalMinutes: 60,
	})
	identity := source.SourceIdentity
	seenRevision := strings.Repeat("a", sourceRevisionHexLength)
	appliedRevision := strings.Repeat("b", sourceRevisionHexLength)
	future := time.Now().Add(time.Hour)
	if err := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ?", source.ID).
		Updates(map[string]any{
			"etag":                  `"cursor-etag"`,
			"last_seen_revision":    seenRevision,
			"last_seen_detail":      `{"provider":"github","release_id":"2","asset_id":"2","tag":"v2","asset_name":"dist.zip"}`,
			"last_applied_revision": appliedRevision,
			"last_applied_detail":   `{"provider":"github","release_id":"1","asset_id":"1","tag":"v1","asset_name":"dist.zip"}`,
			"sync_status":           pagesSourceStatusSyncing,
			"last_error":            "old error",
			"lease_token":           "in-flight",
			"lease_expires_at":      &future,
		}).Error; err != nil {
		t.Fatalf("seed runtime error = %v, want nil", err)
	}

	input := SourceUpdateInput{
		SourceType:           PagesSourceTypeGitHubRelease,
		RepositoryURL:        "https://github.com/scanner/auto-config",
		AutoUpdateEnabled:    true,
		CheckIntervalMinutes: 15,
	}
	if err := validateGitHubSourceInput(input); err != nil {
		t.Fatalf("validateGitHubSourceInput(auto latest) error = %v, want nil", err)
	}
	if err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		changed, err := updateGitHubSourceTx(tx, project.ID, input)
		if err == nil && !changed {
			return errors.New("auto config update was treated as no-op")
		}
		return err
	}); err != nil {
		t.Fatalf("updateGitHubSourceTx(auto latest) error = %v, want nil", err)
	}

	updated, runtime := mustLoadPagesSource(t, ctx, project.ID)
	if updated.SourceIdentity != identity || updated.ConfigVersion != source.ConfigVersion+1 {
		t.Errorf(
			"updated source = identity:%q version:%d, want identity:%q version:%d",
			updated.SourceIdentity,
			updated.ConfigVersion,
			identity,
			source.ConfigVersion+1,
		)
	}
	if !updated.AutoUpdateEnabled || updated.CheckIntervalMinutes != 15 {
		t.Errorf("updated auto config = enabled:%t interval:%d, want true/15", updated.AutoUpdateEnabled, updated.CheckIntervalMinutes)
	}
	if runtime.ETag != `"cursor-etag"` || runtime.LastSeenRevision != seenRevision ||
		runtime.LastAppliedRevision != appliedRevision {
		t.Errorf("runtime cursor changed after auto-only update: %+v", runtime)
	}
	if runtime.LeaseToken != "" || runtime.LeaseExpiresAt != nil || runtime.LastError != "" ||
		runtime.SyncStatus != pagesSourceStatusUpdateAvailable || runtime.NextCheckAt == nil {
		t.Errorf(
			"runtime fence = token:%q expiry:%v error:%q status:%q next:%v",
			runtime.LeaseToken,
			runtime.LeaseExpiresAt,
			runtime.LastError,
			runtime.SyncStatus,
			runtime.NextCheckAt,
		)
	}

	tagConfig, err := buildGitHubSourceConfig(SourceUpdateInput{
		SourceType:           PagesSourceTypeGitHubRelease,
		RepositoryURL:        "https://github.com/scanner/auto-config",
		ReleaseSelector:      githubReleaseSelectorTag,
		ReleaseTag:           "v1",
		AutoUpdateEnabled:    true,
		CheckIntervalMinutes: 60,
	})
	if err != nil {
		t.Fatalf("buildGitHubSourceConfig(tag) error = %v, want nil", err)
	}
	if tagConfig.AutoUpdate || tagConfig.CheckInterval != 0 {
		t.Errorf("tag config auto/interval = %t/%d, want false/0", tagConfig.AutoUpdate, tagConfig.CheckInterval)
	}
}

func TestRecoverExpiredPagesSourceLeaseUsesExactCASAndStableJitter(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "scanner-expired-lease")
	source, _ := mustConfigureGitHubSourceWithoutDispatch(t, ctx, project.ID, SourceUpdateInput{
		SourceType:    PagesSourceTypeGitHubRelease,
		RepositoryURL: "https://github.com/scanner/expired-lease",
	})
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	usePagesSourceScannerClock(t, now)
	expiredAt := now.Add(-time.Minute)
	if err := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ?", source.ID).
		Updates(map[string]any{
			"sync_status":      pagesSourceStatusChecking,
			"lease_token":      "expired-owner",
			"lease_expires_at": &expiredAt,
			"next_check_at":    &expiredAt,
		}).Error; err != nil {
		t.Fatalf("seed expired lease error = %v, want nil", err)
	}

	summary := pagesSourceScanSummary{}
	if err := recoverExpiredPagesSourceLeases(ctx, now, &summary); err != nil {
		t.Fatalf("recoverExpiredPagesSourceLeases() error = %v, want nil", err)
	}
	if summary.ExpiredCandidates != 1 || summary.RecoveredLeases != 1 || summary.FailedSources != 0 {
		t.Errorf("recovery summary = %+v, want one recovered lease", summary)
	}
	_, runtime := mustLoadPagesSource(t, ctx, project.ID)
	wantNext := nextGitHubCheckAt(now, source.ID, minimumCheckInterval)
	if runtime.SyncStatus != pagesSourceStatusFailed || runtime.LastError != errPagesSourceLeaseExpired ||
		runtime.LeaseToken != "" || runtime.LeaseExpiresAt != nil || runtime.NextCheckAt == nil ||
		runtime.NextCheckAt.Sub(wantNext) != 0 {
		t.Errorf(
			"recovered runtime = status:%q error:%q token:%q expiry:%v next:%v, want next %v",
			runtime.SyncStatus,
			runtime.LastError,
			runtime.LeaseToken,
			runtime.LeaseExpiresAt,
			runtime.NextCheckAt,
			wantNext,
		)
	}

	renewedExpiry := now.Add(time.Minute)
	if err := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ?", source.ID).
		Updates(map[string]any{
			"sync_status":      pagesSourceStatusSyncing,
			"lease_token":      "renewed-owner",
			"lease_expires_at": &renewedExpiry,
		}).Error; err != nil {
		t.Fatalf("seed renewed lease error = %v, want nil", err)
	}
	recovered, err := recoverExpiredSourceLease(
		ctx,
		source.ID,
		"renewed-owner",
		expiredAt,
		pagesSourceStatusSyncing,
		now,
		&wantNext,
	)
	if err != nil || recovered {
		t.Fatalf("recoverExpiredSourceLease(stale expiry) = %t, %v; want false, nil", recovered, err)
	}
	_, runtime = mustLoadPagesSource(t, ctx, project.ID)
	if runtime.LeaseToken != "renewed-owner" || runtime.LeaseExpiresAt == nil ||
		runtime.LeaseExpiresAt.Sub(renewedExpiry) != 0 || runtime.SyncStatus != pagesSourceStatusSyncing {
		t.Errorf("stale recovery overwrote renewed lease: %+v", runtime)
	}
}

func TestScheduledAutoSyncPersistsExplicitDeploymentTrigger(t *testing.T) {
	ctx := setupPagesSourceSyncTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "scanner-scheduled-trigger")
	source, _ := mustConfigureGitHubSourceWithoutDispatch(t, ctx, project.ID, SourceUpdateInput{
		SourceType:        PagesSourceTypeGitHubRelease,
		RepositoryURL:     "https://github.com/scanner/scheduled-trigger",
		AutoUpdateEnabled: true,
	})
	packageBytes := testPagesZip(t, map[string]string{"index.html": "scheduled-v1"})
	packageHash := sha256.Sum256(packageBytes)
	release := githubrelease.Release{ID: "scheduled-release", Tag: "v1"}
	asset := githubrelease.Asset{
		ID: "scheduled-asset", Name: defaultGitHubAssetName, State: "uploaded",
		UpdatedAt: time.Date(2026, 7, 19, 12, 30, 0, 0, time.UTC),
	}
	target, err := buildGitHubSourceTarget(release, asset, nil)
	if err != nil {
		t.Fatalf("buildGitHubSourceTarget() error = %v, want nil", err)
	}
	useFakeGitHubReleaseClient(t, &fakeGitHubReleaseClient{
		resolve: func(context.Context, githubrelease.ResolveRequest) (githubrelease.ResolveResult, error) {
			return githubrelease.ResolveResult{Release: release, Asset: asset}, nil
		},
		download: func(context.Context, githubrelease.DownloadRequest) (*githubrelease.DownloadResult, error) {
			path := filepath.Join(t.TempDir(), "scheduled.zip")
			if err := os.WriteFile(path, packageBytes, 0o600); err != nil {
				t.Fatalf("os.WriteFile(scheduled package) error = %v", err)
			}
			return &githubrelease.DownloadResult{
				Path: path, Size: int64(len(packageBytes)), SHA256: hex.EncodeToString(packageHash[:]),
			}, nil
		},
	})
	snapshot, outcome, err := acquireSourceLease(ctx, source.ID, source.ConfigVersion, sourceActionSync)
	if err != nil || outcome != sourceLeaseAcquired {
		t.Fatalf("acquireSourceLease() = %+v, %q, %v; want acquired", snapshot, outcome, err)
	}
	synced, err := syncGitHubSourceWithTrigger(
		ctx,
		snapshot,
		pagesSourceCreatedBySystem,
		target.Revision,
		"",
		pagesSourceTriggerScheduledAutoUpdate,
	)
	if err != nil || synced == nil || synced.Deployment == nil || synced.Stale {
		t.Fatalf("syncGitHubSourceWithTrigger() = %+v, %v; want active deployment", synced, err)
	}
	deployment, err := model.GetPagesDeploymentByID(ctx, synced.Deployment.ID)
	if err != nil {
		t.Fatalf("GetPagesDeploymentByID(%d) error = %v", synced.Deployment.ID, err)
	}
	if deployment.TriggerType != pagesSourceTriggerScheduledAutoUpdate ||
		deployment.CreatedBy != pagesSourceCreatedBySystem {
		t.Errorf(
			"scheduled provenance = trigger:%q actor:%q, want %q/%q",
			deployment.TriggerType,
			deployment.CreatedBy,
			pagesSourceTriggerScheduledAutoUpdate,
			pagesSourceCreatedBySystem,
		)
	}
}

func TestPagesSourceScannerSerialBatchIsolation304AndActualBacklog(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	now := time.Now().Truncate(time.Second)
	usePagesSourceScannerClock(t, now)
	dueAt := now.Add(-time.Hour)

	type fixture struct {
		source     *model.PagesProjectSource
		runtime    *model.PagesProjectSourceRuntime
		repository string
	}
	fixtures := make([]fixture, 0, 22)
	byRepository := make(map[string]int, 22)
	for index := 1; index <= 22; index++ {
		project := mustCreatePagesSourceProject(t, ctx, fmt.Sprintf("scanner-batch-%02d", index))
		repository := fmt.Sprintf("scanner/source-%02d", index)
		source, runtime := mustConfigureGitHubSourceWithoutDispatch(t, ctx, project.ID, SourceUpdateInput{
			SourceType:           PagesSourceTypeGitHubRelease,
			RepositoryURL:        "https://github.com/" + repository,
			AutoUpdateEnabled:    index != 4,
			CheckIntervalMinutes: 60,
		})
		if err := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
			Where("source_id = ?", source.ID).
			Update("next_check_at", &dueAt).Error; err != nil {
			t.Fatalf("mark source %d due error = %v, want nil", source.ID, err)
		}
		fixtures = append(fixtures, fixture{source: source, runtime: runtime, repository: repository})
		byRepository[repository] = index
	}

	busyUntil := now.Add(time.Hour)
	if err := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ?", fixtures[0].source.ID).
		Updates(map[string]any{
			"sync_status":      pagesSourceStatusChecking,
			"lease_token":      "busy-owner",
			"lease_expires_at": &busyUntil,
		}).Error; err != nil {
		t.Fatalf("seed busy source error = %v, want nil", err)
	}

	stored304Revision := strings.Repeat("3", sourceRevisionHexLength)
	if err := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ?", fixtures[2].source.ID).
		Updates(map[string]any{
			"etag":               `"stored-etag"`,
			"last_seen_revision": stored304Revision,
			"last_seen_detail":   `{"provider":"github","release_id":"release-3","asset_id":"3","tag":"v3","asset_name":"dist.zip"}`,
			"sync_status":        pagesSourceStatusUpdateAvailable,
		}).Error; err != nil {
		t.Fatalf("seed 304 cursor error = %v, want nil", err)
	}

	if err := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ?", fixtures[4].source.ID).
		Updates(map[string]any{
			"last_applied_revision": strings.Repeat("a", sourceRevisionHexLength),
			"last_applied_detail":   `{"provider":"github","release_id":"shared-release","asset_id":"old","tag":"v5","asset_name":"dist.zip"}`,
		}).Error; err != nil {
		t.Fatalf("seed replacement cursor error = %v, want nil", err)
	}

	retryAt := now.Add(2 * time.Hour)
	calledRepositories := make([]string, 0, pagesSourceScanBatchSize)
	useFakeGitHubReleaseClient(t, &fakeGitHubReleaseClient{
		resolve: func(_ context.Context, request githubrelease.ResolveRequest) (githubrelease.ResolveResult, error) {
			index := byRepository[request.Repository]
			calledRepositories = append(calledRepositories, request.Repository)
			switch index {
			case 2:
				return githubrelease.ResolveResult{}, &githubrelease.Error{
					Kind: githubrelease.ErrMetadata, StatusCode: 429, RetryAt: &retryAt,
				}
			case 3:
				if request.ETag != `"stored-etag"` {
					t.Errorf("304 source ETag = %q, want stored ETag", request.ETag)
				}
				return githubrelease.ResolveResult{NotModified: true, ETag: request.ETag}, nil
			default:
				releaseID := fmt.Sprintf("release-%d", index)
				if index == 5 {
					releaseID = "shared-release"
				}
				result := githubrelease.ResolveResult{
					ETag:    fmt.Sprintf(`"etag-%d"`, index),
					Release: githubrelease.Release{ID: releaseID, Tag: fmt.Sprintf("v%d", index)},
					Asset: githubrelease.Asset{
						ID:        fmt.Sprintf("asset-%d", index),
						Name:      defaultGitHubAssetName,
						State:     "uploaded",
						UpdatedAt: now.Add(time.Duration(index) * time.Minute),
					},
				}
				if index == 6 {
					result.RetryAt = &retryAt
				}
				return result, nil
			}
		},
		download: func(context.Context, githubrelease.DownloadRequest) (*githubrelease.DownloadResult, error) {
			t.Fatal("scanner downloaded an asset; want check-only behavior")
			return nil, nil
		},
	})

	dispatched := make([]scannerDispatchedSync, 0, pagesSourceScanBatchSize)
	previousDispatch := dispatchPagesSourceAutoSync
	dispatchPagesSourceAutoSync = func(
		_ context.Context,
		source model.PagesProjectSource,
		revision string,
	) (*SourceActionReceipt, error) {
		if source.ID == fixtures[5].source.ID {
			return nil, errors.New("injected dispatch failure")
		}
		dispatched = append(dispatched, scannerDispatchedSync{SourceID: source.ID, Revision: revision})
		return &SourceActionReceipt{ExecutionID: fmt.Sprintf("%d", source.ID), Action: sourceActionSync}, nil
	}
	t.Cleanup(func() { dispatchPagesSourceAutoSync = previousDispatch })

	result, err := (&SourceScanHandler{}).Execute(ctx, []byte("{}"))
	if err != nil {
		t.Fatalf("SourceScanHandler.Execute() error = %v, want nil", err)
	}
	var summary pagesSourceScanSummary
	if err := json.Unmarshal([]byte(result.Detail), &summary); err != nil {
		t.Fatalf("json.Unmarshal(scan detail) error = %v, want nil", err)
	}
	if summary.DueSources != 22 || summary.SelectedSources != pagesSourceScanBatchSize ||
		summary.CheckedSources != 18 || summary.UpdatesFound != 17 || summary.AttentionSources != 1 ||
		summary.DispatchedSyncs != 15 || summary.FailedDispatches != 1 ||
		summary.BusySources != 1 || summary.FailedSources != 2 ||
		summary.Backlog != 3 {
		t.Errorf("scan summary = %+v, want due=22 selected=20 checked=18 updates=17 attention=1 dispatched=15 dispatch_failed=1 busy=1 failed=2 backlog=3", summary)
	}
	if len(summary.ProviderBackoffs) != 1 ||
		summary.ProviderBackoffs[0].SourceID != fixtures[1].source.ID ||
		summary.ProviderBackoffs[0].StatusCode != 429 ||
		summary.ProviderBackoffs[0].RetryAt != retryAt.UTC().Format(time.RFC3339) {
		t.Errorf("scan provider backoffs = %+v, want source=%d status=429 retry_at=%s", summary.ProviderBackoffs, fixtures[1].source.ID, retryAt.UTC().Format(time.RFC3339))
	}

	if len(calledRepositories) != 19 {
		t.Fatalf("Resolve calls = %d, want 19 (one busy source in selected batch)", len(calledRepositories))
	}
	for index, repository := range calledRepositories {
		want := fixtures[index+1].repository
		if repository != want {
			t.Fatalf("Resolve order[%d] = %q, want %q", index, repository, want)
		}
	}

	if !containsDispatchedSource(dispatched, fixtures[2].source.ID, stored304Revision) {
		t.Errorf("304 stored revision was not dispatched: %+v", dispatched)
	}
	if containsDispatchedSource(dispatched, fixtures[3].source.ID, "") {
		t.Errorf("auto=false source was dispatched: %+v", dispatched)
	}
	if containsDispatchedSource(dispatched, fixtures[4].source.ID, "") {
		t.Errorf("attention source was dispatched: %+v", dispatched)
	}

	_, dispatchFailedRuntime := mustLoadPagesSource(t, ctx, fixtures[5].source.ProjectID)
	if dispatchFailedRuntime.SyncStatus != pagesSourceStatusUpdateAvailable ||
		dispatchFailedRuntime.LastError != errPagesSourceTaskDispatchFailed ||
		dispatchFailedRuntime.NextCheckAt == nil || dispatchFailedRuntime.NextCheckAt.Before(retryAt) ||
		dispatchFailedRuntime.LastSeenRevision == "" {
		t.Errorf(
			"dispatch failure runtime = status:%q error:%q next:%v seen:%q, want preserved update and provider deadline >= %v",
			dispatchFailedRuntime.SyncStatus,
			dispatchFailedRuntime.LastError,
			dispatchFailedRuntime.NextCheckAt,
			dispatchFailedRuntime.LastSeenRevision,
			retryAt,
		)
	}
}

func TestPagesSourceScannerIncludesOrphanCleanupSummary(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	now := time.Now().Truncate(time.Second)
	usePagesSourceScannerClock(t, now)

	previousReconcile := reconcilePagesSourceOrphans
	reconcilePagesSourceOrphans = func(
		_ context.Context,
		gotNow time.Time,
	) (PagesOrphanCleanupSummary, error) {
		if !gotNow.Equal(now) {
			t.Errorf("orphan cleanup now = %v, want %v", gotNow, now)
		}
		return PagesOrphanCleanupSummary{
			Candidates:    7,
			Reconciled:    1,
			Referenced:    2,
			LeaseBusy:     1,
			InvalidMarker: 1,
			Skipped:       1,
			Failed:        1,
		}, nil
	}
	t.Cleanup(func() { reconcilePagesSourceOrphans = previousReconcile })

	result, err := (&SourceScanHandler{}).Execute(ctx, []byte("{}"))
	if err != nil {
		t.Fatalf("SourceScanHandler.Execute() error = %v, want nil", err)
	}
	var summary pagesSourceScanSummary
	if err := json.Unmarshal([]byte(result.Detail), &summary); err != nil {
		t.Fatalf("json.Unmarshal(scan detail) error = %v, want nil", err)
	}
	if summary.OrphanCleanup.Candidates != 7 || summary.OrphanCleanup.Reconciled != 1 ||
		summary.OrphanCleanup.Referenced != 2 || summary.OrphanCleanup.LeaseBusy != 1 ||
		summary.OrphanCleanup.InvalidMarker != 1 || summary.OrphanCleanup.Skipped != 1 ||
		summary.OrphanCleanup.Failed != 1 {
		t.Errorf("orphan cleanup summary = %+v, want injected result", summary.OrphanCleanup)
	}
}

func TestPagesSourceScanPayloadAndMeta(t *testing.T) {
	handler := &SourceScanHandler{}
	if normalized, err := handler.ValidatePayload(nil); err != nil || string(normalized) != "{}" {
		t.Errorf("ValidatePayload(nil) = %s, %v; want {}, nil", normalized, err)
	}
	if _, err := handler.ValidatePayload([]byte(`{"unexpected":true}`)); err == nil {
		t.Error("ValidatePayload(unknown field) error = nil, want non-nil")
	}
	if PagesSourceScanMeta.InternalOnly || PagesSourceScanMeta.Type != TaskTypePagesSourceScan ||
		PagesSourceScanMeta.AsynqTask != PagesSourceScanTask || PagesSourceScanMeta.MaxRetry != 0 {
		t.Errorf("PagesSourceScanMeta = %+v, want public bounded scheduled scanner", PagesSourceScanMeta)
	}
	if PagesSourceScanMeta.SupportsTime {
		t.Error("PagesSourceScanMeta.SupportsTime = true, want empty scanner payload")
	}
}

func usePagesSourceScannerClock(t *testing.T, now time.Time) {
	t.Helper()
	previous := pagesSourceScanNow
	pagesSourceScanNow = func() time.Time { return now }
	t.Cleanup(func() { pagesSourceScanNow = previous })
}

func containsDispatchedSource(dispatched []scannerDispatchedSync, sourceID uint, revision string) bool {
	for _, item := range dispatched {
		if item.SourceID == sourceID && (revision == "" || item.Revision == revision) {
			return true
		}
	}
	return false
}
