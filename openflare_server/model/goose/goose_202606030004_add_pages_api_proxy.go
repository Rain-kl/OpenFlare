package goose

import (
	"fmt"

	presslygoose "github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

const versionPagesAPIProxy int64 = 202606030004

// migration202606030004 adds API proxying fields (enabled, path, pass, rewrite)
// to Pages projects.
func migration202606030004(backend string, ctx Context) *presslygoose.Migration {
	return newGORMMigration(
		versionPagesAPIProxy,
		"202606030004_add_pages_api_proxy.go",
		backend,
		ctx,
		migratePagesAPIProxy,
	)
}

func migratePagesAPIProxy(ctx Context, db *gorm.DB, backend string) error {
	if err := ctx.ApplyCurrentSchema(db, backend); err != nil {
		return err
	}
	// Verify that the columns exist
	for _, col := range []string{"api_proxy_enabled", "api_proxy_path", "api_proxy_pass", "api_proxy_rewrite"} {
		if !db.Migrator().HasColumn("pages_projects", col) {
			return fmt.Errorf("column pages_projects.%s is missing", col)
		}
	}
	return nil
}
