package migrate

import (
	"fmt"

	"gorm.io/gorm"
)

type nodeV17 struct{}

func (nodeV17) TableName() string {
	return "nodes"
}

func init() {
	Register(V17())
}

func V17() Migration {
	return Migration{
		FromVersion: 16,
		ToVersion:   17,
		Migrate:     migrateV17,
		Validate:    validateV17,
	}
}

func migrateV17(ctx Context, db *gorm.DB, backend string) error {
	if err := ctx.ApplyCurrentSchema(db, backend); err != nil {
		return err
	}
	return nil
}

func validateV17(ctx Context, db *gorm.DB, backend string) error {
	if err := ctx.ValidateDatabaseSchemaVersion(db, backend, 16); err != nil {
		return err
	}
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}

	migrator := db.Migrator()
	if !migrator.HasColumn(&nodeV17{}, "relay_web_server_enabled") {
		return fmt.Errorf("column nodes.relay_web_server_enabled is missing")
	}

	// Validate columns on a sharded partition table
	for _, shard := range []string{"node_observation_frps_00"} {
		for _, column := range []string{"frps_client_count", "frps_proxies"} {
			if !migrator.HasColumn(shard, column) {
				return fmt.Errorf("column %s.%s is missing", shard, column)
			}
		}
	}

	return nil
}
