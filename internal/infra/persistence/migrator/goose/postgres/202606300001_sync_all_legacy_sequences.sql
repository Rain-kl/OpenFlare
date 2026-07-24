-- +goose Up
SELECT setval(pg_get_serial_sequence('w_users', 'id'), GREATEST(COALESCE((SELECT MAX(id) FROM w_users), 0), 1), COALESCE((SELECT MAX(id) FROM w_users), 0) > 0);
SELECT setval(pg_get_serial_sequence('w_auth_sources', 'id'), GREATEST(COALESCE((SELECT MAX(id) FROM w_auth_sources), 0), 1), COALESCE((SELECT MAX(id) FROM w_auth_sources), 0) > 0);
SELECT setval(pg_get_serial_sequence('w_external_accounts', 'id'), GREATEST(COALESCE((SELECT MAX(id) FROM w_external_accounts), 0), 1), COALESCE((SELECT MAX(id) FROM w_external_accounts), 0) > 0);
SELECT setval(pg_get_serial_sequence('of_origins', 'id'), GREATEST(COALESCE((SELECT MAX(id) FROM of_origins), 0), 1), COALESCE((SELECT MAX(id) FROM of_origins), 0) > 0);
SELECT setval(pg_get_serial_sequence('of_apply_logs', 'id'), GREATEST(COALESCE((SELECT MAX(id) FROM of_apply_logs), 0), 1), COALESCE((SELECT MAX(id) FROM of_apply_logs), 0) > 0);
SELECT setval(pg_get_serial_sequence('of_proxy_routes', 'id'), GREATEST(COALESCE((SELECT MAX(id) FROM of_proxy_routes), 0), 1), COALESCE((SELECT MAX(id) FROM of_proxy_routes), 0) > 0);
SELECT setval(pg_get_serial_sequence('of_nodes', 'id'), GREATEST(COALESCE((SELECT MAX(id) FROM of_nodes), 0), 1), COALESCE((SELECT MAX(id) FROM of_nodes), 0) > 0);
SELECT setval(pg_get_serial_sequence('of_waf_rule_groups', 'id'), GREATEST(COALESCE((SELECT MAX(id) FROM of_waf_rule_groups), 0), 1), COALESCE((SELECT MAX(id) FROM of_waf_rule_groups), 0) > 0);
SELECT setval(pg_get_serial_sequence('of_waf_ip_groups', 'id'), GREATEST(COALESCE((SELECT MAX(id) FROM of_waf_ip_groups), 0), 1), COALESCE((SELECT MAX(id) FROM of_waf_ip_groups), 0) > 0);
SELECT setval(pg_get_serial_sequence('of_tls_certificates', 'id'), GREATEST(COALESCE((SELECT MAX(id) FROM of_tls_certificates), 0), 1), COALESCE((SELECT MAX(id) FROM of_tls_certificates), 0) > 0);
SELECT setval(pg_get_serial_sequence('of_managed_domains', 'id'), GREATEST(COALESCE((SELECT MAX(id) FROM of_managed_domains), 0), 1), COALESCE((SELECT MAX(id) FROM of_managed_domains), 0) > 0);
SELECT setval(pg_get_serial_sequence('of_dns_accounts', 'id'), GREATEST(COALESCE((SELECT MAX(id) FROM of_dns_accounts), 0), 1), COALESCE((SELECT MAX(id) FROM of_dns_accounts), 0) > 0);
SELECT setval(pg_get_serial_sequence('of_acme_accounts', 'id'), GREATEST(COALESCE((SELECT MAX(id) FROM of_acme_accounts), 0), 1), COALESCE((SELECT MAX(id) FROM of_acme_accounts), 0) > 0);
SELECT setval(pg_get_serial_sequence('of_pages_projects', 'id'), GREATEST(COALESCE((SELECT MAX(id) FROM of_pages_projects), 0), 1), COALESCE((SELECT MAX(id) FROM of_pages_projects), 0) > 0);
SELECT setval(pg_get_serial_sequence('of_pages_deployments', 'id'), GREATEST(COALESCE((SELECT MAX(id) FROM of_pages_deployments), 0), 1), COALESCE((SELECT MAX(id) FROM of_pages_deployments), 0) > 0);
SELECT setval(pg_get_serial_sequence('of_pages_deployment_files', 'id'), GREATEST(COALESCE((SELECT MAX(id) FROM of_pages_deployment_files), 0), 1), COALESCE((SELECT MAX(id) FROM of_pages_deployment_files), 0) > 0);

-- +goose Down
SELECT 1;
