// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
)

func TestGetProjectLatestPackageMetadataPreservesZeroTotalSize(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	_, disableStorage := setupPagesStorageMock(t)
	defer disableStorage()

	ctx := context.Background()
	project, err := CreateProject(ctx, Input{
		Name:      "Empty Files",
		Slug:      "empty-files",
		Enabled:   true,
		EntryFile: "index.html",
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	packageBytes := testPagesZip(t, map[string]string{
		"index.html": "",
		".gitkeep":   "",
	})
	deployment, err := UploadDeployment(
		ctx,
		project.ID,
		testPagesMultipartFile(t, "empty-files.zip", packageBytes),
		"test",
	)
	if err != nil {
		t.Fatalf("UploadDeployment() error = %v", err)
	}
	if _, err := ActivateDeployment(ctx, project.ID, deployment.ID); err != nil {
		t.Fatalf("ActivateDeployment() error = %v", err)
	}
	if err := db.DB(ctx).Create(&model.ConfigVersion{
		Version: "v-package-metadata",
		SnapshotJSON: fmt.Sprintf(
			`{"routes":[{"upstream_type":"pages","pages_project_id":%d}]}`,
			project.ID,
		),
		SupportFilesJSON: "[]",
		Checksum:         "package-metadata-config",
		IsActive:         true,
		CreatedBy:        "test",
	}).Error; err != nil {
		t.Fatalf("create active ConfigVersion error = %v", err)
	}

	got, err := GetProjectLatestPackageMetadata(ctx, project.ID)
	if err != nil {
		t.Fatalf("GetProjectLatestPackageMetadata(%d) error = %v", project.ID, err)
	}
	wantHashBytes := sha256.Sum256(packageBytes)
	wantHash := hex.EncodeToString(wantHashBytes[:])
	if got.DeploymentID != deployment.ID || got.Hash != wantHash {
		t.Errorf("GetProjectLatestPackageMetadata(%d) identity = (%d, %q), want (%d, %q)",
			project.ID, got.DeploymentID, got.Hash, deployment.ID, wantHash)
	}
	if got.PackageSize != int64(len(packageBytes)) {
		t.Errorf("GetProjectLatestPackageMetadata(%d).PackageSize = %d, want %d",
			project.ID, got.PackageSize, len(packageBytes))
	}
	if got.FileCount != 2 || got.TotalSize != 0 {
		t.Errorf("GetProjectLatestPackageMetadata(%d) content = (%d files, %d bytes), want (2 files, 0 bytes)",
			project.ID, got.FileCount, got.TotalSize)
	}
}
