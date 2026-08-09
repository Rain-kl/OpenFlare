-- +goose Up
-- 日志列表默认排序与 retention 清理的时间范围条件：补 logged_at 前导索引。
CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_logged_at ON of_node_access_logs (logged_at DESC, id DESC);
-- hosts 过滤为 lower(trim(host)) IN：补表达式索引，普通 (host,...) 索引无法命中函数表达式。
CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_host_lower ON of_node_access_logs (lower(trim(host)));

-- +goose Down
DROP INDEX IF EXISTS idx_of_node_access_logs_host_lower;
DROP INDEX IF EXISTS idx_of_node_access_logs_logged_at;
