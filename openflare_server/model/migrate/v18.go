package migrate

import (
	"fmt"

	"gorm.io/gorm"
)

// WAF IP Group schema changes for v18:
// We introduce the `ext_ips` column to keep track of captured IPs with their capture timestamps.
// If more information needs to be saved for captured IPs, it can be added directly inside this JSON structure.
type wafIPGroupV18 struct {
	ExtIPs string `gorm:"column:ext_ips;type:text;not null;default:'[]'"`
}

func init() {
	Register(V18())
}

func V18() Migration {
	return Migration{
		FromVersion: 17,
		ToVersion:   18,
		Migrate:     migrateV18,
		Validate:    validateV18,
	}
}

func (wafIPGroupV18) TableName() string {
	return "waf_ip_groups"
}

func migrateV18(ctx Context, db *gorm.DB, backend string) error {
	return ctx.ApplyCurrentSchema(db, backend)
}

func validateV18(ctx Context, db *gorm.DB, backend string) error {
	if err := ctx.ValidateDatabaseSchemaVersion(db, backend, 17); err != nil {
		return err
	}
	if db == nil || !db.Migrator().HasTable(&wafIPGroupV18{}) {
		return fmt.Errorf("table waf_ip_groups is missing")
	}
	if !db.Migrator().HasColumn(&wafIPGroupV18{}, "ext_ips") {
		return fmt.Errorf("column waf_ip_groups.ext_ips is missing")
	}
	return nil
}
