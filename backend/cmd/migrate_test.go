// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type migrateTestDB struct {
	db *gorm.DB
}

func (s migrateTestDB) GORM() *gorm.DB { return s.db }

func (s migrateTestDB) DB(ctx context.Context) *gorm.DB { return s.db.WithContext(ctx) }

func (s migrateTestDB) Named(string) *gorm.DB { return s.db }

type migrateTestPlugin struct {
	name string
	db   *gorm.DB
	fs   fstest.MapFS
}

func (p *migrateTestPlugin) Name() string {
	if p.name != "" {
		return p.name
	}
	return "t"
}

func (p *migrateTestPlugin) Apply(ctx *core.Context) error {
	core.Provide[contracts.DBService](ctx, migrateTestDB{db: p.db})
	ctx.Migrations().Register(p.Name(), p.fs)
	return nil
}

func sqliteTableExists(t *testing.T, db *gorm.DB, name string) bool {
	t.Helper()
	var n int
	err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&n).Error
	require.NoError(t, err)
	return n > 0
}

func testMigrationFS() fstest.MapFS {
	return fstest.MapFS{
		"migrations/sqlite/00001_init.sql": &fstest.MapFile{Data: []byte(`-- +goose Up
CREATE TABLE t_up (id INTEGER PRIMARY KEY);

-- +goose Down
DROP TABLE t_up;
`)},
	}
}

func openMigrateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "migrate.db")
	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	return gdb
}

func TestGooseEngineMigrateOrderCreateTableBaselineUp(t *testing.T) {
	gdb := openMigrateTestDB(t)
	var order []string

	app := core.NewApp(
		core.WithMigrationEngine(&gooseEngine{}),
		core.WithMigrationBaseline(func(*core.Context) error {
			require.True(t, sqliteTableExists(t, gdb, "w_schema_versions"), "version table must exist before baseline")
			require.False(t, sqliteTableExists(t, gdb, "t_up"), "plugin Up must not run before baseline")
			order = append(order, "create-table", "baseline")
			return nil
		}),
		core.WithPlugins(&migrateTestPlugin{db: gdb, fs: testMigrationFS()}),
	)

	require.NoError(t, app.Prepare())
	require.NoError(t, app.ApplyPlugins())
	require.NoError(t, app.RunMigrations())

	require.True(t, sqliteTableExists(t, gdb, "t_up"), "plugin Up must run after baseline")
	order = append(order, "up")
	assert.Equal(t, []string{"create-table", "baseline", "up"}, order)
}

func TestGooseEngineBaselineErrorSkipsUp(t *testing.T) {
	gdb := openMigrateTestDB(t)

	app := core.NewApp(
		core.WithMigrationEngine(&gooseEngine{}),
		core.WithMigrationBaseline(func(*core.Context) error {
			require.True(t, sqliteTableExists(t, gdb, "w_schema_versions"), "version table must exist before baseline")
			return assert.AnError
		}),
		core.WithPlugins(&migrateTestPlugin{db: gdb, fs: testMigrationFS()}),
	)

	require.NoError(t, app.Prepare())
	require.NoError(t, app.ApplyPlugins())
	err := app.RunMigrations()
	require.Error(t, err)
	assert.ErrorContains(t, err, "migration baseline")
	assert.False(t, sqliteTableExists(t, gdb, "t_up"), "plugin Up must not run when baseline fails")
}

func TestGooseEngineNilBaselineStillMigrates(t *testing.T) {
	gdb := openMigrateTestDB(t)

	app := core.NewApp(
		core.WithMigrationEngine(&gooseEngine{}),
		core.WithPlugins(&migrateTestPlugin{db: gdb, fs: testMigrationFS()}),
	)

	require.NoError(t, app.Prepare())
	require.NoError(t, app.ApplyPlugins())
	require.NoError(t, app.RunMigrations())
	assert.True(t, sqliteTableExists(t, gdb, "w_schema_versions"))
	assert.True(t, sqliteTableExists(t, gdb, "t_up"))
}

func TestGooseEngineUpgradesFrom00001To00002(t *testing.T) {
	gdb := openMigrateTestDB(t)
	runTestMigrations(t, gdb, testMigrationFS(), "")
	require.True(t, sqliteTableExists(t, gdb, "t_up"))
	require.False(t, sqliteTableExists(t, gdb, "t_v2"))
	require.Equal(t, int64(1), pluginSchemaVersion(t, gdb, "t"))

	runTestMigrations(t, gdb, testMigrationFSWithV2("sqlite"), "")
	require.True(t, sqliteTableExists(t, gdb, "t_up"), "00001 table must survive 00002")
	require.True(t, sqliteTableExists(t, gdb, "t_v2"), "00002 must create t_v2")
	require.Equal(t, int64(2), pluginSchemaVersion(t, gdb, "t"))
	require.Equal(t, 1, tableRowCount(t, gdb, "t_v2"))

	runTestMigrations(t, gdb, testMigrationFSWithV2("sqlite"), "")
	require.Equal(t, int64(2), pluginSchemaVersion(t, gdb, "t"), "second 00002 run must be a no-op")
	require.Equal(t, 1, tableRowCount(t, gdb, "t_v2"), "00002 INSERT must not run twice")
}

func TestGooseEngineStampedV1AppliesOnly00002(t *testing.T) {
	gdb := openMigrateTestDB(t)
	require.NoError(t, gdb.Exec(`CREATE TABLE w_schema_versions (
		plugin_id VARCHAR(64) NOT NULL,
		version_id BIGINT NOT NULL,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (plugin_id, version_id)
	)`).Error)
	require.NoError(t, gdb.Exec(`INSERT INTO w_schema_versions (plugin_id, version_id) VALUES ('t', 1)`).Error)

	runTestMigrations(t, gdb, testMigrationFSWithV2("sqlite"), "")
	require.False(t, sqliteTableExists(t, gdb, "t_up"), "stamped v1 must not re-run 00001")
	require.True(t, sqliteTableExists(t, gdb, "t_v2"), "stamped v1 must still apply 00002")
	require.Equal(t, int64(2), pluginSchemaVersion(t, gdb, "t"))
}

func TestOpenFlareServerUpgradesFrom00001To00002(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "of.db")
	app := cordisPrepare(t, cordisSQLiteSource(t, dbPath))
	require.NoError(t, app.Context().Dispose())

	inspect := openInspectDB(t, dbPath, "")
	if !pluginHasVersion(t, inspect, false, serverPluginStamp, 1) {
		t.Fatal("fresh install did not apply server 00001")
	}
	_ = inspect.Close()

	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	require.NoError(t, err)
	runTestMigrations(t, gdb, serverFollowupFS("sqlite"), "server")

	require.False(t, sqliteTableExists(t, gdb, "should_not_exist_from_00001_rerun"))
	require.True(t, sqliteTableExists(t, gdb, "of_upgrade_probe"))
	require.Equal(t, int64(2), pluginSchemaVersion(t, gdb, "server"))
	require.True(t, sqliteTableExists(t, gdb, "of_zones"), "existing of_* tables must survive 00002")
	require.Equal(t, 1, tableRowCount(t, gdb, "of_upgrade_probe"))
}

func TestGooseEngineUpgradesFrom00001To00002Postgres(t *testing.T) {
	gdb := openMigratePostgresDB(t)
	opts := postgresMigrateOpt()
	runTestMigrations(t, gdb, testPostgresMigrationFS(), "", opts)
	require.True(t, pgTableExists(t, gdb, "t_up"))
	require.False(t, pgTableExists(t, gdb, "t_v2"))
	require.Equal(t, int64(1), pluginSchemaVersion(t, gdb, "t"))

	runTestMigrations(t, gdb, testMigrationFSWithV2("postgres"), "", opts)
	require.True(t, pgTableExists(t, gdb, "t_up"))
	require.True(t, pgTableExists(t, gdb, "t_v2"))
	require.Equal(t, int64(2), pluginSchemaVersion(t, gdb, "t"))
	require.Equal(t, 1, tableRowCount(t, gdb, "t_v2"))

	runTestMigrations(t, gdb, testMigrationFSWithV2("postgres"), "", opts)
	require.Equal(t, int64(2), pluginSchemaVersion(t, gdb, "t"))
	require.Equal(t, 1, tableRowCount(t, gdb, "t_v2"))
}

func TestOpenFlareServerUpgradesFrom00001To00002Postgres(t *testing.T) {
	host, port, user, pass, dbName, sslMode, cleanup := createMigratePostgresDB(t)
	t.Cleanup(cleanup)
	dsn := postgresDSN(host, port, user, pass, dbName, sslMode)
	app := cordisPrepare(t, cordisPostgresSource(t, host, port, user, pass, dbName, sslMode))
	require.NoError(t, app.Context().Dispose())

	inspect := openInspectDB(t, "", dsn)
	if !pluginHasVersion(t, inspect, true, serverPluginStamp, 1) {
		t.Fatal("fresh install did not apply server 00001")
	}
	_ = inspect.Close()

	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	require.NoError(t, err)
	runTestMigrations(t, gdb, serverFollowupFS("postgres"), "server", postgresMigrateOpt())

	require.False(t, pgTableExists(t, gdb, "should_not_exist_from_00001_rerun"))
	require.True(t, pgTableExists(t, gdb, "of_upgrade_probe"))
	require.Equal(t, int64(2), pluginSchemaVersion(t, gdb, "server"))
	require.True(t, pgTableExists(t, gdb, "of_zones"))
	require.Equal(t, 1, tableRowCount(t, gdb, "of_upgrade_probe"))
}

func runTestMigrations(t *testing.T, gdb *gorm.DB, fs fstest.MapFS, pluginName string, opts ...core.AppOption) {
	t.Helper()
	plugin := &migrateTestPlugin{name: pluginName, db: gdb, fs: fs}
	appOpts := []core.AppOption{
		core.WithMigrationEngine(&gooseEngine{}),
		core.WithPlugins(plugin),
	}
	appOpts = append(appOpts, opts...)
	app := core.NewApp(appOpts...)
	require.NoError(t, app.Prepare())
	require.NoError(t, app.ApplyPlugins())
	require.NoError(t, app.RunMigrations())
}

func postgresMigrateOpt() core.AppOption {
	return core.WithConfigSource(core.NewMapSource(map[string]any{
		"database": map[string]any{"enabled": true},
	}))
}

func testPostgresMigrationFS() fstest.MapFS {
	return fstest.MapFS{
		"migrations/postgres/00001_init.sql": &fstest.MapFile{Data: []byte(`-- +goose Up
CREATE TABLE t_up (id BIGINT PRIMARY KEY);

-- +goose Down
DROP TABLE t_up;
`)},
	}
}

func testMigrationFSWithV2(dialect string) fstest.MapFS {
	v1 := `-- +goose Up
CREATE TABLE t_up (id BIGINT PRIMARY KEY);

-- +goose Down
DROP TABLE t_up;
`
	v2 := `-- +goose Up
CREATE TABLE t_v2 (id BIGINT PRIMARY KEY, note TEXT NOT NULL DEFAULT '');
INSERT INTO t_v2 (id, note) VALUES (1, 'from-00002');

-- +goose Down
DROP TABLE t_v2;
`
	if dialect == "sqlite" {
		v1 = `-- +goose Up
CREATE TABLE t_up (id INTEGER PRIMARY KEY);

-- +goose Down
DROP TABLE t_up;
`
		v2 = `-- +goose Up
CREATE TABLE t_v2 (id INTEGER PRIMARY KEY, note TEXT NOT NULL DEFAULT '');
INSERT INTO t_v2 (id, note) VALUES (1, 'from-00002');

-- +goose Down
DROP TABLE t_v2;
`
	}
	return fstest.MapFS{
		"migrations/" + dialect + "/00001_init.sql":     &fstest.MapFile{Data: []byte(v1)},
		"migrations/" + dialect + "/00002_add_t_v2.sql": &fstest.MapFile{Data: []byte(v2)},
	}
}

func serverFollowupFS(dialect string) fstest.MapFS {
	v1 := `-- +goose Up
CREATE TABLE should_not_exist_from_00001_rerun (id INTEGER);

-- +goose Down
DROP TABLE should_not_exist_from_00001_rerun;
`
	v2 := `-- +goose Up
CREATE TABLE of_upgrade_probe (id INTEGER PRIMARY KEY, note TEXT NOT NULL DEFAULT '');
INSERT INTO of_upgrade_probe (id, note) VALUES (1, 'from-00002');

-- +goose Down
DROP TABLE of_upgrade_probe;
`
	if dialect == "postgres" {
		v1 = `-- +goose Up
CREATE TABLE should_not_exist_from_00001_rerun (id BIGINT);

-- +goose Down
DROP TABLE should_not_exist_from_00001_rerun;
`
		v2 = `-- +goose Up
CREATE TABLE of_upgrade_probe (id BIGINT PRIMARY KEY, note TEXT NOT NULL DEFAULT '');
INSERT INTO of_upgrade_probe (id, note) VALUES (1, 'from-00002');

-- +goose Down
DROP TABLE of_upgrade_probe;
`
	}
	return fstest.MapFS{
		"migrations/" + dialect + "/00001_initial.sql":       &fstest.MapFile{Data: []byte(v1)},
		"migrations/" + dialect + "/00002_upgrade_probe.sql": &fstest.MapFile{Data: []byte(v2)},
	}
}

func pluginSchemaVersion(t *testing.T, db *gorm.DB, pluginID string) int64 {
	t.Helper()
	var v int64
	err := db.Raw(`SELECT COALESCE(MAX(version_id), 0) FROM w_schema_versions WHERE plugin_id = ?`, pluginID).Scan(&v).Error
	require.NoError(t, err)
	return v
}

func tableRowCount(t *testing.T, db *gorm.DB, name string) int {
	t.Helper()
	if !safePGIdent(name) {
		t.Fatalf("unsafe table name %q", name)
	}
	var n int
	err := db.Raw("SELECT COUNT(*) FROM " + name).Scan(&n).Error
	require.NoError(t, err)
	return n
}

func pgTableExists(t *testing.T, db *gorm.DB, name string) bool {
	t.Helper()
	var n int
	err := db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = ?`, name).Scan(&n).Error
	require.NoError(t, err)
	return n > 0
}

func openMigratePostgresDB(t *testing.T) *gorm.DB {
	t.Helper()
	host, port, user, pass, dbName, sslMode, cleanup := createMigratePostgresDB(t)
	t.Cleanup(cleanup)
	dsn := postgresDSN(host, port, user, pass, dbName, sslMode)
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	require.NoError(t, err)
	return gdb
}

func createMigratePostgresDB(t *testing.T) (host string, port int, user, pass, dbName, sslMode string, cleanup func()) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_PG_DSN"))
	if dsn == "" {
		t.Skip("TEST_PG_DSN is not set")
	}
	host, port, user, pass, adminDB, sslMode := parsePostgresDSN(t, dsn)
	adminDSN := postgresDSN(host, port, user, pass, adminDB, sslMode)
	admin := openInspectDB(t, "", adminDSN)
	dbName = fmt.Sprintf("of_mig_%d", time.Now().UnixNano())
	if !safePGIdent(dbName) {
		t.Fatalf("generated database name %q is not a safe identifier", dbName)
	}
	if _, err := admin.Exec("CREATE DATABASE " + dbName); err != nil {
		t.Fatalf("CREATE DATABASE %s: %v", dbName, err)
	}
	cleanup = func() {
		_, _ = admin.Exec(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()`, dbName)
		_, _ = admin.Exec("DROP DATABASE IF EXISTS " + dbName)
		_ = admin.Close()
	}
	return host, port, user, pass, dbName, sslMode, cleanup
}
