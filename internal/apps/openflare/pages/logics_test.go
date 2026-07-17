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

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/storage"
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
	restore = storage.MockStorage(
		func(_ context.Context, key string, body io.Reader, _ int64, _ string) error {
			data, err := io.ReadAll(body)
			if err != nil {
				return err
			}
			mockFiles[key] = data
			return nil
		},
		func(_ context.Context, key string) (*storage.Object, error) {
			data, ok := mockFiles[key]
			if !ok {
				return nil, os.ErrNotExist
			}
			return &storage.Object{
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
	storage.IsEnabledFunc = func() bool { return true }
	storage.ResetCache()
	disable = func() {
		storage.IsEnabledFunc = func() bool { return false }
		storage.ResetCache()
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

	storedDeployment, err := model.GetPagesDeploymentByID(ctx, deployment.ID)
	require.NoError(t, err)
	assert.NotZero(t, storedDeployment.UploadID)
	assert.Empty(t, storedDeployment.ArtifactPath)

	var uploadCount int64
	require.NoError(t, db.DB(ctx).Model(&model.Upload{}).Count(&uploadCount).Error)
	assert.Equal(t, int64(1), uploadCount)
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

	storedDeployment, err := model.GetPagesDeploymentByID(ctx, deployment.ID)
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

	body, err := io.ReadAll(packageObj.Body)
	require.NoError(t, err)
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err)
	require.Len(t, reader.File, 1)
	assert.Equal(t, "index.html", reader.File[0].Name)
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
	toDelete := selectDeploymentsToPrune(deployments, 1, 2)
	require.Len(t, toDelete, 2)
	assert.Equal(t, uint(3), toDelete[0].ID)
	assert.Equal(t, uint(2), toDelete[1].ID)

	// active is newest; keep=2 → keep {4,3}, prune {2,1}
	toDelete = selectDeploymentsToPrune(deployments, 4, 2)
	require.Len(t, toDelete, 2)
	assert.Equal(t, uint(2), toDelete[0].ID)
	assert.Equal(t, uint(1), toDelete[1].ID)

	// no active; keep=2 → keep {4,3}
	toDelete = selectDeploymentsToPrune(deployments, 0, 2)
	require.Len(t, toDelete, 2)
	assert.Equal(t, uint(2), toDelete[0].ID)
	assert.Equal(t, uint(1), toDelete[1].ID)

	// keep=1 with active → only active, prune the rest
	toDelete = selectDeploymentsToPrune(deployments, 2, 1)
	require.Len(t, toDelete, 3)
	for _, item := range toDelete {
		assert.NotEqual(t, uint(2), item.ID)
	}

	// already within limit
	assert.Nil(t, selectDeploymentsToPrune(deployments[:2], 4, 2))
	// unlimited
	assert.Nil(t, selectDeploymentsToPrune(deployments, 1, 0))
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
	deployments, err := model.ListPagesDeployments(ctx, project.ID)
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

	deployments, err = model.ListPagesDeployments(ctx, project.ID)
	require.NoError(t, err)
	require.Len(t, deployments, 2, "must be at most N=2, not active+N newest")

	storedProject, err := model.GetPagesProjectByID(ctx, project.ID)
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
