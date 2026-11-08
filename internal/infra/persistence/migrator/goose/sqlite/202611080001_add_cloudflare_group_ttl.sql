-- +goose Up
-- 零值由业务层解释为旧分组的 300 秒 TTL；新建分组显式保存所选值。
ALTER TABLE of_cf_pointing_groups ADD COLUMN ttl INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE of_cf_pointing_groups DROP COLUMN ttl;
