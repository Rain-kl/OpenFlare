// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/upload"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/pagesarchive"
	"gorm.io/gorm"
)

func setupPagesSourceSyncTest(t *testing.T) context.Context {
	t.Helper()
	ctx := setupPagesSourceTest(t)
	_, disableStorage := setupPagesStorageMock(t)
	t.Cleanup(disableStorage)
	return ctx
}

func mustAcquireRemoteSyncLease(
	t *testing.T,
	ctx context.Context,
	source *model.PagesProjectSource,
) *sourceExecutionSnapshot {
	t.Helper()
	snapshot, outcome, err := acquireSourceLease(ctx, source.ID, source.ConfigVersion, sourceActionSync)
	if err != nil {
		t.Fatalf("acquireSourceLease(source=%d) error = %v, want nil", source.ID, err)
	}
	if got, want := outcome, sourceLeaseAcquired; got != want {
		t.Fatalf("acquireSourceLease(source=%d) outcome = %q, want %q", source.ID, got, want)
	}
	if snapshot == nil {
		t.Fatalf("acquireSourceLease(source=%d) snapshot = nil, want non-nil", source.ID)
	}
	return snapshot
}

func mustCreateActiveManualDeployment(
	t *testing.T,
	ctx context.Context,
	projectID uint,
	content string,
) *model.PagesDeployment {
	t.Helper()
	view, err := UploadDeployment(
		ctx,
		projectID,
		testPagesMultipartFile(t, "manual.zip", testPagesZip(t, map[string]string{"index.html": content})),
		"user:1",
	)
	if err != nil {
		t.Fatalf("UploadDeployment(project=%d) error = %v, want nil", projectID, err)
	}
	if _, err := ActivateDeploymentAs(ctx, projectID, view.ID, "user:1"); err != nil {
		t.Fatalf("ActivateDeploymentAs(project=%d, deployment=%d) error = %v, want nil", projectID, view.ID, err)
	}
	deployment, err := model.GetPagesDeploymentByID(ctx, view.ID)
	if err != nil {
		t.Fatalf("GetPagesDeploymentByID(%d) error = %v, want nil", view.ID, err)
	}
	return deployment
}

func newPagesArchiveServer(t *testing.T, status int, body []byte, beforeWrite func() error) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if beforeWrite != nil {
			if err := beforeWrite(); err != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
		writer.Header().Set("Content-Type", "application/zip")
		writer.Header().Set("Content-Disposition", `attachment; filename="site.zip"`)
		writer.WriteHeader(status)
		_, _ = writer.Write(body)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestSyncRemoteSourceAtomicallyActivatesAndReusesChecksum(t *testing.T) {
	ctx := setupPagesSourceSyncTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "sync-success")
	packageBytes := testPagesZip(t, map[string]string{
		"index.html":    "remote-v1",
		"assets/app.js": "console.log('ok')",
	})
	server := newPagesArchiveServer(t, http.StatusOK, packageBytes, nil)
	secret := "sync-query-secret"
	source, _ := mustConfigureRemoteSource(
		t,
		ctx,
		project.ID,
		server.URL+"/site.zip?token="+secret,
		true,
	)

	firstSnapshot := mustAcquireRemoteSyncLease(t, ctx, source)
	first, err := syncRemoteSource(ctx, firstSnapshot, "user:42")
	if err != nil {
		t.Fatalf("syncRemoteSource(first) error = %v, want nil", err)
	}
	if first == nil || first.Stale || first.Reused || first.Deployment == nil {
		t.Fatalf("syncRemoteSource(first) = %+v, want new active deployment", first)
	}
	storedProject, err := model.GetPagesProjectByID(ctx, project.ID)
	if err != nil {
		t.Fatalf("GetPagesProjectByID(%d) error = %v, want nil", project.ID, err)
	}
	if storedProject.ActiveDeploymentID == nil || *storedProject.ActiveDeploymentID != first.Deployment.ID {
		t.Fatalf("project ActiveDeploymentID = %v, want %d", storedProject.ActiveDeploymentID, first.Deployment.ID)
	}
	deployment, err := model.GetPagesDeploymentByID(ctx, first.Deployment.ID)
	if err != nil {
		t.Fatalf("GetPagesDeploymentByID(%d) error = %v, want nil", first.Deployment.ID, err)
	}
	expectedHash := sha256.Sum256(packageBytes)
	if got, want := deployment.Checksum, hex.EncodeToString(expectedHash[:]); got != want {
		t.Errorf("deployment Checksum = %q, want %q", got, want)
	}
	if got, want := deployment.Status, model.PagesDeploymentStatusActive; got != want {
		t.Errorf("deployment Status = %q, want %q", got, want)
	}
	if got, want := deployment.SourceType, PagesSourceTypeRemoteURL; got != want {
		t.Errorf("deployment SourceType = %q, want %q", got, want)
	}
	if deployment.SourceIdentity == nil || *deployment.SourceIdentity != source.SourceIdentity {
		t.Errorf("deployment SourceIdentity = %v, want %q", deployment.SourceIdentity, source.SourceIdentity)
	}
	if deployment.SourceRevision == nil || *deployment.SourceRevision != deployment.Checksum {
		t.Errorf("deployment SourceRevision = %v, want %q", deployment.SourceRevision, deployment.Checksum)
	}
	if got, want := deployment.CreatedBy, "user:42"; got != want {
		t.Errorf("deployment CreatedBy = %q, want %q", got, want)
	}
	if got, want := deployment.TriggerType, pagesSourceTriggerManualSync; got != want {
		t.Errorf("deployment TriggerType = %q, want %q", got, want)
	}
	if strings.Contains(deployment.SourceMeta, secret) || strings.Contains(deployment.SourceLabel, secret) {
		t.Errorf("deployment provenance = label:%q meta:%q, want no query secret", deployment.SourceLabel, deployment.SourceMeta)
	}
	var runtime model.PagesProjectSourceRuntime
	if err := db.DB(ctx).Where("source_id = ?", source.ID).First(&runtime).Error; err != nil {
		t.Fatalf("load source runtime error = %v, want nil", err)
	}
	if got, want := runtime.SyncStatus, pagesSourceStatusIdle; got != want {
		t.Errorf("runtime SyncStatus = %q, want %q", got, want)
	}
	if got, want := runtime.LastAppliedRevision, deployment.Checksum; got != want {
		t.Errorf("runtime LastAppliedRevision = %q, want %q", got, want)
	}
	if runtime.LeaseToken != "" || runtime.LeaseExpiresAt != nil {
		t.Errorf("runtime lease = (%q, %v), want cleared", runtime.LeaseToken, runtime.LeaseExpiresAt)
	}
	var uploadRecord model.Upload
	if err := db.DB(ctx).First(&uploadRecord, deployment.UploadID).Error; err != nil {
		t.Fatalf("load deployment upload %d error = %v, want nil", deployment.UploadID, err)
	}
	if got, want := uploadRecord.Status, model.UploadStatusUsed; got != want {
		t.Errorf("deployment upload Status = %q, want %q", got, want)
	}
	if got, want := uploadRecord.Type, upload.ReservedPagesDeploymentType; got != want {
		t.Errorf("deployment upload Type = %q, want %q", got, want)
	}
	if got, want := fmt.Sprint(uploadRecord.Metadata.Extra[pagesSourceIDMetadataKey]), fmt.Sprint(source.ID); got != want {
		t.Errorf("deployment upload pages_source_id = %q, want %q", got, want)
	}

	secondSnapshot := mustAcquireRemoteSyncLease(t, ctx, source)
	second, err := syncRemoteSource(ctx, secondSnapshot, "user:42")
	if err != nil {
		t.Fatalf("syncRemoteSource(second) error = %v, want nil", err)
	}
	if second == nil || second.Stale || !second.Reused || second.Deployment == nil {
		t.Fatalf("syncRemoteSource(second) = %+v, want reused active deployment", second)
	}
	if got, want := second.Deployment.ID, first.Deployment.ID; got != want {
		t.Errorf("reused deployment ID = %d, want %d", got, want)
	}
	var deploymentCount, uploadCount int64
	if err := db.DB(ctx).Model(&model.PagesDeployment{}).Where("project_id = ?", project.ID).Count(&deploymentCount).Error; err != nil {
		t.Fatalf("count source deployments error = %v, want nil", err)
	}
	if err := db.DB(ctx).Model(&model.Upload{}).Count(&uploadCount).Error; err != nil {
		t.Fatalf("count source uploads error = %v, want nil", err)
	}
	if got, want := deploymentCount, int64(1); got != want {
		t.Errorf("deployment count after identical sync = %d, want %d", got, want)
	}
	if got, want := uploadCount, int64(1); got != want {
		t.Errorf("upload count after identical sync = %d, want %d", got, want)
	}
}

func TestSyncRemoteSourceDownloadFailureKeepsOldActive(t *testing.T) {
	ctx := setupPagesSourceSyncTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "sync-download-fail")
	oldActive := mustCreateActiveManualDeployment(t, ctx, project.ID, "old-active")
	server := newPagesArchiveServer(t, http.StatusBadGateway, []byte("upstream failed"), nil)
	secret := "download-failure-secret"
	source, _ := mustConfigureRemoteSource(
		t,
		ctx,
		project.ID,
		server.URL+"/site.zip?token="+secret,
		true,
	)

	_, err := syncRemoteSource(ctx, mustAcquireRemoteSyncLease(t, ctx, source), "user:2")
	if err == nil {
		t.Fatal("syncRemoteSource(download failure) error = nil, want non-nil")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("syncRemoteSource(download failure) error = %q, want no query secret", err)
	}
	assertPagesSyncFailureState(t, ctx, project.ID, source.ID, oldActive.ID, 1)
}

func TestSyncRemoteSourceArchiveFailureKeepsOldActive(t *testing.T) {
	ctx := setupPagesSourceSyncTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "sync-archive-fail")
	oldActive := mustCreateActiveManualDeployment(t, ctx, project.ID, "old-active")
	server := newPagesArchiveServer(t, http.StatusOK, []byte("not-a-valid-zip"), nil)
	secret := "archive-failure-secret"
	source, _ := mustConfigureRemoteSource(
		t,
		ctx,
		project.ID,
		server.URL+"/site.zip?token="+secret,
		true,
	)

	_, err := syncRemoteSource(ctx, mustAcquireRemoteSyncLease(t, ctx, source), "user:3")
	if err == nil {
		t.Fatal("syncRemoteSource(archive failure) error = nil, want non-nil")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("syncRemoteSource(archive failure) error = %q, want no query secret", err)
	}
	assertPagesSyncFailureState(t, ctx, project.ID, source.ID, oldActive.ID, 1)
}

func TestSyncRemoteSourceFinalFenceCompensatesIngest(t *testing.T) {
	ctx := setupPagesSourceSyncTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "sync-final-fence")
	oldActive := mustCreateActiveManualDeployment(t, ctx, project.ID, "old-active")
	packageBytes := testPagesZip(t, map[string]string{"index.html": "never-activate"})
	mutationResult := make(chan error, 1)
	server := newPagesArchiveServer(t, http.StatusOK, packageBytes, func() error {
		err := db.DB(context.Background()).Model(&model.PagesProject{}).
			Where("id = ?", project.ID).
			Update("content_config_version", gorm.Expr("content_config_version + 1")).Error
		mutationResult <- err
		return err
	})
	source, _ := mustConfigureRemoteSource(
		t,
		ctx,
		project.ID,
		server.URL+"/site.zip?token=final-fence-secret",
		true,
	)

	outcome, err := syncRemoteSource(ctx, mustAcquireRemoteSyncLease(t, ctx, source), "user:4")
	if err != nil {
		t.Fatalf("syncRemoteSource(final fence) error = %v, want nil stale outcome", err)
	}
	select {
	case mutationErr := <-mutationResult:
		if mutationErr != nil {
			t.Fatalf("content version mutation error = %v, want nil", mutationErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("content version mutation was not observed")
	}
	if outcome == nil || !outcome.Stale || outcome.Deployment != nil {
		t.Fatalf("syncRemoteSource(final fence) = %+v, want stale outcome without deployment", outcome)
	}
	storedProject, err := model.GetPagesProjectByID(ctx, project.ID)
	if err != nil {
		t.Fatalf("GetPagesProjectByID(%d) error = %v, want nil", project.ID, err)
	}
	if storedProject.ActiveDeploymentID == nil || *storedProject.ActiveDeploymentID != oldActive.ID {
		t.Errorf("ActiveDeploymentID after final fence = %v, want %d", storedProject.ActiveDeploymentID, oldActive.ID)
	}
	var deployments []model.PagesDeployment
	if err := db.DB(ctx).Where("project_id = ?", project.ID).Find(&deployments).Error; err != nil {
		t.Fatalf("list deployments after final fence error = %v, want nil", err)
	}
	if got, want := len(deployments), 1; got != want {
		t.Errorf("deployment count after final fence = %d, want %d", got, want)
	}
	var uploads []model.Upload
	if err := db.DB(ctx).Order("id asc").Find(&uploads).Error; err != nil {
		t.Fatalf("list uploads after final fence error = %v, want nil", err)
	}
	var compensated *model.Upload
	for index := range uploads {
		if fmt.Sprint(uploads[index].Metadata.Extra[pagesSourceIDMetadataKey]) == fmt.Sprint(source.ID) {
			compensated = &uploads[index]
			break
		}
	}
	if compensated == nil {
		t.Fatalf("source upload after final fence = nil, want compensated upload record")
	}
	if got, want := compensated.Status, model.UploadStatusDeleted; got != want {
		t.Errorf("compensated upload Status = %q, want %q", got, want)
	}
	var danglingCount int64
	if err := db.DB(ctx).Model(&model.PagesDeployment{}).
		Where("upload_id = ?", compensated.ID).
		Count(&danglingCount).Error; err != nil {
		t.Fatalf("count compensated upload references error = %v, want nil", err)
	}
	if got, want := danglingCount, int64(0); got != want {
		t.Errorf("deployments referencing compensated upload = %d, want %d", got, want)
	}
}

func TestCommitSourceDeploymentRechecksLeaseAfterUploadLocks(t *testing.T) {
	ctx := setupPagesSourceSyncTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "sync-expiry-recheck")
	packageBytes := testPagesZip(t, map[string]string{"index.html": "expiry-recheck"})
	server := newPagesArchiveServer(t, http.StatusOK, packageBytes, nil)
	source, _ := mustConfigureRemoteSource(
		t,
		ctx,
		project.ID,
		server.URL+"/site.zip",
		true,
	)
	first, err := syncRemoteSource(ctx, mustAcquireRemoteSyncLease(t, ctx, source), "user:5")
	if err != nil || first == nil || first.Deployment == nil {
		t.Fatalf("syncRemoteSource(seed) = (%+v, %v), want deployment", first, err)
	}
	deployment, err := model.GetPagesDeploymentByID(ctx, first.Deployment.ID)
	if err != nil {
		t.Fatalf("GetPagesDeploymentByID(%d) error = %v, want nil", first.Deployment.ID, err)
	}
	if err := db.DB(ctx).Model(&model.PagesProject{}).
		Where("id = ?", project.ID).
		Update("active_deployment_id", nil).Error; err != nil {
		t.Fatalf("clear active deployment error = %v, want nil", err)
	}
	if err := db.DB(ctx).Model(&model.PagesDeployment{}).
		Where("id = ?", deployment.ID).
		Update("status", model.PagesDeploymentStatusUploaded).Error; err != nil {
		t.Fatalf("reset deployment status error = %v, want nil", err)
	}

	snapshot := mustAcquireRemoteSyncLease(t, ctx, source)
	expiresAt := time.Now().Add(time.Hour)
	if err := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ? AND lease_token = ?", source.ID, snapshot.LeaseToken).
		Update("lease_expires_at", &expiresAt).Error; err != nil {
		t.Fatalf("set deterministic lease expiry error = %v, want nil", err)
	}
	originalNow := sourceCommitNow
	nowCalls := 0
	sourceCommitNow = func() time.Time {
		nowCalls++
		if nowCalls == 1 {
			return expiresAt.Add(-time.Second)
		}
		return expiresAt.Add(time.Second)
	}
	t.Cleanup(func() { sourceCommitNow = originalNow })

	_, _, _, err = commitSourceDeployment(
		ctx,
		snapshot,
		deployment.Checksum,
		deployment.Checksum,
		sourceDetail{Provider: PagesSourceTypeRemoteURL, DisplayName: deployment.SourceLabel},
		deployment.SourceMeta,
		"user:5",
		&deploymentManifest{},
		upload.IngestResult{},
		false,
		nil,
	)
	if !errors.Is(err, errSourceFinalFence) {
		t.Fatalf("commitSourceDeployment(expired after upload lock) error = %v, want %v", err, errSourceFinalFence)
	}
	if nowCalls != 2 {
		t.Fatalf("sourceCommitNow calls = %d, want runtime-lock and post-upload checks", nowCalls)
	}
	storedProject, err := model.GetPagesProjectByID(ctx, project.ID)
	if err != nil {
		t.Fatalf("GetPagesProjectByID(%d) error = %v, want nil", project.ID, err)
	}
	if storedProject.ActiveDeploymentID != nil {
		t.Errorf("ActiveDeploymentID after expiry recheck = %v, want nil", storedProject.ActiveDeploymentID)
	}
	storedDeployment, err := model.GetPagesDeploymentByID(ctx, deployment.ID)
	if err != nil {
		t.Fatalf("GetPagesDeploymentByID(%d) after expiry error = %v, want nil", deployment.ID, err)
	}
	if got, want := storedDeployment.Status, model.PagesDeploymentStatusUploaded; got != want {
		t.Errorf("deployment status after expiry recheck = %q, want %q", got, want)
	}
}

func TestCompensateSourceIngestSurvivesCanceledParentContext(t *testing.T) {
	ctx := setupPagesSourceSyncTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "sync-canceled-compensation")
	source, _ := mustConfigureRemoteSource(
		t,
		ctx,
		project.ID,
		"https://example.com/site.zip",
		false,
	)
	packageBytes := testPagesZip(t, map[string]string{"index.html": "cancel-compensation"})
	packagePath := filepath.Join(t.TempDir(), "site.zip")
	if err := os.WriteFile(packagePath, packageBytes, 0o600); err != nil {
		t.Fatalf("write test package error = %v, want nil", err)
	}
	digest := sha256.Sum256(packageBytes)
	result, err := ingestPagesDeploymentPackageWithSource(
		ctx,
		packagePath,
		hex.EncodeToString(digest[:]),
		project.ID,
		source.ID,
		"site.zip",
		pagesarchive.FormatZip,
	)
	if err != nil {
		t.Fatalf("ingestPagesDeploymentPackageWithSource() error = %v, want nil", err)
	}
	if !result.Created {
		t.Fatal("ingest result Created = false, want a compensatable record")
	}

	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	compensateSourceIngest(canceledCtx, &sourceExecutionSnapshot{
		ProjectID: project.ID,
		SourceID:  source.ID,
	}, &sourceIngestState{
		Result:    result,
		HasIngest: true,
	})

	var uploadRecord model.Upload
	if err := db.DB(ctx).Where("id = ?", result.Upload.ID).First(&uploadRecord).Error; err != nil {
		t.Fatalf("load compensated upload error = %v, want nil", err)
	}
	if got, want := uploadRecord.Status, model.UploadStatusDeleted; got != want {
		t.Errorf("compensated upload status = %q, want %q", got, want)
	}
}

func assertPagesSyncFailureState(
	t *testing.T,
	ctx context.Context,
	projectID uint,
	sourceID uint,
	oldActiveID uint,
	wantDeploymentCount int64,
) {
	t.Helper()
	project, err := model.GetPagesProjectByID(ctx, projectID)
	if err != nil {
		t.Fatalf("GetPagesProjectByID(%d) error = %v, want nil", projectID, err)
	}
	if project.ActiveDeploymentID == nil || *project.ActiveDeploymentID != oldActiveID {
		t.Errorf("project %d ActiveDeploymentID = %v, want %d", projectID, project.ActiveDeploymentID, oldActiveID)
	}
	var deploymentCount int64
	if err := db.DB(ctx).Model(&model.PagesDeployment{}).
		Where("project_id = ?", projectID).
		Count(&deploymentCount).Error; err != nil {
		t.Fatalf("count project %d deployments error = %v, want nil", projectID, err)
	}
	if got, want := deploymentCount, wantDeploymentCount; got != want {
		t.Errorf("project %d deployment count = %d, want %d", projectID, got, want)
	}
	var runtime model.PagesProjectSourceRuntime
	if err := db.DB(ctx).Where("source_id = ?", sourceID).First(&runtime).Error; err != nil {
		t.Fatalf("load source %d runtime error = %v, want nil", sourceID, err)
	}
	if got, want := runtime.SyncStatus, pagesSourceStatusFailed; got != want {
		t.Errorf("source %d runtime SyncStatus = %q, want %q", sourceID, got, want)
	}
	if runtime.LeaseToken != "" || runtime.LeaseExpiresAt != nil {
		t.Errorf("source %d runtime lease = (%q, %v), want cleared", sourceID, runtime.LeaseToken, runtime.LeaseExpiresAt)
	}
}

func TestCommitSourceDeploymentRejectsDeletedTargetUpload(t *testing.T) {
	ctx := setupPagesSourceSyncTest(t)
	project := mustCreatePagesSourceProject(t, ctx, "sync-deleted-upload")
	packageBytes := testPagesZip(t, map[string]string{"index.html": "content"})
	server := newPagesArchiveServer(t, http.StatusOK, packageBytes, nil)
	source, _ := mustConfigureRemoteSource(
		t,
		ctx,
		project.ID,
		server.URL+"/site.zip",
		true,
	)
	snapshot := mustAcquireRemoteSyncLease(t, ctx, source)

	// A pre-existing source deployment whose upload was removed must never be
	// reactivated into a dangling active pointer.
	identity := source.SourceIdentity
	revision := strings.Repeat("d", 64)
	uploadRecord := &model.Upload{
		ID:         987654321,
		UserID:     999,
		FileName:   "deleted.zip",
		FilePath:   "deleted.zip",
		FileSize:   1,
		MimeType:   "application/zip",
		Extension:  "zip",
		Hash:       revision,
		Type:       upload.ReservedPagesDeploymentType,
		Status:     model.UploadStatusDeleted,
		AccessMode: 0,
	}
	if err := db.DB(ctx).Create(uploadRecord).Error; err != nil {
		t.Fatalf("create deleted upload error = %v, want nil", err)
	}
	deployment := &model.PagesDeployment{
		ProjectID:        project.ID,
		DeploymentNumber: 1,
		Checksum:         revision,
		Status:           model.PagesDeploymentStatusUploaded,
		UploadID:         uploadRecord.ID,
		FileCount:        1,
		TotalSize:        1,
		CreatedBy:        "user:1",
		SourceType:       PagesSourceTypeRemoteURL,
		SourceIdentity:   &identity,
		SourceRevision:   &revision,
		SourceLabel:      "deleted.zip",
		SourceMeta:       `{"provider":"remote_url","display_name":"deleted.zip"}`,
		TriggerType:      pagesSourceTriggerManualSync,
	}
	if err := db.DB(ctx).Create(deployment).Error; err != nil {
		t.Fatalf("create source deployment error = %v, want nil", err)
	}
	if err := db.DB(ctx).Create(&model.PagesDeploymentFile{
		DeploymentID: deployment.ID,
		Path:         "index.html",
		Size:         1,
		Checksum:     revision,
	}).Error; err != nil {
		t.Fatalf("create source deployment file error = %v, want nil", err)
	}
	manifest := &deploymentManifest{
		FileCount: 1,
		TotalSize: 1,
		EntryFile: "index.html",
	}
	_, _, _, err := commitSourceDeployment(
		ctx,
		snapshot,
		revision,
		revision,
		sourceDetail{Provider: PagesSourceTypeRemoteURL, DisplayName: "deleted.zip"},
		`{"provider":"remote_url","display_name":"deleted.zip"}`,
		"user:1",
		manifest,
		upload.IngestResult{},
		false,
		nil,
	)
	if !errors.Is(err, errSourceFinalFence) {
		t.Errorf("commitSourceDeployment(deleted upload) error = %v, want %v", err, errSourceFinalFence)
	}
	storedProject, err := model.GetPagesProjectByID(ctx, project.ID)
	if err != nil {
		t.Fatalf("GetPagesProjectByID(%d) error = %v, want nil", project.ID, err)
	}
	if storedProject.ActiveDeploymentID != nil {
		t.Errorf("ActiveDeploymentID after deleted upload rejection = %v, want nil", storedProject.ActiveDeploymentID)
	}
	var activeCount int64
	if err := db.DB(ctx).Model(&model.PagesDeployment{}).
		Where("project_id = ? AND status = ?", project.ID, model.PagesDeploymentStatusActive).
		Count(&activeCount).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("count active deployments error = %v, want nil", err)
	}
	if got, want := activeCount, int64(0); got != want {
		t.Errorf("active deployment count after deleted upload rejection = %d, want %d", got, want)
	}
}
