// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestPagesOrphanMarkerPredicate(t *testing.T) {
	tests := []struct {
		name    string
		dialect string
		want    string
		wantErr bool
	}{
		{
			name:    "postgres jsonb path",
			dialect: "postgres",
			want:    "metadata #>> '{extra,pages_ingest_marker}'",
		},
		{
			name:    "sqlite guarded json extract",
			dialect: "sqlite",
			want:    "CASE WHEN json_valid(w_uploads.metadata) THEN json_extract",
		},
		{
			name:    "unknown dialect rejected",
			dialect: "mysql",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := pagesOrphanMarkerPredicate(test.dialect)
			if gotErr := err != nil; gotErr != test.wantErr {
				t.Fatalf("pagesOrphanMarkerPredicate(%q) error = %v, want error presence = %t", test.dialect, err, test.wantErr)
			}
			if test.want != "" && !strings.Contains(got, test.want) {
				t.Errorf("pagesOrphanMarkerPredicate(%q) = %q, want substring %q", test.dialect, got, test.want)
			}
		})
	}
}

func TestListPagesOrphanUploadCandidatesFiltersAndLimits(t *testing.T) {
	ctx := context.Background()
	gormDB := setupPagesCleanupModelTestDB(t)
	cutoff := time.Now().UTC().Add(-2 * time.Hour)
	old := cutoff.Add(-time.Minute)
	marker := UploadMetadata{Extra: map[string]any{
		"pages_ingest_marker": "pages_deployment_v2",
		"pages_project_id":    "1",
	}}

	valid := make([]Upload, 0, PagesOrphanUploadCandidateLimit+1)
	for index := 0; index < PagesOrphanUploadCandidateLimit+1; index++ {
		valid = append(valid, pagesCleanupModelUpload(uint64(index+100), 999, "openflare_pages_deployment", UploadStatusUsed, old, marker))
	}
	if err := gormDB.Create(&valid).Error; err != nil {
		t.Fatalf("create valid candidates error = %v, want nil", err)
	}

	referenced := pagesCleanupModelUpload(1, 999, "openflare_pages_deployment", UploadStatusUsed, old, marker)
	wrongOwner := pagesCleanupModelUpload(2, 1000, "openflare_pages_deployment", UploadStatusUsed, old, marker)
	wrongType := pagesCleanupModelUpload(3, 999, "generic", UploadStatusUsed, old, marker)
	wrongStatus := pagesCleanupModelUpload(4, 999, "openflare_pages_deployment", UploadStatusPending, old, marker)
	fresh := pagesCleanupModelUpload(5, 999, "openflare_pages_deployment", UploadStatusUsed, cutoff, marker)
	wrongMarker := pagesCleanupModelUpload(6, 999, "openflare_pages_deployment", UploadStatusUsed, old, UploadMetadata{Extra: map[string]any{
		"pages_ingest_marker": "pages_deployment_v1",
		"pages_project_id":    "1",
	}})
	for _, upload := range []Upload{referenced, wrongOwner, wrongType, wrongStatus, fresh, wrongMarker} {
		if err := gormDB.Create(&upload).Error; err != nil {
			t.Fatalf("create filtered upload %d error = %v, want nil", upload.ID, err)
		}
	}
	if err := gormDB.Create(&PagesDeployment{
		ProjectID:        1,
		DeploymentNumber: 1,
		Checksum:         "referenced",
		Status:           PagesDeploymentStatusUploaded,
		UploadID:         referenced.ID,
	}).Error; err != nil {
		t.Fatalf("create referenced deployment error = %v, want nil", err)
	}

	invalidJSON := pagesCleanupModelUpload(7, 999, "openflare_pages_deployment", UploadStatusUsed, old, marker)
	if err := gormDB.Create(&invalidJSON).Error; err != nil {
		t.Fatalf("create invalid JSON upload error = %v, want nil", err)
	}
	if err := gormDB.Table((Upload{}).TableName()).Where("id = ?", invalidJSON.ID).
		UpdateColumn("metadata", "{invalid").Error; err != nil {
		t.Fatalf("corrupt upload metadata error = %v, want nil", err)
	}

	got, err := ListPagesOrphanUploadCandidates(ctx, PagesOrphanUploadCandidateQuery{
		SystemUserID:  999,
		UploadType:    "openflare_pages_deployment",
		Marker:        "pages_deployment_v2",
		CreatedBefore: cutoff,
	})
	if err != nil {
		t.Fatalf("ListPagesOrphanUploadCandidates() error = %v, want nil", err)
	}
	if len(got) != PagesOrphanUploadCandidateLimit {
		t.Fatalf("ListPagesOrphanUploadCandidates() count = %d, want %d", len(got), PagesOrphanUploadCandidateLimit)
	}
	for index, candidate := range got {
		wantID := uint64(index + 100)
		if candidate.ID != wantID {
			t.Errorf("ListPagesOrphanUploadCandidates()[%d].ID = %d, want %d", index, candidate.ID, wantID)
		}
	}
}

func TestListPagesOrphanUploadCandidatesSkipsInvalidSQLiteJSON(t *testing.T) {
	ctx := context.Background()
	gormDB := setupPagesCleanupModelTestDB(t)
	cutoff := time.Now().UTC().Add(-2 * time.Hour)
	upload := pagesCleanupModelUpload(1, 999, "openflare_pages_deployment", UploadStatusUsed, cutoff.Add(-time.Minute), UploadMetadata{})
	if err := gormDB.Create(&upload).Error; err != nil {
		t.Fatalf("create invalid JSON candidate error = %v, want nil", err)
	}
	if err := gormDB.Table((Upload{}).TableName()).Where("id = ?", upload.ID).
		UpdateColumn("metadata", "{invalid").Error; err != nil {
		t.Fatalf("corrupt upload metadata error = %v, want nil", err)
	}

	got, err := ListPagesOrphanUploadCandidates(ctx, PagesOrphanUploadCandidateQuery{
		SystemUserID:  999,
		UploadType:    "openflare_pages_deployment",
		Marker:        "pages_deployment_v2",
		CreatedBefore: cutoff,
	})
	if err != nil {
		t.Fatalf("ListPagesOrphanUploadCandidates(invalid JSON) error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("ListPagesOrphanUploadCandidates(invalid JSON) count = %d, want 0", len(got))
	}
}

func setupPagesCleanupModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open Pages cleanup model test database error = %v, want nil", err)
	}
	if err := gormDB.AutoMigrate(&Upload{}, &PagesDeployment{}); err != nil {
		t.Fatalf("migrate Pages cleanup model test database error = %v, want nil", err)
	}
	db.SetDB(gormDB)
	t.Cleanup(func() { db.SetDB(nil) })
	return gormDB
}

func pagesCleanupModelUpload(
	id uint64,
	userID uint64,
	uploadType string,
	status UploadStatus,
	createdAt time.Time,
	metadata UploadMetadata,
) Upload {
	return Upload{
		ID:         id,
		UserID:     userID,
		FileName:   "site.zip",
		FilePath:   "pages/site.zip",
		FileSize:   10,
		MimeType:   "application/zip",
		Extension:  "zip",
		Hash:       "checksum",
		Type:       uploadType,
		Status:     status,
		AccessMode: 0,
		Metadata:   metadata,
		CreatedAt:  createdAt,
		UpdatedAt:  createdAt,
	}
}
