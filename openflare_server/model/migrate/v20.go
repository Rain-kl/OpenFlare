// v20 records low-frequency Relay frps counters on nodes so the management UI
// can show whether the relay runtime is alive and reporting tunnel load.
package migrate

import (
	"fmt"

	"gorm.io/gorm"
)

type nodeV20 struct {
	ID                   uint `gorm:"primaryKey"`
	RelayFrpsConnections int  `gorm:"column:relay_frps_connections"`
	RelayFrpsProxyCount  int  `gorm:"column:relay_frps_proxy_count"`
}

func (nodeV20) TableName() string {
	return "nodes"
}

func init() {
	Register(V20())
}

func V20() Migration {
	return Migration{
		FromVersion: 19,
		ToVersion:   20,
		Migrate:     migrateV20,
		Validate:    validateV20,
	}
}

func migrateV20(ctx Context, db *gorm.DB, backend string) error {
	if !db.Migrator().HasColumn(&nodeV20{}, "relay_frps_connections") {
		if err := db.Migrator().AddColumn(&nodeV20{}, "RelayFrpsConnections"); err != nil {
			return err
		}
	}
	if !db.Migrator().HasColumn(&nodeV20{}, "relay_frps_proxy_count") {
		if err := db.Migrator().AddColumn(&nodeV20{}, "RelayFrpsProxyCount"); err != nil {
			return err
		}
	}
	if err := ctx.ApplyCurrentSchema(db, backend); err != nil {
		return err
	}
	return validateV20(ctx, db, backend)
}

func validateV20(ctx Context, db *gorm.DB, backend string) error {
	if err := ctx.ValidateDatabaseSchemaVersion(db, backend, 19); err != nil {
		return err
	}
	if !db.Migrator().HasColumn(&nodeV20{}, "relay_frps_connections") {
		return fmt.Errorf("column nodes.relay_frps_connections is missing")
	}
	if !db.Migrator().HasColumn(&nodeV20{}, "relay_frps_proxy_count") {
		return fmt.Errorf("column nodes.relay_frps_proxy_count is missing")
	}
	return nil
}
