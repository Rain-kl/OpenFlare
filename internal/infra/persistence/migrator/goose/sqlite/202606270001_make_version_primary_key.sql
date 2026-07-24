-- +goose Up
-- SQLite table reconstruction to set version as primary key and drop id column

-- 1. 创建临时新表
CREATE TABLE of_config_versions_new (
    version VARCHAR(32) PRIMARY KEY,
    snapshot_json TEXT NOT NULL,
    main_config TEXT NOT NULL DEFAULT '',
    rendered_config TEXT NOT NULL,
    support_files_json TEXT NOT NULL DEFAULT '[]',
    checksum VARCHAR(64) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    created_by VARCHAR(64) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 2. 拷贝数据
INSERT INTO of_config_versions_new (version, snapshot_json, main_config, rendered_config, support_files_json, checksum, is_active, created_by, created_at)
SELECT version, snapshot_json, main_config, rendered_config, support_files_json, checksum, is_active, created_by, created_at
FROM of_config_versions;

-- 3. 删除旧表
DROP TABLE of_config_versions;

-- 4. 重命名新表
ALTER TABLE of_config_versions_new RENAME TO of_config_versions;

-- 5. 重建索引（is_active 索引）
CREATE INDEX IF NOT EXISTS idx_of_config_versions_is_active ON of_config_versions (is_active);

-- +goose Down
DROP TABLE IF EXISTS of_config_versions;
CREATE TABLE of_config_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    version VARCHAR(32) NOT NULL,
    snapshot_json TEXT NOT NULL,
    main_config TEXT NOT NULL DEFAULT '',
    rendered_config TEXT NOT NULL,
    support_files_json TEXT NOT NULL DEFAULT '[]',
    checksum VARCHAR(64) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    created_by VARCHAR(64) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_config_versions_version ON of_config_versions (version);
CREATE INDEX IF NOT EXISTS idx_of_config_versions_is_active ON of_config_versions (is_active);
