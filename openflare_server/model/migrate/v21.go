// v21 renames agent_token to access_token, unifies versions, and separates node observabilities.
package migrate

import (
	"fmt"
	"log/slog"

	"gorm.io/gorm"
)

type nodeV21 struct{}

func (nodeV21) TableName() string {
	return "nodes"
}

func init() {
	Register(V21())
}

func V21() Migration {
	return Migration{
		FromVersion: 20,
		ToVersion:   21,
		Migrate:     migrateV21,
		Validate:    validateV21,
	}
}

func migrateV21(ctx Context, db *gorm.DB, backend string) error {
	slog.Info("starting v21 database migration (Node Optimization & Observation Split)")

	migrator := db.Migrator()

	if migrator.HasColumn(&nodeV21{}, "agent_token") {
		if err := migrator.RenameColumn(&nodeV21{}, "agent_token", "access_token"); err != nil {
			return fmt.Errorf("failed to rename agent_token to access_token: %w", err)
		}
	}

	if migrator.HasColumn(&nodeV21{}, "agent_version") {
		if err := migrator.RenameColumn(&nodeV21{}, "agent_version", "version"); err != nil {
			return fmt.Errorf("failed to rename agent_version to version: %w", err)
		}
	}

	if migrator.HasColumn(&nodeV21{}, "nginx_version") {
		if err := migrator.RenameColumn(&nodeV21{}, "nginx_version", "ext_version"); err != nil {
			return fmt.Errorf("failed to rename nginx_version to ext_version: %w", err)
		}
	}

	// Drop old merged columns
	columnsToDrop := []string{
		"relay_version",
		"relay_frp_version",
		"relay_frps_connections",
		"relay_frps_proxy_count",
	}

	for _, col := range columnsToDrop {
		if migrator.HasColumn(&nodeV21{}, col) {
			if err := migrator.DropColumn(&nodeV21{}, col); err != nil {
				slog.Warn("failed to drop column in v21 migration", "column", col, "error", err)
			}
		}
	}

	if err := ctx.ApplyCurrentSchema(db, backend); err != nil {
		return err
	}

	slog.Info("completed v21 database migration")
	return validateV21(ctx, db, backend)
}

func validateV21(ctx Context, db *gorm.DB, backend string) error {
	if err := ctx.ValidateDatabaseSchemaVersion(db, backend, 21); err != nil {
		return err
	}
	return nil
}
