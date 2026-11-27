-- +goose Up
INSERT INTO of_node_tags (name, slug, color, description, default_weight, created_at, updated_at)
VALUES
    ('优化线路', 'optimized', 'emerald', 'CN2 GIA、AS9929 等低抖动优质跨境或骨干优化线路节点', 1.2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('国际线路', 'international', 'blue', '通用国际 BGP 互联或海外多线接入节点', 1.0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('大流量线路', 'high-bandwidth', 'amber', '高带宽与高并发容量节点，适合大流量分发与负载均衡', 1.1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (slug) DO NOTHING;

INSERT INTO of_dns_policies (
    name, description, scope_type, scope_domains_json, strategy_type,
    priority, ttl, max_answers, enabled, is_default,
    fallback_strategy, fallback_node_ids_json, config_json, jdm_json, revision,
    created_at, updated_at
)
SELECT
    '智能分配策略',
    '结合实测网络延迟（55%）、节点综合负载（30%）、地理距离（15%）与节点标签权重自动选择最优边缘节点',
    'all',
    '[]',
    'smart',
    100,
    60,
    1,
    TRUE,
    TRUE,
    'load_balance',
    '[]',
    '{"mode":"smart","weights":{"latency":0.55,"load":0.3,"geo":0.15},"use_tag_weights":true,"filters":{"node_ids":[],"countries":[],"tags":[],"network_groups":[]}}',
    '{"nodes":[{"id":"input","name":"Request","type":"inputNode"},{"id":"expr-default","name":"Smart Allocation","type":"expressionNode","content":{"expressions":[{"id":"mode","key":"mode","value":"\"smart\""},{"id":"w_lat","key":"weights.latency","value":"0.55"},{"id":"w_load","key":"weights.load","value":"0.3"},{"id":"w_geo","key":"weights.geo","value":"0.15"},{"id":"use_tags","key":"use_tag_weights","value":"true"}]}},{"id":"output","name":"Decision","type":"outputNode"}],"edges":[{"id":"e-in","sourceId":"input","targetId":"expr-default"},{"id":"e-out","sourceId":"expr-default","targetId":"output"}]}',
    1,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
WHERE NOT EXISTS (
    SELECT 1 FROM of_dns_policies WHERE name = '智能分配策略'
);

INSERT INTO of_dns_policies (
    name, description, scope_type, scope_domains_json, strategy_type,
    priority, ttl, max_answers, enabled, is_default,
    fallback_strategy, fallback_node_ids_json, config_json, jdm_json, revision,
    created_at, updated_at
)
SELECT
    '容量负载均衡策略',
    '依据全量健康边缘节点的容量权重、实时负载余量与标签权重进行均衡调度',
    'all',
    '[]',
    'load_balance',
    200,
    60,
    1,
    TRUE,
    FALSE,
    'all_healthy',
    '[]',
    '{"mode":"load_balance","weights":{"latency":0,"load":1,"geo":0},"use_tag_weights":true,"filters":{"node_ids":[],"countries":[],"tags":[],"network_groups":[]}}',
    '{"nodes":[{"id":"input","name":"Request","type":"inputNode"},{"id":"expr-default","name":"Capacity Load Balance","type":"expressionNode","content":{"expressions":[{"id":"mode","key":"mode","value":"\"load_balance\""},{"id":"use_tags","key":"use_tag_weights","value":"true"}]}},{"id":"output","name":"Decision","type":"outputNode"}],"edges":[{"id":"e-in","sourceId":"input","targetId":"expr-default"},{"id":"e-out","sourceId":"expr-default","targetId":"output"}]}',
    1,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
WHERE NOT EXISTS (
    SELECT 1 FROM of_dns_policies WHERE name = '容量负载均衡策略'
);

INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES
    ('smartdns_enabled', 'true', 'system', 0, '是否启用 SmartDNS 权威智能解析服务', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('smartdns_delegation_domain', '', 'system', 0, 'SmartDNS 全局委派主域名（如 edge.example.com）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('smartdns_ns_hosts', '[]', 'system', 0, 'SmartDNS 权威 NS 主机名列表（JSON 数组）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('smartdns_default_ttl', '60', 'system', 0, 'SmartDNS 默认解析应答 TTL（秒）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('smartdns_discovery_token', '', 'system', 0, '独立 SmartDNS 实例自动发现注册令牌', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('smartdns_snapshot_interval_seconds', '10', 'system', 0, 'SmartDNS 不可变运行时快照刷新间隔（秒）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('smartdns_update_repo', 'Rain-kl/OpenFlare', 'system', 0, '独立 SmartDNS 实例发布版本查询仓库', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs WHERE key IN (
    'smartdns_enabled',
    'smartdns_delegation_domain',
    'smartdns_ns_hosts',
    'smartdns_default_ttl',
    'smartdns_discovery_token',
    'smartdns_snapshot_interval_seconds',
    'smartdns_update_repo'
);
DELETE FROM of_dns_policies WHERE name IN ('智能分配策略', '容量负载均衡策略');
DELETE FROM of_node_tags WHERE slug IN ('optimized', 'international', 'high-bandwidth');
