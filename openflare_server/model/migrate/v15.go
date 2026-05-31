package migrate

import (
	"fmt"

	"gorm.io/gorm"
)

type nodeV15 struct {
	IPManualOverride bool `gorm:"column:ip_manual_override;not null;default:false"`
}

func init() {
	Register(V15())
}

func V15() Migration {
	return Migration{
		FromVersion: 14,
		ToVersion:   15,
		Migrate:     migrateV15,
		Validate:    validateV15,
	}
}

func (nodeV15) TableName() string {
	return "nodes"
}

func migrateV15(ctx Context, db *gorm.DB, backend string) error {
	return ctx.ApplyCurrentSchema(db, backend)
}

func validateV15(ctx Context, db *gorm.DB, backend string) error {
	if err := ctx.ValidateDatabaseSchemaVersion(db, backend, 14); err != nil {
		return err
	}
	if db == nil || !db.Migrator().HasColumn(&nodeV15{}, "ip_manual_override") {
		return fmt.Errorf("column nodes.ip_manual_override is missing")
	}
	return nil
}
