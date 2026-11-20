-- +goose Up
CREATE TABLE of_edge_snapshot_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    source_revision BIGINT NOT NULL DEFAULT 0,
    compiled_revision BIGINT NOT NULL DEFAULT 0
);

-- +goose StatementBegin
CREATE FUNCTION openflare_dirty_edge_snapshot() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER dirty_edge_proxy_routes AFTER INSERT OR UPDATE OR DELETE ON of_proxy_routes
FOR EACH STATEMENT EXECUTE FUNCTION openflare_dirty_edge_snapshot();

CREATE TRIGGER dirty_edge_zone_domains AFTER INSERT OR UPDATE OR DELETE ON of_zone_domains
FOR EACH STATEMENT EXECUTE FUNCTION openflare_dirty_edge_snapshot();

CREATE TRIGGER dirty_edge_pow_policies AFTER INSERT OR UPDATE OR DELETE ON of_pow_policies
FOR EACH STATEMENT EXECUTE FUNCTION openflare_dirty_edge_snapshot();

CREATE TRIGGER dirty_edge_waf_rule_groups AFTER INSERT OR UPDATE OR DELETE ON of_waf_rule_groups
FOR EACH STATEMENT EXECUTE FUNCTION openflare_dirty_edge_snapshot();

CREATE TRIGGER dirty_edge_waf_rule_group_bindings AFTER INSERT OR UPDATE OR DELETE ON of_waf_rule_group_bindings
FOR EACH STATEMENT EXECUTE FUNCTION openflare_dirty_edge_snapshot();

CREATE TRIGGER dirty_edge_tls_certificates AFTER INSERT OR UPDATE OR DELETE ON of_tls_certificates
FOR EACH STATEMENT EXECUTE FUNCTION openflare_dirty_edge_snapshot();

CREATE TRIGGER dirty_edge_pages_projects AFTER INSERT OR UPDATE OR DELETE ON of_pages_projects
FOR EACH STATEMENT EXECUTE FUNCTION openflare_dirty_edge_snapshot();

CREATE TRIGGER dirty_edge_pages_deployments AFTER INSERT OR UPDATE OR DELETE ON of_pages_deployments
FOR EACH STATEMENT EXECUTE FUNCTION openflare_dirty_edge_snapshot();

CREATE TRIGGER dirty_edge_setting_insert AFTER INSERT ON w_system_configs
FOR EACH ROW WHEN (NEW.key IN ('edge_trusted_proxy_cidrs', 'edge_default_limit_conn_per_server', 'edge_default_limit_conn_per_ip',
    'edge_default_limit_rate', 'edge_default_limit_req_per_ip', 'origin_error_page_enabled',
    'origin_error_page_status_codes', 'origin_error_page_html', 'origin_error_page_get_only',
    'pow_page', 'site_auth_html', 'sw_offline_enabled', 'sw_offline_html', 'sw_offline_domains'))
EXECUTE FUNCTION openflare_dirty_edge_snapshot();

CREATE TRIGGER dirty_edge_setting_update AFTER UPDATE OF key, value ON w_system_configs
FOR EACH ROW WHEN ((NEW.key IN ('edge_trusted_proxy_cidrs', 'edge_default_limit_conn_per_server', 'edge_default_limit_conn_per_ip',
    'edge_default_limit_rate', 'edge_default_limit_req_per_ip', 'origin_error_page_enabled',
    'origin_error_page_status_codes', 'origin_error_page_html', 'origin_error_page_get_only',
    'pow_page', 'site_auth_html', 'sw_offline_enabled', 'sw_offline_html', 'sw_offline_domains') OR OLD.key IN ('edge_trusted_proxy_cidrs', 'edge_default_limit_conn_per_server', 'edge_default_limit_conn_per_ip',
    'edge_default_limit_rate', 'edge_default_limit_req_per_ip', 'origin_error_page_enabled',
    'origin_error_page_status_codes', 'origin_error_page_html', 'origin_error_page_get_only',
    'pow_page', 'site_auth_html', 'sw_offline_enabled', 'sw_offline_html', 'sw_offline_domains')) AND (NEW.key IS DISTINCT FROM OLD.key OR NEW.value IS DISTINCT FROM OLD.value))
EXECUTE FUNCTION openflare_dirty_edge_snapshot();

CREATE TRIGGER dirty_edge_setting_delete AFTER DELETE ON w_system_configs
FOR EACH ROW WHEN (OLD.key IN ('edge_trusted_proxy_cidrs', 'edge_default_limit_conn_per_server', 'edge_default_limit_conn_per_ip',
    'edge_default_limit_rate', 'edge_default_limit_req_per_ip', 'origin_error_page_enabled',
    'origin_error_page_status_codes', 'origin_error_page_html', 'origin_error_page_get_only',
    'pow_page', 'site_auth_html', 'sw_offline_enabled', 'sw_offline_html', 'sw_offline_domains'))
EXECUTE FUNCTION openflare_dirty_edge_snapshot();

CREATE TRIGGER dirty_edge_relay_insert AFTER INSERT ON of_nodes
FOR EACH ROW WHEN (NEW.node_type = 'tunnel_relay')
EXECUTE FUNCTION openflare_dirty_edge_snapshot();

CREATE TRIGGER dirty_edge_relay_update AFTER UPDATE OF node_type, ip, relay_vhost_http_port, relay_agent_access_addr, relay_client_access_addr ON of_nodes
FOR EACH ROW WHEN ((NEW.node_type = 'tunnel_relay' OR OLD.node_type = 'tunnel_relay') AND (NEW.node_type IS DISTINCT FROM OLD.node_type OR NEW.ip IS DISTINCT FROM OLD.ip OR NEW.relay_vhost_http_port IS DISTINCT FROM OLD.relay_vhost_http_port OR NEW.relay_agent_access_addr IS DISTINCT FROM OLD.relay_agent_access_addr OR NEW.relay_client_access_addr IS DISTINCT FROM OLD.relay_client_access_addr))
EXECUTE FUNCTION openflare_dirty_edge_snapshot();

CREATE TRIGGER dirty_edge_relay_delete AFTER DELETE ON of_nodes
FOR EACH ROW WHEN (OLD.node_type = 'tunnel_relay')
EXECUTE FUNCTION openflare_dirty_edge_snapshot();

-- +goose Down
DROP TRIGGER dirty_edge_proxy_routes ON of_proxy_routes;
DROP TRIGGER dirty_edge_zone_domains ON of_zone_domains;
DROP TRIGGER dirty_edge_pow_policies ON of_pow_policies;
DROP TRIGGER dirty_edge_waf_rule_groups ON of_waf_rule_groups;
DROP TRIGGER dirty_edge_waf_rule_group_bindings ON of_waf_rule_group_bindings;
DROP TRIGGER dirty_edge_tls_certificates ON of_tls_certificates;
DROP TRIGGER dirty_edge_pages_projects ON of_pages_projects;
DROP TRIGGER dirty_edge_pages_deployments ON of_pages_deployments;
DROP TRIGGER dirty_edge_setting_insert ON w_system_configs;
DROP TRIGGER dirty_edge_setting_update ON w_system_configs;
DROP TRIGGER dirty_edge_setting_delete ON w_system_configs;
DROP TRIGGER dirty_edge_relay_insert ON of_nodes;
DROP TRIGGER dirty_edge_relay_update ON of_nodes;
DROP TRIGGER dirty_edge_relay_delete ON of_nodes;
DROP FUNCTION openflare_dirty_edge_snapshot();
DROP TABLE of_edge_snapshot_state;
