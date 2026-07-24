-- +goose Up
ALTER TABLE of_pages_projects
    ADD COLUMN IF NOT EXISTS content_config_version INTEGER NOT NULL DEFAULT 0;

ALTER TABLE of_pages_deployments
    ADD COLUMN IF NOT EXISTS source_type VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_identity CHAR(64),
    ADD COLUMN IF NOT EXISTS source_revision CHAR(64),
    ADD COLUMN IF NOT EXISTS source_label VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_meta TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS trigger_type VARCHAR(32) NOT NULL DEFAULT '';

UPDATE of_pages_deployments
SET source_type = 'manual_upload',
    trigger_type = 'manual_upload';

CREATE TABLE IF NOT EXISTS of_pages_project_sources (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL,
    source_type VARCHAR(32) NOT NULL DEFAULT '',
    remote_url TEXT NOT NULL DEFAULT '',
    allow_insecure BOOLEAN NOT NULL DEFAULT FALSE,
    github_repository VARCHAR(255) NOT NULL DEFAULT '',
    release_selector VARCHAR(16) NOT NULL DEFAULT '',
    release_tag VARCHAR(255) NOT NULL DEFAULT '',
    asset_name VARCHAR(255) NOT NULL DEFAULT '',
    auto_update_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    check_interval_minutes INTEGER NOT NULL DEFAULT 0,
    config_version INTEGER NOT NULL DEFAULT 0,
    source_identity CHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_of_pages_project_sources_project_id
    ON of_pages_project_sources (project_id);

CREATE TABLE IF NOT EXISTS of_pages_project_source_runtime (
    source_id BIGINT PRIMARY KEY,
    etag VARCHAR(512) NOT NULL DEFAULT '',
    last_seen_revision CHAR(64) NOT NULL DEFAULT '',
    last_seen_detail TEXT NOT NULL DEFAULT '',
    last_applied_revision CHAR(64) NOT NULL DEFAULT '',
    last_applied_detail TEXT NOT NULL DEFAULT '',
    sync_status VARCHAR(32) NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    last_checked_at TIMESTAMPTZ,
    last_synced_at TIMESTAMPTZ,
    next_check_at TIMESTAMPTZ,
    lease_expires_at TIMESTAMPTZ,
    lease_token VARCHAR(64) NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_of_pages_project_source_runtime_next_check_at
    ON of_pages_project_source_runtime (next_check_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_of_pages_deployments_project_number
    ON of_pages_deployments (project_id, deployment_number);

CREATE UNIQUE INDEX IF NOT EXISTS idx_of_pages_deployments_source_revision
    ON of_pages_deployments (project_id, source_identity, source_revision)
    WHERE source_identity IS NOT NULL AND source_revision IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_of_pages_deployments_source_revision;
DROP INDEX IF EXISTS idx_of_pages_deployments_project_number;
DROP INDEX IF EXISTS idx_of_pages_project_source_runtime_next_check_at;
DROP TABLE IF EXISTS of_pages_project_source_runtime;
DROP INDEX IF EXISTS idx_of_pages_project_sources_project_id;
DROP TABLE IF EXISTS of_pages_project_sources;

ALTER TABLE of_pages_deployments
    DROP COLUMN IF EXISTS trigger_type,
    DROP COLUMN IF EXISTS source_meta,
    DROP COLUMN IF EXISTS source_label,
    DROP COLUMN IF EXISTS source_revision,
    DROP COLUMN IF EXISTS source_identity,
    DROP COLUMN IF EXISTS source_type;

ALTER TABLE of_pages_projects
    DROP COLUMN IF EXISTS content_config_version;
