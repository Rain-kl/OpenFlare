package goose

import (
	"fmt"

	presslygoose "github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

const versionPagesSPAFallbackPath int64 = 202606030002

// migration202606030002 adds a configurable SPA fallback path for Pages
// projects. Existing projects keep the previous /index.html behavior.
func migration202606030002(backend string, ctx Context) *presslygoose.Migration {
	return newGORMMigration(
		versionPagesSPAFallbackPath,
		"202606030002_add_pages_spa_fallback_path.go",
		backend,
		ctx,
		migratePagesSPAFallbackPath,
	)
}

func migratePagesSPAFallbackPath(ctx Context, db *gorm.DB, backend string) error {
	if err := ctx.ApplyCurrentSchema(db, backend); err != nil {
		return err
	}
	if err := db.Exec(
		`UPDATE pages_projects SET spa_fallback_path = '/index.html' WHERE spa_fallback_path IS NULL OR TRIM(spa_fallback_path) = ''`,
	).Error; err != nil {
		return fmt.Errorf("backfill pages_projects.spa_fallback_path: %w", err)
	}
	if !db.Migrator().HasColumn("pages_projects", "spa_fallback_path") {
		return fmt.Errorf("column pages_projects.spa_fallback_path is missing")
	}
	return nil
}
