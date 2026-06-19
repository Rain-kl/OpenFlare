// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package migrator

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(up202606050001, down202606050001)
}

func up202606050001(ctx context.Context, tx *sql.Tx) error {
	dialect := gooseDialect()
	if dialect == dialectSqlite {
		if err := fixSqliteGooseDbVersion(ctx, tx); err != nil {
			return err
		}
	}
	tableExistsQuery := tableExistsSQL(dialect)

	checkTable := func(name string) (bool, error) {
		var count int
		err := tx.QueryRowContext(ctx, tableExistsQuery, name).Scan(&count)
		return count > 0, err
	}

	// Check if this is a legacy database (by checking if the "options" table exists).
	// On a clean install, "options" won't exist, so this will be a quick no-op.
	hasOptions, err := checkTable("options")
	if err != nil {
		return fmt.Errorf("check table options failed: %w", err)
	}

	if !hasOptions {
		// Clean install or already migrated, no action needed.
		return nil
	}

	// Helper to drop table if it exists
	dropTable := func(tableName string) error {
		dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName)
		if dialect == dialectPostgres {
			dropSQL += cascadeSuffix
		}
		if _, err := tx.ExecContext(ctx, dropSQL); err != nil {
			return fmt.Errorf("drop table %s failed: %w", tableName, err)
		}
		return nil
	}

	// 1. Drop legacy observability and log tables to free space and avoid conflict
	obsPrefixes := []string{
		"node_system_profiles",
		"node_health_events",
		"node_access_logs_",
		"node_observation_openresties_",
		"node_observation_frpcs_",
		"node_observation_frps_",
		"node_metric_snapshots_",
		"node_request_reports_",
	}

	for _, prefix := range obsPrefixes {
		if err := dropTablesWithPrefix(ctx, tx, tablesWithPrefixSQL(dialect), prefix, dropTable); err != nil {
			return err
		}
	}

	// 2. Rename custom and platform legacy tables to legacy_ prefix
	tablesToRename := []string{
		"users",
		"auth_sources",
		"external_accounts",
		"options",
		"origins",
		"apply_logs",
		"proxy_routes",
		"nodes",
		"waf_rule_groups",
		"waf_rule_group_bindings",
		"waf_ip_groups",
		"tls_certificates",
		"managed_domains",
		"dns_accounts",
		"acme_accounts",
		"config_versions",
		"pages_projects",
		"pages_deployments",
		"pages_deployment_files",
	}

	for _, oldName := range tablesToRename {
		if err := renameLegacyTable(ctx, tx, oldName, checkTable, dropTable); err != nil {
			return err
		}
	}

	return nil
}

func dropTablesWithPrefix(ctx context.Context, tx *sql.Tx, query, prefix string, dropTable func(string) error) error {
	rows, err := tx.QueryContext(ctx, query, prefix+"%")
	if err != nil {
		return fmt.Errorf("query legacy tables for prefix %s failed: %w", prefix, err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		tables = append(tables, name)
	}

	for _, table := range tables {
		if err := dropTable(table); err != nil {
			return err
		}
	}
	return nil
}

func renameLegacyTable(ctx context.Context, tx *sql.Tx, oldName string, checkTable func(string) (bool, error), dropTable func(string) error) error {
	exists, err := checkTable(oldName)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	newName := "legacy_" + oldName
	newExists, err := checkTable(newName)
	if err != nil {
		return err
	}
	if newExists {
		if err := dropTable(newName); err != nil {
			return err
		}
	}

	renameSQL := fmt.Sprintf("ALTER TABLE %s RENAME TO %s", oldName, newName)
	if _, err := tx.ExecContext(ctx, renameSQL); err != nil {
		return fmt.Errorf("rename table %s to %s failed: %w", oldName, newName, err)
	}
	return nil
}

func down202606050001(_ context.Context, _ *sql.Tx) error {
	// Down migration is a no-op as the legacy DB is rolled forward during migration.
	return nil
}

func fixSqliteGooseDbVersion(ctx context.Context, tx *sql.Tx) error {
	var createSQL string
	err := tx.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type='table' AND name='goose_db_version'").Scan(&createSQL)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	if strings.Contains(strings.ToUpper(createSQL), "AUTOINCREMENT") {
		return nil // Already upgraded
	}

	if _, err := tx.ExecContext(ctx, "ALTER TABLE goose_db_version RENAME TO old_goose_db_version"); err != nil {
		return fmt.Errorf("rename goose_db_version failed: %w", err)
	}

	newTableSQL := `CREATE TABLE goose_db_version (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL,
		is_applied INTEGER NOT NULL,
		tstamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := tx.ExecContext(ctx, newTableSQL); err != nil {
		return fmt.Errorf("create new goose_db_version failed: %w", err)
	}

	copySQL := `INSERT INTO goose_db_version (id, version_id, is_applied, tstamp)
		SELECT id, version_id, is_applied, tstamp FROM old_goose_db_version;`
	if _, err := tx.ExecContext(ctx, copySQL); err != nil {
		return fmt.Errorf("copy goose_db_version data failed: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "DROP TABLE old_goose_db_version"); err != nil {
		return fmt.Errorf("drop old_goose_db_version failed: %w", err)
	}

	return nil
}
