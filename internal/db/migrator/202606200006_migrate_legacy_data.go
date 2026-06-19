// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package migrator

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(up202606200006, down202606200006)
}

type migrationTask struct {
	legacyName string
	sqliteSQL  string
	pgSQL      string
}

func up202606200006(ctx context.Context, tx *sql.Tx) error {
	dialect := gooseDialect()
	tableExistsQuery := tableExistsSQL(dialect)

	checkTable := func(name string) (bool, error) {
		var count int
		err := tx.QueryRowContext(ctx, tableExistsQuery, name).Scan(&count)
		return count > 0, err
	}

	// Helper to migrate table if it exists
	migrateTable := func(task migrationTask) error {
		exists, err := checkTable(task.legacyName)
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}
		var sqlToRun string
		if dialect == dialectSqlite {
			sqlToRun = task.sqliteSQL
		} else {
			sqlToRun = task.pgSQL
		}
		if _, err := tx.ExecContext(ctx, sqlToRun); err != nil {
			return fmt.Errorf("execute migration query for %s failed: %w", task.legacyName, err)
		}
		return nil
	}

	tasks := getMigrationTasks()
	for _, task := range tasks {
		if err := migrateTable(task); err != nil {
			return err
		}
	}

	// Drop legacy tables that exist
	legacyTables := []string{
		"legacy_users",
		"legacy_auth_sources",
		"legacy_external_accounts",
		"legacy_options",
		"legacy_origins",
		"legacy_apply_logs",
		"legacy_proxy_routes",
		"legacy_nodes",
		"legacy_waf_rule_groups",
		"legacy_waf_rule_group_bindings",
		"legacy_waf_ip_groups",
		"legacy_tls_certificates",
		"legacy_managed_domains",
		"legacy_dns_accounts",
		"legacy_acme_accounts",
		"legacy_config_versions",
		"legacy_pages_projects",
		"legacy_pages_deployments",
		"legacy_pages_deployment_files",
	}

	for _, table := range legacyTables {
		exists, err := checkTable(table)
		if err != nil {
			return err
		}
		if exists {
			dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS %s", table)
			if dialect == dialectPostgres {
				dropSQL += cascadeSuffix
			}
			if _, err := tx.ExecContext(ctx, dropSQL); err != nil {
				return fmt.Errorf("drop legacy table %s failed: %w", table, err)
			}
		}
	}

	return nil
}

func getMigrationTasks() []migrationTask {
	var tasks []migrationTask
	tasks = append(tasks, getUserAndOptionTasks()...)
	tasks = append(tasks, getOriginAndRouteTasks()...)
	tasks = append(tasks, getWafTasks()...)
	tasks = append(tasks, getTLSTasks()...)
	tasks = append(tasks, getPagesAndConfigTasks()...)
	return tasks
}

func getUserAndOptionTasks() []migrationTask {
	return []migrationTask{
		{
			legacyName: "legacy_users",
			sqliteSQL: `INSERT OR REPLACE INTO w_users (id, username, password, nickname, email, is_active, is_admin, created_at, updated_at)
				SELECT id, username, password, display_name, email,
				       CASE WHEN status = 1 THEN 1 ELSE 0 END,
				       CASE WHEN role = 100 THEN 1 ELSE 0 END,
				       CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
				FROM legacy_users;`,
			pgSQL: `INSERT INTO w_users (id, username, password, nickname, email, is_active, is_admin, created_at, updated_at)
				SELECT id, username, password, display_name, email,
				       CASE WHEN status = 1 THEN TRUE ELSE FALSE END,
				       CASE WHEN role = 100 THEN TRUE ELSE FALSE END,
				       CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
				FROM legacy_users
				ON CONFLICT (id) DO UPDATE SET
					username = EXCLUDED.username,
					password = EXCLUDED.password,
					nickname = EXCLUDED.nickname,
					email = EXCLUDED.email,
					is_active = EXCLUDED.is_active,
					is_admin = EXCLUDED.is_admin,
					updated_at = EXCLUDED.updated_at;`,
		},
		{
			legacyName: "legacy_auth_sources",
			sqliteSQL: `INSERT OR REPLACE INTO w_auth_sources (id, name, type, display_name, is_active, client_id, client_secret, openid_discovery_url, scopes, icon_url, created_at, updated_at)
				SELECT id, name, type, display_name, is_active, client_id, client_secret, openid_discovery_url, scopes, icon_url, created_at, updated_at
				FROM legacy_auth_sources;`,
			pgSQL: `INSERT INTO w_auth_sources (id, name, type, display_name, is_active, client_id, client_secret, openid_discovery_url, scopes, icon_url, created_at, updated_at)
				SELECT id, name, type, display_name, is_active::boolean, client_id, client_secret, openid_discovery_url, scopes, icon_url, created_at, updated_at
				FROM legacy_auth_sources
				ON CONFLICT (id) DO UPDATE SET
					name = EXCLUDED.name,
					type = EXCLUDED.type,
					display_name = EXCLUDED.display_name,
					is_active = EXCLUDED.is_active,
					client_id = EXCLUDED.client_id,
					client_secret = EXCLUDED.client_secret,
					openid_discovery_url = EXCLUDED.openid_discovery_url,
					scopes = EXCLUDED.scopes,
					icon_url = EXCLUDED.icon_url,
					updated_at = EXCLUDED.updated_at;`,
		},
		{
			legacyName: "legacy_external_accounts",
			sqliteSQL: `INSERT OR REPLACE INTO w_external_accounts (id, auth_source_id, user_id, external_id, external_username, email, created_at, updated_at)
				SELECT id, auth_source_id, user_id, external_id, external_username, email, created_at, updated_at
				FROM legacy_external_accounts;`,
			pgSQL: `INSERT INTO w_external_accounts (id, auth_source_id, user_id, external_id, external_username, email, created_at, updated_at)
				SELECT id, auth_source_id, user_id, external_id, external_username, email, created_at, updated_at
				FROM legacy_external_accounts
				ON CONFLICT (id) DO UPDATE SET
					auth_source_id = EXCLUDED.auth_source_id,
					user_id = EXCLUDED.user_id,
					external_id = EXCLUDED.external_id,
					external_username = EXCLUDED.external_username,
					email = EXCLUDED.email,
					updated_at = EXCLUDED.updated_at;`,
		},
		{
			legacyName: "legacy_options",
			sqliteSQL: `INSERT OR REPLACE INTO of_options (key, value)
				SELECT key, COALESCE(value, '') FROM legacy_options;`,
			pgSQL: `INSERT INTO of_options (key, value)
				SELECT key, COALESCE(value, '') FROM legacy_options
				ON CONFLICT (key) DO UPDATE SET
					value = EXCLUDED.value;`,
		},
	}
}

func getOriginAndRouteTasks() []migrationTask {
	return []migrationTask{
		{
			legacyName: "legacy_origins",
			sqliteSQL: `INSERT OR REPLACE INTO of_origins (id, name, address, remark, created_at, updated_at)
				SELECT id, name, address, COALESCE(remark, ''), created_at, updated_at
				FROM legacy_origins;`,
			pgSQL: `INSERT INTO of_origins (id, name, address, remark, created_at, updated_at)
				SELECT id, name, address, COALESCE(remark, ''), created_at, updated_at
				FROM legacy_origins
				ON CONFLICT (id) DO UPDATE SET
					name = EXCLUDED.name,
					address = EXCLUDED.address,
					remark = EXCLUDED.remark,
					updated_at = EXCLUDED.updated_at;`,
		},
		{
			legacyName: "legacy_apply_logs",
			sqliteSQL: `INSERT OR REPLACE INTO of_apply_logs (id, node_id, version, result, message, checksum, main_config_checksum, route_config_checksum, support_file_count, created_at)
				SELECT id, node_id, version, result, message, COALESCE(checksum, ''), COALESCE(main_config_checksum, ''), COALESCE(route_config_checksum, ''), COALESCE(support_file_count, 0), created_at
				FROM legacy_apply_logs;`,
			pgSQL: `INSERT INTO of_apply_logs (id, node_id, version, result, message, checksum, main_config_checksum, route_config_checksum, support_file_count, created_at)
				SELECT id, node_id, version, result, message, COALESCE(checksum, ''), COALESCE(main_config_checksum, ''), COALESCE(route_config_checksum, ''), COALESCE(support_file_count, 0), created_at
				FROM legacy_apply_logs
				ON CONFLICT (id) DO UPDATE SET
					node_id = EXCLUDED.node_id,
					version = EXCLUDED.version,
					result = EXCLUDED.result,
					message = EXCLUDED.message,
					checksum = EXCLUDED.checksum,
					main_config_checksum = EXCLUDED.main_config_checksum,
					route_config_checksum = EXCLUDED.route_config_checksum,
					support_file_count = EXCLUDED.support_file_count;`,
		},
		{
			legacyName: "legacy_proxy_routes",
			sqliteSQL: `INSERT OR REPLACE INTO of_proxy_routes (id, site_name, domain, domains, origin_id, origin_url, origin_host, upstreams, enabled, enable_https, cert_id, cert_ids, domain_cert_ids, redirect_http, limit_conn_per_server, limit_conn_per_ip, limit_rate, cache_enabled, cache_policy, cache_rules, custom_headers, basic_auth_enabled, basic_auth_username, basic_auth_password, remark, upstream_type, tunnel_node_id, tunnel_target_addr, tunnel_target_protocol, pages_project_id, created_at, updated_at)
				SELECT id, COALESCE(site_name, ''), domain, COALESCE(domains, '[]'), origin_id, origin_url, COALESCE(origin_host, ''), COALESCE(upstreams, '[]'), enabled, enable_https, cert_id, COALESCE(cert_ids, '[]'), COALESCE(domain_cert_ids, '[]'), redirect_http, limit_conn_per_server, limit_conn_per_ip, COALESCE(limit_rate, ''), cache_enabled, COALESCE(cache_policy, ''), COALESCE(cache_rules, '[]'), COALESCE(custom_headers, '[]'), basic_auth_enabled, COALESCE(basic_auth_username, ''), COALESCE(basic_auth_password, ''), COALESCE(remark, ''), COALESCE(upstream_type, 'direct'), tunnel_node_id, COALESCE(tunnel_target_addr, ''), COALESCE(tunnel_target_protocol, ''), pages_project_id, created_at, updated_at
				FROM legacy_proxy_routes;`,
			pgSQL: `INSERT INTO of_proxy_routes (id, site_name, domain, domains, origin_id, origin_url, origin_host, upstreams, enabled, enable_https, cert_id, cert_ids, domain_cert_ids, redirect_http, limit_conn_per_server, limit_conn_per_ip, limit_rate, cache_enabled, cache_policy, cache_rules, custom_headers, basic_auth_enabled, basic_auth_username, basic_auth_password, remark, upstream_type, tunnel_node_id, tunnel_target_addr, tunnel_target_protocol, pages_project_id, created_at, updated_at)
				SELECT id, COALESCE(site_name, ''), domain, COALESCE(domains, '[]'), origin_id, origin_url, COALESCE(origin_host, ''), COALESCE(upstreams, '[]'), enabled, enable_https, cert_id, COALESCE(cert_ids, '[]'), COALESCE(domain_cert_ids, '[]'), redirect_http, limit_conn_per_server, limit_conn_per_ip, COALESCE(limit_rate, ''), cache_enabled, COALESCE(cache_policy, ''), COALESCE(cache_rules, '[]'), COALESCE(custom_headers, '[]'), basic_auth_enabled, COALESCE(basic_auth_username, ''), COALESCE(basic_auth_password, ''), COALESCE(remark, ''), COALESCE(upstream_type, 'direct'), tunnel_node_id, COALESCE(tunnel_target_addr, ''), COALESCE(tunnel_target_protocol, ''), pages_project_id, created_at, updated_at
				FROM legacy_proxy_routes
				ON CONFLICT (id) DO UPDATE SET
					site_name = EXCLUDED.site_name,
					domain = EXCLUDED.domain,
					domains = EXCLUDED.domains,
					origin_id = EXCLUDED.origin_id,
					origin_url = EXCLUDED.origin_url,
					origin_host = EXCLUDED.origin_host,
					upstreams = EXCLUDED.upstreams,
					enabled = EXCLUDED.enabled,
					enable_https = EXCLUDED.enable_https,
					cert_id = EXCLUDED.cert_id,
					cert_ids = EXCLUDED.cert_ids,
					domain_cert_ids = EXCLUDED.domain_cert_ids,
					redirect_http = EXCLUDED.redirect_http,
					limit_conn_per_server = EXCLUDED.limit_conn_per_server,
					limit_conn_per_ip = EXCLUDED.limit_conn_per_ip,
					limit_rate = EXCLUDED.limit_rate,
					cache_enabled = EXCLUDED.cache_enabled,
					cache_policy = EXCLUDED.cache_policy,
					cache_rules = EXCLUDED.cache_rules,
					custom_headers = EXCLUDED.custom_headers,
					basic_auth_enabled = EXCLUDED.basic_auth_enabled,
					basic_auth_username = EXCLUDED.basic_auth_username,
					basic_auth_password = EXCLUDED.basic_auth_password,
					remark = EXCLUDED.remark,
					upstream_type = EXCLUDED.upstream_type,
					tunnel_node_id = EXCLUDED.tunnel_node_id,
					tunnel_target_addr = EXCLUDED.tunnel_target_addr,
					tunnel_target_protocol = EXCLUDED.tunnel_target_protocol,
					pages_project_id = EXCLUDED.pages_project_id,
					updated_at = EXCLUDED.updated_at;`,
		},
		{
			legacyName: "legacy_nodes",
			sqliteSQL: `INSERT OR REPLACE INTO of_nodes (id, node_id, name, ip, ip_manual_override, geo_name, geo_latitude, geo_longitude, geo_manual_override, access_token, auto_update_enabled, update_requested, update_channel, update_tag, restart_openresty_requested, version, ext_version, openresty_status, openresty_message, status, current_version, last_seen_at, last_error, created_at, updated_at, node_type, relay_bind_port, relay_vhost_http_port, relay_auth_token, relay_agent_access_addr, relay_client_access_addr, relay_client_proxy_url, capabilities_json, relay_status, relay_web_server_enabled)
				SELECT id, node_id, name, COALESCE(ip, ''), COALESCE(ip_manual_override, 0), COALESCE(geo_name, ''), geo_latitude, geo_longitude, COALESCE(geo_manual_override, 0), COALESCE(access_token, ''), COALESCE(auto_update_enabled, 0), COALESCE(update_requested, 0), COALESCE(update_channel, 'stable'), COALESCE(update_tag, ''), COALESCE(restart_openresty_requested, 0), COALESCE(version, ''), COALESCE(ext_version, ''), COALESCE(openresty_status, 'unknown'), openresty_message, COALESCE(status, 'offline'), COALESCE(current_version, ''), last_seen_at, last_error, created_at, updated_at, COALESCE(node_type, 'edge_node'), COALESCE(relay_bind_port, 0), COALESCE(relay_vhost_http_port, 0), COALESCE(relay_auth_token, ''), COALESCE(relay_agent_access_addr, ''), COALESCE(relay_client_access_addr, ''), COALESCE(relay_client_proxy_url, ''), COALESCE(capabilities_json, '[]'), COALESCE(relay_status, 'unknown'), COALESCE(relay_web_server_enabled, 0)
				FROM legacy_nodes;`,
			pgSQL: `INSERT INTO of_nodes (id, node_id, name, ip, ip_manual_override, geo_name, geo_latitude, geo_longitude, geo_manual_override, access_token, auto_update_enabled, update_requested, update_channel, update_tag, restart_openresty_requested, version, ext_version, openresty_status, openresty_message, status, current_version, last_seen_at, last_error, created_at, updated_at, node_type, relay_bind_port, relay_vhost_http_port, relay_auth_token, relay_agent_access_addr, relay_client_access_addr, relay_client_proxy_url, capabilities_json, relay_status, relay_web_server_enabled)
				SELECT id, node_id, name, COALESCE(ip, ''), COALESCE(ip_manual_override, FALSE), COALESCE(geo_name, ''), geo_latitude, geo_longitude, COALESCE(geo_manual_override, FALSE), COALESCE(access_token, ''), COALESCE(auto_update_enabled, FALSE), COALESCE(update_requested, FALSE), COALESCE(update_channel, 'stable'), COALESCE(update_tag, ''), COALESCE(restart_openresty_requested, FALSE), COALESCE(version, ''), COALESCE(ext_version, ''), COALESCE(openresty_status, 'unknown'), openresty_message, COALESCE(status, 'offline'), COALESCE(current_version, ''), last_seen_at, last_error, created_at, updated_at, COALESCE(node_type, 'edge_node'), COALESCE(relay_bind_port, 0), COALESCE(relay_vhost_http_port, 0), COALESCE(relay_auth_token, ''), COALESCE(relay_agent_access_addr, ''), COALESCE(relay_client_access_addr, ''), COALESCE(relay_client_proxy_url, ''), COALESCE(capabilities_json, '[]'), COALESCE(relay_status, 'unknown'), COALESCE(relay_web_server_enabled, FALSE)
				FROM legacy_nodes
				ON CONFLICT (id) DO UPDATE SET
					node_id = EXCLUDED.node_id,
					name = EXCLUDED.name,
					ip = EXCLUDED.ip,
					ip_manual_override = EXCLUDED.ip_manual_override,
					geo_name = EXCLUDED.geo_name,
					geo_latitude = EXCLUDED.geo_latitude,
					geo_longitude = EXCLUDED.geo_longitude,
					geo_manual_override = EXCLUDED.geo_manual_override,
					access_token = EXCLUDED.access_token,
					auto_update_enabled = EXCLUDED.auto_update_enabled,
					update_requested = EXCLUDED.update_requested,
					update_channel = EXCLUDED.update_channel,
					update_tag = EXCLUDED.update_tag,
					restart_openresty_requested = EXCLUDED.restart_openresty_requested,
					version = EXCLUDED.version,
					ext_version = EXCLUDED.ext_version,
					openresty_status = EXCLUDED.openresty_status,
					openresty_message = EXCLUDED.openresty_message,
					status = EXCLUDED.status,
					current_version = EXCLUDED.current_version,
					last_seen_at = EXCLUDED.last_seen_at,
					last_error = EXCLUDED.last_error,
					updated_at = EXCLUDED.updated_at,
					node_type = EXCLUDED.node_type,
					relay_bind_port = EXCLUDED.relay_bind_port,
					relay_vhost_http_port = EXCLUDED.relay_vhost_http_port,
					relay_auth_token = EXCLUDED.relay_auth_token,
					relay_agent_access_addr = EXCLUDED.relay_agent_access_addr,
					relay_client_access_addr = EXCLUDED.relay_client_access_addr,
					relay_client_proxy_url = EXCLUDED.relay_client_proxy_url,
					capabilities_json = EXCLUDED.capabilities_json,
					relay_status = EXCLUDED.relay_status,
					relay_web_server_enabled = EXCLUDED.relay_web_server_enabled;`,
		},
	}
}

func getWafTasks() []migrationTask {
	return []migrationTask{
		{
			legacyName: "legacy_waf_rule_groups",
			sqliteSQL: `INSERT OR REPLACE INTO of_waf_rule_groups (id, name, enabled, is_global, block_status_code, block_response_body, ip_whitelist, ip_blacklist, ip_whitelist_groups, ip_blacklist_groups, country_whitelist, country_blacklist, region_whitelist, region_blacklist, pow_enabled, pow_config, remark, created_at, updated_at)
				SELECT id, name, COALESCE(enabled, 1), COALESCE(is_global, 0), COALESCE(block_status_code, 418), COALESCE(block_response_body, ''), COALESCE(ip_whitelist, '[]'), COALESCE(ip_blacklist, '[]'), COALESCE(ip_whitelist_groups, '[]'), COALESCE(ip_blacklist_groups, '[]'), COALESCE(country_whitelist, '[]'), COALESCE(country_blacklist, '[]'), COALESCE(region_whitelist, '[]'), COALESCE(region_blacklist, '[]'), COALESCE(pow_enabled, 0), COALESCE(pow_config, '{}'), COALESCE(remark, ''), created_at, updated_at
				FROM legacy_waf_rule_groups;`,
			pgSQL: `INSERT INTO of_waf_rule_groups (id, name, enabled, is_global, block_status_code, block_response_body, ip_whitelist, ip_blacklist, ip_whitelist_groups, ip_blacklist_groups, country_whitelist, country_blacklist, region_whitelist, region_blacklist, pow_enabled, pow_config, remark, created_at, updated_at)
				SELECT id, name, COALESCE(enabled::boolean, TRUE), COALESCE(is_global::boolean, FALSE), COALESCE(block_status_code, 418), COALESCE(block_response_body, ''), COALESCE(ip_whitelist, '[]'), COALESCE(ip_blacklist, '[]'), COALESCE(ip_whitelist_groups, '[]'), COALESCE(ip_blacklist_groups, '[]'), COALESCE(country_whitelist, '[]'), COALESCE(country_blacklist, '[]'), COALESCE(region_whitelist, '[]'), COALESCE(region_blacklist, '[]'), COALESCE(pow_enabled::boolean, FALSE), COALESCE(pow_config, '{}'), COALESCE(remark, ''), created_at, updated_at
				FROM legacy_waf_rule_groups
				ON CONFLICT (id) DO UPDATE SET
					name = EXCLUDED.name,
					enabled = EXCLUDED.enabled,
					is_global = EXCLUDED.is_global,
					block_status_code = EXCLUDED.block_status_code,
					block_response_body = EXCLUDED.block_response_body,
					ip_whitelist = EXCLUDED.ip_whitelist,
					ip_blacklist = EXCLUDED.ip_blacklist,
					ip_whitelist_groups = EXCLUDED.ip_whitelist_groups,
					ip_blacklist_groups = EXCLUDED.ip_blacklist_groups,
					country_whitelist = EXCLUDED.country_whitelist,
					country_blacklist = EXCLUDED.country_blacklist,
					region_whitelist = EXCLUDED.region_whitelist,
					region_blacklist = EXCLUDED.region_blacklist,
					pow_enabled = EXCLUDED.pow_enabled,
					pow_config = EXCLUDED.pow_config,
					remark = EXCLUDED.remark,
					updated_at = EXCLUDED.updated_at;`,
		},
		{
			legacyName: "legacy_waf_rule_group_bindings",
			sqliteSQL: `INSERT OR REPLACE INTO of_waf_rule_group_bindings (id, rule_group_id, proxy_route_id, created_at)
				SELECT id, rule_group_id, proxy_route_id, created_at
				FROM legacy_waf_rule_group_bindings;`,
			pgSQL: `INSERT INTO of_waf_rule_group_bindings (id, rule_group_id, proxy_route_id, created_at)
				SELECT id, rule_group_id, proxy_route_id, created_at
				FROM legacy_waf_rule_group_bindings
				ON CONFLICT (id) DO UPDATE SET
					rule_group_id = EXCLUDED.rule_group_id,
					proxy_route_id = EXCLUDED.proxy_route_id;`,
		},
		{
			legacyName: "legacy_waf_ip_groups",
			sqliteSQL: `INSERT OR REPLACE INTO of_waf_ip_groups (id, name, type, enabled, ip_list, auto_config, ext_ips, subscription_url, subscription_format, subscription_mapping_rule, sync_interval_minutes, last_synced_at, next_sync_at, last_sync_status, last_sync_message, remark, created_at, updated_at)
				SELECT id, name, type, COALESCE(enabled, 1), COALESCE(ip_list, '[]'), COALESCE(auto_config, '{}'), COALESCE(ext_ips, '[]'), COALESCE(subscription_url, ''), COALESCE(subscription_format, 'text'), COALESCE(subscription_mapping_rule, ''), COALESCE(sync_interval_minutes, 1440), last_synced_at, next_sync_at, COALESCE(last_sync_status, ''), COALESCE(last_sync_message, ''), COALESCE(remark, ''), created_at, updated_at
				FROM legacy_waf_ip_groups;`,
			pgSQL: `INSERT INTO of_waf_ip_groups (id, name, type, enabled, ip_list, auto_config, ext_ips, subscription_url, subscription_format, subscription_mapping_rule, sync_interval_minutes, last_synced_at, next_sync_at, last_sync_status, last_sync_message, remark, created_at, updated_at)
				SELECT id, name, type, COALESCE(enabled::boolean, TRUE), COALESCE(ip_list, '[]'), COALESCE(auto_config, '{}'), COALESCE(ext_ips, '[]'), COALESCE(subscription_url, ''), COALESCE(subscription_format, 'text'), COALESCE(subscription_mapping_rule, ''), COALESCE(sync_interval_minutes, 1440), last_synced_at, next_sync_at, COALESCE(last_sync_status, ''), COALESCE(last_sync_message, ''), COALESCE(remark, ''), created_at, updated_at
				FROM legacy_waf_ip_groups
				ON CONFLICT (id) DO UPDATE SET
					name = EXCLUDED.name,
					type = EXCLUDED.type,
					enabled = EXCLUDED.enabled,
					ip_list = EXCLUDED.ip_list,
					auto_config = EXCLUDED.auto_config,
					ext_ips = EXCLUDED.ext_ips,
					subscription_url = EXCLUDED.subscription_url,
					subscription_format = EXCLUDED.subscription_format,
					subscription_mapping_rule = EXCLUDED.subscription_mapping_rule,
					sync_interval_minutes = EXCLUDED.sync_interval_minutes,
					last_synced_at = EXCLUDED.last_synced_at,
					next_sync_at = EXCLUDED.next_sync_at,
					last_sync_status = EXCLUDED.last_sync_status,
					last_sync_message = EXCLUDED.last_sync_message,
					remark = EXCLUDED.remark,
					updated_at = EXCLUDED.updated_at;`,
		},
	}
}

func getTLSTasks() []migrationTask {
	return []migrationTask{
		{
			legacyName: "legacy_tls_certificates",
			sqliteSQL: `INSERT OR REPLACE INTO of_tls_certificates (id, name, cert_pem, key_pem, not_before, not_after, remark, provider, acme_account_id, dns_account_id, key_algorithm, auto_renew, primary_domain, other_domains, disable_cname, skip_dns, dns1, dns2, apply_status, apply_message, created_at, updated_at)
				SELECT id, name, cert_pem, key_pem, not_before, not_after, COALESCE(remark, ''), COALESCE(provider, 'upload'), COALESCE(acme_account_id, 0), COALESCE(dns_account_id, 0), COALESCE(key_algorithm, ''), COALESCE(auto_renew, 0), COALESCE(primary_domain, ''), COALESCE(other_domains, ''), COALESCE(disable_cname, 0), COALESCE(skip_dns, 0), COALESCE(dns1, ''), COALESCE(dns2, ''), COALESCE(apply_status, 'ready'), COALESCE(apply_message, ''), created_at, updated_at
				FROM legacy_tls_certificates;`,
			pgSQL: `INSERT INTO of_tls_certificates (id, name, cert_pem, key_pem, not_before, not_after, remark, provider, acme_account_id, dns_account_id, key_algorithm, auto_renew, primary_domain, other_domains, disable_cname, skip_dns, dns1, dns2, apply_status, apply_message, created_at, updated_at)
				SELECT id, name, cert_pem, key_pem, not_before, not_after, COALESCE(remark, ''), COALESCE(provider, 'upload'), COALESCE(acme_account_id, 0), COALESCE(dns_account_id, 0), COALESCE(key_algorithm, ''), COALESCE(auto_renew, FALSE), COALESCE(primary_domain, ''), COALESCE(other_domains, ''), COALESCE(disable_cname, FALSE), COALESCE(skip_dns, FALSE), COALESCE(dns1, ''), COALESCE(dns2, ''), COALESCE(apply_status, 'ready'), COALESCE(apply_message, ''), created_at, updated_at
				FROM legacy_tls_certificates
				ON CONFLICT (id) DO UPDATE SET
					name = EXCLUDED.name,
					cert_pem = EXCLUDED.cert_pem,
					key_pem = EXCLUDED.key_pem,
					not_before = EXCLUDED.not_before,
					not_after = EXCLUDED.not_after,
					remark = EXCLUDED.remark,
					provider = EXCLUDED.provider,
					acme_account_id = EXCLUDED.acme_account_id,
					dns_account_id = EXCLUDED.dns_account_id,
					key_algorithm = EXCLUDED.key_algorithm,
					auto_renew = EXCLUDED.auto_renew,
					primary_domain = EXCLUDED.primary_domain,
					other_domains = EXCLUDED.other_domains,
					disable_cname = EXCLUDED.disable_cname,
					skip_dns = EXCLUDED.skip_dns,
					dns1 = EXCLUDED.dns1,
					dns2 = EXCLUDED.dns2,
					apply_status = EXCLUDED.apply_status,
					apply_message = EXCLUDED.apply_message,
					updated_at = EXCLUDED.updated_at;`,
		},
		{
			legacyName: "legacy_managed_domains",
			sqliteSQL: `INSERT OR REPLACE INTO of_managed_domains (id, domain, cert_id, enabled, remark, created_at, updated_at)
				SELECT id, domain, cert_id, COALESCE(enabled, 1), COALESCE(remark, ''), created_at, updated_at
				FROM legacy_managed_domains;`,
			pgSQL: `INSERT INTO of_managed_domains (id, domain, cert_id, enabled, remark, created_at, updated_at)
				SELECT id, domain, cert_id, COALESCE(enabled, TRUE), COALESCE(remark, ''), created_at, updated_at
				FROM legacy_managed_domains
				ON CONFLICT (id) DO UPDATE SET
					domain = EXCLUDED.domain,
					cert_id = EXCLUDED.cert_id,
					enabled = EXCLUDED.enabled,
					remark = EXCLUDED.remark,
					updated_at = EXCLUDED.updated_at;`,
		},
		{
			legacyName: "legacy_dns_accounts",
			sqliteSQL: `INSERT OR REPLACE INTO of_dns_accounts (id, name, type, authorization, created_at, updated_at)
				SELECT id, name, type, authorization, created_at, updated_at
				FROM legacy_dns_accounts;`,
			pgSQL: `INSERT INTO of_dns_accounts (id, name, type, "authorization", created_at, updated_at)
				SELECT id, name, type, "authorization", created_at, updated_at
				FROM legacy_dns_accounts
				ON CONFLICT (id) DO UPDATE SET
					name = EXCLUDED.name,
					type = EXCLUDED.type,
					"authorization" = EXCLUDED."authorization",
					updated_at = EXCLUDED.updated_at;`,
		},
		{
			legacyName: "legacy_acme_accounts",
			sqliteSQL: `INSERT OR REPLACE INTO of_acme_accounts (id, email, url, private_key, created_at, updated_at)
				SELECT id, COALESCE(email, ''), COALESCE(url, ''), COALESCE(private_key, ''), created_at, updated_at
				FROM legacy_acme_accounts;`,
			pgSQL: `INSERT INTO of_acme_accounts (id, email, url, private_key, created_at, updated_at)
				SELECT id, COALESCE(email, ''), COALESCE(url, ''), COALESCE(private_key, ''), created_at, updated_at
				FROM legacy_acme_accounts
				ON CONFLICT (id) DO UPDATE SET
					email = EXCLUDED.email,
					url = EXCLUDED.url,
					private_key = EXCLUDED.private_key,
					updated_at = EXCLUDED.updated_at;`,
		},
	}
}

func getPagesAndConfigTasks() []migrationTask {
	return []migrationTask{
		{
			legacyName: "legacy_config_versions",
			sqliteSQL: `INSERT OR REPLACE INTO of_config_versions (id, version, snapshot_json, main_config, rendered_config, support_files_json, checksum, is_active, created_by, created_at)
				SELECT id, version, snapshot_json, COALESCE(main_config, ''), rendered_config, COALESCE(support_files_json, '[]'), checksum, COALESCE(is_active, 0), created_by, created_at
				FROM legacy_config_versions;`,
			pgSQL: `INSERT INTO of_config_versions (id, version, snapshot_json, main_config, rendered_config, support_files_json, checksum, is_active, created_by, created_at)
				SELECT id, version, snapshot_json, COALESCE(main_config, ''), rendered_config, COALESCE(support_files_json, '[]'), checksum, COALESCE(is_active, FALSE), created_by, created_at
				FROM legacy_config_versions
				ON CONFLICT (id) DO UPDATE SET
					version = EXCLUDED.version,
					snapshot_json = EXCLUDED.snapshot_json,
					main_config = EXCLUDED.main_config,
					rendered_config = EXCLUDED.rendered_config,
					support_files_json = EXCLUDED.support_files_json,
					checksum = EXCLUDED.checksum,
					is_active = EXCLUDED.is_active,
					created_by = EXCLUDED.created_by;`,
		},
		{
			legacyName: "legacy_pages_projects",
			sqliteSQL: `INSERT OR REPLACE INTO of_pages_projects (id, name, slug, description, enabled, spa_fallback_enabled, spa_fallback_path, api_proxy_enabled, api_proxy_path, api_proxy_pass, api_proxy_rewrite, active_deployment_id, root_dir, entry_file, created_at, updated_at)
				SELECT id, name, slug, COALESCE(description, ''), COALESCE(enabled, 1), COALESCE(spa_fallback_enabled, 0), COALESCE(spa_fallback_path, '/index.html'), COALESCE(api_proxy_enabled, 0), COALESCE(api_proxy_path, ''), COALESCE(api_proxy_pass, ''), COALESCE(api_proxy_rewrite, ''), active_deployment_id, COALESCE(root_dir, ''), COALESCE(entry_file, 'index.html'), created_at, updated_at
				FROM legacy_pages_projects;`,
			pgSQL: `INSERT INTO of_pages_projects (id, name, slug, description, enabled, spa_fallback_enabled, spa_fallback_path, api_proxy_enabled, api_proxy_path, api_proxy_pass, api_proxy_rewrite, active_deployment_id, root_dir, entry_file, created_at, updated_at)
				SELECT id, name, slug, COALESCE(description, ''), COALESCE(enabled, TRUE), COALESCE(spa_fallback_enabled, FALSE), COALESCE(spa_fallback_path, '/index.html'), COALESCE(api_proxy_enabled, FALSE), COALESCE(api_proxy_path, ''), COALESCE(api_proxy_pass, ''), COALESCE(api_proxy_rewrite, ''), active_deployment_id, COALESCE(root_dir, ''), COALESCE(entry_file, 'index.html'), created_at, updated_at
				FROM legacy_pages_projects
				ON CONFLICT (id) DO UPDATE SET
					name = EXCLUDED.name,
					slug = EXCLUDED.slug,
					description = EXCLUDED.description,
					enabled = EXCLUDED.enabled,
					spa_fallback_enabled = EXCLUDED.spa_fallback_enabled,
					spa_fallback_path = EXCLUDED.spa_fallback_path,
					api_proxy_enabled = EXCLUDED.api_proxy_enabled,
					api_proxy_path = EXCLUDED.api_proxy_path,
					api_proxy_pass = EXCLUDED.api_proxy_pass,
					api_proxy_rewrite = EXCLUDED.api_proxy_rewrite,
					active_deployment_id = EXCLUDED.active_deployment_id,
					root_dir = EXCLUDED.root_dir,
					entry_file = EXCLUDED.entry_file,
					updated_at = EXCLUDED.updated_at;`,
		},
		{
			legacyName: "legacy_pages_deployments",
			sqliteSQL: `INSERT OR REPLACE INTO of_pages_deployments (id, project_id, deployment_number, checksum, status, artifact_path, file_count, total_size, created_by, created_at, activated_at)
				SELECT id, project_id, deployment_number, checksum, COALESCE(status, 'uploaded'), artifact_path, COALESCE(file_count, 0), COALESCE(total_size, 0), COALESCE(created_by, ''), created_at, activated_at
				FROM legacy_pages_deployments;`,
			pgSQL: `INSERT INTO of_pages_deployments (id, project_id, deployment_number, checksum, status, artifact_path, file_count, total_size, created_by, created_at, activated_at)
				SELECT id, project_id, deployment_number, checksum, COALESCE(status, 'uploaded'), artifact_path, COALESCE(file_count, 0), COALESCE(total_size, 0), COALESCE(created_by, ''), created_at, activated_at
				FROM legacy_pages_deployments
				ON CONFLICT (id) DO UPDATE SET
					project_id = EXCLUDED.project_id,
					deployment_number = EXCLUDED.deployment_number,
					checksum = EXCLUDED.checksum,
					status = EXCLUDED.status,
					artifact_path = EXCLUDED.artifact_path,
					file_count = EXCLUDED.file_count,
					total_size = EXCLUDED.total_size,
					created_by = EXCLUDED.created_by,
					activated_at = EXCLUDED.activated_at;`,
		},
		{
			legacyName: "legacy_pages_deployment_files",
			sqliteSQL: `INSERT OR REPLACE INTO of_pages_deployment_files (id, deployment_id, path, size, checksum, created_at)
				SELECT id, deployment_id, path, COALESCE(size, 0), checksum, created_at
				FROM legacy_pages_deployment_files;`,
			pgSQL: `INSERT INTO of_pages_deployment_files (id, deployment_id, path, size, checksum, created_at)
				SELECT id, deployment_id, path, COALESCE(size, 0), checksum, created_at
				FROM legacy_pages_deployment_files
				ON CONFLICT (id) DO UPDATE SET
					deployment_id = EXCLUDED.deployment_id,
					path = EXCLUDED.path,
					size = EXCLUDED.size,
					checksum = EXCLUDED.checksum;`,
		},
	}
}

func down202606200006(_ context.Context, _ *sql.Tx) error {
	// Down migration is a no-op
	return nil
}
