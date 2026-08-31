// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	ofserver "Wavelet/openflare/plugins/server"
	"Wavelet/openflare/plugins/server/migrate"
	"Wavelet/plugins/domain/admin"
	"Wavelet/plugins/domain/auth"
	"Wavelet/plugins/domain/cap"
	"Wavelet/plugins/domain/message_gateway"
	"Wavelet/plugins/domain/risk_control"
	"Wavelet/plugins/domain/system"
	"Wavelet/plugins/domain/upload"
	"Wavelet/plugins/domain/user"
	"Wavelet/plugins/drivers/driver_asynq_cron"
	"Wavelet/plugins/drivers/driver_asynq_worker"
	"Wavelet/plugins/drivers/driver_http"
	"Wavelet/plugins/drivers/driver_inproc_cron"
	"Wavelet/plugins/drivers/driver_inproc_worker"
	"Wavelet/plugins/infra/cache"
	"Wavelet/plugins/infra/cache_memory"
	"Wavelet/plugins/infra/config"
	"Wavelet/plugins/infra/logger"
	"Wavelet/plugins/infra/storage"
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"time"

	"github.com/pressly/goose/v3"
	goosedb "github.com/pressly/goose/v3/database"
	"gorm.io/gorm"

	infradb "Wavelet/plugins/infra/database"
)

const (
	defaultShutdownTimeout = 15 * time.Second
	defaultHTTPAddr        = "127.0.0.1:8000"

	// migrationAdvisoryLockKey serializes baseline + plugin Up across Postgres
	// sessions (ASCII "wave"). SQLite is single-writer and needs no extra lock.
	migrationAdvisoryLockKey int64 = 0x77617665
)

// runProfileApp prepares and runs the application for a given profile.
func runProfileApp(profile core.Profile, mode string, listensForHTTP bool) {
	app := newOpenFlareApp(profile)
	if err := app.Prepare(); err != nil {
		log.Fatalf("[%s] prepare failed: %v\n", mode, err)
	}
	state := startupState{
		mode:           mode,
		listensForHTTP: listensForHTTP,
		env:            app.Context().Config().String("app.env", "production"),
	}
	if listensForHTTP {
		state.addr = app.Context().Config().String("app.addr", defaultHTTPAddr)
	}
	printStartupBanner(state)
	if err := app.Run(); err != nil {
		log.Fatalf("[%s] run failed: %v\n", mode, err)
	}
}

// newOpenFlareApp creates a core.App wired with Wavelet platform plugins plus the OpenFlare server plugin.
func newOpenFlareApp(profile core.Profile, opts ...core.AppOption) *core.App {
	src, err := config.NewSource()
	if err != nil {
		log.Fatalf("[App] load config source failed: %v\n", err)
	}

	appOpts := []core.AppOption{
		core.WithProfile(profile),
		core.WithConfigSource(src),
		core.WithShutdownTimeout(defaultShutdownTimeout),
		core.WithMigrationBaseline(migrate.Legacy),
	}
	appOpts = append(appOpts, opts...)

	app := core.NewApp(appOpts...)

	// 1. Register standard infrastructure plugins
	app.Use(
		infradb.New(),
		logger.New(),
		storage.New(),
	)

	// 2. Register Cache and Async/Cron Drivers (both gated: cache vs cache_memory, asynq vs inproc)
	app.Use(
		cache.New(),
		cache_memory.New(),
		driver_asynq_worker.New(),
		driver_inproc_worker.New(),
		driver_asynq_cron.New(),
		driver_inproc_cron.New(),
	)

	// 3. Register all 8 domain business plugins (admin first to ensure schema and base config tables exist)
	app.Use(
		admin.New(),
		user.New(),
		auth.New(),
		message_gateway.New(),
		risk_control.New(),
		upload.New(),
		cap.New(),
		system.New(),
	)

	// 4. OpenFlare business routes (after domain plugins, before the HTTP driver)
	app.Use(
		ofserver.New(),
	)

	// 5. Bind Goose migration engine
	app.SetMigrationEngine(&gooseEngine{})

	// 6. Mount HTTP runtime driver
	app.Use(
		driver_http.New(),
	)

	return app
}

// ─── Schema Version Store ──────────────────────────────────────────────────────

// sharedStore implements database.Store using a single w_schema_versions table.
// All plugins share this table, with plugin_id as the discriminator.
//
// Schema:
//
//	w_schema_versions (
//	    plugin_id   VARCHAR(64) NOT NULL,
//	    version_id  BIGINT      NOT NULL,
//	    applied_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
//	    PRIMARY KEY (plugin_id, version_id)
//	)
type sharedStore struct {
	pluginID string
	dialect  string // "postgres" or "sqlite3"
}

func (s *sharedStore) Tablename() string { return "w_schema_versions" }

func (s *sharedStore) CreateVersionTable(ctx context.Context, db goosedb.DBTxConn) error {
	_, err := db.ExecContext(ctx, schemaVersionsDDL(s.dialect))
	return err
}

func schemaVersionsDDL(dialect string) string {
	timeType := "TIMESTAMPTZ"
	if dialect == "sqlite3" || dialect == "sqlite" {
		timeType = "DATETIME"
	}
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS w_schema_versions (
		plugin_id   VARCHAR(64)  NOT NULL,
		version_id  BIGINT       NOT NULL,
		applied_at  %s  NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (plugin_id, version_id)
	)`, timeType)
}

//nolint:mnd
func (s *sharedStore) Insert(ctx context.Context, db goosedb.DBTxConn, req goosedb.InsertRequest) error {
	p := s.placeholder
	_, err := db.ExecContext(ctx,
		fmt.Sprintf("INSERT INTO w_schema_versions (plugin_id, version_id) VALUES (%s, %s) ON CONFLICT (plugin_id, version_id) DO NOTHING", p(1), p(2)),
		s.pluginID, req.Version)
	return err
}

//nolint:mnd
func (s *sharedStore) Delete(ctx context.Context, db goosedb.DBTxConn, version int64) error {
	p := s.placeholder
	_, err := db.ExecContext(ctx,
		fmt.Sprintf("DELETE FROM w_schema_versions WHERE plugin_id = %s AND version_id = %s", p(1), p(2)),
		s.pluginID, version)
	return err
}

//nolint:mnd
func (s *sharedStore) GetMigration(ctx context.Context, db goosedb.DBTxConn, version int64) (*goosedb.GetMigrationResult, error) {
	p := s.placeholder
	var t time.Time
	err := db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT applied_at FROM w_schema_versions WHERE plugin_id = %s AND version_id = %s", p(1), p(2)),
		s.pluginID, version).Scan(&t)
	if err == sql.ErrNoRows {
		return nil, goosedb.ErrVersionNotFound
	}
	if err != nil {
		return nil, err
	}
	return &goosedb.GetMigrationResult{Timestamp: t, IsApplied: true}, nil
}

func (s *sharedStore) GetLatestVersion(ctx context.Context, db goosedb.DBTxConn) (int64, error) {
	p := s.placeholder
	var version int64
	err := db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COALESCE(MAX(version_id), 0) FROM w_schema_versions WHERE plugin_id = %s", p(1)),
		s.pluginID).Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

func (s *sharedStore) ListMigrations(ctx context.Context, db goosedb.DBTxConn) ([]*goosedb.ListMigrationsResult, error) {
	p := s.placeholder
	rows, err := db.QueryContext(ctx,
		fmt.Sprintf("SELECT version_id, TRUE FROM w_schema_versions WHERE plugin_id = %s ORDER BY version_id DESC", p(1)),
		s.pluginID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []*goosedb.ListMigrationsResult
	for rows.Next() {
		var r goosedb.ListMigrationsResult
		if err := rows.Scan(&r.Version, &r.IsApplied); err != nil {
			return nil, err
		}
		results = append(results, &r)
	}
	return results, rows.Err()
}

func (s *sharedStore) placeholder(n int) string {
	if s.dialect == "postgres" {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

// ─── Migration Engine ──────────────────────────────────────────────────────────

// gooseEngine implements core.MigrationEngine by iterating all plugin-registered
// migration entries and applying each plugin's migrations against the shared DB.
//
// Each plugin owns its own `migrations/*.sql` directory, embedded via go:embed
// and registered via ctx.Migrations().Register(pluginID, embedFS).
//
// Version tracking: all plugins share a single w_schema_versions table with
// plugin_id as the discriminator column. Querying this table shows the current
// migration version of every plugin at a glance.
type gooseEngine struct{}

func (e *gooseEngine) Migrate(ctx *core.Context, entries []core.MigrationEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// Resolve DBService from the IoC container.
	var dbSvc contracts.DBService
	if err := core.Using[contracts.DBService](ctx, func(svc contracts.DBService) {
		dbSvc = svc
	}); err != nil {
		return fmt.Errorf("migration: resolve DBService: %w", err)
	}

	gormDB := dbSvc.GORM()
	if gormDB == nil {
		return fmt.Errorf("migration: DBService.GORM() returned nil")
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return fmt.Errorf("migration: get underlying DB from GORM: %w", err)
	}

	dialect := gooseDialectFromGORM(gormDB, ctx)
	dialectStr := string(dialect)
	goCtx := context.Background()
	if ctx != nil {
		goCtx = ctx.GoContext()
	}
	if goCtx == nil {
		goCtx = context.Background()
	}

	bootstrap := &sharedStore{dialect: dialectStr}
	if err := bootstrap.CreateVersionTable(goCtx, sqlDB); err != nil {
		return fmt.Errorf("migration: create version table: %w", err)
	}

	if dialect == goose.DialectPostgres {
		conn, lockErr := sqlDB.Conn(goCtx)
		if lockErr != nil {
			return fmt.Errorf("migration: pin connection for advisory lock: %w", lockErr)
		}
		defer func() { _ = conn.Close() }()
		if _, lockErr = conn.ExecContext(goCtx, "SELECT pg_advisory_lock($1)", migrationAdvisoryLockKey); lockErr != nil {
			return fmt.Errorf("migration: advisory lock: %w", lockErr)
		}
		defer func() {
			_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", migrationAdvisoryLockKey)
		}()
	}

	if fn := ctx.MigrationBaseline(); fn != nil {
		if err := fn(ctx); err != nil {
			return fmt.Errorf("migration baseline: %w", err)
		}
	}

	for _, entry := range entries {
		store := &sharedStore{
			pluginID: entry.PluginID,
			dialect:  dialectStr,
		}

		migrationFS := findMigrationFS(entry.FS, dialect)
		provider, err := goose.NewProvider(goose.DialectCustom, sqlDB, migrationFS, goose.WithStore(store))
		if err != nil {
			return fmt.Errorf("migration %s: create provider: %w", entry.PluginID, err)
		}

		results, err := provider.Up(context.Background())
		if err != nil {
			return fmt.Errorf("migration %s: apply %w", entry.PluginID, err)
		}

		version, vErr := provider.GetDBVersion(context.Background())
		if vErr != nil {
			version = 0
		}

		if len(results) > 0 {
			log.Printf("[migrate] %s: applied %d migration(s) (v%d)", entry.PluginID, len(results), version)
		} else {
			log.Printf("[migrate] %s: v%d", entry.PluginID, version)
		}
	}

	return nil
}

// gooseDialectFromGORM prefers the live driver; config is only a fallback when
// GORM has no dialector yet (tests that inject a stub DBService).
func gooseDialectFromGORM(gormDB *gorm.DB, ctx *core.Context) goose.Dialect {
	if gormDB != nil && gormDB.Dialector != nil && gormDB.Dialector.Name() == "postgres" {
		return goose.DialectPostgres
	}
	if gormDB != nil && gormDB.Dialector != nil && gormDB.Dialector.Name() == "sqlite" {
		return goose.DialectSQLite3
	}
	return gooseDialect(ctx)
}

// gooseDialect returns the goose dialect based on the configured database engine.
func gooseDialect(ctx *core.Context) goose.Dialect {
	if ctx != nil && ctx.Config() != nil && ctx.Config().Bool("database.enabled", false) {
		return goose.DialectPostgres
	}
	return goose.DialectSQLite3
}

func findMigrationFS(rootFS fs.FS, dialect goose.Dialect) fs.FS {
	dialectDir := "postgres"
	if dialect == goose.DialectSQLite3 {
		dialectDir = "sqlite"
	}

	// 1. Direct search for dialect folder (e.g., "sqlite", "migrations/sqlite", "logstore/migrations/sqlite")
	for _, subDir := range []string{
		dialectDir,
		"migrations/" + dialectDir,
		"logstore/migrations/" + dialectDir,
	} {
		if sub, err := fs.Sub(rootFS, subDir); err == nil {
			if matches, err := fs.Glob(sub, "*.sql"); err == nil && len(matches) > 0 {
				return sub
			}
		}
	}

	// 2. Recursive walk to find a directory named dialectDir with *.sql files
	var foundDir string
	_ = fs.WalkDir(rootFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && filepath.Base(path) == dialectDir {
			if sub, subErr := fs.Sub(rootFS, path); subErr == nil {
				if matches, globErr := fs.Glob(sub, "*.sql"); globErr == nil && len(matches) > 0 {
					foundDir = path
					return fs.SkipAll
				}
			}
		}
		return nil
	})

	if foundDir != "" && foundDir != "." {
		if sub, err := fs.Sub(rootFS, foundDir); err == nil {
			return sub
		}
	}

	// 3. Fallback to generic migrations / root if dialect specific is not present
	for _, subDir := range []string{"migrations", "logstore/migrations"} {
		if sub, err := fs.Sub(rootFS, subDir); err == nil {
			if matches, err := fs.Glob(sub, "*.sql"); err == nil && len(matches) > 0 {
				return sub
			}
		}
	}

	return rootFS
}
