-- +goose Up
INSERT INTO of_tunnel_relay_pools (pool_key, name, description, region, is_default, routing_policy_json, created_at, updated_at)
VALUES ('default', 'Default Pool', 'Default global tunnel relay pool', '', TRUE, '{}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (pool_key) DO NOTHING;

INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES
    ('relay_discovery_token', '', 'system', 0, '中继节点自动发现注册令牌', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('tunnel_update_repo', 'Rain-kl/OpenFlare', 'system', 0, '隧道组件（openflare-relay / openflared）GitHub Release 更新仓库', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('tunnel_heartbeat_interval', '5000', 'system', 0, '隧道中继与连接器心跳周期（毫秒）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('tunnel_lease_ttl_seconds', '30', 'system', 0, '中继连接租约有效时间（秒）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('tunnel_default_ha_relays', '2', 'business', 0, '每条隧道连接器默认并行挂载的冗余中继节点数', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs
WHERE key IN (
    'relay_discovery_token',
    'tunnel_update_repo',
    'tunnel_heartbeat_interval',
    'tunnel_lease_ttl_seconds',
    'tunnel_default_ha_relays'
);

DELETE FROM of_tunnel_relay_pools WHERE pool_key = 'default';
