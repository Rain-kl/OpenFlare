// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"Wavelet/core"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const (
	goldenRoot        = "/Users/ryan/Code/Go/OpenFlare"
	goldCommit        = "9f79fb99"
	goldGooseVersion  = int64(202608090003)
	sampleZoneDomain  = "l3-upgrade-golden.example"
	goldMigrateWait   = 75 * time.Second
	legacyPluginStamp = "openflare/legacy"
	serverPluginStamp = "server"
)

var (
	goldBinOnce sync.Once
	goldBinPath string
	goldSrcDir  string
	goldBinErr  error
)

func TestUpgradeFromGolden(t *testing.T) {
	t.Run("sqlite", func(t *testing.T) {
		tmp := t.TempDir()
		dbPath := filepath.Join(tmp, "a.db")
		runGoldenAPI(t, tmp, goldSQLiteEnv(t, tmp, dbPath), func() bool {
			return sqliteReady(dbPath)
		})
		assertUpgradeFromGolden(t, upgradeDB{
			sqlitePath: dbPath,
			source:     cordisSQLiteSource(t, dbPath),
		})
	})
}

func TestUpgradePostgresFromGolden(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_PG_DSN"))
	if dsn == "" {
		t.Skip("TEST_PG_DSN is not set")
	}

	host, port, user, pass, adminDB, sslMode := parsePostgresDSN(t, dsn)
	adminDSN := postgresDSN(host, port, user, pass, adminDB, sslMode)
	admin := openInspectDB(t, "", adminDSN)
	t.Cleanup(func() { _ = admin.Close() })

	dbName := fmt.Sprintf("of_l3_%d", time.Now().UnixNano())
	if !safePGIdent(dbName) {
		t.Fatalf("generated database name %q is not a safe identifier", dbName)
	}
	if _, err := admin.Exec("CREATE DATABASE " + dbName); err != nil {
		t.Fatalf("CREATE DATABASE %s: %v", dbName, err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()`, dbName)
		_, _ = admin.Exec("DROP DATABASE IF EXISTS " + dbName)
	})

	tmp := t.TempDir()
	testDSN := postgresDSN(host, port, user, pass, dbName, sslMode)
	runGoldenAPI(t, tmp, goldPostgresEnv(t, tmp, host, port, user, pass, dbName, sslMode), func() bool {
		return postgresReady(testDSN)
	})
	assertUpgradeFromGolden(t, upgradeDB{
		pgDSN:  testDSN,
		source: cordisPostgresSource(t, host, port, user, pass, dbName, sslMode),
	})
}

func TestUpgradePostgresFromExistingDump(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_PG_EXISTING_DSN"))
	if dsn == "" {
		t.Skip("TEST_PG_EXISTING_DSN is not set")
	}

	host, port, user, pass, dbName, sslMode := parsePostgresDSN(t, dsn)
	spec := upgradeDB{
		pgDSN:  dsn,
		source: cordisPostgresSource(t, host, port, user, pass, dbName, sslMode),
	}

	inspect := openInspectDB(t, "", spec.pgDSN)
	beforeCounts := countNamedTables(t, inspect, productionCountTables)
	beforeTables := listPublicTables(t, inspect)
	_ = inspect.Close()

	assertUpgradeFromGolden(t, spec)

	inspect = openInspectDB(t, "", spec.pgDSN)
	defer func() { _ = inspect.Close() }()
	afterCounts := countNamedTables(t, inspect, productionCountTables)
	for _, name := range productionCountTables {
		if afterCounts[name] < beforeCounts[name] {
			t.Errorf("row count dropped for %s: before %d after %d", name, beforeCounts[name], afterCounts[name])
		}
	}
	afterTables := listPublicTables(t, inspect)
	for name := range beforeTables {
		if !afterTables[name] {
			t.Errorf("table %s dropped", name)
		}
	}
	for _, name := range []string{"w_schema_versions", "w_message_channels", "w_message_bindings", "w_message_pairing_codes"} {
		if !afterTables[name] {
			t.Errorf("expected upgrade to create %s", name)
		}
	}
	var n int
	if err := inspect.QueryRow(`SELECT COUNT(*) FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhparent WHERE c.relname IN ('of_node_access_logs', 'w_user_access_logs')`).Scan(&n); err != nil {
		t.Fatalf("count partitions: %v", err)
	}
	if n < 8 {
		t.Errorf("partition children = %d, want at least 8", n)
	}
}

var productionCountTables = []string{
	"of_zones", "of_zone_domains", "of_proxy_routes", "of_nodes", "of_origins",
	"of_tls_certificates", "of_waf_rule_groups", "of_pages_projects",
	"w_users", "w_schedules", "w_system_configs", "w_templates", "w_uploads",
	"of_node_access_logs", "w_user_access_logs",
}

func countNamedTables(t *testing.T, db *sql.DB, tables []string) map[string]int {
	t.Helper()
	out := make(map[string]int, len(tables))
	for _, name := range tables {
		if !safePGIdent(name) {
			t.Fatalf("unsafe table name %q", name)
		}
		var n int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + name).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", name, err)
		}
		out[name] = n
	}
	return out
}

func listPublicTables(t *testing.T, db *sql.DB) map[string]bool {
	t.Helper()
	rows, err := db.Query(`SELECT tablename FROM pg_tables WHERE schemaname = 'public'`)
	if err != nil {
		t.Fatalf("list public tables: %v", err)
	}
	defer func() { _ = rows.Close() }()
	out := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		out[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("list public tables: %v", err)
	}
	return out
}

type upgradeDB struct {
	sqlitePath string
	pgDSN      string
	source     core.ConfigSource
}

func assertUpgradeFromGolden(t *testing.T, spec upgradeDB) {
	t.Helper()

	inspect := openInspectDB(t, spec.sqlitePath, spec.pgDSN)
	before := dumpOfSchema(t, inspect, spec.pgDSN != "")
	insertSQL := `INSERT INTO of_zones (domain) VALUES (?)`
	if spec.pgDSN != "" {
		insertSQL = `INSERT INTO of_zones (domain) VALUES ($1)`
	}
	if _, err := inspect.Exec(insertSQL, sampleZoneDomain); err != nil {
		t.Fatalf("insert sample of_zones row: %v", err)
	}
	_ = inspect.Close()

	app := cordisPrepare(t, spec.source)
	legacyRows := schemaPluginRows(t, spec, legacyPluginStamp)
	assertStampedUpgrade(t, spec, before, legacyRows)
	if err := app.Context().Dispose(); err != nil {
		t.Fatalf("dispose first app: %v", err)
	}

	app2 := cordisPrepare(t, spec.source)
	t.Cleanup(func() { _ = app2.Context().Dispose() })
	if got := schemaPluginRows(t, spec, legacyPluginStamp); got != legacyRows {
		t.Fatalf("second Prepare increased %s rows: got %d, want %d", legacyPluginStamp, got, legacyRows)
	}
	assertStampedUpgrade(t, spec, before, legacyRows)
}

func cordisPrepare(t *testing.T, src core.ConfigSource) *core.App {
	t.Helper()
	app := newOpenFlareApp(core.ProfileAPI, core.WithConfigSource(src))
	if err := app.Prepare(); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if err := app.ApplyPlugins(); err != nil {
		t.Fatalf("ApplyPlugins: %v", err)
	}
	if err := app.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return app
}

func assertStampedUpgrade(t *testing.T, spec upgradeDB, before map[string][]string, legacyRows int) {
	t.Helper()
	db := openInspectDB(t, spec.sqlitePath, spec.pgDSN)
	defer func() { _ = db.Close() }()
	postgres := spec.pgDSN != ""

	if got := gooseMaxVersion(t, db); got != goldGooseVersion {
		t.Errorf("goose_db_version max = %d, want %d", got, goldGooseVersion)
	}
	if legacyRows < 2 {
		t.Errorf("w_schema_versions %s rows = %d, want at least 2 (0 and %d)", legacyPluginStamp, legacyRows, goldGooseVersion)
	}
	if !pluginHasVersion(t, db, postgres, legacyPluginStamp, 0) {
		t.Errorf("missing w_schema_versions (%s, 0)", legacyPluginStamp)
	}
	if !pluginHasVersion(t, db, postgres, legacyPluginStamp, goldGooseVersion) {
		t.Errorf("missing w_schema_versions (%s, %d)", legacyPluginStamp, goldGooseVersion)
	}
	if !pluginHasVersion(t, db, postgres, serverPluginStamp, 1) {
		t.Errorf("missing w_schema_versions (%s, 1)", serverPluginStamp)
	}

	var domain string
	q := `SELECT domain FROM of_zones WHERE domain = ?`
	if postgres {
		q = `SELECT domain FROM of_zones WHERE domain = $1`
	}
	if err := db.QueryRow(q, sampleZoneDomain).Scan(&domain); err != nil {
		t.Errorf("sample of_zones row missing after upgrade: %v", err)
	}

	after := dumpOfSchema(t, db, postgres)
	for table, cols := range before {
		got, ok := after[table]
		if !ok {
			t.Errorf("of_* table %s dropped", table)
			continue
		}
		have := make(map[string]bool, len(got))
		for _, c := range got {
			have[c] = true
		}
		for _, c := range cols {
			if !have[c] {
				t.Errorf("of_* column %s.%s dropped", table, c)
			}
		}
	}
}

func runGoldenAPI(t *testing.T, workDir string, env []string, ready func() bool) {
	t.Helper()
	bin := buildGoldenBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), goldMigrateWait)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "api")
	cmd.Dir = workDir
	cmd.Env = env
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		t.Fatalf("start golden api: %v", err)
	}

	waitErr := make(chan error, 1)
	go func() { waitErr <- cmd.Wait() }()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ready() {
			killGolden(cmd)
			<-waitErr
			return
		}
		select {
		case err := <-waitErr:
			if ready() {
				return
			}
			t.Fatalf("golden api exited before goose %d: %v\n%s", goldGooseVersion, err, out.String())
		case <-ctx.Done():
			killGolden(cmd)
			<-waitErr
			t.Fatalf("timeout waiting for golden goose %d\n%s", goldGooseVersion, out.String())
		case <-ticker.C:
		}
	}
}

func killGolden(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}

func buildGoldenBinary(t *testing.T) string {
	t.Helper()
	goldBinOnce.Do(func() {
		src, err := os.MkdirTemp("", "of-gold-src-")
		if err != nil {
			goldBinErr = err
			return
		}
		archive := exec.Command("git", "-C", goldenRoot, "archive", goldCommit)
		extract := exec.Command("tar", "-x", "-C", src)
		pipe, err := archive.StdoutPipe()
		if err != nil {
			goldBinErr = fmt.Errorf("gold archive pipe: %w", err)
			return
		}
		extract.Stdin = pipe
		var archiveErr, extractErr bytes.Buffer
		archive.Stderr = &archiveErr
		extract.Stderr = &extractErr
		if err := archive.Start(); err != nil {
			goldBinErr = fmt.Errorf("git archive %s: %w", goldCommit, err)
			return
		}
		if err := extract.Start(); err != nil {
			_ = archive.Process.Kill()
			goldBinErr = fmt.Errorf("extract gold %s: %w", goldCommit, err)
			return
		}
		if err := extract.Wait(); err != nil {
			_ = archive.Wait()
			goldBinErr = fmt.Errorf("extract gold %s: %w\n%s", goldCommit, err, extractErr.String())
			return
		}
		if err := archive.Wait(); err != nil {
			goldBinErr = fmt.Errorf("git archive %s: %w\n%s", goldCommit, err, archiveErr.String())
			return
		}
		if _, err := os.Stat(filepath.Join(src, "main.go")); err != nil {
			goldBinErr = fmt.Errorf("gold %s at %s: %w", goldCommit, src, err)
			return
		}
		goldSrcDir = src

		dir, err := os.MkdirTemp("", "of-gold-bin-")
		if err != nil {
			goldBinErr = err
			return
		}
		out := filepath.Join(dir, "gold")
		cmd := exec.Command("go", "build", "-o", out, ".")
		cmd.Dir = src
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		if err := cmd.Run(); err != nil {
			goldBinErr = fmt.Errorf("go build golden %s: %w\n%s", goldCommit, err, buf.String())
			return
		}
		goldBinPath = out
	})
	if goldBinErr != nil {
		t.Fatalf("%v", goldBinErr)
	}
	return goldBinPath
}

func copyGoldConfig(t *testing.T, dir string) string {
	t.Helper()
	buildGoldenBinary(t)
	dst := filepath.Join(dir, "config.yaml")
	src, err := os.Open(filepath.Join(goldSrcDir, "config.example.yaml")) //nolint:gosec // extracted gold snapshot
	if err != nil {
		t.Fatalf("open golden config.example.yaml: %v", err)
	}
	defer func() { _ = src.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // test temp file
	if err != nil {
		t.Fatalf("create temp config.yaml: %v", err)
	}
	if _, err := io.Copy(out, src); err != nil {
		_ = out.Close()
		t.Fatalf("copy golden config: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close temp config.yaml: %v", err)
	}
	return dst
}

func goldSQLiteEnv(t *testing.T, dir, dbPath string) []string {
	t.Helper()
	cfg := copyGoldConfig(t, dir)
	addr := freeLocalAddr(t)
	return filteredGoldEnv(
		"CONFIG_PATH="+cfg,
		"SQLITE_PATH="+dbPath,
		"DB_ENABLED=false",
		"REDIS_ENABLED=false",
		"CLICKHOUSE_ENABLED=false",
		"APP_ENV=testing",
		"APP_ADDR="+addr,
	)
}

func goldPostgresEnv(t *testing.T, dir, host string, port int, user, pass, dbName, sslMode string) []string {
	t.Helper()
	cfg := copyGoldConfig(t, dir)
	addr := freeLocalAddr(t)
	return filteredGoldEnv(
		"CONFIG_PATH="+cfg,
		"DB_ENABLED=true",
		"DB_HOST="+host,
		"DB_PORT="+strconv.Itoa(port),
		"DB_USERNAME="+user,
		"DB_PASSWORD="+pass,
		"DB_NAME="+dbName,
		"DB_SSL_MODE="+sslMode,
		"REDIS_ENABLED=false",
		"CLICKHOUSE_ENABLED=false",
		"APP_ENV=testing",
		"APP_ADDR="+addr,
	)
}

func filteredGoldEnv(extra ...string) []string {
	drop := map[string]bool{
		"CONFIG_PATH":        true,
		"SQLITE_PATH":        true,
		"DB_ENABLED":         true,
		"DB_HOST":            true,
		"DB_PORT":            true,
		"DB_USERNAME":        true,
		"DB_PASSWORD":        true,
		"DB_NAME":            true,
		"DB_SSL_MODE":        true,
		"REDIS_ENABLED":      true,
		"REDIS_ADDR":         true,
		"CLICKHOUSE_ENABLED": true,
		"CLICKHOUSE_HOST":    true,
		"APP_ENV":            true,
		"APP_ADDR":           true,
	}
	env := make([]string, 0, len(os.Environ())+len(extra))
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if drop[k] {
			continue
		}
		env = append(env, kv)
	}
	return append(env, extra...)
}

func freeLocalAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for free port: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

func cordisSQLiteSource(t *testing.T, dbPath string) core.ConfigSource {
	t.Helper()
	return core.NewMapSource(map[string]any{
		"app": map[string]any{
			"addr": "127.0.0.1:0",
			"env":  "testing",
		},
		"redis": map[string]any{
			"enabled": false,
		},
		"clickhouse": map[string]any{
			"enabled": false,
		},
		"database": map[string]any{
			"enabled":     false,
			"sqlite_path": dbPath,
		},
	})
}

func cordisPostgresSource(t *testing.T, host string, port int, user, pass, dbName, sslMode string) core.ConfigSource {
	t.Helper()
	return core.NewMapSource(map[string]any{
		"app": map[string]any{
			"addr": "127.0.0.1:0",
			"env":  "testing",
		},
		"redis": map[string]any{
			"enabled": false,
		},
		"clickhouse": map[string]any{
			"enabled": false,
		},
		"database": map[string]any{
			"enabled":  true,
			"host":     host,
			"port":     port,
			"username": user,
			"password": pass,
			"database": dbName,
			"ssl_mode": sslMode,
		},
	})
}

func sqliteReady(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return false
	}
	gdb, err := gorm.Open(sqlite.Open("file:"+path+"?mode=ro&_pragma=busy_timeout(1000)"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		return false
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return false
	}
	defer func() { _ = sqlDB.Close() }()
	return migratedReady(sqlDB, false)
}

func postgresReady(dsn string) bool {
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		return false
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return false
	}
	defer func() { _ = sqlDB.Close() }()
	return migratedReady(sqlDB, true)
}

func migratedReady(db *sql.DB, postgres bool) bool {
	if gooseMaxVersionSilent(db) != goldGooseVersion {
		return false
	}
	var n int
	var err error
	if postgres {
		err = db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'of_nodes'`).Scan(&n)
	} else {
		err = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'of_nodes'`).Scan(&n)
	}
	return err == nil && n > 0
}

func openInspectDB(t *testing.T, sqlitePath, pgDSN string) *sql.DB {
	t.Helper()
	var gdb *gorm.DB
	var err error
	if pgDSN != "" {
		gdb, err = gorm.Open(postgres.Open(pgDSN), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	} else {
		gdb, err = gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	}
	if err != nil {
		t.Fatalf("open inspect db: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("inspect sql.DB: %v", err)
	}
	return sqlDB
}

func dumpOfSchema(t *testing.T, db *sql.DB, postgres bool) map[string][]string {
	t.Helper()
	tables := ofTables(t, db, postgres)
	out := make(map[string][]string, len(tables))
	for _, table := range tables {
		out[table] = ofColumns(t, db, postgres, table)
	}
	if len(out) == 0 {
		t.Fatal("no of_* tables in golden database")
	}
	return out
}

func ofTables(t *testing.T, db *sql.DB, postgres bool) []string {
	t.Helper()
	var rows *sql.Rows
	var err error
	if postgres {
		rows, err = db.Query(`SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename LIKE 'of_%' ORDER BY tablename`)
	} else {
		rows, err = db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name LIKE 'of_%' ORDER BY name`)
	}
	if err != nil {
		t.Fatalf("list of_* tables: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan of_* table: %v", err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("list of_* tables: %v", err)
	}
	return tables
}

func ofColumns(t *testing.T, db *sql.DB, postgres bool, table string) []string {
	t.Helper()
	var rows *sql.Rows
	var err error
	if postgres {
		rows, err = db.Query(`SELECT column_name FROM information_schema.columns WHERE table_schema = 'public' AND table_name = $1 ORDER BY ordinal_position`, table)
	} else {
		rows, err = db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	}
	if err != nil {
		t.Fatalf("list columns for %s: %v", table, err)
	}
	defer func() { _ = rows.Close() }()
	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan column for %s: %v", table, err)
		}
		cols = append(cols, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("list columns for %s: %v", table, err)
	}
	return cols
}

func gooseMaxVersion(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	v := gooseMaxVersionSilent(db)
	if v < 0 {
		t.Fatal("read goose_db_version max failed")
	}
	return v
}

func gooseMaxVersionSilent(db *sql.DB) int64 {
	var v int64
	if err := db.QueryRow(`SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version`).Scan(&v); err != nil {
		return -1
	}
	return v
}

func schemaPluginRows(t *testing.T, spec upgradeDB, pluginID string) int {
	t.Helper()
	db := openInspectDB(t, spec.sqlitePath, spec.pgDSN)
	defer func() { _ = db.Close() }()
	q := `SELECT COUNT(*) FROM w_schema_versions WHERE plugin_id = ?`
	if spec.pgDSN != "" {
		q = `SELECT COUNT(*) FROM w_schema_versions WHERE plugin_id = $1`
	}
	var n int
	if err := db.QueryRow(q, pluginID).Scan(&n); err != nil {
		t.Fatalf("count w_schema_versions %s: %v", pluginID, err)
	}
	return n
}

func pluginHasVersion(t *testing.T, db *sql.DB, postgres bool, pluginID string, version int64) bool {
	t.Helper()
	q := `SELECT COUNT(*) FROM w_schema_versions WHERE plugin_id = ? AND version_id = ?`
	if postgres {
		q = `SELECT COUNT(*) FROM w_schema_versions WHERE plugin_id = $1 AND version_id = $2`
	}
	var n int
	if err := db.QueryRow(q, pluginID, version).Scan(&n); err != nil {
		t.Fatalf("lookup w_schema_versions (%s, %d): %v", pluginID, version, err)
	}
	return n > 0
}

func parsePostgresDSN(t *testing.T, dsn string) (host string, port int, user, pass, dbName, sslMode string) {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("TEST_PG_DSN: %v", err)
	}
	host = u.Hostname()
	if host == "" {
		host = "127.0.0.1"
	}
	port = 5432
	if p := u.Port(); p != "" {
		port, err = strconv.Atoi(p)
		if err != nil {
			t.Fatalf("TEST_PG_DSN port: %v", err)
		}
	}
	if u.User != nil {
		user = u.User.Username()
		pass, _ = u.User.Password()
	}
	dbName = strings.Trim(u.Path, "/")
	if dbName == "" {
		dbName = "postgres"
	}
	sslMode = u.Query().Get("sslmode")
	if sslMode == "" {
		sslMode = "disable"
	}
	return
}

func postgresDSN(host string, port int, user, pass, dbName, sslMode string) string {
	u := &url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
		Path:   dbName,
	}
	if user != "" {
		u.User = url.UserPassword(user, pass)
	}
	q := url.Values{}
	q.Set("sslmode", sslMode)
	u.RawQuery = q.Encode()
	return u.String()
}

var pgIdent = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func safePGIdent(name string) bool {
	return pgIdent.MatchString(name)
}
