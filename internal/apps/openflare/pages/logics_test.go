// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/apps/upload"
	"github.com/Rain-kl/Wavelet/internal/infra/objectstore"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPagesTestDB(t *testing.T) func() {
	t.Helper()

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(
		&model.User{},
		&model.Upload{},
		&model.UploadStat{},
		&model.TaskExecution{},
		&model.PagesProject{},
		&model.PagesDeployment{},
		&model.PagesDeploymentFile{},
		&model.PagesProjectSource{},
		&model.PagesProjectSourceRuntime{},
		&model.ConfigVersion{},
		&model.SystemConfig{},
	))
	require.NoError(t, sqliteDB.Create(&model.User{
		ID:       999,
		Username: "system",
		Password: "*",
		Nickname: "系统",
		IsActive: true,
	}).Error)
	require.NoError(t, sqliteDB.Create([]model.SystemConfig{
		{
			Key:         model.ConfigKeyPagesMaxPackageSizeMB,
			Value:       "100",
			Type:        "business",
			Description: "Pages 部署包上传大小上限（MiB）",
		},
		{
			Key:         model.ConfigKeyPagesMaxHistoryCount,
			Value:       "0", // unlimited for existing tests
			Type:        "business",
			Description: "Pages 每个项目最大历史部署保留数（0 表示不限制）",
		},
	}).Error)

	db.SetDB(sqliteDB)
	// Clear process-global system config RAM cache so tests do not see stale values.
	_ = repository.InvalidateSystemConfigCache(context.Background(), model.ConfigKeyPagesMaxPackageSizeMB)
	_ = repository.InvalidateSystemConfigCache(context.Background(), model.ConfigKeyPagesMaxHistoryCount)
	return func() {
		db.SetDB(nil)
	}
}

func setupPagesStorageMock(t *testing.T) (restore func(), disable func()) {
	t.Helper()
	mockFiles := make(map[string][]byte)
	restore = objectstore.MockStorage(
		func(_ context.Context, key string, body io.Reader, _ int64, _ string) error {
			data, err := io.ReadAll(body)
			if err != nil {
				return err
			}
			mockFiles[key] = data
			return nil
		},
		func(_ context.Context, key string) (*objectstore.Object, error) {
			data, ok := mockFiles[key]
			if !ok {
				return nil, os.ErrNotExist
			}
			return &objectstore.Object{
				Body:          io.NopCloser(bytes.NewReader(data)),
				ContentLength: int64(len(data)),
				ContentType:   "application/zip",
			}, nil
		},
		func(_ context.Context, key string) error {
			delete(mockFiles, key)
			return nil
		},
	)
	objectstore.IsEnabledFunc = func() bool { return true }
	objectstore.ResetCache()
	disable = func() {
		objectstore.IsEnabledFunc = func() bool { return false }
		objectstore.ResetCache()
		restore()
	}
	return restore, disable
}

func TestCreateProject(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	ctx := context.Background()

	project, err := CreateProject(ctx, Input{
		Name:               "Marketing Site",
		Slug:               "marketing-site",
		Description:        "public site",
		Enabled:            true,
		SPAFallbackEnabled: true,
		SPAFallbackPath:    "/index.html",
		EntryFile:          "index.html",
	})
	require.NoError(t, err)
	assert.NotZero(t, project.ID)
	assert.Equal(t, "Marketing Site", project.Name)
	assert.Equal(t, "marketing-site", project.Slug)
	assert.Equal(t, "public site", project.Description)
	assert.True(t, project.Enabled)
	assert.True(t, project.SPAFallbackEnabled)
	assert.Equal(t, "/index.html", project.SPAFallbackPath)
	assert.Equal(t, "index.html", project.EntryFile)
	assert.Equal(t, int64(0), project.DeploymentCount)

	_, err = CreateProject(ctx, Input{
		Name: "Duplicate Slug",
		Slug: "marketing-site",
	})
	require.Error(t, err)
	assert.Equal(t, errPagesSlugExists, err.Error())
}

func TestCreateProjectRejectsUnsafeFallbackPath(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := CreateProject(ctx, Input{
		Name:               "Unsafe Fallback",
		Slug:               "unsafe-fallback",
		Enabled:            true,
		SPAFallbackEnabled: true,
		SPAFallbackPath:    "/index.html; proxy_pass http://evil",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "回退路径")
}

func TestCreateProjectRejectsUnsafeContentPaths(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	ctx := context.Background()

	rootDirs := []string{"/public", "public/../dist", "C:/public", `public\\dist`, "./public", "public\x00dist"}
	for index, rootDir := range rootDirs {
		_, err := CreateProject(ctx, Input{
			Name:      fmt.Sprintf("Unsafe Root %d", index),
			Slug:      fmt.Sprintf("unsafe-root-%d", index),
			RootDir:   rootDir,
			EntryFile: "index.html",
		})
		require.Error(t, err, rootDir)
	}

	entryFiles := []string{"/index.html", "../index.html", "C:/index.html", `public\\index.html`, "./index.html", "index.html;bad"}
	for index, entryFile := range entryFiles {
		_, err := CreateProject(ctx, Input{
			Name:      fmt.Sprintf("Unsafe Entry %d", index),
			Slug:      fmt.Sprintf("unsafe-entry-%d", index),
			EntryFile: entryFile,
		})
		require.Error(t, err, entryFile)
	}
}

func TestUpdateProjectValidatesActiveDeploymentEntry(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	_, disableStorage := setupPagesStorageMock(t)
	defer disableStorage()
	ctx := context.Background()

	project, err := CreateProject(ctx, Input{
		Name:      "Content Root",
		Slug:      "content-root",
		Enabled:   true,
		EntryFile: "index.html",
	})
	require.NoError(t, err)
	deployment, err := UploadDeployment(ctx, project.ID, testPagesMultipartFile(t, "site.zip", testPagesZip(t, map[string]string{
		"index.html":      "root",
		"dist/index.html": "dist",
	})), "user:1")
	require.NoError(t, err)
	staleCandidate, err := UploadDeployment(ctx, project.ID, testPagesMultipartFile(t, "stale.zip", testPagesZip(t, map[string]string{
		"index.html": "stale",
	})), "user:1")
	require.NoError(t, err)
	_, err = ActivateDeployment(ctx, project.ID, deployment.ID)
	require.NoError(t, err)

	updated, err := UpdateProject(ctx, project.ID, Input{
		Name:      project.Name,
		Slug:      project.Slug,
		Enabled:   true,
		RootDir:   "dist",
		EntryFile: "index.html",
	})
	require.NoError(t, err)
	assert.Equal(t, "dist", updated.RootDir)

	_, err = ActivateDeployment(ctx, project.ID, staleCandidate.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), errPagesEntryFileMissing)

	_, err = UpdateProject(ctx, project.ID, Input{
		Name:      project.Name,
		Slug:      project.Slug,
		Enabled:   true,
		RootDir:   "missing",
		EntryFile: "index.html",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), errPagesEntryFileMissing)

	stored, err := repository.GetPagesProjectByID(ctx, project.ID)
	require.NoError(t, err)
	assert.Equal(t, "dist", stored.RootDir)
	assert.Equal(t, "index.html", stored.EntryFile)
}

func TestUploadDeploymentAcceptsZeroByteFiles(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	_, disableStorage := setupPagesStorageMock(t)
	defer disableStorage()
	ctx := context.Background()

	project, err := CreateProject(ctx, Input{
		Name:    "Zero Byte Site",
		Slug:    "zero-byte-site",
		Enabled: true,
	})
	require.NoError(t, err)

	deployment, err := UploadDeployment(ctx, project.ID, testPagesMultipartFile(t, "site.zip", testPagesZip(t, map[string]string{
		"index.html": "ok",
		".gitkeep":   "",
	})), "root")
	require.NoError(t, err)
	assert.Equal(t, 2, deployment.FileCount)
}

func TestUploadDeploymentStoresPackageInUploadFramework(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	_, disableStorage := setupPagesStorageMock(t)
	defer disableStorage()
	ctx := context.Background()

	project, err := CreateProject(ctx, Input{
		Name:    "Upload Framework Site",
		Slug:    "upload-framework-site",
		Enabled: true,
	})
	require.NoError(t, err)

	deployment, err := UploadDeployment(ctx, project.ID, testPagesMultipartFile(t, "site.zip", testPagesZip(t, map[string]string{
		"index.html": "ok",
	})), "root")
	require.NoError(t, err)
	assert.NotZero(t, deployment.UploadID)

	storedDeployment, err := repository.GetPagesDeploymentByID(ctx, deployment.ID)
	require.NoError(t, err)
	assert.NotZero(t, storedDeployment.UploadID)
	assert.Empty(t, storedDeployment.ArtifactPath)

	var uploadCount int64
	require.NoError(t, db.DB(ctx).Model(&model.Upload{}).Count(&uploadCount).Error)
	assert.Equal(t, int64(1), uploadCount)
	var uploadRecord model.Upload
	require.NoError(t, db.DB(ctx).First(&uploadRecord, storedDeployment.UploadID).Error)
	assert.Equal(t, upload.ReservedPagesDeploymentType, uploadRecord.Type)
	assert.Equal(t, pagesIngestMarkerV2, uploadRecord.Metadata.Extra[pagesIngestMarkerKey])
	assert.Equal(t, fmt.Sprint(project.ID), uploadRecord.Metadata.Extra[pagesProjectIDMetadataKey])
	assert.NotContains(t, uploadRecord.Metadata.Extra, "project_slug")
	assert.NotContains(t, uploadRecord.Metadata.Extra, "format")
}

func TestOpenDeploymentPackageHydratesLegacyArtifactPath(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	_, disableStorage := setupPagesStorageMock(t)
	defer disableStorage()
	ctx := context.Background()

	project, err := CreateProject(ctx, Input{
		Name:    "Legacy Site",
		Slug:    "openspeedtest",
		Enabled: true,
	})
	require.NoError(t, err)

	artifactDir := filepath.Join(t.TempDir(), "pages", "artifacts", project.Slug)
	require.NoError(t, os.MkdirAll(artifactDir, 0o755))
	artifactPath := filepath.Join(artifactDir, "legacy-checksum.zip")
	require.NoError(t, os.WriteFile(artifactPath, testPagesZip(t, map[string]string{"index.html": "legacy"}), 0o644))

	deployment := &model.PagesDeployment{
		ProjectID:        project.ID,
		DeploymentNumber: 1,
		Checksum:         "legacy-checksum",
		Status:           model.PagesDeploymentStatusUploaded,
		ArtifactPath:     artifactPath,
		FileCount:        1,
		TotalSize:        10,
		CreatedBy:        "test",
	}
	require.NoError(t, db.DB(ctx).Create(deployment).Error)
	require.NoError(t, db.DB(ctx).Create(&model.PagesDeploymentFile{
		DeploymentID: deployment.ID,
		Path:         "index.html",
		Size:         6,
		Checksum:     "legacy-checksum",
	}).Error)

	_, err = ActivateDeployment(ctx, project.ID, deployment.ID)
	require.NoError(t, err)

	require.NoError(t, db.DB(ctx).Create(&model.ConfigVersion{
		Version:          "v2026-legacy",
		SnapshotJSON:     fmt.Sprintf(`{"routes":[{"upstream_type":"pages","pages_deployment":{"deployment_id":%d}}]}`, deployment.ID),
		MainConfig:       "",
		RenderedConfig:   "",
		SupportFilesJSON: "[]",
		Checksum:         "legacy-config-checksum",
		IsActive:         true,
		CreatedBy:        "test",
	}).Error)

	packageObj, err := OpenDeploymentPackage(ctx, deployment.ID)
	require.NoError(t, err)
	defer packageObj.Body.Close()
	assert.Equal(t, fmt.Sprintf("pages-deployment-%d.zip", deployment.ID), packageObj.FileName)

	body, err := io.ReadAll(packageObj.Body)
	require.NoError(t, err)
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err)
	require.Len(t, reader.File, 1)
	assert.Equal(t, "index.html", reader.File[0].Name)

	storedDeployment, err := repository.GetPagesDeploymentByID(ctx, deployment.ID)
	require.NoError(t, err)
	assert.NotZero(t, storedDeployment.UploadID)
	assert.Empty(t, storedDeployment.ArtifactPath)

	var uploadCount int64
	require.NoError(t, db.DB(ctx).Model(&model.Upload{}).Count(&uploadCount).Error)
	assert.Equal(t, int64(1), uploadCount)

	packageObj2, err := OpenDeploymentPackage(ctx, deployment.ID)
	require.NoError(t, err)
	defer packageObj2.Body.Close()
	body2, err := io.ReadAll(packageObj2.Body)
	require.NoError(t, err)
	assert.Equal(t, body, body2)
}

func TestOpenDeploymentPackageRequiresActiveConfigSnapshot(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	_, disableStorage := setupPagesStorageMock(t)
	defer disableStorage()
	ctx := context.Background()

	project, err := CreateProject(ctx, Input{
		Name:    "Published Site",
		Slug:    "published-site",
		Enabled: true,
	})
	require.NoError(t, err)

	deployment, err := UploadDeployment(ctx, project.ID, testPagesMultipartFile(t, "site.zip", testPagesZip(t, map[string]string{
		"index.html": "ok",
	})), "root")
	require.NoError(t, err)

	_, err = ActivateDeployment(ctx, project.ID, deployment.ID)
	require.NoError(t, err)

	_, err = OpenDeploymentPackage(ctx, deployment.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "激活配置")

	require.NoError(t, db.DB(ctx).Create(&model.ConfigVersion{
		Version:          "v2026-001",
		SnapshotJSON:     fmt.Sprintf(`{"routes":[{"upstream_type":"pages","pages_deployment":{"deployment_id":%d}}]}`, deployment.ID),
		MainConfig:       "",
		RenderedConfig:   "",
		SupportFilesJSON: "[]",
		Checksum:         "test-checksum",
		IsActive:         true,
		CreatedBy:        "test",
	}).Error)

	packageObj, err := OpenDeploymentPackage(ctx, deployment.ID)
	require.NoError(t, err)
	defer packageObj.Body.Close()
	assert.Equal(t, fmt.Sprintf("pages-deployment-%d.zip", deployment.ID), packageObj.FileName)

	// Latest-by-project resolves the active package once the project is on active config.
	depID, hash, err := GetProjectLatestPackageHash(ctx, project.ID)
	require.NoError(t, err)
	assert.Equal(t, deployment.ID, depID)
	assert.NotEmpty(t, hash)

	latestPkg, err := OpenProjectLatestPackage(ctx, project.ID)
	require.NoError(t, err)
	defer latestPkg.Body.Close()

	body, err := io.ReadAll(packageObj.Body)
	require.NoError(t, err)
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err)
	require.Len(t, reader.File, 1)
	assert.Equal(t, "index.html", reader.File[0].Name)
}

func TestProjectLatestRejectsWhenNotOnActiveConfigOrNotActive(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	_, disableStorage := setupPagesStorageMock(t)
	defer disableStorage()
	ctx := context.Background()

	project, err := CreateProject(ctx, Input{Name: "Gate", Slug: "gate", Enabled: true})
	require.NoError(t, err)
	d1, err := UploadDeployment(ctx, project.ID, testPagesMultipartFile(t, "a.zip", testPagesZip(t, map[string]string{"index.html": "a"})), "root")
	require.NoError(t, err)
	d2, err := UploadDeployment(ctx, project.ID, testPagesMultipartFile(t, "b.zip", testPagesZip(t, map[string]string{"index.html": "b"})), "root")
	require.NoError(t, err)
	_, err = ActivateDeployment(ctx, project.ID, d2.ID)
	require.NoError(t, err)

	// No active main config → reject.
	_, _, err = GetProjectLatestPackageHash(ctx, project.ID)
	require.Error(t, err)

	require.NoError(t, db.DB(ctx).Create(&model.ConfigVersion{
		Version: "v-gate",
		SnapshotJSON: fmt.Sprintf(
			`{"routes":[{"upstream_type":"pages","pages_project_id":%d,"pages_deployment":{"project_id":%d,"deployment_id":%d}}]}`,
			project.ID, project.ID, d2.ID,
		),
		SupportFilesJSON: "[]",
		Checksum:         "c-gate",
		IsActive:         true,
		CreatedBy:        "test",
	}).Error)

	// Non-active historical deployment must not be downloadable.
	_, err = OpenDeploymentPackage(ctx, d1.ID)
	require.Error(t, err)

	// Active latest works.
	_, hash, err := GetProjectLatestPackageHash(ctx, project.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	// Disabled project rejects.
	require.NoError(t, db.DB(ctx).Model(&model.PagesProject{}).Where("id = ?", project.ID).Update("enabled", false).Error)
	_, _, err = GetProjectLatestPackageHash(ctx, project.ID)
	require.Error(t, err)
}

func testPagesZip(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		file, err := writer.Create(name)
		require.NoError(t, err)
		_, err = file.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}

func TestUploadDeploymentAcceptsTarGz(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	_, disableStorage := setupPagesStorageMock(t)
	defer disableStorage()
	ctx := context.Background()

	project, err := CreateProject(ctx, Input{
		Name:    "TarGz Site",
		Slug:    "tar-gz-site",
		Enabled: true,
	})
	require.NoError(t, err)

	deployment, err := UploadDeployment(ctx, project.ID, testPagesMultipartFile(t, "site.tar.gz", testPagesTarGz(t, map[string]string{
		"index.html": "tar-ok",
		"app.js":     "1",
	})), "root")
	require.NoError(t, err)
	assert.Equal(t, 2, deployment.FileCount)
	assert.NotZero(t, deployment.UploadID)
}

func TestSelectDeploymentsToPruneKeepsActiveAndNewest(t *testing.T) {
	// ids 4(newest) ... 1(oldest); active is oldest id=1; keep=2 → keep {1,4}, prune {3,2}
	deployments := []model.PagesDeployment{
		{ID: 4, ProjectID: 1},
		{ID: 3, ProjectID: 1},
		{ID: 2, ProjectID: 1},
		{ID: 1, ProjectID: 1},
	}
	toDelete := selectDeploymentsToPrune(deployments, 1, 0, 2)
	require.Len(t, toDelete, 2)
	assert.Equal(t, uint(3), toDelete[0].ID)
	assert.Equal(t, uint(2), toDelete[1].ID)

	// active is newest; keep=2 → keep {4,3}, prune {2,1}
	toDelete = selectDeploymentsToPrune(deployments, 4, 0, 2)
	require.Len(t, toDelete, 2)
	assert.Equal(t, uint(2), toDelete[0].ID)
	assert.Equal(t, uint(1), toDelete[1].ID)

	// no active; keep=2 → keep {4,3}
	toDelete = selectDeploymentsToPrune(deployments, 0, 0, 2)
	require.Len(t, toDelete, 2)
	assert.Equal(t, uint(2), toDelete[0].ID)
	assert.Equal(t, uint(1), toDelete[1].ID)

	// keep=1 with active → only active, prune the rest
	toDelete = selectDeploymentsToPrune(deployments, 2, 0, 1)
	require.Len(t, toDelete, 3)
	for _, item := range toDelete {
		assert.NotEqual(t, uint(2), item.ID)
	}

	// already within limit
	assert.Nil(t, selectDeploymentsToPrune(deployments[:2], 4, 0, 2))
	// unlimited
	assert.Nil(t, selectDeploymentsToPrune(deployments, 1, 0, 0))

	// history=1 temporarily preserves active plus the freshly uploaded candidate.
	toDelete = selectDeploymentsToPrune(deployments, 2, 4, 1)
	require.Len(t, toDelete, 2)
	assert.Equal(t, uint(3), toDelete[0].ID)
	assert.Equal(t, uint(1), toDelete[1].ID)
	assert.Equal(t, uint(4), resolveLatestCandidateID(deployments, 2, true))
	assert.Zero(t, resolveLatestCandidateID(deployments, 2, false))
}

func TestPruneProjectDeploymentHistory(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	_, disableStorage := setupPagesStorageMock(t)
	defer disableStorage()
	ctx := context.Background()

	require.NoError(t, db.DB(ctx).Model(&model.SystemConfig{}).
		Where("key = ?", model.ConfigKeyPagesMaxHistoryCount).
		Update("value", "2").Error)
	require.NoError(t, repository.InvalidateSystemConfigCache(ctx, model.ConfigKeyPagesMaxHistoryCount))

	project, err := CreateProject(ctx, Input{
		Name:    "History Site",
		Slug:    "history-site",
		Enabled: true,
	})
	require.NoError(t, err)

	var ids []uint
	for i := 0; i < 3; i++ {
		deployment, uploadErr := UploadDeployment(ctx, project.ID, testPagesMultipartFile(t, "site.zip", testPagesZip(t, map[string]string{
			"index.html": fmt.Sprintf("v%d", i),
		})), "root")
		require.NoError(t, uploadErr)
		ids = append(ids, deployment.ID)
	}
	// After 3 uploads with keep=2 and no active: only 2 newest remain.
	deployments, err := repository.ListPagesDeployments(ctx, project.ID)
	require.NoError(t, err)
	require.Len(t, deployments, 2)
	assert.Equal(t, ids[2], deployments[0].ID)
	assert.Equal(t, ids[1], deployments[1].ID)

	// Activate the older of the remaining two, then upload again.
	_, err = ActivateDeployment(ctx, project.ID, ids[1])
	require.NoError(t, err)

	latest, err := UploadDeployment(ctx, project.ID, testPagesMultipartFile(t, "site.zip", testPagesZip(t, map[string]string{
		"index.html": "v-latest",
	})), "root")
	require.NoError(t, err)

	deployments, err = repository.ListPagesDeployments(ctx, project.ID)
	require.NoError(t, err)
	require.Len(t, deployments, 2, "must be at most N=2, not active+N newest")

	storedProject, err := repository.GetPagesProjectByID(ctx, project.ID)
	require.NoError(t, err)
	require.NotNil(t, storedProject.ActiveDeploymentID)
	assert.Equal(t, ids[1], *storedProject.ActiveDeploymentID)

	kept := map[uint]struct{}{}
	for _, item := range deployments {
		kept[item.ID] = struct{}{}
	}
	_, hasActive := kept[ids[1]]
	_, hasLatest := kept[latest.ID]
	assert.True(t, hasActive, "active deployment must be retained")
	assert.True(t, hasLatest, "newest deployment must fill remaining slot")
}

func TestHistoryCountOnePreservesFreshCandidateUntilActivation(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	_, disableStorage := setupPagesStorageMock(t)
	defer disableStorage()
	ctx := context.Background()

	require.NoError(t, db.DB(ctx).Model(&model.SystemConfig{}).
		Where("key = ?", model.ConfigKeyPagesMaxHistoryCount).
		Update("value", "1").Error)
	require.NoError(t, repository.InvalidateSystemConfigCache(ctx, model.ConfigKeyPagesMaxHistoryCount))

	project, err := CreateProject(ctx, Input{Name: "Single History", Slug: "single-history", Enabled: true})
	require.NoError(t, err)
	active, err := UploadDeployment(ctx, project.ID, testPagesMultipartFile(t, "v1.zip", testPagesZip(t, map[string]string{
		"index.html": "v1",
	})), "user:1")
	require.NoError(t, err)
	_, err = ActivateDeployment(ctx, project.ID, active.ID)
	require.NoError(t, err)

	oldCandidate, err := UploadDeployment(ctx, project.ID, testPagesMultipartFile(t, "v2.zip", testPagesZip(t, map[string]string{
		"index.html": "v2",
	})), "user:1")
	require.NoError(t, err)
	deployments, err := repository.ListPagesDeployments(ctx, project.ID)
	require.NoError(t, err)
	require.Len(t, deployments, 2)

	newCandidate, err := UploadDeployment(ctx, project.ID, testPagesMultipartFile(t, "v3.zip", testPagesZip(t, map[string]string{
		"index.html": "v3",
	})), "user:1")
	require.NoError(t, err)
	deployments, err = repository.ListPagesDeployments(ctx, project.ID)
	require.NoError(t, err)
	require.Len(t, deployments, 2)
	kept := map[uint]bool{}
	for _, deployment := range deployments {
		kept[deployment.ID] = true
	}
	assert.True(t, kept[active.ID])
	assert.True(t, kept[newCandidate.ID])
	assert.False(t, kept[oldCandidate.ID])
	var removedUpload model.Upload
	require.NoError(t, db.DB(ctx).First(&removedUpload, oldCandidate.UploadID).Error)
	assert.Equal(t, model.UploadStatusDeleted, removedUpload.Status)

	_, err = ActivateDeployment(ctx, project.ID, newCandidate.ID)
	require.NoError(t, err)
	deployments, err = repository.ListPagesDeployments(ctx, project.ID)
	require.NoError(t, err)
	require.Len(t, deployments, 1)
	assert.Equal(t, newCandidate.ID, deployments[0].ID)
}

func TestPruneUsesLockTimeNewestCandidateInsteadOfStaleCaller(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	_, disableStorage := setupPagesStorageMock(t)
	defer disableStorage()
	ctx := context.Background()

	project, err := CreateProject(ctx, Input{Name: "Concurrent Candidate", Slug: "concurrent-candidate", Enabled: true})
	require.NoError(t, err)
	active, err := UploadDeployment(ctx, project.ID, testPagesMultipartFile(t, "v1.zip", testPagesZip(t, map[string]string{
		"index.html": "v1",
	})), "user:1")
	require.NoError(t, err)
	_, err = ActivateDeployment(ctx, project.ID, active.ID)
	require.NoError(t, err)
	staleCandidate, err := UploadDeployment(ctx, project.ID, testPagesMultipartFile(t, "v2.zip", testPagesZip(t, map[string]string{
		"index.html": "v2",
	})), "user:1")
	require.NoError(t, err)
	newCandidate, err := UploadDeployment(ctx, project.ID, testPagesMultipartFile(t, "v3.zip", testPagesZip(t, map[string]string{
		"index.html": "v3",
	})), "user:1")
	require.NoError(t, err)

	deleted, err := pruneProjectDeploymentHistoryOnce(ctx, project.ID, 1, staleCandidate.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, deleted)
	deployments, err := repository.ListPagesDeployments(ctx, project.ID)
	require.NoError(t, err)
	require.Len(t, deployments, 2)
	kept := map[uint]bool{}
	for _, deployment := range deployments {
		kept[deployment.ID] = true
	}
	assert.True(t, kept[active.ID])
	assert.True(t, kept[newCandidate.ID])
	assert.False(t, kept[staleCandidate.ID])
}

func TestDeleteDeploymentAndProjectSoftDeleteUnreferencedArtifacts(t *testing.T) {
	cleanup := setupPagesTestDB(t)
	defer cleanup()
	_, disableStorage := setupPagesStorageMock(t)
	defer disableStorage()
	ctx := context.Background()

	project, err := CreateProject(ctx, Input{Name: "Delete Artifacts", Slug: "delete-artifacts", Enabled: true})
	require.NoError(t, err)
	first, err := UploadDeployment(ctx, project.ID, testPagesMultipartFile(t, "first.zip", testPagesZip(t, map[string]string{
		"index.html": "first",
	})), "user:1")
	require.NoError(t, err)
	second, err := UploadDeployment(ctx, project.ID, testPagesMultipartFile(t, "second.zip", testPagesZip(t, map[string]string{
		"index.html": "second",
	})), "user:1")
	require.NoError(t, err)

	require.NoError(t, DeleteDeployment(ctx, project.ID, second.ID))
	var secondUpload model.Upload
	require.NoError(t, db.DB(ctx).First(&secondUpload, second.UploadID).Error)
	assert.Equal(t, model.UploadStatusDeleted, secondUpload.Status)

	require.NoError(t, DeleteProject(ctx, project.ID))
	var firstUpload model.Upload
	require.NoError(t, db.DB(ctx).First(&firstUpload, first.UploadID).Error)
	assert.Equal(t, model.UploadStatusDeleted, firstUpload.Status)
	_, err = repository.GetPagesProjectByID(ctx, project.ID)
	assert.Error(t, err)
}

func testPagesTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	gzWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzWriter)
	for name, content := range files {
		require.NoError(t, tarWriter.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}))
		_, err := tarWriter.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzWriter.Close())
	return buffer.Bytes()
}

func testPagesMultipartFile(t *testing.T, fileName string, content []byte) *multipart.FileHeader {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("package", fileName)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest("POST", "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, req.ParseMultipartForm(int64(len(content))+1024))

	file, header, err := req.FormFile("package")
	require.NoError(t, err)
	file.Close()
	return header
}
