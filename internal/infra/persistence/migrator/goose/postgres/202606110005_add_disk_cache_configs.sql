-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES 
  ('disk_cache_max_size_mb', '100', 'system', 0, '磁盘缓存最大空间大小 (MB)', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('disk_cache_ttl_minutes', '60', 'system', 0, '磁盘缓存默认有效期 (分钟)', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('disk_cache_lru_enabled', 'true', 'system', 0, '是否启用 LRU 淘汰机制', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs WHERE key IN ('disk_cache_max_size_mb', 'disk_cache_ttl_minutes', 'disk_cache_lru_enabled');
