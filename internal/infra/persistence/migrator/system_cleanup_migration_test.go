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
	systemCleanupPreviousMigration = int64(202608090001)
	systemCleanupMigration         = int64(202608090002)
	systemCleanupTaskType          = "system_cleanup"
)

func TestSystemCleanupScheduleMigrationSQLite(t *testing.T) {
	dbPath := t.TempDir() + "/system-cleanup-migration.db"
	gormDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	sqlDB, err := gormDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	runSystemCleanupScheduleMigration(t, gormDB, sqlDB, dialectSqlite, "goose/sqlite")
}

func TestSystemCleanupScheduleMigrationPostgres(t *testing.T) {
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

	schema := fmt.Sprintf("system_cleanup_migration_%d", time.Now().UnixNano())
	require.Regexp(t, `^[a-z0-9_]+$`, schema)
	require.NoError(t, gormDB.Exec(`CREATE SCHEMA "`+schema+`"`).Error)
	require.NoError(t, gormDB.Exec(`SET search_path TO "`+schema+`"`).Error)
	t.Cleanup(func() {
		assert.NoError(t, gormDB.Exec("SET search_path TO public").Error)
		assert.NoError(t, gormDB.Exec(`DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`).Error)
		assert.NoError(t, sqlDB.Close())
	})

	runSystemCleanupScheduleMigration(t, gormDB, sqlDB, dialectPostgres, "goose/postgres")
}

func runSystemCleanupScheduleMigration(
	t *testing.T,
	gormDB *gorm.DB,
	sqlDB *sql.DB,
	dialect string,
	dir string,
) {
	t.Helper()
	goose.SetBaseFS(migrationFS)
	require.NoError(t, goose.SetDialect(dialect))
	require.NoError(t, goose.UpTo(sqlDB, dir, systemCleanupPreviousMigration))

	assertSystemCleanupCron(t, gormDB, "0 */2 * * *", "迁移前应为每 2 小时")

	require.NoError(t, goose.UpTo(sqlDB, dir, systemCleanupMigration))
	assertSystemCleanupCron(t, gormDB, "0 3 * * *", "迁移后应为每日凌晨 3 点")

	require.NoError(t, goose.DownTo(sqlDB, dir, systemCleanupPreviousMigration))
	assertSystemCleanupCron(t, gormDB, "0 */2 * * *", "回滚后恢复每 2 小时")
}

func assertSystemCleanupCron(t *testing.T, gormDB *gorm.DB, wantCron, msg string) {
	t.Helper()
	var schedule model.Schedule
	require.NoError(t, gormDB.Where("task_type = ?", systemCleanupTaskType).First(&schedule).Error)
	assert.Equal(t, wantCron, schedule.Cron, msg)
}
