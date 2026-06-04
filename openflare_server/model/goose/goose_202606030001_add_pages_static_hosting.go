package goose

import (
	"fmt"

	presslygoose "github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

const versionPagesStaticHosting int64 = 202606030001

// migration202606030001 adds OpenFlare Pages static hosting tables and the
// proxy_routes.pages_project_id binding used by the global release snapshot.
func migration202606030001(backend string, ctx Context) *presslygoose.Migration {
	return newGORMMigration(
		versionPagesStaticHosting,
		"202606030001_add_pages_static_hosting.go",
		backend,
		ctx,
		migratePagesStaticHosting,
	)
}

func migratePagesStaticHosting(ctx Context, db *gorm.DB, backend string) error {
	if err := ctx.ApplyCurrentSchema(db, backend); err != nil {
		return err
	}
	if err := db.Exec(
		`UPDATE proxy_routes SET upstream_type = 'direct' WHERE upstream_type IS NULL OR TRIM(upstream_type) = ''`,
	).Error; err != nil {
		return fmt.Errorf("backfill proxy_routes.upstream_type: %w", err)
	}
	return validatePagesStaticHosting(db)
}

func validatePagesStaticHosting(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	for _, table := range []string{"pages_projects", "pages_deployments", "pages_deployment_files"} {
		if !db.Migrator().HasTable(table) {
			return fmt.Errorf("table %s is missing", table)
		}
	}
	for _, column := range []string{"upstream_type", "pages_project_id"} {
		if !db.Migrator().HasColumn("proxy_routes", column) {
			return fmt.Errorf("column proxy_routes.%s is missing", column)
		}
	}
	for _, column := range []string{"slug", "active_deployment_id", "spa_fallback_enabled", "spa_fallback_path"} {
		if !db.Migrator().HasColumn("pages_projects", column) {
			return fmt.Errorf("column pages_projects.%s is missing", column)
		}
	}
	for _, column := range []string{"project_id", "checksum", "artifact_path"} {
		if !db.Migrator().HasColumn("pages_deployments", column) {
			return fmt.Errorf("column pages_deployments.%s is missing", column)
		}
	}
	return nil
}
