-- +goose Up
CREATE TABLE IF NOT EXISTS of_config_versions (
    id BIGSERIAL PRIMARY KEY,
    version VARCHAR(32) NOT NULL,
    snapshot_json TEXT NOT NULL,
    main_config TEXT NOT NULL DEFAULT '',
    rendered_config TEXT NOT NULL,
    support_files_json TEXT NOT NULL DEFAULT '[]',
    checksum VARCHAR(64) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    created_by VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_config_versions_version ON of_config_versions (version);
CREATE INDEX IF NOT EXISTS idx_of_config_versions_is_active ON of_config_versions (is_active);

-- +goose Down
DROP TABLE IF EXISTS of_config_versions;