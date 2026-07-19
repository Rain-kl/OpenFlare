// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package migrator

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	pagesSourcePreviousMigration = int64(202607190001)
	pagesSourceMigration         = int64(202607190002)
	pagesMigrationProjectID      = uint(900001)
	pagesMigrationDeploymentID   = uint(900001)
)

func TestPagesSourceMigrationSQLiteUpDownUp(t *testing.T) {
	dbPath := t.TempDir() + "/pages-source-migration.db"
	gormDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)

	sqlDB, err := gormDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	runPagesSourceMigrationUpDownUp(t, gormDB, sqlDB, dialectSqlite, "goose/sqlite")

	var indexSQL string
	require.NoError(t, gormDB.Raw(
		"SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?",
		"idx_of_pages_deployments_source_revision",
	).Scan(&indexSQL).Error)
	assert.Contains(t, strings.ToUpper(indexSQL), "WHERE SOURCE_IDENTITY IS NOT NULL AND SOURCE_REVISION IS NOT NULL")
}

func TestPagesSourceMigrationPostgresUpDownUp(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("OPENFLARE_TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("OPENFLARE_TEST_POSTGRES_DSN is not set")
	}

	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	sqlDB, err := gormDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	schema := fmt.Sprintf("pages_source_migration_%d", time.Now().UnixNano())
	require.Regexp(t, `^[a-z0-9_]+$`, schema)
	require.NoError(t, gormDB.Exec(`CREATE SCHEMA "`+schema+`"`).Error)
	require.NoError(t, gormDB.Exec(`SET search_path TO "`+schema+`"`).Error)
	t.Cleanup(func() {
		assert.NoError(t, gormDB.Exec("SET search_path TO public").Error)
		assert.NoError(t, gormDB.Exec(`DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`).Error)
		assert.NoError(t, sqlDB.Close())
	})

	runPagesSourceMigrationUpDownUp(t, gormDB, sqlDB, dialectPostgres, "goose/postgres")
}

func runPagesSourceMigrationUpDownUp(
	t *testing.T,
	gormDB *gorm.DB,
	sqlDB *sql.DB,
	dialect string,
	dir string,
) {
	t.Helper()

	goose.SetBaseFS(migrationFS)
	require.NoError(t, goose.SetDialect(dialect))
	require.NoError(t, goose.UpTo(sqlDB, dir, pagesSourcePreviousMigration))
	seedPrePagesSourceMigrationData(t, gormDB)

	require.NoError(t, goose.UpTo(sqlDB, dir, pagesSourceMigration))
	assertPagesSourceMigrationUp(t, gormDB)

	require.NoError(t, goose.DownTo(sqlDB, dir, pagesSourcePreviousMigration))
	assertPagesSourceMigrationDown(t, gormDB)

	require.NoError(t, goose.UpTo(sqlDB, dir, pagesSourceMigration))
	assertPagesSourceMigrationUpAgain(t, gormDB)
}

func seedPrePagesSourceMigrationData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	require.NoError(t, gormDB.Exec(`
		INSERT INTO of_pages_projects (
			id, name, slug, description, enabled, active_deployment_id, root_dir, entry_file
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		pagesMigrationProjectID,
		"Migration Site",
		"migration-site",
		"keep-project-data",
		true,
		pagesMigrationDeploymentID,
		"public",
		"home.html",
	).Error)
	require.NoError(t, gormDB.Exec(`
		INSERT INTO of_pages_deployments (
			id, project_id, deployment_number, checksum, status, upload_id, artifact_path,
			file_count, total_size, created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		pagesMigrationDeploymentID,
		pagesMigrationProjectID,
		1,
		strings.Repeat("a", 64),
		model.PagesDeploymentStatusActive,
		uint64(700001),
		"legacy/package.zip",
		2,
		int64(128),
		"user:1",
	).Error)
}

func assertPagesSourceMigrationUp(t *testing.T, gormDB *gorm.DB) {
	t.Helper()
	migrator := gormDB.Migrator()
	assert.True(t, migrator.HasTable(&model.PagesProjectSource{}))
	assert.True(t, migrator.HasTable(&model.PagesProjectSourceRuntime{}))
	assert.True(t, migrator.HasColumn(&model.PagesProject{}, "ContentConfigVersion"))
	assert.True(t, migrator.HasColumn(&model.PagesDeployment{}, "SourceType"))
	assert.True(t, migrator.HasColumn(&model.PagesDeployment{}, "SourceIdentity"))
	assert.True(t, migrator.HasColumn(&model.PagesDeployment{}, "SourceRevision"))
	assert.True(t, migrator.HasIndex(&model.PagesProjectSource{}, "idx_of_pages_project_sources_project_id"))
	assert.True(t, migrator.HasIndex(&model.PagesProjectSourceRuntime{}, "idx_of_pages_project_source_runtime_next_check_at"))
	assert.True(t, migrator.HasIndex(&model.PagesDeployment{}, "idx_of_pages_deployments_project_number"))
	assert.True(t, migrator.HasIndex(&model.PagesDeployment{}, "idx_of_pages_deployments_source_revision"))

	var project model.PagesProject
	require.NoError(t, gormDB.First(&project, pagesMigrationProjectID).Error)
	assert.Equal(t, 0, project.ContentConfigVersion)
	assert.Equal(t, "keep-project-data", project.Description)
	assert.Equal(t, "public", project.RootDir)
	assert.Equal(t, "home.html", project.EntryFile)

	var deployment model.PagesDeployment
	require.NoError(t, gormDB.First(&deployment, pagesMigrationDeploymentID).Error)
	assert.Equal(t, "manual_upload", deployment.SourceType)
	assert.Equal(t, "manual_upload", deployment.TriggerType)
	assert.Nil(t, deployment.SourceIdentity)
	assert.Nil(t, deployment.SourceRevision)
	assert.Equal(t, uint64(700001), deployment.UploadID)

	sourceID := createMigrationSourceRuntime(t, gormDB)
	assertPagesSourceConstraints(t, gormDB, sourceID)
}

func createMigrationSourceRuntime(t *testing.T, gormDB *gorm.DB) uint {
	t.Helper()
	source := model.PagesProjectSource{
		ProjectID:            pagesMigrationProjectID,
		SourceType:           "remote_url",
		RemoteURL:            "https://example.com/site.zip?token=secret",
		AllowInsecure:        false,
		CheckIntervalMinutes: 0,
		ConfigVersion:        1,
		SourceIdentity:       strings.Repeat("b", 64),
	}
	require.NoError(t, gormDB.Create(&source).Error)
	require.NotZero(t, source.ID)
	require.NoError(t, gormDB.Create(&model.PagesProjectSourceRuntime{
		SourceID:   source.ID,
		SyncStatus: "idle",
	}).Error)
	return source.ID
}

func assertPagesSourceConstraints(t *testing.T, gormDB *gorm.DB, sourceID uint) {
	t.Helper()

	duplicateSource := model.PagesProjectSource{
		ProjectID:      pagesMigrationProjectID,
		SourceType:     "remote_url",
		ConfigVersion:  1,
		SourceIdentity: strings.Repeat("c", 64),
	}
	assert.Error(t, gormDB.Create(&duplicateSource).Error)

	for number := 2; number <= 3; number++ {
		require.NoError(t, createMigrationDeployment(
			gormDB,
			number,
			strings.Repeat(string(rune('a'+number)), 64),
			nil,
			nil,
		))
	}

	identity := strings.Repeat("d", 64)
	revision := strings.Repeat("e", 64)
	require.NoError(t, createMigrationDeployment(
		gormDB,
		4,
		strings.Repeat("f", 64),
		&identity,
		&revision,
	))
	assert.Error(t, createMigrationDeployment(
		gormDB,
		5,
		strings.Repeat("0", 64),
		&identity,
		&revision,
	))
	assert.Error(t, createMigrationDeployment(
		gormDB,
		1,
		strings.Repeat("1", 64),
		nil,
		nil,
	))

	var runtime model.PagesProjectSourceRuntime
	require.NoError(t, gormDB.First(&runtime, sourceID).Error)
	assert.Equal(t, "idle", runtime.SyncStatus)
}

func createMigrationDeployment(
	gormDB *gorm.DB,
	deploymentNumber int,
	checksum string,
	identity *string,
	revision *string,
) error {
	return gormDB.Create(&model.PagesDeployment{
		ProjectID:        pagesMigrationProjectID,
		DeploymentNumber: deploymentNumber,
		Checksum:         checksum,
		Status:           model.PagesDeploymentStatusUploaded,
		UploadID:         uint64(710000 + deploymentNumber),
		ArtifactPath:     fmt.Sprintf("legacy/%d.zip", deploymentNumber),
		SourceType:       "manual_upload",
		SourceIdentity:   identity,
		SourceRevision:   revision,
		TriggerType:      "manual_upload",
	}).Error
}

func assertPagesSourceMigrationDown(t *testing.T, gormDB *gorm.DB) {
	t.Helper()
	migrator := gormDB.Migrator()
	assert.False(t, migrator.HasTable(&model.PagesProjectSource{}))
	assert.False(t, migrator.HasTable(&model.PagesProjectSourceRuntime{}))
	assert.False(t, migrator.HasColumn(&model.PagesProject{}, "ContentConfigVersion"))
	assert.False(t, migrator.HasColumn(&model.PagesDeployment{}, "SourceType"))
	assert.False(t, migrator.HasColumn(&model.PagesDeployment{}, "SourceIdentity"))
	assert.False(t, migrator.HasColumn(&model.PagesDeployment{}, "SourceRevision"))
	assert.False(t, migrator.HasIndex(&model.PagesDeployment{}, "idx_of_pages_deployments_project_number"))
	assert.False(t, migrator.HasIndex(&model.PagesDeployment{}, "idx_of_pages_deployments_source_revision"))
	assert.True(t, migrator.HasIndex(&model.PagesDeployment{}, "idx_of_pages_deployments_project_id"))
	assert.True(t, migrator.HasIndex(&model.PagesDeployment{}, "idx_of_pages_deployments_upload_id"))

	var project struct {
		Description        string
		RootDir            string
		EntryFile          string
		ActiveDeploymentID *uint
	}
	require.NoError(t, gormDB.Table("of_pages_projects").Where("id = ?", pagesMigrationProjectID).Take(&project).Error)
	assert.Equal(t, "keep-project-data", project.Description)
	assert.Equal(t, "public", project.RootDir)
	assert.Equal(t, "home.html", project.EntryFile)
	require.NotNil(t, project.ActiveDeploymentID)
	assert.Equal(t, pagesMigrationDeploymentID, *project.ActiveDeploymentID)

	var deployment struct {
		UploadID     uint64
		ArtifactPath string
		FileCount    int
		TotalSize    int64
	}
	require.NoError(t, gormDB.Table("of_pages_deployments").Where("id = ?", pagesMigrationDeploymentID).Take(&deployment).Error)
	assert.Equal(t, uint64(700001), deployment.UploadID)
	assert.Equal(t, "legacy/package.zip", deployment.ArtifactPath)
	assert.Equal(t, 2, deployment.FileCount)
	assert.Equal(t, int64(128), deployment.TotalSize)

	var count int64
	require.NoError(t, gormDB.Table("of_pages_deployments").Where("project_id = ?", pagesMigrationProjectID).Count(&count).Error)
	assert.Equal(t, int64(4), count)
}

func assertPagesSourceMigrationUpAgain(t *testing.T, gormDB *gorm.DB) {
	t.Helper()
	assert.True(t, gormDB.Migrator().HasTable(&model.PagesProjectSource{}))
	assert.True(t, gormDB.Migrator().HasTable(&model.PagesProjectSourceRuntime{}))
	assert.True(t, gormDB.Migrator().HasColumn(&model.PagesProject{}, "ContentConfigVersion"))
	assert.True(t, gormDB.Migrator().HasColumn(&model.PagesDeployment{}, "SourceRevision"))

	var count int64
	require.NoError(t, gormDB.Table("of_pages_deployments").
		Where("project_id = ? AND source_type = ? AND trigger_type = ?", pagesMigrationProjectID, "manual_upload", "manual_upload").
		Count(&count).Error)
	assert.Equal(t, int64(4), count)

	require.NoError(t, gormDB.Table("of_pages_project_sources").Count(&count).Error)
	assert.Zero(t, count, "source config is intentionally removed by Down and is not reconstructable")
}
