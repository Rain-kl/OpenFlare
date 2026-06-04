package goose

import (
	"fmt"

	presslygoose "github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

const versionPagesDeploymentDropUnusedFields int64 = 202606040003

// migration202606040003 drops RootDir and EntryFile fields from Pages deployments.
func migration202606040003(backend string, ctx Context) *presslygoose.Migration {
	return newGORMMigration(
		versionPagesDeploymentDropUnusedFields,
		"202606040003_drop_pages_deployment_unused_fields.go",
		backend,
		ctx,
		migratePagesDeploymentDropUnusedFields,
	)
}

func migratePagesDeploymentDropUnusedFields(ctx Context, db *gorm.DB, backend string) error {
	// Drop columns root_dir and entry_file from pages_deployments
	if db.Migrator().HasColumn("pages_deployments", "root_dir") {
		if err := db.Migrator().DropColumn("pages_deployments", "root_dir"); err != nil {
			return fmt.Errorf("failed to drop pages_deployments.root_dir: %w", err)
		}
	}
	if db.Migrator().HasColumn("pages_deployments", "entry_file") {
		if err := db.Migrator().DropColumn("pages_deployments", "entry_file"); err != nil {
			return fmt.Errorf("failed to drop pages_deployments.entry_file: %w", err)
		}
	}
	return nil
}
