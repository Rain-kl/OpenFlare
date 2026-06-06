// v16 is the first database migration after the V15 formal release baseline.
// It folds the previously drafted v16-v21 schema work into a single official
// upgrade: tunnel-relay fields, WAF IP groups, current node identity/version
// columns, and split node observation tables. The migration also backfills
// legacy node columns and removes obsolete pre-release tunnel metadata when
// present, so V15 deployments can upgrade directly to the new formal schema.
package migrate

import (
	"fmt"
	"log/slog"

	"gorm.io/gorm"
)

func (nodeV16) TableName() string {
	return "nodes"
}

func (tunnelV16) TableName() string {
	return "tunnels"
}

func (proxyRouteV16) TableName() string {
	return "proxy_routes"
}

type nodeV16 struct{}

type tunnelV16 struct{}

type proxyRouteV16 struct{}

type wafIPGroupV16 struct{}

type wafRuleGroupV16 struct{}

func (wafIPGroupV16) TableName() string {
	return "waf_ip_groups"
}

func (wafRuleGroupV16) TableName() string {
	return "waf_rule_groups"
}

func init() {
	Register(V16())
}

func V16() Migration {
	return Migration{
		FromVersion: 15,
		ToVersion:   16,
		Migrate:     migrateV16,
		Validate:    validateV16,
	}
}

func migrateV16(ctx Context, db *gorm.DB, backend string) error {
	if err := ctx.ApplyCurrentSchema(db, backend); err != nil {
		return err
	}

	migrator := db.Migrator()
	if migrator.HasColumn(&nodeV16{}, "agent_token") {
		if err := db.Exec(`UPDATE nodes SET access_token = agent_token WHERE access_token IS NULL OR access_token = ''`).Error; err != nil {
			return fmt.Errorf("backfill nodes.access_token from agent_token: %w", err)
		}
	}
	if migrator.HasColumn(&nodeV16{}, "agent_version") {
		if err := db.Exec(`UPDATE nodes SET version = agent_version WHERE version = '' OR version IS NULL`).Error; err != nil {
			return fmt.Errorf("backfill nodes.version from agent_version: %w", err)
		}
	}
	if migrator.HasColumn(&nodeV16{}, "nginx_version") {
		if err := db.Exec(`UPDATE nodes SET ext_version = nginx_version WHERE ext_version IS NULL OR ext_version = ''`).Error; err != nil {
			return fmt.Errorf("backfill nodes.ext_version from nginx_version: %w", err)
		}
	}
	if err := ctx.DropLegacyNodeColumns(db, backend); err != nil {
		return err
	}

	if err := db.Exec("UPDATE nodes SET node_type = 'edge_node' WHERE node_type = '' OR node_type IS NULL").Error; err != nil {
		return fmt.Errorf("backfill nodes.node_type: %w", err)
	}
	if err := db.Exec("UPDATE proxy_routes SET upstream_type = 'direct' WHERE upstream_type = '' OR upstream_type IS NULL").Error; err != nil {
		return fmt.Errorf("backfill proxy_routes.upstream_type: %w", err)
	}

	if migrator.HasColumn(&proxyRouteV16{}, "tunnel_id") {
		if err := db.Model(&proxyRouteV16{}).Where("upstream_type = ?", "tunnel").Update("upstream_type", "direct").Error; err != nil {
			return fmt.Errorf("reset pre-release tunnel proxy routes: %w", err)
		}
		// Drop the legacy index idx_proxy_routes_tunnel_id if it exists, to avoid errors on dropping the tunnel_id column (especially on SQLite).
		if migrator.HasIndex(&proxyRouteV16{}, "idx_proxy_routes_tunnel_id") {
			if err := migrator.DropIndex(&proxyRouteV16{}, "idx_proxy_routes_tunnel_id"); err != nil {
				return fmt.Errorf("drop index idx_proxy_routes_tunnel_id failed: %w", err)
			}
		}
		if err := migrator.DropColumn(&proxyRouteV16{}, "tunnel_id"); err != nil {
			return fmt.Errorf("drop pre-release proxy_routes.tunnel_id: %w", err)
		}
	}
	if migrator.HasTable(&tunnelV16{}) {
		if err := migrator.DropTable(&tunnelV16{}); err != nil {
			return fmt.Errorf("drop pre-release tunnels table: %w", err)
		}
		slog.Info("dropped pre-release tunnels table during v16 migration")
	}

	return nil
}

func validateV16(ctx Context, db *gorm.DB, backend string) error {
	if err := ctx.ValidateDatabaseSchemaVersion(db, backend, 15); err != nil {
		return err
	}
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}

	migrator := db.Migrator()
	for _, column := range []string{
		"access_token",
		"version",
		"ext_version",
		"node_type",
		"relay_bind_port",
		"relay_vhost_http_port",
		"relay_auth_token",
		"relay_agent_access_addr",
		"relay_client_access_addr",
		"relay_client_proxy_url",
		"relay_status",
	} {
		if !migrator.HasColumn(&nodeV16{}, column) {
			return fmt.Errorf("column nodes.%s is missing", column)
		}
	}
	for _, column := range []string{
		"upstream_type",
		"tunnel_node_id",
		"tunnel_target_addr",
		"tunnel_target_protocol",
	} {
		if !migrator.HasColumn(&proxyRouteV16{}, column) {
			return fmt.Errorf("column proxy_routes.%s is missing", column)
		}
	}
	if migrator.HasColumn(&proxyRouteV16{}, "tunnel_id") {
		return fmt.Errorf("column proxy_routes.tunnel_id should not exist in v16")
	}
	if migrator.HasTable(&tunnelV16{}) {
		return fmt.Errorf("table tunnels should not exist in v16")
	}
	for _, column := range []string{
		"agent_token",
		"agent_version",
		"nginx_version",
		"relay_version",
		"relay_frp_version",
		"relay_frps_connections",
		"relay_frps_proxy_count",
	} {
		if migrator.HasColumn(&nodeV16{}, column) {
			return fmt.Errorf("column nodes.%s should not exist in v16", column)
		}
	}
	if !migrator.HasTable(&wafIPGroupV16{}) {
		return fmt.Errorf("table waf_ip_groups is missing")
	}
	for _, column := range []string{
		"ip_whitelist_groups",
		"ip_blacklist_groups",
	} {
		if !migrator.HasColumn(&wafRuleGroupV16{}, column) {
			return fmt.Errorf("column waf_rule_groups.%s is missing", column)
		}
	}
	if !migrator.HasColumn(&wafIPGroupV16{}, "ext_ips") {
		return fmt.Errorf("column waf_ip_groups.ext_ips is missing")
	}
	return nil
}
