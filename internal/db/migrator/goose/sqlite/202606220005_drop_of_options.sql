-- +goose Up
-- of_options 配置已于 202606220004 迁移至 w_system_configs，删除遗留表。
DROP TABLE IF EXISTS of_options;

-- +goose Down
-- 回滚时重建空表结构（数据无法恢复，迁移来源已为 w_system_configs）。
CREATE TABLE IF NOT EXISTS of_options (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);
