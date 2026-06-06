package goose

import (
	"encoding/json"
	"fmt"

	presslygoose "github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

const versionNodeCapabilitiesJSON int64 = 202606020001

// migration202606020001 adds a future-proof JSON field for node capability
// summaries after the legacy v17 migration bridge.
func migration202606020001(backend string, ctx Context) *presslygoose.Migration {
	return newGORMMigration(
		versionNodeCapabilitiesJSON,
		"202606020001_add_node_capabilities_json.go",
		backend,
		ctx,
		migrateNodeCapabilitiesJSON,
	)
}

func migrateNodeCapabilitiesJSON(ctx Context, db *gorm.DB, backend string) error {
	if err := ctx.ApplyCurrentSchema(db, backend); err != nil {
		return err
	}
	emptyJSON, err := json.Marshal([]string{})
	if err != nil {
		return fmt.Errorf("marshal default node capabilities: %w", err)
	}
	if err := db.Exec(
		`UPDATE nodes SET capabilities_json = ? WHERE capabilities_json IS NULL OR TRIM(capabilities_json) = ''`,
		string(emptyJSON),
	).Error; err != nil {
		return fmt.Errorf("backfill nodes.capabilities_json: %w", err)
	}
	return validateNodeCapabilitiesJSON(db)
}

func validateNodeCapabilitiesJSON(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if !db.Migrator().HasColumn("nodes", "capabilities_json") {
		return fmt.Errorf("column nodes.capabilities_json is missing")
	}
	return nil
}
