// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package migrator

import (
	"context"
	"io/fs"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestLegacyFRPUpgradeMigration(t *testing.T) {
	upgradeDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	sqlDB, err := upgradeDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	gooseFS, err := fs.Sub(migrationFS, "goose/sqlite")
	require.NoError(t, err)
	provider, err := goose.NewProvider(goose.DialectSQLite3, sqlDB, gooseFS)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = provider.UpTo(ctx, 202611230002)
	require.NoError(t, err)

	now := time.Now().UTC()
	// Seed legacy edge node (must survive), legacy FRP relay & client nodes (must be deleted)
	require.NoError(t, upgradeDB.Exec(`
		INSERT INTO of_nodes (node_id, name, node_type, engine, status, relay_bind_port, relay_vhost_http_port, created_at, updated_at)
		VALUES
			('edge-1', 'Edge Node 1', 'edge_node', 'pingora', 'online', 0, 0, ?, ?),
			('relay-1', 'Legacy Relay', 'tunnel_relay', 'openresty', 'online', 7000, 8080, ?, ?),
			('client-1', 'Legacy Tunnel', 'tunnel_client', 'openresty', 'online', 0, 0, ?, ?)
	`, now, now, now, now, now, now).Error)

	var legacyClientID int64
	require.NoError(t, upgradeDB.Raw(`SELECT id FROM of_nodes WHERE node_id = 'client-1'`).Scan(&legacyClientID).Error)
	require.NotZero(t, legacyClientID)

	// Seed legacy child records for FRP nodes and edge node
	require.NoError(t, upgradeDB.Exec(`
		INSERT INTO of_node_system_profiles (node_id, hostname, reported_at, created_at, updated_at)
		VALUES ('edge-1', 'edge-host', ?, ?, ?), ('relay-1', 'relay-host', ?, ?, ?), ('client-1', 'client-host', ?, ?, ?)
	`, now, now, now, now, now, now, now, now, now).Error)

	require.NoError(t, upgradeDB.Exec(`
		INSERT INTO of_node_health_events (node_id, event_type, severity, status, message, first_triggered_at, last_triggered_at, reported_at, created_at, updated_at)
		VALUES ('edge-1', 'ok', 'info', 'resolved', 'ok', ?, ?, ?, ?, ?), ('relay-1', 'frps_unhealthy', 'warning', 'active', 'down', ?, ?, ?, ?, ?)
	`, now, now, now, now, now, now, now, now, now, now).Error)

	require.NoError(t, upgradeDB.Exec(`
		INSERT INTO of_apply_logs (node_id, version, result, message, created_at)
		VALUES ('edge-1', 'v1', 'success', 'ok', ?), ('client-1', 'v1', 'success', 'ok', ?)
	`, now, now).Error)

	require.NoError(t, upgradeDB.Exec(`
		INSERT INTO of_node_obs_frps (id, node_id, captured_at, frps_connections, frps_proxy_count, frps_client_count, frps_proxies, created_at)
		VALUES (1, 'relay-1', ?, 1, 1, 1, '[]', ?)
	`, now, now).Error)

	require.NoError(t, upgradeDB.Exec(`
		INSERT INTO of_node_obs_frpc (id, node_id, captured_at, tunnel_status, connected_relays_count, created_at)
		VALUES (1, 'client-1', ?, 'online', 1, ?)
	`, now, now).Error)

	// Seed legacy tunnel proxy route + normal direct route
	require.NoError(t, upgradeDB.Exec(`
		INSERT INTO of_proxy_routes (site_name, origin_url, enabled, upstream_type, tunnel_node_id, tunnel_target_addr, tunnel_target_protocol, created_at, updated_at)
		VALUES
			('site-tunnel', '', 1, 'tunnel', ?, '127.0.0.1:8080', 'http', ?, ?),
			('site-direct', 'https://example.com', 1, 'direct', NULL, '', '', ?, ?)
	`, legacyClientID, now, now, now, now).Error)

	// Run remaining migrations to latest
	_, err = provider.Up(ctx)
	require.NoError(t, err)

	// Legacy FRP nodes deleted; edge node preserved
	var nodeIDs []string
	require.NoError(t, upgradeDB.Raw(`SELECT node_id FROM of_nodes ORDER BY node_id`).Scan(&nodeIDs).Error)
	assert.Equal(t, []string{"edge-1"}, nodeIDs)

	// Child tables cleaned for FRP nodes only
	var remainingProfiles int64
	require.NoError(t, upgradeDB.Raw(`SELECT COUNT(*) FROM of_node_system_profiles WHERE node_id IN ('relay-1', 'client-1')`).Scan(&remainingProfiles).Error)
	assert.Zero(t, remainingProfiles)

	var edgeProfiles int64
	require.NoError(t, upgradeDB.Raw(`SELECT COUNT(*) FROM of_node_system_profiles WHERE node_id = 'edge-1'`).Scan(&edgeProfiles).Error)
	assert.Equal(t, int64(1), edgeProfiles)

	// Legacy tunnel proxy route safely disabled and unlinked; direct route intact
	var tunnelEnabled bool
	var tunnelTargetAddr string
	var tunnelID *int64
	row := upgradeDB.Raw(`SELECT enabled, tunnel_target_addr, tunnel_id FROM of_proxy_routes WHERE site_name = 'site-tunnel'`).Row()
	require.NoError(t, row.Scan(&tunnelEnabled, &tunnelTargetAddr, &tunnelID))
	assert.False(t, tunnelEnabled)
	assert.Empty(t, tunnelTargetAddr)
	assert.Nil(t, tunnelID)

	var directEnabled bool
	require.NoError(t, upgradeDB.Raw(`SELECT enabled FROM of_proxy_routes WHERE site_name = 'site-direct'`).Scan(&directEnabled).Error)
	assert.True(t, directEnabled)

	// Legacy FRP tables, columns, triggers, and configs removed
	assert.False(t, upgradeDB.Migrator().HasTable("of_node_obs_frps"))
	assert.False(t, upgradeDB.Migrator().HasTable("of_node_obs_frpc"))
	assert.False(t, upgradeDB.Migrator().HasColumn("of_nodes", "relay_bind_port"))
	assert.False(t, upgradeDB.Migrator().HasColumn("of_nodes", "relay_vhost_http_port"))
	assert.False(t, upgradeDB.Migrator().HasColumn("of_proxy_routes", "tunnel_node_id"))
	assert.True(t, upgradeDB.Migrator().HasColumn("of_proxy_routes", "tunnel_id"))
	assert.True(t, upgradeDB.Migrator().HasColumn("of_proxy_routes", "tunnel_options_json"))

	var legacyConfigCount int64
	require.NoError(t, upgradeDB.Raw(`SELECT COUNT(*) FROM w_system_configs WHERE key IN ('relay_frps_web_ui_enabled', 'relay_frps_web_ui_port')`).Scan(&legacyConfigCount).Error)
	assert.Zero(t, legacyConfigCount)

	// New tables and default pool created
	for _, table := range []string{
		"of_tunnel_relay_pools",
		"of_tunnel_relays",
		"of_tunnels",
		"of_tunnel_connectors",
		"of_tunnel_attachments",
		"of_tunnel_services",
	} {
		assert.True(t, upgradeDB.Migrator().HasTable(table), "expected table %s to exist", table)
	}

	var defaultPoolCount int64
	require.NoError(t, upgradeDB.Raw(`SELECT COUNT(*) FROM of_tunnel_relay_pools WHERE pool_key = 'default' AND is_default = 1`).Scan(&defaultPoolCount).Error)
	assert.Equal(t, int64(1), defaultPoolCount)
}
