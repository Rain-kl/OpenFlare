-- +goose Up
CREATE TABLE of_edge_snapshot_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    source_revision BIGINT NOT NULL DEFAULT 0,
    compiled_revision BIGINT NOT NULL DEFAULT 0
);

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_proxy_routes_insert AFTER INSERT ON of_proxy_routes
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_proxy_routes_update AFTER UPDATE ON of_proxy_routes
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_proxy_routes_delete AFTER DELETE ON of_proxy_routes
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_zone_domains_insert AFTER INSERT ON of_zone_domains
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_zone_domains_update AFTER UPDATE ON of_zone_domains
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_zone_domains_delete AFTER DELETE ON of_zone_domains
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_pow_policies_insert AFTER INSERT ON of_pow_policies
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_pow_policies_update AFTER UPDATE ON of_pow_policies
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_pow_policies_delete AFTER DELETE ON of_pow_policies
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_waf_rule_groups_insert AFTER INSERT ON of_waf_rule_groups
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_waf_rule_groups_update AFTER UPDATE ON of_waf_rule_groups
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_waf_rule_groups_delete AFTER DELETE ON of_waf_rule_groups
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_waf_rule_group_bindings_insert AFTER INSERT ON of_waf_rule_group_bindings
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_waf_rule_group_bindings_update AFTER UPDATE ON of_waf_rule_group_bindings
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_waf_rule_group_bindings_delete AFTER DELETE ON of_waf_rule_group_bindings
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_tls_certificates_insert AFTER INSERT ON of_tls_certificates
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_tls_certificates_update AFTER UPDATE ON of_tls_certificates
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_tls_certificates_delete AFTER DELETE ON of_tls_certificates
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_pages_projects_insert AFTER INSERT ON of_pages_projects
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_pages_projects_update AFTER UPDATE ON of_pages_projects
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_pages_projects_delete AFTER DELETE ON of_pages_projects
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_pages_deployments_insert AFTER INSERT ON of_pages_deployments
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_pages_deployments_update AFTER UPDATE ON of_pages_deployments
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_pages_deployments_delete AFTER DELETE ON of_pages_deployments
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_setting_insert AFTER INSERT ON w_system_configs
WHEN NEW.key IN ('edge_trusted_proxy_cidrs', 'edge_default_limit_conn_per_server', 'edge_default_limit_conn_per_ip',
    'edge_default_limit_rate', 'edge_default_limit_req_per_ip', 'origin_error_page_enabled',
    'origin_error_page_status_codes', 'origin_error_page_html', 'origin_error_page_get_only',
    'pow_page', 'site_auth_html', 'sw_offline_enabled', 'sw_offline_html', 'sw_offline_domains')
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_setting_update AFTER UPDATE OF key, value ON w_system_configs
WHEN (NEW.key IN ('edge_trusted_proxy_cidrs', 'edge_default_limit_conn_per_server', 'edge_default_limit_conn_per_ip',
    'edge_default_limit_rate', 'edge_default_limit_req_per_ip', 'origin_error_page_enabled',
    'origin_error_page_status_codes', 'origin_error_page_html', 'origin_error_page_get_only',
    'pow_page', 'site_auth_html', 'sw_offline_enabled', 'sw_offline_html', 'sw_offline_domains') OR OLD.key IN ('edge_trusted_proxy_cidrs', 'edge_default_limit_conn_per_server', 'edge_default_limit_conn_per_ip',
    'edge_default_limit_rate', 'edge_default_limit_req_per_ip', 'origin_error_page_enabled',
    'origin_error_page_status_codes', 'origin_error_page_html', 'origin_error_page_get_only',
    'pow_page', 'site_auth_html', 'sw_offline_enabled', 'sw_offline_html', 'sw_offline_domains')) AND (NEW.key IS NOT OLD.key OR NEW.value IS NOT OLD.value)
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_setting_delete AFTER DELETE ON w_system_configs
WHEN OLD.key IN ('edge_trusted_proxy_cidrs', 'edge_default_limit_conn_per_server', 'edge_default_limit_conn_per_ip',
    'edge_default_limit_rate', 'edge_default_limit_req_per_ip', 'origin_error_page_enabled',
    'origin_error_page_status_codes', 'origin_error_page_html', 'origin_error_page_get_only',
    'pow_page', 'site_auth_html', 'sw_offline_enabled', 'sw_offline_html', 'sw_offline_domains')
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_relay_insert AFTER INSERT ON of_nodes
WHEN NEW.node_type = 'tunnel_relay'
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_relay_update AFTER UPDATE OF node_type, ip, relay_vhost_http_port, relay_agent_access_addr, relay_client_access_addr ON of_nodes
WHEN (NEW.node_type = 'tunnel_relay' OR OLD.node_type = 'tunnel_relay') AND (NEW.node_type IS NOT OLD.node_type OR NEW.ip IS NOT OLD.ip OR NEW.relay_vhost_http_port IS NOT OLD.relay_vhost_http_port OR NEW.relay_agent_access_addr IS NOT OLD.relay_agent_access_addr OR NEW.relay_client_access_addr IS NOT OLD.relay_client_access_addr)
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_relay_delete AFTER DELETE ON of_nodes
WHEN OLD.node_type = 'tunnel_relay'
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER dirty_edge_proxy_routes_insert;
DROP TRIGGER dirty_edge_proxy_routes_update;
DROP TRIGGER dirty_edge_proxy_routes_delete;
DROP TRIGGER dirty_edge_zone_domains_insert;
DROP TRIGGER dirty_edge_zone_domains_update;
DROP TRIGGER dirty_edge_zone_domains_delete;
DROP TRIGGER dirty_edge_pow_policies_insert;
DROP TRIGGER dirty_edge_pow_policies_update;
DROP TRIGGER dirty_edge_pow_policies_delete;
DROP TRIGGER dirty_edge_waf_rule_groups_insert;
DROP TRIGGER dirty_edge_waf_rule_groups_update;
DROP TRIGGER dirty_edge_waf_rule_groups_delete;
DROP TRIGGER dirty_edge_waf_rule_group_bindings_insert;
DROP TRIGGER dirty_edge_waf_rule_group_bindings_update;
DROP TRIGGER dirty_edge_waf_rule_group_bindings_delete;
DROP TRIGGER dirty_edge_tls_certificates_insert;
DROP TRIGGER dirty_edge_tls_certificates_update;
DROP TRIGGER dirty_edge_tls_certificates_delete;
DROP TRIGGER dirty_edge_pages_projects_insert;
DROP TRIGGER dirty_edge_pages_projects_update;
DROP TRIGGER dirty_edge_pages_projects_delete;
DROP TRIGGER dirty_edge_pages_deployments_insert;
DROP TRIGGER dirty_edge_pages_deployments_update;
DROP TRIGGER dirty_edge_pages_deployments_delete;
DROP TRIGGER dirty_edge_setting_insert;
DROP TRIGGER dirty_edge_setting_update;
DROP TRIGGER dirty_edge_setting_delete;
DROP TRIGGER dirty_edge_relay_insert;
DROP TRIGGER dirty_edge_relay_update;
DROP TRIGGER dirty_edge_relay_delete;
DROP TABLE of_edge_snapshot_state;
