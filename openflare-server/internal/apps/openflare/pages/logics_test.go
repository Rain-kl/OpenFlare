// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
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
	))
	require.NoError(t, sqliteDB.Create(&model.User{
		ID:       999,
		Username: "system",
		Password: "*",
		Nickname: "系统",
		IsActive: true,
	}).Error)

	db.SetDB(sqliteDB)
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

	storedDeployment, err := model.GetPagesDeploymentByID(ctx, deployment.ID)
	require.NoError(t, err)
	assert.NotZero(t, storedDeployment.UploadID)
	assert.Empty(t, storedDeployment.ArtifactPath)

	var uploadCount int64
	require.NoError(t, db.DB(ctx).Model(&model.Upload{}).Count(&uploadCount).Error)
	assert.Equal(t, int64(1), uploadCount)
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

	_, _, err = OpenDeploymentPackage(ctx, deployment.ID)
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

	packageObj, fileName, err := OpenDeploymentPackage(ctx, deployment.ID)
	require.NoError(t, err)
	defer packageObj.Body.Close()
	assert.Equal(t, fmt.Sprintf("pages-deployment-%d.zip", deployment.ID), fileName)

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