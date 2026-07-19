// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
)

func setupPagesSourceTest(t *testing.T) context.Context {
	t.Helper()
	cleanup := setupPagesTestDB(t)
	t.Cleanup(cleanup)
	sqlDB, err := db.DB(t.Context()).DB()
	if err != nil {
		t.Fatalf("db.DB().DB() error = %v, want nil", err)
	}
	// SQLite :memory: is scoped to one connection. Keeping one connection also
	// makes lease tests exercise the production CAS without creating empty
	// per-connection databases.
	sqlDB.SetMaxOpenConns(1)
	return t.Context()
}

func TestRevisionViewReadsLegacySourceDetailLabel(t *testing.T) {
	tests := []struct {
		name   string
		detail string
		want   string
	}{
		{
			name:   "remote",
			detail: `{"provider":"remote_url","label":"legacy.zip"}`,
			want:   "legacy.zip",
		},
		{
			name:   "github",
			detail: `{"provider":"github","label":"v1.2.3","asset_name":"dist.zip"}`,
			want:   "v1.2.3",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			view := revisionView(strings.Repeat("a", 64), test.detail)
			if view.Label != test.want {
				t.Errorf("revisionView(%s).Label = %q, want %q", test.name, view.Label, test.want)
			}
		})
	}
}

func mustCreatePagesSourceProject(t *testing.T, ctx context.Context, slug string) *model.PagesProject {
	t.Helper()
	view, err := CreateProject(ctx, Input{
		Name:      "Source " + slug,
		Slug:      slug,
		Enabled:   true,
		EntryFile: "index.html",
	})
	if err != nil {
		t.Fatalf("CreateProject(%q) error = %v, want nil", slug, err)
	}
	project, err := model.GetPagesProjectByID(ctx, view.ID)
	if err != nil {
		t.Fatalf("GetPagesProjectByID(%d) error = %v, want nil", view.ID, err)
	}
	return project
}

func mustConfigureRemoteSource(
	t *testing.T,
	ctx context.Context,
	projectID uint,
	remoteURL string,
	policy string,
) (*model.PagesProjectSource, *model.PagesProjectSourceRuntime) {
	t.Helper()
	_, err := UpdateSource(ctx, projectID, SourceUpdateInput{
		SourceType:          PagesSourceTypeRemoteURL,
		RemoteURLSet:        true,
		RemoteURL:           remoteURL,
		RemoteNetworkPolicy: policy,
	})
	if err != nil {
		t.Fatalf("UpdateSource(%d, %q) error = %v, want nil", projectID, remoteURL, err)
	}
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

func TestValidateRemoteSourceInputRejectsModeIncompatibleFields(t *testing.T) {
	tests := []struct {
		name  string
		input SourceUpdateInput
	}{
		{
			name: "missing source type",
			input: SourceUpdateInput{
				RemoteURLSet: true,
				RemoteURL:    "https://example.com/site.zip",
			},
		},
		{
			name: "github type reserved for phase two",
			input: SourceUpdateInput{
				SourceType:    PagesSourceTypeGitHubRelease,
				RepositoryURL: "https://github.com/example/site",
			},
		},
		{
			name: "remote rejects repository field",
			input: SourceUpdateInput{
				SourceType:    PagesSourceTypeRemoteURL,
				RemoteURLSet:  true,
				RemoteURL:     "https://example.com/site.zip",
				RepositoryURL: "https://github.com/example/site",
			},
		},
		{
			name: "remote rejects automatic updates",
			input: SourceUpdateInput{
				SourceType:        PagesSourceTypeRemoteURL,
				RemoteURLSet:      true,
				RemoteURL:         "https://example.com/site.zip",
				AutoUpdateEnabled: true,
			},
		},
		{
			name: "url value requires replacement flag",
			input: SourceUpdateInput{
				SourceType: PagesSourceTypeRemoteURL,
				RemoteURL:  "https://example.com/site.zip",
			},
		},
		{
			name: "invalid network policy",
			input: SourceUpdateInput{
				SourceType:          PagesSourceTypeRemoteURL,
				RemoteURLSet:        true,
				RemoteURL:           "https://example.com/site.zip",
				RemoteNetworkPolicy: "private",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateRemoteSourceInput(test.input); err == nil {
				t.Errorf("validateRemoteSourceInput(%+v) error = nil, want non-nil", test.input)
			}
		})
	}
}

func TestUpdateSourceNewRemoteRequiresExplicitURL(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "remote-requires-url")
	_, err := UpdateSource(ctx, project.ID, SourceUpdateInput{
		SourceType: PagesSourceTypeRemoteURL,
	})
	if err == nil {
		t.Fatal("UpdateSource(new remote without URL) error = nil, want non-nil")
	}
	if got, want := err.Error(), errPagesSourceRemoteURLRequired; got != want {
		t.Errorf("UpdateSource(new remote without URL) error = %q, want %q", got, want)
	}
}

func TestRemoteSourceCRUDPreservesSecretAndResetsRuntimeByIdentity(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "remote-crud")
	firstURL := "https://Artifacts.Example.com:443/dist/site.zip?token=first-secret&expires=1"
	source, runtime := mustConfigureRemoteSource(t, ctx, project.ID, firstURL, RemoteNetworkPolicyPublic)

	if got, want := source.ConfigVersion, 1; got != want {
		t.Errorf("new source ConfigVersion = %d, want %d", got, want)
	}
	if got, want := runtime.SyncStatus, pagesSourceStatusIdle; got != want {
		t.Errorf("new runtime SyncStatus = %q, want %q", got, want)
	}
	view, err := GetSource(ctx, project.ID)
	if err != nil {
		t.Fatalf("GetSource(%d) error = %v, want nil", project.ID, err)
	}
	if got, want := view.DisplayURL, "https://Artifacts.Example.com:443/dist/site.zip?***"; got != want {
		t.Errorf("GetSource(%d).DisplayURL = %q, want %q", project.ID, got, want)
	}
	encodedView, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("json.Marshal(GetSource(%d)) error = %v, want nil", project.ID, err)
	}
	if strings.Contains(string(encodedView), "first-secret") || strings.Contains(string(encodedView), "expires=1") {
		t.Errorf("GetSource(%d) JSON = %s, want credential-free view", project.ID, encodedView)
	}
	if _, err := UpdateSource(ctx, project.ID, SourceUpdateInput{
		SourceType:          PagesSourceTypeRemoteURL,
		RemoteURLSet:        true,
		RemoteURL:           firstURL,
		RemoteNetworkPolicy: RemoteNetworkPolicyPublic,
	}); err != nil {
		t.Fatalf("UpdateSource(%d, no-op) error = %v, want nil", project.ID, err)
	}
	var unchangedSource model.PagesProjectSource
	if err := db.DB(ctx).Where("id = ?", source.ID).First(&unchangedSource).Error; err != nil {
		t.Fatalf("load no-op source error = %v, want nil", err)
	}
	if got, want := unchangedSource.ConfigVersion, source.ConfigVersion; got != want {
		t.Errorf("no-op source ConfigVersion = %d, want unchanged %d", got, want)
	}

	seenRevision := strings.Repeat("a", 64)
	appliedRevision := strings.Repeat("b", 64)
	future := time.Now().Add(time.Hour)
	if err := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ?", source.ID).
		Updates(map[string]any{
			"last_seen_revision":    seenRevision,
			"last_seen_detail":      `{"provider":"remote_url","display_name":"new.zip"}`,
			"last_applied_revision": appliedRevision,
			"last_applied_detail":   `{"provider":"remote_url","display_name":"old.zip"}`,
			"sync_status":           pagesSourceStatusSyncing,
			"lease_token":           "in-flight",
			"lease_expires_at":      &future,
		}).Error; err != nil {
		t.Fatalf("seed source runtime error = %v, want nil", err)
	}

	// Omit the secret URL while changing policy. The stored URL and cursor must
	// survive, while the in-flight lease is fenced.
	if _, err := UpdateSource(ctx, project.ID, SourceUpdateInput{
		SourceType:          PagesSourceTypeRemoteURL,
		RemoteURLSet:        false,
		RemoteNetworkPolicy: RemoteNetworkPolicyTrustedInternal,
	}); err != nil {
		t.Fatalf("UpdateSource(%d, preserve URL) error = %v, want nil", project.ID, err)
	}
	preservedSource, preservedRuntime, err := loadSourceByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("loadSourceByProject(%d) error = %v, want nil", project.ID, err)
	}
	if got, want := preservedSource.RemoteURL, firstURL; got != want {
		t.Errorf("preserved RemoteURL = %q, want %q", got, want)
	}
	if got, want := preservedSource.ConfigVersion, 2; got != want {
		t.Errorf("preserved source ConfigVersion = %d, want %d", got, want)
	}
	if got, want := preservedSource.SourceIdentity, source.SourceIdentity; got != want {
		t.Errorf("preserved source identity = %q, want %q", got, want)
	}
	if got, want := preservedRuntime.LastSeenRevision, seenRevision; got != want {
		t.Errorf("preserved LastSeenRevision = %q, want %q", got, want)
	}
	if got, want := preservedRuntime.SyncStatus, pagesSourceStatusUpdateAvailable; got != want {
		t.Errorf("preserved runtime SyncStatus = %q, want %q", got, want)
	}
	if preservedRuntime.LeaseToken != "" || preservedRuntime.LeaseExpiresAt != nil {
		t.Errorf("preserved runtime lease = (%q, %v), want cleared", preservedRuntime.LeaseToken, preservedRuntime.LeaseExpiresAt)
	}

	// Replacing only the query secret keeps the canonical identity and cursors.
	queryReplacementURL := "https://artifacts.example.com/dist/site.zip?token=second-secret"
	if _, err := UpdateSource(ctx, project.ID, SourceUpdateInput{
		SourceType:          PagesSourceTypeRemoteURL,
		RemoteURLSet:        true,
		RemoteURL:           queryReplacementURL,
		RemoteNetworkPolicy: RemoteNetworkPolicyTrustedInternal,
	}); err != nil {
		t.Fatalf("UpdateSource(%d, query replacement) error = %v, want nil", project.ID, err)
	}
	querySource, queryRuntime, err := loadSourceByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("loadSourceByProject(%d) after query replacement error = %v, want nil", project.ID, err)
	}
	if got, want := querySource.SourceIdentity, source.SourceIdentity; got != want {
		t.Errorf("query replacement identity = %q, want %q", got, want)
	}
	if got, want := queryRuntime.LastSeenRevision, seenRevision; got != want {
		t.Errorf("query replacement LastSeenRevision = %q, want %q", got, want)
	}

	// Replacing the path changes identity and clears all remote cursors.
	pathReplacementURL := "https://artifacts.example.com/dist/other.zip?token=third-secret"
	if _, err := UpdateSource(ctx, project.ID, SourceUpdateInput{
		SourceType:          PagesSourceTypeRemoteURL,
		RemoteURLSet:        true,
		RemoteURL:           pathReplacementURL,
		RemoteNetworkPolicy: RemoteNetworkPolicyTrustedInternal,
	}); err != nil {
		t.Fatalf("UpdateSource(%d, path replacement) error = %v, want nil", project.ID, err)
	}
	pathSource, pathRuntime, err := loadSourceByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("loadSourceByProject(%d) after path replacement error = %v, want nil", project.ID, err)
	}
	if pathSource.SourceIdentity == source.SourceIdentity {
		t.Errorf("path replacement identity = %q, want a new identity", pathSource.SourceIdentity)
	}
	if pathRuntime.LastSeenRevision != "" || pathRuntime.LastAppliedRevision != "" {
		t.Errorf("path replacement cursors = (%q, %q), want empty", pathRuntime.LastSeenRevision, pathRuntime.LastAppliedRevision)
	}
	if got, want := pathRuntime.SyncStatus, pagesSourceStatusIdle; got != want {
		t.Errorf("path replacement SyncStatus = %q, want %q", got, want)
	}
	pathView, err := GetSource(ctx, project.ID)
	if err != nil {
		t.Fatalf("GetSource(%d) after path replacement error = %v, want nil", project.ID, err)
	}
	pathJSON, err := json.Marshal(pathView)
	if err != nil {
		t.Fatalf("json.Marshal(path view) error = %v, want nil", err)
	}
	for _, secret := range []string{"first-secret", "second-secret", "third-secret"} {
		if strings.Contains(string(pathJSON), secret) {
			t.Errorf("path view JSON = %s, want no secret %q", pathJSON, secret)
		}
	}
}

func TestRemoteSourceIdentityIgnoresQueryAndNormalizesDefaultPort(t *testing.T) {
	first, err := parseRemoteSourceURL("HTTPS://Artifacts.Example.com:443/dist/../dist/site.zip?token=one")
	if err != nil {
		t.Fatalf("parseRemoteSourceURL(first) error = %v, want nil", err)
	}
	second, err := parseRemoteSourceURL("https://artifacts.example.com/dist/site.zip?token=two")
	if err != nil {
		t.Fatalf("parseRemoteSourceURL(second) error = %v, want nil", err)
	}
	if got, want := remoteSourceIdentity(first), remoteSourceIdentity(second); got != want {
		t.Errorf("remoteSourceIdentity(first) = %q, want %q", got, want)
	}
}

func TestDeleteSourceIsIdempotentAndKeepsDeploymentState(t *testing.T) {
	ctx := setupPagesSourceTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "source-delete")
	source, _ := mustConfigureRemoteSource(
		t,
		ctx,
		project.ID,
		"https://example.com/site.zip?token=delete-secret",
		RemoteNetworkPolicyPublic,
	)
	deployment := &model.PagesDeployment{
		ProjectID:        project.ID,
		DeploymentNumber: 1,
		Checksum:         strings.Repeat("c", 64),
		Status:           model.PagesDeploymentStatusActive,
		CreatedBy:        "user:1",
		SourceType:       "manual_upload",
		TriggerType:      "manual_upload",
	}
	if err := db.DB(ctx).Create(deployment).Error; err != nil {
		t.Fatalf("create deployment error = %v, want nil", err)
	}
	if err := db.DB(ctx).Model(&model.PagesProject{}).
		Where("id = ?", project.ID).
		Update("active_deployment_id", deployment.ID).Error; err != nil {
		t.Fatalf("set active deployment error = %v, want nil", err)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		view, err := DeleteSource(ctx, project.ID)
		if err != nil {
			t.Fatalf("DeleteSource(%d), attempt %d error = %v, want nil", project.ID, attempt, err)
		}
		if got, want := view.SourceType, PagesSourceTypeManual; got != want {
			t.Errorf("DeleteSource(%d), attempt %d SourceType = %q, want %q", project.ID, attempt, got, want)
		}
	}
	var sourceCount, runtimeCount, deploymentCount int64
	if err := db.DB(ctx).Model(&model.PagesProjectSource{}).Where("id = ?", source.ID).Count(&sourceCount).Error; err != nil {
		t.Fatalf("count source error = %v, want nil", err)
	}
	if err := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).Where("source_id = ?", source.ID).Count(&runtimeCount).Error; err != nil {
		t.Fatalf("count runtime error = %v, want nil", err)
	}
	if err := db.DB(ctx).Model(&model.PagesDeployment{}).Where("id = ?", deployment.ID).Count(&deploymentCount).Error; err != nil {
		t.Fatalf("count deployment error = %v, want nil", err)
	}
	if sourceCount != 0 || runtimeCount != 0 || deploymentCount != 1 {
		t.Errorf("DeleteSource counts = source:%d runtime:%d deployment:%d, want 0, 0, 1", sourceCount, runtimeCount, deploymentCount)
	}
	storedProject, err := model.GetPagesProjectByID(ctx, project.ID)
	if err != nil {
		t.Fatalf("GetPagesProjectByID(%d) error = %v, want nil", project.ID, err)
	}
	if storedProject.ActiveDeploymentID == nil || *storedProject.ActiveDeploymentID != deployment.ID {
		t.Errorf("active deployment = %v, want %d", storedProject.ActiveDeploymentID, deployment.ID)
	}

	manual, err := GetSource(ctx, project.ID)
	if err != nil {
		t.Fatalf("GetSource(%d) after delete error = %v, want nil", project.ID, err)
	}
	if got, want := fmt.Sprint(manual.SourceType), PagesSourceTypeManual; got != want {
		t.Errorf("GetSource(%d).SourceType = %q, want %q", project.ID, got, want)
	}
}
