// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/upload"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestParsePagesOrphanMarker(t *testing.T) {
	tests := []struct {
		name         string
		extra        map[string]any
		wantProject  uint
		wantSource   uint
		wantSourceOK bool
		wantErr      bool
	}{
		{
			name: "manual source marker",
			extra: map[string]any{
				pagesIngestMarkerKey:      pagesIngestMarkerV2,
				pagesProjectIDMetadataKey: "12",
			},
			wantProject: 12,
		},
		{
			name: "persistent source marker",
			extra: map[string]any{
				pagesIngestMarkerKey:      pagesIngestMarkerV2,
				pagesProjectIDMetadataKey: "12",
				pagesSourceIDMetadataKey:  "34",
			},
			wantProject:  12,
			wantSource:   34,
			wantSourceOK: true,
		},
		{
			name: "project ID must be canonical decimal",
			extra: map[string]any{
				pagesIngestMarkerKey:      pagesIngestMarkerV2,
				pagesProjectIDMetadataKey: "012",
			},
			wantErr: true,
		},
		{
			name: "source ID must be a string",
			extra: map[string]any{
				pagesIngestMarkerKey:      pagesIngestMarkerV2,
				pagesProjectIDMetadataKey: "12",
				pagesSourceIDMetadataKey:  float64(34),
			},
			wantErr: true,
		},
		{
			name: "zero ID rejected",
			extra: map[string]any{
				pagesIngestMarkerKey:      pagesIngestMarkerV2,
				pagesProjectIDMetadataKey: "0",
			},
			wantErr: true,
		},
		{
			name: "wrong marker version rejected",
			extra: map[string]any{
				pagesIngestMarkerKey:      "pages_deployment_v1",
				pagesProjectIDMetadataKey: "12",
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parsePagesOrphanMarker(model.UploadMetadata{Extra: test.extra})
			if gotErr := err != nil; gotErr != test.wantErr {
				t.Fatalf("parsePagesOrphanMarker(%v) error = %v, want error presence = %t", test.extra, err, test.wantErr)
			}
			if test.wantErr {
				return
			}
			if got.ProjectID != test.wantProject {
				t.Errorf("parsePagesOrphanMarker(%v).ProjectID = %d, want %d", test.extra, got.ProjectID, test.wantProject)
			}
			if gotSourceOK := got.SourceID != nil; gotSourceOK != test.wantSourceOK {
				t.Fatalf("parsePagesOrphanMarker(%v).SourceID presence = %t, want %t", test.extra, gotSourceOK, test.wantSourceOK)
			}
			if got.SourceID != nil && *got.SourceID != test.wantSource {
				t.Errorf("parsePagesOrphanMarker(%v).SourceID = %d, want %d", test.extra, *got.SourceID, test.wantSource)
			}
		})
	}
}

func TestReconcilePagesOrphanUploadsDeletesEligibleUploadOnce(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	ctx := context.Background()
	now := time.Now().UTC()
	project := createPagesOrphanProject(t, ctx, "eligible-orphan")
	candidate := createPagesOrphanUpload(t, ctx, now.Add(-3*time.Hour), project.ID, nil)
	if err := upload.RebuildUploadStats(ctx); err != nil {
		t.Fatalf("RebuildUploadStats() error = %v, want nil", err)
	}

	summary, err := ReconcilePagesOrphanUploads(ctx, now)
	if err != nil {
		t.Fatalf("ReconcilePagesOrphanUploads() error = %v, want nil", err)
	}
	if summary.Candidates != 1 || summary.Reconciled != 1 || cleanupOutcomeTotal(summary) != 1 {
		t.Errorf("ReconcilePagesOrphanUploads() summary = %+v, want one reconciled candidate", summary)
	}
	assertPagesCleanupUploadStatus(t, ctx, candidate.ID, model.UploadStatusDeleted)
	assertPagesCleanupTotalStat(t, ctx, 0)

	second, err := ReconcilePagesOrphanUploads(ctx, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("second ReconcilePagesOrphanUploads() error = %v, want nil", err)
	}
	if second.Candidates != 0 || cleanupOutcomeTotal(second) != 0 {
		t.Errorf("second ReconcilePagesOrphanUploads() summary = %+v, want empty", second)
	}
	assertPagesCleanupTotalStat(t, ctx, 0)
}

func TestReconcilePagesOrphanUploadsAllowsDeletedProjectAndSource(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	ctx := context.Background()
	now := time.Now().UTC()
	missingSourceID := uint(9876)
	candidate := createPagesOrphanUpload(t, ctx, now.Add(-3*time.Hour), 8765, &missingSourceID)

	summary, err := ReconcilePagesOrphanUploads(ctx, now)
	if err != nil {
		t.Fatalf("ReconcilePagesOrphanUploads() error = %v, want nil", err)
	}
	if summary.Candidates != 1 || summary.Reconciled != 1 || cleanupOutcomeTotal(summary) != 1 {
		t.Errorf("ReconcilePagesOrphanUploads() summary = %+v, want deleted project/source treated as one orphan", summary)
	}
	assertPagesCleanupUploadStatus(t, ctx, candidate.ID, model.UploadStatusDeleted)
}

func TestReconcilePagesOrphanUploadsSkipsBusyLeaseAndSourceMismatch(t *testing.T) {
	t.Run("unexpired source lease", func(t *testing.T) {
		cleanup := setupPagesTestDB(t)
		defer cleanup()
		ctx := context.Background()
		realNow := time.Now().UTC()
		// A deliberately future scanner snapshot proves lease freshness uses the
		// real clock after the runtime lock, not this isolation-cutoff input.
		scannerNow := realNow.Add(24 * time.Hour)
		project := createPagesOrphanProject(t, ctx, "busy-orphan")
		source := createPagesOrphanSource(t, ctx, project.ID)
		future := realNow.Add(time.Hour)
		if err := db.DB(ctx).Create(&model.PagesProjectSourceRuntime{
			SourceID:       source.ID,
			LeaseToken:     "busy-worker",
			LeaseExpiresAt: &future,
		}).Error; err != nil {
			t.Fatalf("create busy source runtime error = %v, want nil", err)
		}
		candidate := createPagesOrphanUpload(t, ctx, realNow.Add(-3*time.Hour), project.ID, &source.ID)

		summary, err := ReconcilePagesOrphanUploads(ctx, scannerNow)
		if err != nil {
			t.Fatalf("ReconcilePagesOrphanUploads() error = %v, want nil", err)
		}
		if summary.LeaseBusy != 1 || cleanupOutcomeTotal(summary) != 1 {
			t.Errorf("ReconcilePagesOrphanUploads() summary = %+v, want one lease-busy candidate", summary)
		}
		assertPagesCleanupUploadStatus(t, ctx, candidate.ID, model.UploadStatusUsed)
	})

	t.Run("source belongs to another project", func(t *testing.T) {
		cleanup := setupPagesTestDB(t)
		defer cleanup()
		ctx := context.Background()
		now := time.Now().UTC()
		markerProject := createPagesOrphanProject(t, ctx, "marker-project")
		actualProject := createPagesOrphanProject(t, ctx, "actual-project")
		source := createPagesOrphanSource(t, ctx, actualProject.ID)
		candidate := createPagesOrphanUpload(t, ctx, now.Add(-3*time.Hour), markerProject.ID, &source.ID)

		summary, err := ReconcilePagesOrphanUploads(ctx, now)
		if err != nil {
			t.Fatalf("ReconcilePagesOrphanUploads() error = %v, want nil", err)
		}
		if summary.InvalidMarker != 1 || cleanupOutcomeTotal(summary) != 1 {
			t.Errorf("ReconcilePagesOrphanUploads() summary = %+v, want one ownership mismatch", summary)
		}
		assertPagesCleanupUploadStatus(t, ctx, candidate.ID, model.UploadStatusUsed)
	})
}

func TestReconcilePagesOrphanUploadsRejectsMalformedMarker(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	ctx := context.Background()
	now := time.Now().UTC()
	candidate := createPagesOrphanUpload(t, ctx, now.Add(-3*time.Hour), 1, nil)
	metadata := candidate.Metadata
	metadata.Extra[pagesProjectIDMetadataKey] = "01"
	candidate.Metadata = metadata
	if err := db.DB(ctx).Save(candidate).Error; err != nil {
		t.Fatalf("seed malformed candidate marker error = %v, want nil", err)
	}

	summary, err := ReconcilePagesOrphanUploads(ctx, now)
	if err != nil {
		t.Fatalf("ReconcilePagesOrphanUploads() error = %v, want nil", err)
	}
	if summary.InvalidMarker != 1 || cleanupOutcomeTotal(summary) != 1 {
		t.Errorf("ReconcilePagesOrphanUploads() summary = %+v, want one invalid marker", summary)
	}
	assertPagesCleanupUploadStatus(t, ctx, candidate.ID, model.UploadStatusUsed)
}

func TestPagesOrphanCleanupAndDeploymentCommitInterleavings(t *testing.T) {
	t.Run("deployment reference commits first", func(t *testing.T) {
		cleanup := setupPagesTestDB(t)
		defer cleanup()
		ctx := context.Background()
		now := time.Now().UTC()
		project := createPagesOrphanProject(t, ctx, "deployment-first")
		candidate := createPagesOrphanUpload(t, ctx, now.Add(-3*time.Hour), project.ID, nil)
		marker, err := parsePagesOrphanMarker(candidate.Metadata)
		if err != nil {
			t.Fatalf("parsePagesOrphanMarker() error = %v, want nil", err)
		}
		if err := db.DB(ctx).Create(&model.PagesDeployment{
			ProjectID:        project.ID,
			DeploymentNumber: 1,
			Checksum:         "deployment-first",
			Status:           model.PagesDeploymentStatusUploaded,
			UploadID:         candidate.ID,
		}).Error; err != nil {
			t.Fatalf("create deployment reference error = %v, want nil", err)
		}

		outcome, err := reconcilePagesOrphanUploadCandidate(ctx, candidate, marker, 999, now.Add(-2*time.Hour))
		if err != nil {
			t.Fatalf("reconcilePagesOrphanUploadCandidate() error = %v, want nil", err)
		}
		if outcome != pagesOrphanCleanupReferenced {
			t.Errorf("reconcilePagesOrphanUploadCandidate() outcome = %d, want %d", outcome, pagesOrphanCleanupReferenced)
		}
		assertPagesCleanupUploadStatus(t, ctx, candidate.ID, model.UploadStatusUsed)
	})

	t.Run("cleanup commits first", func(t *testing.T) {
		cleanup := setupPagesTestDB(t)
		defer cleanup()
		ctx := context.Background()
		now := time.Now().UTC()
		project := createPagesOrphanProject(t, ctx, "cleanup-first")
		candidate := createPagesOrphanUpload(t, ctx, now.Add(-3*time.Hour), project.ID, nil)

		summary, err := ReconcilePagesOrphanUploads(ctx, now)
		if err != nil {
			t.Fatalf("ReconcilePagesOrphanUploads() error = %v, want nil", err)
		}
		if summary.Reconciled != 1 {
			t.Fatalf("ReconcilePagesOrphanUploads() summary = %+v, want one reconciled candidate", summary)
		}

		target := &model.PagesDeployment{ProjectID: project.ID, UploadID: candidate.ID}
		err = db.DB(ctx).Transaction(func(tx *gorm.DB) error {
			var lockedProject model.PagesProject
			if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).First(&lockedProject, project.ID).Error; err != nil {
				return err
			}
			return lockSourceDeploymentUploadsTx(tx, target, upload.IngestResult{}, false)
		})
		if !errors.Is(err, errSourceFinalFence) {
			t.Errorf("final deployment upload lock after cleanup error = %v, want %v", err, errSourceFinalFence)
		}
		var references int64
		if err := db.DB(ctx).Model(&model.PagesDeployment{}).Where("upload_id = ?", candidate.ID).Count(&references).Error; err != nil {
			t.Fatalf("count deployment references error = %v, want nil", err)
		}
		if references != 0 {
			t.Errorf("deployment references after cleanup-first interleaving = %d, want 0", references)
		}
	})
}

func cleanupOutcomeTotal(summary PagesOrphanCleanupSummary) int {
	return summary.Reconciled + summary.Referenced + summary.LeaseBusy + summary.InvalidMarker + summary.Skipped + summary.Failed
}

func createPagesOrphanProject(t *testing.T, ctx context.Context, slug string) *model.PagesProject {
	t.Helper()
	project := &model.PagesProject{Name: slug, Slug: slug, Enabled: true}
	if err := db.DB(ctx).Create(project).Error; err != nil {
		t.Fatalf("create Pages orphan project %q error = %v, want nil", slug, err)
	}
	return project
}

func createPagesOrphanSource(t *testing.T, ctx context.Context, projectID uint) *model.PagesProjectSource {
	t.Helper()
	source := &model.PagesProjectSource{
		ProjectID:      projectID,
		SourceType:     PagesSourceTypeRemoteURL,
		ConfigVersion:  1,
		SourceIdentity: "orphan-source-identity",
	}
	if err := db.DB(ctx).Create(source).Error; err != nil {
		t.Fatalf("create Pages orphan source for project %d error = %v, want nil", projectID, err)
	}
	return source
}

func createPagesOrphanUpload(
	t *testing.T,
	ctx context.Context,
	createdAt time.Time,
	projectID uint,
	sourceID *uint,
) *model.Upload {
	t.Helper()
	extra := map[string]any{
		pagesIngestMarkerKey:      pagesIngestMarkerV2,
		pagesProjectIDMetadataKey: strconv.FormatUint(uint64(projectID), 10),
	}
	if sourceID != nil {
		extra[pagesSourceIDMetadataKey] = strconv.FormatUint(uint64(*sourceID), 10)
	}
	candidate := &model.Upload{
		UserID:     999,
		FileName:   "site.zip",
		FilePath:   "pages/orphan-site.zip",
		FileSize:   64,
		MimeType:   "application/zip",
		Extension:  "zip",
		Hash:       "orphan-checksum",
		Type:       upload.ReservedPagesDeploymentType,
		Status:     model.UploadStatusUsed,
		AccessMode: 0,
		Metadata:   model.UploadMetadata{Extra: extra},
		CreatedAt:  createdAt,
		UpdatedAt:  createdAt,
	}
	if err := db.DB(ctx).Create(candidate).Error; err != nil {
		t.Fatalf("create Pages orphan upload error = %v, want nil", err)
	}
	return candidate
}

func assertPagesCleanupUploadStatus(t *testing.T, ctx context.Context, uploadID uint64, want model.UploadStatus) {
	t.Helper()
	var got model.Upload
	if err := db.DB(ctx).First(&got, uploadID).Error; err != nil {
		t.Fatalf("load upload %d error = %v, want nil", uploadID, err)
	}
	if got.Status != want {
		t.Errorf("upload %d status = %q, want %q", uploadID, got.Status, want)
	}
}

func assertPagesCleanupTotalStat(t *testing.T, ctx context.Context, want int64) {
	t.Helper()
	var stat model.UploadStat
	if err := db.DB(ctx).Where("dimension = ? AND stat_key = ?", model.UploadStatDimensionTotal, "").First(&stat).Error; err != nil {
		t.Fatalf("load total upload stat error = %v, want nil", err)
	}
	if stat.FileCount != want {
		t.Errorf("total upload stat FileCount = %d, want %d", stat.FileCount, want)
	}
}
