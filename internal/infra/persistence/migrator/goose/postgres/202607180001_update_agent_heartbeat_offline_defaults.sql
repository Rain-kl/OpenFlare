-- +goose Up
-- 将仍为历史默认值的心跳/离线配置升级为新默认：心跳 3s、离线 60s。
-- 已由管理员改成其他值的配置不受影响。
UPDATE w_system_configs
SET value = '3000',
    updated_at = CURRENT_TIMESTAMP
WHERE key = 'agent_heartbeat_interval'
  AND value = '10000';

UPDATE w_system_configs
SET value = '60000',
    updated_at = CURRENT_TIMESTAMP
WHERE key = 'node_offline_threshold'
  AND value = '120000';

-- +goose Down
UPDATE w_system_configs
SET value = '10000',
    updated_at = CURRENT_TIMESTAMP
WHERE key = 'agent_heartbeat_interval'
  AND value = '3000';

UPDATE w_system_configs
SET value = '120000',
    updated_at = CURRENT_TIMESTAMP
WHERE key = 'node_offline_threshold'
  AND value = '60000';
