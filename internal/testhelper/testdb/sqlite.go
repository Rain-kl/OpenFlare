// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package testdb sets up databases for integration tests that use real migrations.
package testdb

import (
	"path/filepath"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/infra/config"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/infra/persistence/migrator"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/repository/logstore"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// OpenMigratedSQLite installs a temporary SQLite database as the process datastore.
// Tests using it must run serially because it changes process-wide configuration.
func OpenMigratedSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.sqlite")), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	previousEnabled := config.Config.Database.Enabled
	config.Config.Database.Enabled = false
	db.SetDB(database)
	logstore.ResetForTest()
	t.Cleanup(func() {
		logstore.ResetForTest()
		repository.ResetSystemConfigRAMCacheForTest()
		config.Config.Database.Enabled = previousEnabled
		db.SetDB(nil)
		require.NoError(t, sqlDB.Close())
	})
	migrator.Migrate()
	return database
}
