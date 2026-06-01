package migrate

import (
	"fmt"
	"log/slog"

	"gorm.io/gorm"
)

type proxyRouteV19 struct {
	ID           uint   `gorm:"primaryKey"`
	TunnelID     *uint  `gorm:"column:tunnel_id"`
	TunnelNodeID *uint  `gorm:"column:tunnel_node_id"`
	UpstreamType string `gorm:"column:upstream_type"`
}

type tunnelV19 struct{}

func (tunnelV19) TableName() string {
	return "tunnels"
}

func (proxyRouteV19) TableName() string {
	return "proxy_routes"
}

func init() {
	Register(V19())
}

func V19() Migration {
	return Migration{
		FromVersion: 18,
		ToVersion:   19,
		Migrate:     migrateV19,
		Validate:    validateV19,
	}
}

func migrateV19(ctx Context, db *gorm.DB, backend string) error {
	// Drop tunnels table
	if db.Migrator().HasTable(&tunnelV19{}) {
		if err := db.Migrator().DropTable(&tunnelV19{}); err != nil {
			return fmt.Errorf("failed to drop tunnels table: %w", err)
		}
		slog.Info("dropped tunnels table")
	}

	// Add tunnel_node_id column
	if !db.Migrator().HasColumn(&proxyRouteV19{}, "tunnel_node_id") {
		if err := db.Migrator().AddColumn(&proxyRouteV19{}, "TunnelNodeID"); err != nil {
			return fmt.Errorf("failed to add tunnel_node_id to proxy_routes: %w", err)
		}
		slog.Info("added tunnel_node_id column to proxy_routes")
	}

	// Drop old tunnel_id column
	if db.Migrator().HasColumn(&proxyRouteV19{}, "tunnel_id") {
		// Update routes that previously used tunnel to be disabled or direct to prevent dangling refs
		db.Model(&proxyRouteV19{}).Where("upstream_type = ?", "tunnel").Update("upstream_type", "direct")
		if err := db.Migrator().DropColumn(&proxyRouteV19{}, "tunnel_id"); err != nil {
			return fmt.Errorf("failed to drop tunnel_id column from proxy_routes: %w", err)
		}
		slog.Info("dropped tunnel_id column from proxy_routes")
	}

	return ctx.ApplyCurrentSchema(db, backend)
}

func validateV19(ctx Context, db *gorm.DB, backend string) error {
	if err := ctx.ValidateDatabaseSchemaVersion(db, backend, 18); err != nil {
		return err
	}

	if db.Migrator().HasTable(&tunnelV19{}) {
		return fmt.Errorf("table tunnels should be dropped in v19")
	}

	if !db.Migrator().HasColumn(&proxyRouteV19{}, "tunnel_node_id") {
		return fmt.Errorf("column proxy_routes.tunnel_node_id is missing")
	}

	if db.Migrator().HasColumn(&proxyRouteV19{}, "tunnel_id") {
		return fmt.Errorf("column proxy_routes.tunnel_id should be dropped in v19")
	}

	return nil
}
