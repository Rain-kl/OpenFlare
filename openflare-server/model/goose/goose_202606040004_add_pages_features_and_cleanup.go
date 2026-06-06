package goose

import (
	"fmt"

	presslygoose "github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

const versionPagesFeaturesAndCleanup int64 = 202606040004

// migration202606040004 merges migrations 202606030004, 202606040001, 202606040002, and 202606040003.
// It adds Pages API proxying fields and RootDir/EntryFile to Pages projects,
// backfills default entry_file to 'index.html', and ensures unused fields (root_dir, entry_file)
// are dropped from Pages deployments.
func migration202606040004(backend string, ctx Context) *presslygoose.Migration {
	return newGORMMigration(
		versionPagesFeaturesAndCleanup,
		"202606040004_add_pages_features_and_cleanup.go",
		backend,
		ctx,
		migratePagesFeaturesAndCleanup,
	)
}

func migratePagesFeaturesAndCleanup(ctx Context, db *gorm.DB, backend string) error {
	if err := ctx.ApplyCurrentSchema(db, backend); err != nil {
		return err
	}

	// 1. Verify Pages projects columns
	cols := []string{
		"api_proxy_enabled", "api_proxy_path", "api_proxy_pass", "api_proxy_rewrite",
		"root_dir", "entry_file",
	}
	for _, col := range cols {
		if !db.Migrator().HasColumn("pages_projects", col) {
			return fmt.Errorf("column pages_projects.%s is missing", col)
		}
	}

	// 2. Backfill pages_projects.entry_file to 'index.html' if empty
	type PagesProject struct {
		ID        uint   `gorm:"primaryKey"`
		EntryFile string `gorm:"size:512;not null;default:'index.html'"`
	}
	if err := db.Model(&PagesProject{}).Where("entry_file = '' OR entry_file IS NULL").Update("entry_file", "index.html").Error; err != nil {
		return fmt.Errorf("failed to backfill pages_projects.entry_file: %w", err)
	}

	// 3. Drop unused fields root_dir and entry_file from pages_deployments if they exist
	if db.Migrator().HasColumn("pages_deployments", "root_dir") {
		if err := db.Exec("ALTER TABLE pages_deployments DROP COLUMN root_dir").Error; err != nil {
			return fmt.Errorf("failed to drop pages_deployments.root_dir: %w", err)
		}
	}
	if db.Migrator().HasColumn("pages_deployments", "entry_file") {
		if err := db.Exec("ALTER TABLE pages_deployments DROP COLUMN entry_file").Error; err != nil {
			return fmt.Errorf("failed to drop pages_deployments.entry_file: %w", err)
		}
	}

	return nil
}
