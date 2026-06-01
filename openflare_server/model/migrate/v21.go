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
	dropLegacyNodeColumn := func(col string) error {
		slog.Info("v21: keeping legacy nodes column to avoid lock-heavy schema rewrite", "backend", backend, "column", col)
		return nil
	}

	// Rename agent_token → access_token (target may already exist if AutoMigrate ran earlier)
	if migrator.HasColumn(&nodeV21{}, "agent_token") {
		if migrator.HasColumn(&nodeV21{}, "access_token") {
			slog.Info("v21: access_token column already exists, backfilling from agent_token")
			if err := db.Exec(`UPDATE nodes SET access_token = agent_token WHERE access_token IS NULL OR access_token = ''`).Error; err != nil {
				return fmt.Errorf("failed to backfill access_token from agent_token: %w", err)
			}
			if err := dropLegacyNodeColumn("agent_token"); err != nil {
				slog.Warn("failed to drop agent_token after backfill", "error", err)
			}
		} else {
			if err := migrator.RenameColumn(&nodeV21{}, "agent_token", "access_token"); err != nil {
				return fmt.Errorf("failed to rename agent_token to access_token: %w", err)
			}
		}
	}

	// Rename agent_version → version (target may already exist if AutoMigrate ran earlier)
	if migrator.HasColumn(&nodeV21{}, "agent_version") {
		if migrator.HasColumn(&nodeV21{}, "version") {
			// version already created by AutoMigrate with empty default; backfill from agent_version
			slog.Info("v21: version column already exists, backfilling from agent_version")
			if err := db.Exec(`UPDATE nodes SET version = agent_version WHERE version = ''`).Error; err != nil {
				return fmt.Errorf("failed to backfill version from agent_version: %w", err)
			}
			if err := dropLegacyNodeColumn("agent_version"); err != nil {
				slog.Warn("failed to drop agent_version after backfill", "error", err)
			}
		} else {
			if err := migrator.RenameColumn(&nodeV21{}, "agent_version", "version"); err != nil {
				return fmt.Errorf("failed to rename agent_version to version: %w", err)
			}
		}
	}

	// Rename nginx_version → ext_version (target may already exist if AutoMigrate ran earlier)
	if migrator.HasColumn(&nodeV21{}, "nginx_version") {
		if migrator.HasColumn(&nodeV21{}, "ext_version") {
			slog.Info("v21: ext_version column already exists, backfilling from nginx_version")
			if err := db.Exec(`UPDATE nodes SET ext_version = nginx_version WHERE ext_version IS NULL OR ext_version = ''`).Error; err != nil {
				return fmt.Errorf("failed to backfill ext_version from nginx_version: %w", err)
			}
			if err := dropLegacyNodeColumn("nginx_version"); err != nil {
				slog.Warn("failed to drop nginx_version after backfill", "error", err)
			}
		} else {
			if err := migrator.RenameColumn(&nodeV21{}, "nginx_version", "ext_version"); err != nil {
				return fmt.Errorf("failed to rename nginx_version to ext_version: %w", err)
			}
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
			if err := dropLegacyNodeColumn(col); err != nil {
				slog.Warn("failed to drop column in v21 migration", "column", col, "error", err)
			}
		}
	}

	if err := ctx.ApplyCurrentSchemaExcept(db, backend, "nodes"); err != nil {
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
