-- +goose Up
-- Marker: Zone 域名从旧路由列 / of_managed_domains 的导入由 migrator.Migrate()
-- 在全部 goose SQL 应用后自动执行（publicsuffix 根域解析无法用纯 SQL 正确完成）。
-- 本文件仅占位版本号，保证升级链路有序：建表 → 导入 → 删旧列。
SELECT 1;

-- +goose Down
SELECT 1;
