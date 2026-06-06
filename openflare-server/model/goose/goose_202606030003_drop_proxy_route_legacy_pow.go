package goose

import (
	"fmt"

	presslygoose "github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

const versionDropProxyRouteLegacyPoW int64 = 202606030003

// migration202606030003 drops the legacy pow_enabled and pow_config columns
// from proxy_routes table, since PoW is now entirely managed under WAF rule groups.
func migration202606030003(backend string, ctx Context) *presslygoose.Migration {
	return newGORMMigration(
		versionDropProxyRouteLegacyPoW,
		"202606030003_drop_proxy_route_legacy_pow.go",
		backend,
		ctx,
		migrateDropProxyRouteLegacyPoW,
	)
}

func migrateDropProxyRouteLegacyPoW(ctx Context, db *gorm.DB, backend string) error {
	if err := ctx.ApplyCurrentSchema(db, backend); err != nil {
		return err
	}
	// Drop pow_enabled column if exists
	if db.Migrator().HasColumn("proxy_routes", "pow_enabled") {
		if err := db.Exec("ALTER TABLE proxy_routes DROP COLUMN pow_enabled").Error; err != nil {
			return fmt.Errorf("drop proxy_routes.pow_enabled: %w", err)
		}
	}
	// Drop pow_config column if exists
	if db.Migrator().HasColumn("proxy_routes", "pow_config") {
		if err := db.Exec("ALTER TABLE proxy_routes DROP COLUMN pow_config").Error; err != nil {
			return fmt.Errorf("drop proxy_routes.pow_config: %w", err)
		}
	}
	return nil
}
