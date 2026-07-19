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
	pagesSourceScanPreviousMigration = int64(202607190001)
	pagesSourceScanMigration         = int64(202607190002)
	pagesSourceScanTaskType          = "of_pages_source_scan"
)

func TestPagesSourceScanScheduleMigrationSQLite(t *testing.T) {
	dbPath := t.TempDir() + "/pages-source-scan-migration.db"
	gormDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	sqlDB, err := gormDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	runPagesSourceScanScheduleMigration(t, gormDB, sqlDB, dialectSqlite, "goose/sqlite")
}

func TestPagesSourceScanScheduleMigrationPostgres(t *testing.T) {
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

	schema := fmt.Sprintf("pages_source_scan_migration_%d", time.Now().UnixNano())
	require.Regexp(t, `^[a-z0-9_]+$`, schema)
	require.NoError(t, gormDB.Exec(`CREATE SCHEMA "`+schema+`"`).Error)
	require.NoError(t, gormDB.Exec(`SET search_path TO "`+schema+`"`).Error)
	t.Cleanup(func() {
		assert.NoError(t, gormDB.Exec("SET search_path TO public").Error)
		assert.NoError(t, gormDB.Exec(`DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`).Error)
		assert.NoError(t, sqlDB.Close())
	})

	runPagesSourceScanScheduleMigration(t, gormDB, sqlDB, dialectPostgres, "goose/postgres")
}

func runPagesSourceScanScheduleMigration(
	t *testing.T,
	gormDB *gorm.DB,
	sqlDB *sql.DB,
	dialect string,
	dir string,
) {
	t.Helper()
	goose.SetBaseFS(migrationFS)
	require.NoError(t, goose.SetDialect(dialect))
	require.NoError(t, goose.UpTo(sqlDB, dir, pagesSourceScanPreviousMigration))

	var previousMaxID uint64
	require.NoError(t, gormDB.Table("w_schedules").Select("COALESCE(MAX(id), 0)").Scan(&previousMaxID).Error)
	require.NoError(t, goose.UpTo(sqlDB, dir, pagesSourceScanMigration))
	seeded := assertPagesSourceScanSchedule(t, gormDB)
	assert.NotZero(t, seeded.ID)
	if dialect == dialectPostgres {
		assert.Greater(t, seeded.ID, previousMaxID)
	}

	require.NoError(t, goose.DownTo(sqlDB, dir, pagesSourceScanPreviousMigration))
	assertPagesSourceScanScheduleMissing(t, gormDB)

	custom := model.Schedule{
		ID:       900001,
		Name:     "用户保留的 Pages 扫描任务",
		TaskType: pagesSourceScanTaskType,
		Cron:     "0 * * * *",
		Payload:  `{"custom":true}`,
		IsActive: false,
	}
	require.NoError(t, gormDB.Create(&custom).Error)
	require.NoError(t, goose.UpTo(sqlDB, dir, pagesSourceScanMigration))
	var schedules []model.Schedule
	require.NoError(t, gormDB.Where("task_type = ?", pagesSourceScanTaskType).Find(&schedules).Error)
	require.Len(t, schedules, 1)
	assert.Equal(t, custom.ID, schedules[0].ID)
	assert.Equal(t, custom.Name, schedules[0].Name)

	require.NoError(t, goose.DownTo(sqlDB, dir, pagesSourceScanPreviousMigration))
	var retained model.Schedule
	require.NoError(t, gormDB.First(&retained, custom.ID).Error)
	assert.Equal(t, custom.TaskType, retained.TaskType)
}

func TestPagesSourceScanScheduleMigrationsUseDatabaseGeneratedIDs(t *testing.T) {
	for _, name := range []string{
		"goose/postgres/202607190002_seed_pages_source_scan.sql",
		"goose/sqlite/202607190002_seed_pages_source_scan.sql",
	} {
		t.Run(name, func(t *testing.T) {
			content, err := migrationFS.ReadFile(name)
			require.NoError(t, err)
			normalized := strings.ToLower(string(content))
			assert.NotContains(t, normalized, "insert into w_schedules (id,")
			assert.NotContains(t, normalized, "coalesce(max(id)")
		})
	}

	postgresContent, err := migrationFS.ReadFile("goose/postgres/202607190002_seed_pages_source_scan.sql")
	require.NoError(t, err)
	compactPostgres := strings.Join(strings.Fields(strings.ToLower(string(postgresContent))), " ")
	assert.Contains(
		t,
		compactPostgres,
		"select setval( pg_get_serial_sequence('w_schedules', 'id'), greatest( 1,",
		"sequence synchronization must retain a valid lower bound for an empty table",
	)
}

func assertPagesSourceScanSchedule(t *testing.T, gormDB *gorm.DB) model.Schedule {
	t.Helper()
	var schedules []model.Schedule
	require.NoError(t, gormDB.Where("task_type = ?", pagesSourceScanTaskType).Find(&schedules).Error)
	require.Len(t, schedules, 1)
	schedule := schedules[0]
	assert.Equal(t, "OpenFlare Pages 部署源扫描", schedule.Name)
	assert.Equal(t, "*/5 * * * *", schedule.Cron)
	assert.Equal(t, "{}", schedule.Payload)
	assert.True(t, schedule.IsActive)
	return schedule
}

func assertPagesSourceScanScheduleMissing(t *testing.T, gormDB *gorm.DB) {
	t.Helper()
	var count int64
	require.NoError(t, gormDB.Model(&model.Schedule{}).
		Where("task_type = ?", pagesSourceScanTaskType).
		Count(&count).Error)
	assert.Zero(t, count)
}
