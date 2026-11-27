-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES
    ('netintel_data_dir', 'data/netintel', 'system', 0, '网络情报数据库（GeoIP/ASN/bgp.tools）本地存储目录', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('netintel_geoip_source', 'fyralabs', 'system', 0, 'GeoIP/ASN 数据库下载源（fyralabs 或 sapics）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('netintel_bgptools_user_agent', 'OpenFlare-NetIntel/1.0 (+https://github.com/Rain-kl/OpenFlare)', 'system', 0, '请求 bgp.tools 公开数据接口时携带的 User-Agent 标识', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('netintel_caida_enabled', 'false', 'system', 0, '是否开启运行期 CAIDA AS Relationships 数据集同步（需遵循 CAIDA AUA）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('netintel_auto_probe_enabled', 'true', 'system', 0, '是否启用边缘节点后台自动路由质量探测以补全网络质量矩阵', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

SELECT setval(
    pg_get_serial_sequence('w_schedules', 'id'),
    GREATEST(
        1,
        COALESCE((SELECT MAX(id) FROM w_schedules), 0),
        COALESCE((
            SELECT sequences.last_value
            FROM pg_sequences AS sequences
            WHERE format('%I.%I', sequences.schemaname, sequences.sequencename)::regclass =
                  pg_get_serial_sequence('w_schedules', 'id')::regclass
        ), 0)
    ),
    TRUE
);

INSERT INTO w_schedules (name, task_type, cron, payload, is_active, hide_success_log, created_at, updated_at)
SELECT
    'OpenFlare 网络情报数据库同步',
    'of_netintel_sync',
    '0 */6 * * *',
    '{}',
    TRUE,
    TRUE,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
WHERE NOT EXISTS (
    SELECT 1 FROM w_schedules WHERE task_type = 'of_netintel_sync'
);

INSERT INTO w_schedules (name, task_type, cron, payload, is_active, hide_success_log, created_at, updated_at)
SELECT
    'OpenFlare 网络质量矩阵聚合',
    'of_netquality_rollup',
    '*/5 * * * *',
    '{}',
    TRUE,
    TRUE,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
WHERE NOT EXISTS (
    SELECT 1 FROM w_schedules WHERE task_type = 'of_netquality_rollup'
);

INSERT INTO w_schedules (name, task_type, cron, payload, is_active, hide_success_log, created_at, updated_at)
SELECT
    'OpenFlare 后台路由质量探测',
    'of_netquality_probe',
    '*/30 * * * *',
    '{}',
    TRUE,
    TRUE,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
WHERE NOT EXISTS (
    SELECT 1 FROM w_schedules WHERE task_type = 'of_netquality_probe'
);

-- +goose Down
DELETE FROM w_schedules WHERE task_type IN ('of_netintel_sync', 'of_netquality_rollup', 'of_netquality_probe');
DELETE FROM w_system_configs WHERE key IN (
    'netintel_data_dir',
    'netintel_geoip_source',
    'netintel_bgptools_user_agent',
    'netintel_caida_enabled',
    'netintel_auto_probe_enabled'
);
