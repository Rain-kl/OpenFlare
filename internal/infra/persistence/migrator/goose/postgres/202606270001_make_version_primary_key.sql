-- +goose Up
-- Remove old PRIMARY KEY on id, set version as PRIMARY KEY, drop id column

-- 1. 移除旧主键约束
ALTER TABLE of_config_versions DROP CONSTRAINT IF EXISTS of_config_versions_pkey;

-- 2. 移除原先在 version 字段的唯一索引（因为 version 变成主键，主键隐含唯一性）
DROP INDEX IF EXISTS idx_of_config_versions_version;

-- 3. 将 version 设为新主键
ALTER TABLE of_config_versions ADD PRIMARY KEY (version);

-- 4. 彻底删除 id 列
ALTER TABLE of_config_versions DROP COLUMN IF EXISTS id;

-- +goose Down
ALTER TABLE of_config_versions DROP CONSTRAINT IF EXISTS of_config_versions_pkey;
ALTER TABLE of_config_versions ADD COLUMN IF NOT EXISTS id BIGSERIAL PRIMARY KEY;
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_config_versions_version ON of_config_versions (version);
