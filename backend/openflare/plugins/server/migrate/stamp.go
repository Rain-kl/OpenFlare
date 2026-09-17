// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package migrate records pre-Cordis schema versions and applies of_* SQL.
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"unicode"

	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/openflare/plugins/server/domain/site/zone"
)

const (
	legacyPluginID           = "openflare/legacy"
	serverPluginID           = "server"
	serverInitialVersion     = int64(1)
	zoneImportSQLVersion     = int64(202607120002)
	zoneDropLegacySQLVersion = int64(202607130001)
	gooseVersionTable        = "goose_db_version"
)

// Legacy copies goose_db_version into w_schema_versions so the 76-file mixed
// chain is not re-run. Fresh databases have no goose table and are left alone.
func Legacy(ctx *core.Context) error {
	dbSvc, err := core.Inject[contracts.DBService](ctx)
	if err != nil {
		return fmt.Errorf("stamp: inject DBService: %w", err)
	}
	gormDB := dbSvc.GORM()
	if gormDB == nil {
		return fmt.Errorf("stamp: DBService.GORM() returned nil")
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		return fmt.Errorf("stamp: get sql.DB: %w", err)
	}

	goCtx := context.Background()
	if ctx != nil && ctx.GoContext() != nil {
		goCtx = ctx.GoContext()
	}
	postgres := gormDB.Dialector != nil && gormDB.Name() == "postgres"

	exists, err := gooseTableExists(goCtx, sqlDB, postgres)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	col, err := gooseVersionColumn(goCtx, sqlDB, postgres)
	if err != nil {
		return err
	}
	maxVer, err := gooseMaxVersion(goCtx, sqlDB, col)
	if err != nil {
		return err
	}
	if err := insertStamps(goCtx, sqlDB, postgres, maxVer); err != nil {
		return err
	}
	return maybeImportZones(goCtx, sqlDB, postgres, maxVer)
}

func gooseTableExists(ctx context.Context, db *sql.DB, postgres bool) (bool, error) {
	var n int
	var err error
	if postgres {
		err = db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		`, gooseVersionTable).Scan(&n)
	} else {
		err = db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
			gooseVersionTable,
		).Scan(&n)
	}
	if err != nil {
		return false, fmt.Errorf("stamp: detect %s: %w", gooseVersionTable, err)
	}
	return n > 0, nil
}

func gooseVersionColumn(ctx context.Context, db *sql.DB, postgres bool) (string, error) {
	var rows *sql.Rows
	var err error
	if postgres {
		rows, err = db.QueryContext(ctx, `
			SELECT column_name FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1
		`, gooseVersionTable)
	} else {
		rows, err = db.QueryContext(ctx, `SELECT name FROM pragma_table_info(?)`, gooseVersionTable)
	}
	if err != nil {
		return "", fmt.Errorf("stamp: list %s columns: %w", gooseVersionTable, err)
	}
	defer func() { _ = rows.Close() }()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return "", fmt.Errorf("stamp: scan %s columns: %w", gooseVersionTable, err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("stamp: list %s columns: %w", gooseVersionTable, err)
	}

	hasVersionID, hasVersion := false, false
	for _, name := range names {
		switch name {
		case "version_id":
			hasVersionID = true
		case "version":
			hasVersion = true
		}
	}
	switch {
	case hasVersionID:
		return "version_id", nil
	case hasVersion:
		return "version", nil
	default:
		return "", fmt.Errorf("stamp: %s has no version_id or version column", gooseVersionTable)
	}
}

func gooseMaxVersion(ctx context.Context, db *sql.DB, column string) (int64, error) {
	if !safeIdent(column) {
		return 0, fmt.Errorf("stamp: unsafe version column %q", column)
	}
	var maxVer int64
	q := fmt.Sprintf("SELECT COALESCE(MAX(%s), 0) FROM %s", column, gooseVersionTable)
	if err := db.QueryRowContext(ctx, q).Scan(&maxVer); err != nil {
		return 0, fmt.Errorf("stamp: max %s: %w", gooseVersionTable, err)
	}
	return maxVer, nil
}

func insertStamps(ctx context.Context, db *sql.DB, postgres bool, maxVer int64) error {
	q := `INSERT INTO w_schema_versions (plugin_id, version_id) VALUES (?, ?) ON CONFLICT (plugin_id, version_id) DO NOTHING`
	if postgres {
		q = `INSERT INTO w_schema_versions (plugin_id, version_id) VALUES ($1, $2) ON CONFLICT (plugin_id, version_id) DO NOTHING`
	}
	stamps := []struct {
		plugin string
		ver    int64
	}{
		{legacyPluginID, 0},
		{legacyPluginID, maxVer},
		{serverPluginID, serverInitialVersion},
	}
	for _, s := range stamps {
		if _, err := db.ExecContext(ctx, q, s.plugin, s.ver); err != nil {
			return fmt.Errorf("stamp: insert (%s, %d): %w", s.plugin, s.ver, err)
		}
	}
	return nil
}

func maybeImportZones(ctx context.Context, db *sql.DB, postgres bool, maxVer int64) error {
	if maxVer < zoneImportSQLVersion || maxVer >= zoneDropLegacySQLVersion {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("stamp: begin zone import: %w", err)
	}
	report, err := zone.ImportLegacyTx(ctx, tx, postgres)
	if err != nil {
		_ = tx.Rollback()
		return report.LogAndReturn(err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("stamp: commit zone import: %w", err)
	}
	return nil
}

func safeIdent(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
