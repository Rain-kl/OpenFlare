package goose

import (
	"fmt"

	presslygoose "github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

const versionPagesProjectRootDir int64 = 202606040002

// migration202606040002 adds RootDir and EntryFile fields to Pages projects.
func migration202606040002(backend string, ctx Context) *presslygoose.Migration {
	return newGORMMigration(
		versionPagesProjectRootDir,
		"202606040002_add_pages_project_root_dir.go",
		backend,
		ctx,
		migratePagesProjectRootDir,
	)
}

func migratePagesProjectRootDir(ctx Context, db *gorm.DB, backend string) error {
	if err := ctx.ApplyCurrentSchema(db, backend); err != nil {
		return err
	}
	// Verify that the columns exist
	if !db.Migrator().HasColumn("pages_projects", "root_dir") {
		return fmt.Errorf("column pages_projects.root_dir is missing")
	}
	if !db.Migrator().HasColumn("pages_projects", "entry_file") {
		return fmt.Errorf("column pages_projects.entry_file is missing")
	}
	// Backfill existing rows: if entry_file is empty, set to 'index.html'
	type PagesProject struct {
		ID        uint   `gorm:"primaryKey"`
		EntryFile string `gorm:"size:512;not null;default:'index.html'"`
	}
	if err := db.Model(&PagesProject{}).Where("entry_file = '' OR entry_file IS NULL").Update("entry_file", "index.html").Error; err != nil {
		return fmt.Errorf("failed to backfill pages_projects.entry_file: %w", err)
	}
	return nil
}
