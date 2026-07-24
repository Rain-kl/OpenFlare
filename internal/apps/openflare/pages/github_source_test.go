// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/integration/githubrelease"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/hibiken/asynq"
	"gorm.io/gorm"
)

type fakeGitHubReleaseClient struct {
	resolve  func(context.Context, githubrelease.ResolveRequest) (githubrelease.ResolveResult, error)
	download func(context.Context, githubrelease.DownloadRequest) (*githubrelease.DownloadResult, error)
}

func (client *fakeGitHubReleaseClient) Resolve(
	ctx context.Context,
	request githubrelease.ResolveRequest,
) (githubrelease.ResolveResult, error) {
	return client.resolve(ctx, request)
}

func (client *fakeGitHubReleaseClient) Download(
	ctx context.Context,
	request githubrelease.DownloadRequest,
) (*githubrelease.DownloadResult, error) {
	return client.download(ctx, request)
}

func useFakeGitHubReleaseClient(t *testing.T, client githubReleaseAPI) {
	t.Helper()
	previous := newGitHubReleaseClient
	newGitHubReleaseClient = func() githubReleaseAPI { return client }
	t.Cleanup(func() { newGitHubReleaseClient = previous })
}

func mustConfigureGitHubSourceWithoutDispatch(
	t *testing.T,
	ctx context.Context,
	projectID uint,
	input SourceUpdateInput,
) (*model.PagesProjectSource, *model.PagesProjectSourceRuntime) {
	t.Helper()
	if err := validateGitHubSourceInput(input); err != nil {
		t.Fatalf("validateGitHubSourceInput(%+v) error = %v, want nil", input, err)
	}
	if err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		_, err := updateGitHubSourceTx(tx, projectID, input)
		return err
	}); err != nil {
		t.Fatalf("updateGitHubSourceTx(project=%d) error = %v, want nil", projectID, err)
	}
	return mustLoadPagesSource(t, ctx, projectID)
}

func mustLoadPagesSource(
	t *testing.T,
	ctx context.Context,
	projectID uint,
) (*model.PagesProjectSource, *model.PagesProjectSourceRuntime) {
	t.Helper()
	var source model.PagesProjectSource
	if err := db.DB(ctx).Where("project_id = ?", projectID).First(&source).Error; err != nil {
		t.Fatalf("load source for project %d error = %v, want nil", projectID, err)
	}
	var runtime model.PagesProjectSourceRuntime
	if err := db.DB(ctx).Where("source_id = ?", source.ID).First(&runtime).Error; err != nil {
		t.Fatalf("load runtime for source %d error = %v, want nil", source.ID, err)
	}
	return &source, &runtime
}

func TestGitHubSourceValidationNormalizationAndProviderSwitch(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	setupPagesSourceDispatchTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "github-config")
	input := SourceUpdateInput{
		SourceType:    PagesSourceTypeGitHubRelease,
		RepositoryURL: "https://github.com/OpenFlare/site.git",
	}
	result, err := UpdateSourceAs(ctx, project.ID, input, "user:42")
	if err != nil {
		t.Fatalf("UpdateSourceAs(GitHub) error = %v, want nil", err)
	}
	if result.CheckTask == nil || result.CheckTask.Action != sourceActionCheck || result.Warning != "" {
		t.Errorf("UpdateSourceAs(GitHub) result = %+v, want initial check receipt without warning", result)
	}
	execution, err := model.GetTaskExecutionByTaskID(ctx, result.CheckTask.TaskID)
	if err != nil {
		t.Fatalf("GetTaskExecutionByTaskID(%q) error = %v, want nil", result.CheckTask.TaskID, err)
	}
	var actionPayload SourceActionPayload
	if err := json.Unmarshal([]byte(execution.Payload), &actionPayload); err != nil {
		t.Fatalf("json.Unmarshal(initial check payload) error = %v, want nil", err)
	}
	if actionPayload.Actor != "user:42" || actionPayload.Action != sourceActionCheck ||
		actionPayload.TargetRevision != "" || actionPayload.ConfirmedRevision != "" {
		t.Errorf("initial check payload = %+v, want real actor and credential-free check", actionPayload)
	}
	source, runtime := mustLoadPagesSource(t, ctx, project.ID)
	if got, want := source.GitHubRepository, "OpenFlare/site"; got != want {
		t.Errorf("GitHubRepository = %q, want %q", got, want)
	}
	if got, want := source.ReleaseSelector, githubReleaseSelectorLatest; got != want {
		t.Errorf("ReleaseSelector = %q, want %q", got, want)
	}
	if got, want := source.AssetName, defaultGitHubAssetName; got != want {
		t.Errorf("AssetName = %q, want %q", got, want)
	}
	if got, want := source.CheckIntervalMinutes, defaultCheckInterval; got != want {
		t.Errorf("CheckIntervalMinutes = %d, want %d", got, want)
	}
	if got, want := source.SourceIdentity, "dbbd25307aaa3b88bc25353476940a049428655bd8421ac63045fdcb5fb23c9d"; got != want {
		t.Errorf("SourceIdentity = %q, want %q", got, want)
	}
	if runtime.NextCheckAt == nil {
		t.Error("GitHub latest NextCheckAt = nil, want scheduled value")
	}

	var taskCount int64
	if err := db.DB(ctx).Model(&model.TaskExecution{}).Count(&taskCount).Error; err != nil {
		t.Fatalf("count initial checks error = %v, want nil", err)
	}
	if _, err := UpdateSourceAs(ctx, project.ID, input, "user:42"); err != nil {
		t.Fatalf("UpdateSourceAs(GitHub no-op) error = %v, want nil", err)
	}
	var noOpTaskCount int64
	if err := db.DB(ctx).Model(&model.TaskExecution{}).Count(&noOpTaskCount).Error; err != nil {
		t.Fatalf("count no-op checks error = %v, want nil", err)
	}
	if noOpTaskCount != taskCount {
		t.Errorf("no-op initial check count = %d, want unchanged %d", noOpTaskCount, taskCount)
	}

	secret := "provider-switch-secret"
	if _, err := UpdateSource(ctx, project.ID, SourceUpdateInput{
		SourceType:    PagesSourceTypeRemoteURL,
		RemoteURL:     "https://artifacts.example.com/site.zip?token=" + secret,
		AllowInsecure: false,
	}); err != nil {
		t.Fatalf("UpdateSource(GitHub to Remote) error = %v, want nil", err)
	}
	remote, _ := mustLoadPagesSource(t, ctx, project.ID)
	if remote.GitHubRepository != "" || remote.ReleaseSelector != "" || remote.AssetName != "" ||
		remote.AutoUpdateEnabled || remote.CheckIntervalMinutes != 0 {
		t.Errorf("Remote switched source retained GitHub fields: %+v", remote)
	}
	if _, err := UpdateSourceAs(ctx, project.ID, input, "user:42"); err != nil {
		t.Fatalf("UpdateSourceAs(Remote to GitHub) error = %v, want nil", err)
	}
	github, _ := mustLoadPagesSource(t, ctx, project.ID)
	if github.RemoteURL != "" || github.AllowInsecure {
		t.Errorf("GitHub switched source retained Remote fields: URL=%q allow_insecure=%v", github.RemoteURL, github.AllowInsecure)
	}
}

func TestGitHubSourceSaveSurvivesInitialCheckDispatchFailure(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "github-dispatch-warning")
	result, err := UpdateSourceAs(ctx, project.ID, SourceUpdateInput{
		SourceType:    PagesSourceTypeGitHubRelease,
		RepositoryURL: "https://github.com/a/b",
	}, "user:9")
	if err != nil {
		t.Fatalf("UpdateSourceAs(dispatch failure) error = %v, want saved source with warning", err)
	}
	if result.CheckTask != nil || result.Warning != errPagesSourceInitialCheckWarning {
		t.Errorf("UpdateSourceAs(dispatch failure) result = %+v, want warning and nil check task", result)
	}
	source, runtime := mustLoadPagesSource(t, ctx, project.ID)
	if source.GitHubRepository != "a/b" || runtime.SyncStatus != pagesSourceStatusFailed ||
		runtime.LastError != errPagesSourceInitialCheckWarning {
		t.Errorf("saved source/runtime = repo:%q status:%q error:%q", source.GitHubRepository, runtime.SyncStatus, runtime.LastError)
	}
}

func TestGitHubSourceRejectsUnsafeOrModeIncompatibleFields(t *testing.T) {
	tests := []SourceUpdateInput{
		{SourceType: PagesSourceTypeGitHubRelease, RepositoryURL: "http://github.com/a/b"},
		{SourceType: PagesSourceTypeGitHubRelease, RepositoryURL: "https://github.com/a%20b/repo"},
		{SourceType: PagesSourceTypeGitHubRelease, RepositoryURL: "https://github.com/a/b/extra"},
		{SourceType: PagesSourceTypeGitHubRelease, RepositoryURL: "https://github.com//a/b"},
		{SourceType: PagesSourceTypeGitHubRelease, RepositoryURL: "https://github.com/a/b/"},
		{SourceType: PagesSourceTypeGitHubRelease, RepositoryURL: "https://github.com/a/b?"},
		{SourceType: PagesSourceTypeGitHubRelease, RepositoryURL: "https://github.com/a/b#"},
		{SourceType: PagesSourceTypeGitHubRelease, RepositoryURL: "https://github.com/a/b", AssetName: "dist\n.zip"},
		{SourceType: PagesSourceTypeGitHubRelease, RepositoryURL: "https://github.com/a/b", AssetName: "dist\u202e.zip"},
		{SourceType: PagesSourceTypeGitHubRelease, RepositoryURL: "https://github.com/a/b", AssetName: "dir/dist.zip"},
		{SourceType: PagesSourceTypeGitHubRelease, RepositoryURL: "https://github.com/a/b", ReleaseSelector: "tag", ReleaseTag: "v1", CheckIntervalMinutes: 60},
		{SourceType: PagesSourceTypeGitHubRelease, RepositoryURL: "https://github.com/a/b", ReleaseSelector: "tag", ReleaseTag: "v1", AutoUpdateEnabled: true},
		{SourceType: PagesSourceTypeGitHubRelease, RepositoryURL: "https://github.com/a/b", ReleaseSelector: "tag", ReleaseTag: " v1"},
		{SourceType: PagesSourceTypeGitHubRelease, RepositoryURL: "https://github.com/a/b", ReleaseSelector: "tag", ReleaseTag: "v1\n"},
		{SourceType: PagesSourceTypeGitHubRelease, RepositoryURL: "https://github.com/a/b", ReleaseSelector: "tag", ReleaseTag: "v1\u2028draft"},
		{SourceType: PagesSourceTypeGitHubRelease, RepositoryURL: "https://github.com/a/b", ReleaseSelector: "tag", ReleaseTag: `v1\draft`},
		{SourceType: PagesSourceTypeGitHubRelease, RepositoryURL: "https://github.com/a/b", ReleaseSelector: "tag", ReleaseTag: "release//v1"},
		{SourceType: PagesSourceTypeGitHubRelease, RepositoryURL: "https://github.com/a/b", ReleaseSelector: "tag", ReleaseTag: "release/.draft"},
		{SourceType: PagesSourceTypeGitHubRelease, RepositoryURL: "https://github.com/a/b", ReleaseSelector: "tag", ReleaseTag: "release/v1.lock"},
		{SourceType: PagesSourceTypeGitHubRelease, RepositoryURL: "https://github.com/a/b", ReleaseSelector: "latest", ReleaseTag: "v1"},
	}
	for _, input := range tests {
		if err := validateGitHubSourceInput(input); err == nil {
			t.Errorf("validateGitHubSourceInput(%+v) error = nil, want non-nil", input)
		}
	}
}

func TestGitHubSourceAcceptsLegalAssetAndTagCharacters(t *testing.T) {
	tests := []SourceUpdateInput{
		{
			SourceType:    PagesSourceTypeGitHubRelease,
			RepositoryURL: "https://github.com/a/b",
			AssetName:     "dist?channel=stable&part#1.zip",
		},
		{
			SourceType:    PagesSourceTypeGitHubRelease,
			RepositoryURL: "https://github.com/a/b",
			AssetName:     " dist.zip ",
		},
		{
			SourceType:      PagesSourceTypeGitHubRelease,
			RepositoryURL:   "https://github.com/a/b",
			ReleaseSelector: "tag",
			ReleaseTag:      "release/v1#stable&build=1",
			AssetName:       "dist.zip",
		},
		{
			SourceType:      PagesSourceTypeGitHubRelease,
			RepositoryURL:   "https://github.com/a/b.git",
			ReleaseSelector: "tag",
			ReleaseTag:      "@",
			AssetName:       "dist.zip",
		},
		{
			SourceType:      PagesSourceTypeGitHubRelease,
			RepositoryURL:   "https://github.com/a/b",
			ReleaseSelector: "tag",
			ReleaseTag:      "release/v1.LOCK",
			AssetName:       "dist.zip",
		},
	}
	for _, input := range tests {
		if err := validateGitHubSourceInput(input); err != nil {
			t.Errorf("validateGitHubSourceInput(%+v) error = %v, want nil", input, err)
		}
	}
}

func TestInitialCheckFailureUsesExactConfigFence(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "github-initial-fence")
	source, _ := mustConfigureGitHubSourceWithoutDispatch(t, ctx, project.ID, SourceUpdateInput{
		SourceType:    PagesSourceTypeGitHubRelease,
		RepositoryURL: "https://github.com/a/b",
	})
	staleVersion := source.ConfigVersion
	if err := db.DB(ctx).Model(source).Update("config_version", staleVersion+1).Error; err != nil {
		t.Fatalf("increment source config version error = %v, want nil", err)
	}
	markInitialCheckDispatchFailed(ctx, source.ID, staleVersion)
	_, runtime := mustLoadPagesSource(t, ctx, project.ID)
	if runtime.SyncStatus != pagesSourceStatusIdle || runtime.LastError != "" {
		t.Errorf("stale initial failure runtime = status:%q error:%q, want unchanged idle", runtime.SyncStatus, runtime.LastError)
	}
}

func TestGitHubCheckUsesETagAndDetectsSameReleaseReplacement(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "github-check")
	source, _ := mustConfigureGitHubSourceWithoutDispatch(t, ctx, project.ID, SourceUpdateInput{
		SourceType:    PagesSourceTypeGitHubRelease,
		RepositoryURL: "https://github.com/a/b",
	})
	appliedRevision := strings.Repeat("a", 64)
	appliedDetail := `{"provider":"github","release_id":"100","asset_id":"1","tag":"release/v1","asset_name":"dist.zip"}`
	if err := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).Where("source_id = ?", source.ID).Updates(map[string]any{
		"etag":                  `"old-etag"`,
		"last_applied_revision": appliedRevision,
		"last_applied_detail":   appliedDetail,
	}).Error; err != nil {
		t.Fatalf("seed GitHub runtime error = %v, want nil", err)
	}
	updatedAt := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	var gotETag string
	useFakeGitHubReleaseClient(t, &fakeGitHubReleaseClient{
		resolve: func(_ context.Context, request githubrelease.ResolveRequest) (githubrelease.ResolveResult, error) {
			gotETag = request.ETag
			return githubrelease.ResolveResult{
				ETag:    `"new-etag"`,
				Release: githubrelease.Release{ID: "100", Tag: "release/v1"},
				Asset:   githubrelease.Asset{ID: "2", Name: "dist.zip", State: "uploaded", UpdatedAt: updatedAt},
			}, nil
		},
		download: func(context.Context, githubrelease.DownloadRequest) (*githubrelease.DownloadResult, error) {
			t.Fatal("Download called during check, want resolve only")
			return nil, nil
		},
	})
	snapshot, outcome, err := acquireSourceLease(ctx, source.ID, source.ConfigVersion, sourceActionCheck)
	if err != nil || outcome != sourceLeaseAcquired {
		t.Fatalf("acquire check lease = (%+v, %q, %v), want acquired", snapshot, outcome, err)
	}
	result, err := checkGitHubSource(ctx, snapshot)
	if err != nil {
		t.Fatalf("checkGitHubSource() error = %v, want nil", err)
	}
	if result.Stale {
		t.Error("checkGitHubSource() stale = true, want false")
	}
	if got, want := gotETag, `"old-etag"`; got != want {
		t.Errorf("Resolve ETag = %q, want %q", got, want)
	}
	_, runtime := mustLoadPagesSource(t, ctx, project.ID)
	if runtime.SyncStatus != pagesSourceStatusAttention || runtime.LastSeenRevision == "" {
		t.Errorf("replacement runtime = status:%q seen:%q, want attention with revision", runtime.SyncStatus, runtime.LastSeenRevision)
	}
	view, err := GetSource(ctx, project.ID)
	if err != nil {
		t.Fatalf("GetSource() error = %v, want nil", err)
	}
	if view.LastSeen == nil || view.LastSeen.Label != "release/v1" {
		t.Errorf("LastSeen = %+v, want full tag with slash", view.LastSeen)
	}
	if err := preflightGitHubSyncConfirmation(ctx, source.ID, ""); err == nil || err.Error() != errPagesSourceConfirmationNeeded {
		t.Errorf("preflight without confirmation error = %v, want %q", err, errPagesSourceConfirmationNeeded)
	}
	if err := preflightGitHubSyncConfirmation(ctx, source.ID, runtime.LastSeenRevision); err != nil {
		t.Errorf("preflight exact confirmation error = %v, want nil", err)
	}
}

func TestGitHubCheckNotModifiedRefreshesRuntimeWithoutDeployment(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "github-304")
	source, _ := mustConfigureGitHubSourceWithoutDispatch(t, ctx, project.ID, SourceUpdateInput{
		SourceType:    PagesSourceTypeGitHubRelease,
		RepositoryURL: "https://github.com/a/b",
	})
	useFakeGitHubReleaseClient(t, &fakeGitHubReleaseClient{
		resolve: func(_ context.Context, request githubrelease.ResolveRequest) (githubrelease.ResolveResult, error) {
			return githubrelease.ResolveResult{NotModified: true, ETag: `"same"`}, nil
		},
		download: func(context.Context, githubrelease.DownloadRequest) (*githubrelease.DownloadResult, error) {
			t.Fatal("Download called for 304 check")
			return nil, nil
		},
	})
	snapshot, _, _ := acquireSourceLease(ctx, source.ID, source.ConfigVersion, sourceActionCheck)
	if _, err := checkGitHubSource(ctx, snapshot); err != nil {
		t.Fatalf("checkGitHubSource(304) error = %v, want nil", err)
	}
	_, runtime := mustLoadPagesSource(t, ctx, project.ID)
	if runtime.ETag != `"same"` || runtime.LastCheckedAt == nil || runtime.NextCheckAt == nil || runtime.LeaseToken != "" {
		t.Errorf("304 runtime = %+v, want refreshed timestamps/etag and released lease", runtime)
	}
	var deployments int64
	if err := db.DB(ctx).Model(&model.PagesDeployment{}).Where("project_id = ?", project.ID).Count(&deployments).Error; err != nil {
		t.Fatalf("count deployments error = %v, want nil", err)
	}
	if deployments != 0 {
		t.Errorf("deployments after check = %d, want 0", deployments)
	}
}

func TestGitHubTargetMismatchPreservesAttentionAndExpeditesRecheck(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "github-target-mismatch")
	source, _ := mustConfigureGitHubSourceWithoutDispatch(t, ctx, project.ID, SourceUpdateInput{
		SourceType:    PagesSourceTypeGitHubRelease,
		RepositoryURL: "https://github.com/a/b",
	})
	appliedRevision := strings.Repeat("a", 64)
	if err := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).Where("source_id = ?", source.ID).Updates(map[string]any{
		"last_applied_revision": appliedRevision,
		"last_applied_detail":   `{"provider":"github","release_id":"100","asset_id":"1","tag":"v1","asset_name":"dist.zip"}`,
	}).Error; err != nil {
		t.Fatalf("seed applied runtime error = %v, want nil", err)
	}
	retryAt := time.Now().Add(2 * time.Hour).UTC()
	useFakeGitHubReleaseClient(t, &fakeGitHubReleaseClient{
		resolve: func(context.Context, githubrelease.ResolveRequest) (githubrelease.ResolveResult, error) {
			return githubrelease.ResolveResult{
				Release: githubrelease.Release{ID: "100", Tag: "v1"},
				Asset: githubrelease.Asset{
					ID: "2", Name: "dist.zip", State: "uploaded",
					UpdatedAt: time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC),
				},
				RetryAt: &retryAt,
			}, nil
		},
		download: func(context.Context, githubrelease.DownloadRequest) (*githubrelease.DownloadResult, error) {
			t.Fatal("Download called after target mismatch")
			return nil, nil
		},
	})
	snapshot, _, _ := acquireSourceLease(ctx, source.ID, source.ConfigVersion, sourceActionSync)
	outcome, err := syncGitHubSource(ctx, snapshot, pagesSourceCreatedBySystem, strings.Repeat("b", 64), "")
	if err != nil {
		t.Fatalf("syncGitHubSource(target mismatch) error = %v, want nil stale outcome", err)
	}
	if outcome == nil || !outcome.Stale {
		t.Errorf("syncGitHubSource(target mismatch) = %+v, want stale", outcome)
	}
	_, runtime := mustLoadPagesSource(t, ctx, project.ID)
	if runtime.SyncStatus != pagesSourceStatusAttention {
		t.Errorf("target mismatch SyncStatus = %q, want %q", runtime.SyncStatus, pagesSourceStatusAttention)
	}
	if runtime.NextCheckAt == nil || runtime.NextCheckAt.Before(retryAt) {
		t.Errorf("target mismatch NextCheckAt = %v, want server deadline >= %v", runtime.NextCheckAt, retryAt)
	}
	var deployments int64
	if err := db.DB(ctx).Model(&model.PagesDeployment{}).Where("project_id = ?", project.ID).Count(&deployments).Error; err != nil {
		t.Fatalf("count mismatch deployments error = %v, want nil", err)
	}
	if deployments != 0 {
		t.Errorf("target mismatch deployments = %d, want 0", deployments)
	}
}

func TestGitHubCheckLostLeaseReturnsStaleWithoutOverwritingRuntime(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "github-check-fence")
	source, _ := mustConfigureGitHubSourceWithoutDispatch(t, ctx, project.ID, SourceUpdateInput{
		SourceType:    PagesSourceTypeGitHubRelease,
		RepositoryURL: "https://github.com/a/b",
	})
	useFakeGitHubReleaseClient(t, &fakeGitHubReleaseClient{
		resolve: func(context.Context, githubrelease.ResolveRequest) (githubrelease.ResolveResult, error) {
			return githubrelease.ResolveResult{}, errors.New("transient provider failure")
		},
		download: func(context.Context, githubrelease.DownloadRequest) (*githubrelease.DownloadResult, error) {
			return nil, errors.New("unexpected")
		},
	})
	snapshot, _, _ := acquireSourceLease(ctx, source.ID, source.ConfigVersion, sourceActionCheck)
	if err := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).Where("source_id = ?", source.ID).Updates(map[string]any{
		"lease_token":      "new-owner",
		"lease_expires_at": time.Now().Add(time.Minute),
		"sync_status":      pagesSourceStatusSyncing,
		"last_error":       "new-owner-state",
	}).Error; err != nil {
		t.Fatalf("replace lease owner error = %v, want nil", err)
	}
	result, err := checkGitHubSource(ctx, snapshot)
	if err != nil {
		t.Fatalf("checkGitHubSource(lost lease) error = %v, want stale no-op", err)
	}
	if result == nil || !result.Stale {
		t.Errorf("checkGitHubSource(lost lease) = %+v, want stale", result)
	}
	_, runtime := mustLoadPagesSource(t, ctx, project.ID)
	if runtime.LeaseToken != "new-owner" || runtime.LastError != "new-owner-state" || runtime.SyncStatus != pagesSourceStatusSyncing {
		t.Errorf("lost lease runtime = token:%q error:%q status:%q, want new owner state", runtime.LeaseToken, runtime.LastError, runtime.SyncStatus)
	}
}

func TestGitHubCheckRateLimitUsesServerDeadlineAndSuppressesFastRetry(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "github-rate-limit")
	source, _ := mustConfigureGitHubSourceWithoutDispatch(t, ctx, project.ID, SourceUpdateInput{
		SourceType:    PagesSourceTypeGitHubRelease,
		RepositoryURL: "https://github.com/a/b",
	})
	retryAt := time.Now().Add(2 * time.Hour).UTC()
	useFakeGitHubReleaseClient(t, &fakeGitHubReleaseClient{
		resolve: func(context.Context, githubrelease.ResolveRequest) (githubrelease.ResolveResult, error) {
			return githubrelease.ResolveResult{}, &githubrelease.Error{
				Kind: githubrelease.ErrMetadata, StatusCode: 429, RetryAt: &retryAt,
			}
		},
		download: func(context.Context, githubrelease.DownloadRequest) (*githubrelease.DownloadResult, error) {
			return nil, errors.New("unexpected")
		},
	})
	snapshot, _, _ := acquireSourceLease(ctx, source.ID, source.ConfigVersion, sourceActionCheck)
	_, err := checkGitHubSource(ctx, snapshot)
	if err == nil || err.Error() != errPagesSourceSyncFailed {
		t.Fatalf("checkGitHubSource(rate limit) error = %v, want safe sync failure", err)
	}
	if !shouldSkipGitHubActionRetry(err) {
		t.Error("shouldSkipGitHubActionRetry(rate limit) = false, want true")
	}
	_, runtime := mustLoadPagesSource(t, ctx, project.ID)
	if runtime.NextCheckAt == nil || runtime.NextCheckAt.Before(retryAt) || runtime.SyncStatus != pagesSourceStatusFailed {
		t.Errorf("rate limit runtime = next:%v status:%q, want deadline >= %v and failed", runtime.NextCheckAt, runtime.SyncStatus, retryAt)
	}
}

func TestGitHubCheckInvalidResolvedTargetUsesServerDeadline(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "github-invalid-check-target")
	source, _ := mustConfigureGitHubSourceWithoutDispatch(t, ctx, project.ID, SourceUpdateInput{
		SourceType:    PagesSourceTypeGitHubRelease,
		RepositoryURL: "https://github.com/a/b",
	})
	retryAt := time.Now().Add(2 * time.Hour).UTC()
	useFakeGitHubReleaseClient(t, &fakeGitHubReleaseClient{
		resolve: func(context.Context, githubrelease.ResolveRequest) (githubrelease.ResolveResult, error) {
			return githubrelease.ResolveResult{
				Release: githubrelease.Release{ID: "1", Tag: "v1"},
				Asset: githubrelease.Asset{
					ID: "2", Name: "dist.zip", State: "uploaded",
					UpdatedAt: time.Now().UTC(), Digest: "sha256:invalid",
				},
				RetryAt: &retryAt,
			}, nil
		},
		download: func(context.Context, githubrelease.DownloadRequest) (*githubrelease.DownloadResult, error) {
			t.Fatal("Download called after invalid check target")
			return nil, nil
		},
	})
	snapshot, _, _ := acquireSourceLease(ctx, source.ID, source.ConfigVersion, sourceActionCheck)
	_, err := checkGitHubSource(ctx, snapshot)
	if err == nil || err.Error() != errPagesSourceDigestInvalid {
		t.Fatalf("checkGitHubSource(invalid target) error = %v, want %q", err, errPagesSourceDigestInvalid)
	}
	if !isPermanentSourceSyncError(err) {
		t.Error("invalid check target classification = retryable, want permanent")
	}
	_, runtime := mustLoadPagesSource(t, ctx, project.ID)
	if runtime.SyncStatus != pagesSourceStatusFailed || runtime.LastError != errPagesSourceDigestInvalid ||
		runtime.NextCheckAt == nil || runtime.NextCheckAt.Before(retryAt) || runtime.LeaseToken != "" {
		t.Errorf(
			"invalid check target runtime = status:%q error:%q next:%v lease:%q, want failed/%q/deadline >= %v/cleared",
			runtime.SyncStatus, runtime.LastError, runtime.NextCheckAt, runtime.LeaseToken,
			errPagesSourceDigestInvalid, retryAt,
		)
	}
}

func TestGitHubSyncInvalidResolvedTargetUsesServerDeadline(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "github-invalid-sync-target")
	source, _ := mustConfigureGitHubSourceWithoutDispatch(t, ctx, project.ID, SourceUpdateInput{
		SourceType:    PagesSourceTypeGitHubRelease,
		RepositoryURL: "https://github.com/a/b",
	})
	retryAt := time.Now().Add(2 * time.Hour).UTC()
	useFakeGitHubReleaseClient(t, &fakeGitHubReleaseClient{
		resolve: func(context.Context, githubrelease.ResolveRequest) (githubrelease.ResolveResult, error) {
			return githubrelease.ResolveResult{
				Release: githubrelease.Release{ID: "1", Tag: "v1"},
				Asset: githubrelease.Asset{
					ID: "2", Name: "dist.zip", State: "uploaded",
					UpdatedAt: time.Now().UTC(), Digest: "sha256:invalid",
				},
				RetryAt: &retryAt,
			}, nil
		},
		download: func(context.Context, githubrelease.DownloadRequest) (*githubrelease.DownloadResult, error) {
			t.Fatal("Download called after invalid sync target")
			return nil, nil
		},
	})
	snapshot, _, _ := acquireSourceLease(ctx, source.ID, source.ConfigVersion, sourceActionSync)
	_, err := syncGitHubSource(ctx, snapshot, "user:7", "", "")
	if err == nil || err.Error() != errPagesSourceDigestInvalid {
		t.Fatalf("syncGitHubSource(invalid target) error = %v, want %q", err, errPagesSourceDigestInvalid)
	}
	if !isPermanentSourceSyncError(err) {
		t.Error("invalid sync target classification = retryable, want permanent")
	}
	_, runtime := mustLoadPagesSource(t, ctx, project.ID)
	if runtime.SyncStatus != pagesSourceStatusFailed || runtime.LastError != errPagesSourceDigestInvalid ||
		runtime.NextCheckAt == nil || runtime.NextCheckAt.Before(retryAt) || runtime.LeaseToken != "" {
		t.Errorf(
			"invalid sync target runtime = status:%q error:%q next:%v lease:%q, want failed/%q/deadline >= %v/cleared",
			runtime.SyncStatus, runtime.LastError, runtime.NextCheckAt, runtime.LeaseToken,
			errPagesSourceDigestInvalid, retryAt,
		)
	}
}

func TestGitHubCheckHandlerSkipsProviderFastRetry(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		retryDate bool
	}{
		{name: "bad request", status: http.StatusBadRequest},
		{name: "rate limited forbidden", status: http.StatusForbidden, retryDate: true},
		{name: "too many requests", status: http.StatusTooManyRequests, retryDate: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := setupPagesSourceTest(t)
			project := mustCreatePagesSourceProject(t, ctx, "github-handler-"+strings.ReplaceAll(test.name, " ", "-"))
			source, _ := mustConfigureGitHubSourceWithoutDispatch(t, ctx, project.ID, SourceUpdateInput{
				SourceType:    PagesSourceTypeGitHubRelease,
				RepositoryURL: "https://github.com/a/b",
			})
			var retryAt *time.Time
			if test.retryDate {
				deadline := time.Now().Add(2 * time.Hour).UTC()
				retryAt = &deadline
			}
			useFakeGitHubReleaseClient(t, &fakeGitHubReleaseClient{
				resolve: func(context.Context, githubrelease.ResolveRequest) (githubrelease.ResolveResult, error) {
					return githubrelease.ResolveResult{}, &githubrelease.Error{
						Kind: githubrelease.ErrMetadata, StatusCode: test.status, RetryAt: retryAt,
					}
				},
				download: func(context.Context, githubrelease.DownloadRequest) (*githubrelease.DownloadResult, error) {
					t.Fatal("Download called after provider check failure")
					return nil, nil
				},
			})
			raw, err := json.Marshal(SourceActionPayload{
				SourceID: source.ID, ConfigVersion: source.ConfigVersion,
				Action: sourceActionCheck, Actor: "user:7",
			})
			if err != nil {
				t.Fatalf("json.Marshal(check payload) error = %v, want nil", err)
			}
			result, err := (&SourceActionHandler{}).Execute(ctx, raw)
			if result != nil || err == nil || !errors.Is(err, asynq.SkipRetry) {
				t.Fatalf("SourceActionHandler.Execute(status %d) = result:%+v error:%v, want SkipRetry", test.status, result, err)
			}
			_, runtime := mustLoadPagesSource(t, ctx, project.ID)
			if runtime.SyncStatus != pagesSourceStatusFailed || runtime.NextCheckAt == nil || runtime.LeaseToken != "" {
				t.Errorf("provider failure runtime = status:%q next:%v lease:%q", runtime.SyncStatus, runtime.NextCheckAt, runtime.LeaseToken)
			}
			if retryAt != nil && (runtime.NextCheckAt == nil || runtime.NextCheckAt.Before(*retryAt)) {
				t.Errorf("provider failure NextCheckAt = %v, want deadline >= %v", runtime.NextCheckAt, *retryAt)
			}
		})
	}
}

func TestGitHubSyncActivatesWithMetadataRevisionAndPackageChecksum(t *testing.T) {
	ctx := setupPagesSourceSyncTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "github-sync")
	source, _ := mustConfigureGitHubSourceWithoutDispatch(t, ctx, project.ID, SourceUpdateInput{
		SourceType:    PagesSourceTypeGitHubRelease,
		RepositoryURL: "https://github.com/a/b",
		AssetName:     "dist.zip",
	})
	packageBytes := testPagesZip(t, map[string]string{"index.html": "github-v1"})
	packageHash := sha256.Sum256(packageBytes)
	updatedAt := time.Date(2026, 7, 19, 11, 0, 0, 0, time.UTC)
	retryAt := time.Now().Add(2 * time.Hour).UTC()
	release := githubrelease.Release{ID: "200", Tag: "release/v2"}
	asset := githubrelease.Asset{ID: "10", Name: "dist.zip", State: "uploaded", UpdatedAt: updatedAt}
	client := &fakeGitHubReleaseClient{
		resolve: func(context.Context, githubrelease.ResolveRequest) (githubrelease.ResolveResult, error) {
			return githubrelease.ResolveResult{Release: release, Asset: asset, RetryAt: &retryAt}, nil
		},
		download: func(_ context.Context, request githubrelease.DownloadRequest) (*githubrelease.DownloadResult, error) {
			path := filepath.Join(t.TempDir(), "download")
			if err := os.WriteFile(path, packageBytes, 0o600); err != nil {
				t.Fatalf("os.WriteFile(download) error = %v, want nil", err)
			}
			return &githubrelease.DownloadResult{
				Path: path, Size: int64(len(packageBytes)), SHA256: hex.EncodeToString(packageHash[:]),
			}, nil
		},
	}
	useFakeGitHubReleaseClient(t, client)
	snapshot, _, _ := acquireSourceLease(ctx, source.ID, source.ConfigVersion, sourceActionSync)
	outcome, err := syncGitHubSource(ctx, snapshot, "user:7", "", "")
	if err != nil {
		t.Fatalf("syncGitHubSource() error = %v, want nil", err)
	}
	if outcome == nil || outcome.Deployment == nil || outcome.Stale {
		t.Fatalf("syncGitHubSource() = %+v, want active deployment", outcome)
	}
	deployment, err := model.GetPagesDeploymentByID(ctx, outcome.Deployment.ID)
	if err != nil {
		t.Fatalf("GetPagesDeploymentByID(%d) error = %v, want nil", outcome.Deployment.ID, err)
	}
	if got, want := deployment.Checksum, hex.EncodeToString(packageHash[:]); got != want {
		t.Errorf("deployment Checksum = %q, want package hash %q", got, want)
	}
	if deployment.SourceRevision == nil || *deployment.SourceRevision == deployment.Checksum {
		t.Errorf("deployment SourceRevision = %v, want metadata revision distinct from package checksum", deployment.SourceRevision)
	}
	if got, want := deployment.SourceLabel, "release/v2"; got != want {
		t.Errorf("deployment SourceLabel = %q, want %q", got, want)
	}
	if deployment.SourceType != PagesSourceTypeGitHubRelease || deployment.TriggerType != pagesSourceTriggerManualSync ||
		deployment.CreatedBy != "user:7" {
		t.Errorf("deployment provenance = type:%q trigger:%q actor:%q", deployment.SourceType, deployment.TriggerType, deployment.CreatedBy)
	}
	if strings.Contains(deployment.SourceMeta, "http") || strings.Contains(deployment.SourceMeta, "token") {
		t.Errorf("deployment SourceMeta = %q, want no URL or token", deployment.SourceMeta)
	}
	if !strings.Contains(deployment.SourceMeta, `"tag":"release/v2"`) || strings.Contains(deployment.SourceMeta, `"label"`) {
		t.Errorf("deployment SourceMeta = %q, want provider-specific tag field", deployment.SourceMeta)
	}
	_, runtime := mustLoadPagesSource(t, ctx, project.ID)
	if runtime.NextCheckAt == nil || runtime.NextCheckAt.Before(retryAt) {
		t.Errorf("sync runtime NextCheckAt = %v, want server deadline >= %v", runtime.NextCheckAt, retryAt)
	}

	secondSnapshot, _, _ := acquireSourceLease(ctx, source.ID, source.ConfigVersion, sourceActionSync)
	second, err := syncGitHubSource(ctx, secondSnapshot, "user:7", "", "")
	if err != nil {
		t.Fatalf("syncGitHubSource(idempotent) error = %v, want nil", err)
	}
	if second == nil || !second.Reused || second.Deployment == nil || second.Deployment.ID != outcome.Deployment.ID {
		t.Errorf("syncGitHubSource(idempotent) = %+v, want reused deployment %d", second, outcome.Deployment.ID)
	}
}

func TestGitHubSyncActivatesExactConfirmedReplacement(t *testing.T) {
	ctx := setupPagesSourceSyncTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "github-confirm-replacement")
	source, _ := mustConfigureGitHubSourceWithoutDispatch(t, ctx, project.ID, SourceUpdateInput{
		SourceType:    PagesSourceTypeGitHubRelease,
		RepositoryURL: "https://github.com/a/b",
		AssetName:     "dist.zip",
	})
	release := githubrelease.Release{ID: "300", Tag: "v3"}
	asset := githubrelease.Asset{
		ID: "12", Name: "dist.zip", State: "uploaded",
		UpdatedAt: time.Date(2026, 7, 19, 13, 0, 0, 0, time.UTC),
	}
	target, err := buildGitHubSourceTarget(release, asset, nil)
	if err != nil {
		t.Fatalf("buildGitHubSourceTarget() error = %v, want nil", err)
	}
	if err := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).Where("source_id = ?", source.ID).Updates(map[string]any{
		"last_seen_revision":    target.Revision,
		"last_seen_detail":      target.DetailJSON,
		"last_applied_revision": strings.Repeat("a", 64),
		"last_applied_detail":   `{"provider":"github","release_id":"300","asset_id":"11","tag":"v3","asset_name":"dist.zip"}`,
		"sync_status":           pagesSourceStatusAttention,
	}).Error; err != nil {
		t.Fatalf("seed replacement cursor error = %v, want nil", err)
	}
	packageBytes := testPagesZip(t, map[string]string{"index.html": "confirmed-v3"})
	packageHash := sha256.Sum256(packageBytes)
	useFakeGitHubReleaseClient(t, &fakeGitHubReleaseClient{
		resolve: func(context.Context, githubrelease.ResolveRequest) (githubrelease.ResolveResult, error) {
			return githubrelease.ResolveResult{Release: release, Asset: asset}, nil
		},
		download: func(context.Context, githubrelease.DownloadRequest) (*githubrelease.DownloadResult, error) {
			path := filepath.Join(t.TempDir(), "confirmed.zip")
			if err := os.WriteFile(path, packageBytes, 0o600); err != nil {
				t.Fatalf("os.WriteFile(confirmed package) error = %v, want nil", err)
			}
			return &githubrelease.DownloadResult{
				Path: path, Size: int64(len(packageBytes)), SHA256: hex.EncodeToString(packageHash[:]),
			}, nil
		},
	})
	snapshot, _, _ := acquireSourceLease(ctx, source.ID, source.ConfigVersion, sourceActionSync)
	outcome, err := syncGitHubSource(ctx, snapshot, "user:9", "", target.Revision)
	if err != nil {
		t.Fatalf("syncGitHubSource(confirmed replacement) error = %v, want nil", err)
	}
	if outcome == nil || outcome.Deployment == nil || outcome.Stale {
		t.Fatalf("syncGitHubSource(confirmed replacement) = %+v, want active deployment", outcome)
	}
	_, runtime := mustLoadPagesSource(t, ctx, project.ID)
	if runtime.SyncStatus != pagesSourceStatusIdle || runtime.LastAppliedRevision != target.Revision {
		t.Errorf("confirmed replacement runtime = status:%q applied:%q, want idle/%q", runtime.SyncStatus, runtime.LastAppliedRevision, target.Revision)
	}
}

func TestSourceActionPayloadSeparatesSystemTargetAndUserConfirmation(t *testing.T) {
	handler := &SourceActionHandler{}
	revision := strings.Repeat("a", 64)
	invalid := []SourceActionPayload{
		{SourceID: 1, ConfigVersion: 1, Action: sourceActionSync, Actor: "user:1", TargetRevision: revision},
		{SourceID: 1, ConfigVersion: 1, Action: sourceActionSync, Actor: pagesSourceCreatedBySystem, TriggerType: pagesSourceTriggerManualSync},
		{SourceID: 1, ConfigVersion: 1, Action: sourceActionSync, Actor: pagesSourceCreatedBySystem, ConfirmedRevision: revision},
		{SourceID: 1, ConfigVersion: 1, Action: sourceActionSync, Actor: pagesSourceCreatedBySystem, TargetRevision: revision, ConfirmedRevision: revision},
	}
	for _, payload := range invalid {
		raw, _ := json.Marshal(payload)
		if normalized, err := handler.ValidatePayload(raw); err == nil {
			t.Errorf("ValidatePayload(%+v) = %s, nil; want error", payload, normalized)
		}
	}
	valid := []SourceActionPayload{
		{SourceID: 1, ConfigVersion: 1, Action: sourceActionSync, Actor: pagesSourceCreatedBySystem, TriggerType: pagesSourceTriggerScheduledAutoUpdate, TargetRevision: revision},
		{SourceID: 1, ConfigVersion: 1, Action: sourceActionSync, Actor: "user:1", TriggerType: pagesSourceTriggerManualSync, ConfirmedRevision: revision},
	}
	for _, payload := range valid {
		raw, _ := json.Marshal(payload)
		if _, err := handler.ValidatePayload(raw); err != nil {
			t.Errorf("ValidatePayload(%+v) error = %v, want nil", payload, err)
		}
	}
}

func TestGitHubProviderErrorsMapToSafeRetryClassification(t *testing.T) {
	tests := []struct {
		name      string
		provider  error
		want      string
		permanent bool
		skipRetry bool
	}{
		{
			name: "asset missing", provider: &githubrelease.Error{Kind: githubrelease.ErrAssetNotFound, StatusCode: 200},
			want: errPagesSourceReleaseNotFound, permanent: true, skipRetry: true,
		},
		{
			name: "digest mismatch", provider: &githubrelease.Error{Kind: githubrelease.ErrDigestMismatch, StatusCode: 200},
			want: errPagesSourceDigestMismatch, permanent: true, skipRetry: true,
		},
		{
			name: "rate limit", provider: &githubrelease.Error{
				Kind: githubrelease.ErrMetadata, StatusCode: 429,
				RetryAt: func() *time.Time { value := time.Now().Add(time.Hour); return &value }(),
			},
			want: errPagesSourceSyncFailed, permanent: false, skipRetry: true,
		},
		{
			name: "network", provider: &githubrelease.Error{Kind: githubrelease.ErrDownload},
			want: errPagesSourceSyncFailed, permanent: false, skipRetry: false,
		},
		{
			name: "forbidden without retry", provider: &githubrelease.Error{Kind: githubrelease.ErrMetadata, StatusCode: 403},
			want: errPagesSourceSyncFailed, permanent: true, skipRetry: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			domainErr := githubSourceDomainError(test.provider)
			if got := domainErr.Error(); got != test.want {
				t.Errorf("githubSourceDomainError() = %q, want %q", got, test.want)
			}
			var typedDomainError *githubSourceProviderDomainError
			if !errors.As(domainErr, &typedDomainError) {
				t.Fatalf("githubSourceDomainError() type = %T, want *githubSourceProviderDomainError", domainErr)
			}
			if got := typedDomainError.permanent; got != test.permanent {
				t.Errorf("githubSourceProviderDomainError.permanent = %t, want %t", got, test.permanent)
			}
			if got := shouldSkipGitHubActionRetry(domainErr); got != test.skipRetry {
				t.Errorf("shouldSkipGitHubActionRetry() = %t, want %t", got, test.skipRetry)
			}
			if strings.Contains(domainErr.Error(), "status=") || strings.Contains(domainErr.Error(), "repo=") {
				t.Errorf("githubSourceDomainError() = %q, want stable Pages message", domainErr)
			}
		})
	}
}

func TestGitHubSyncRejectsStaleConfirmationWithoutChangingActive(t *testing.T) {
	ctx := setupPagesSourceSyncTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "github-confirm-stale")
	oldActive := mustCreateActiveManualDeployment(t, ctx, project.ID, "old")
	source, _ := mustConfigureGitHubSourceWithoutDispatch(t, ctx, project.ID, SourceUpdateInput{
		SourceType:    PagesSourceTypeGitHubRelease,
		RepositoryURL: "https://github.com/a/b",
	})
	asset := githubrelease.Asset{ID: "2", Name: "dist.zip", State: "uploaded", UpdatedAt: time.Now().UTC()}
	useFakeGitHubReleaseClient(t, &fakeGitHubReleaseClient{
		resolve: func(context.Context, githubrelease.ResolveRequest) (githubrelease.ResolveResult, error) {
			return githubrelease.ResolveResult{Release: githubrelease.Release{ID: "1", Tag: "v1"}, Asset: asset}, nil
		},
		download: func(context.Context, githubrelease.DownloadRequest) (*githubrelease.DownloadResult, error) {
			t.Fatal("Download called for stale confirmation")
			return nil, nil
		},
	})
	snapshot, _, _ := acquireSourceLease(ctx, source.ID, source.ConfigVersion, sourceActionSync)
	_, err := syncGitHubSource(ctx, snapshot, "user:1", "", strings.Repeat("f", 64))
	if err == nil || err.Error() != errPagesSourceConfirmationStale {
		t.Errorf("syncGitHubSource(stale confirmation) error = %v, want %q", err, errPagesSourceConfirmationStale)
	}
	storedProject, loadErr := model.GetPagesProjectByID(ctx, project.ID)
	if loadErr != nil {
		t.Fatalf("GetPagesProjectByID() error = %v, want nil", loadErr)
	}
	if storedProject.ActiveDeploymentID == nil || *storedProject.ActiveDeploymentID != oldActive.ID {
		t.Errorf("ActiveDeploymentID = %v, want old active %d", storedProject.ActiveDeploymentID, oldActive.ID)
	}
}
