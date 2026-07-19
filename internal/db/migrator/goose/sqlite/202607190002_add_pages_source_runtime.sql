-- +goose Up
ALTER TABLE of_pages_projects
    ADD COLUMN content_config_version INTEGER NOT NULL DEFAULT 0;

ALTER TABLE of_pages_deployments
    ADD COLUMN source_type TEXT NOT NULL DEFAULT '';
ALTER TABLE of_pages_deployments
    ADD COLUMN source_identity TEXT;
ALTER TABLE of_pages_deployments
    ADD COLUMN source_revision TEXT;
ALTER TABLE of_pages_deployments
    ADD COLUMN source_label TEXT NOT NULL DEFAULT '';
ALTER TABLE of_pages_deployments
    ADD COLUMN source_meta TEXT NOT NULL DEFAULT '';
ALTER TABLE of_pages_deployments
    ADD COLUMN trigger_type TEXT NOT NULL DEFAULT '';

UPDATE of_pages_deployments
SET source_type = 'manual_upload',
    trigger_type = 'manual_upload';

CREATE TABLE IF NOT EXISTS of_pages_project_sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL,
    source_type TEXT NOT NULL DEFAULT '',
    remote_url TEXT NOT NULL DEFAULT '',
    allow_insecure INTEGER NOT NULL DEFAULT 0,
    github_repository TEXT NOT NULL DEFAULT '',
    release_selector TEXT NOT NULL DEFAULT '',
    release_tag TEXT NOT NULL DEFAULT '',
    asset_name TEXT NOT NULL DEFAULT '',
    auto_update_enabled INTEGER NOT NULL DEFAULT 0,
    check_interval_minutes INTEGER NOT NULL DEFAULT 0,
    config_version INTEGER NOT NULL DEFAULT 0,
    source_identity TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_of_pages_project_sources_project_id
    ON of_pages_project_sources (project_id);

CREATE TABLE IF NOT EXISTS of_pages_project_source_runtime (
    source_id INTEGER PRIMARY KEY,
    etag TEXT NOT NULL DEFAULT '',
    last_seen_revision TEXT NOT NULL DEFAULT '',
    last_seen_detail TEXT NOT NULL DEFAULT '',
    last_applied_revision TEXT NOT NULL DEFAULT '',
    last_applied_detail TEXT NOT NULL DEFAULT '',
    sync_status TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    last_checked_at DATETIME,
    last_synced_at DATETIME,
    next_check_at DATETIME,
    lease_expires_at DATETIME,
    lease_token TEXT NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
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

-- SQLite 的 Down 必须重建受影响表，完整移除新增列并保留原有数据与索引。
CREATE TABLE of_pages_deployments_before_source (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL,
    deployment_number INTEGER NOT NULL,
    checksum TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'uploaded',
    upload_id INTEGER NOT NULL DEFAULT 0,
    artifact_path TEXT NOT NULL,
    file_count INTEGER NOT NULL DEFAULT 0,
    total_size INTEGER NOT NULL DEFAULT 0,
    created_by TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    activated_at DATETIME
);

INSERT INTO of_pages_deployments_before_source (
    id, project_id, deployment_number, checksum, status, upload_id, artifact_path,
    file_count, total_size, created_by, created_at, activated_at
)
SELECT
    id, project_id, deployment_number, checksum, status, upload_id, artifact_path,
    file_count, total_size, created_by, created_at, activated_at
FROM of_pages_deployments;

DROP TABLE of_pages_deployments;
ALTER TABLE of_pages_deployments_before_source RENAME TO of_pages_deployments;

CREATE INDEX IF NOT EXISTS idx_of_pages_deployments_project_id
    ON of_pages_deployments (project_id);
CREATE INDEX IF NOT EXISTS idx_of_pages_deployments_checksum
    ON of_pages_deployments (checksum);
CREATE INDEX IF NOT EXISTS idx_of_pages_deployments_status
    ON of_pages_deployments (status);
CREATE INDEX IF NOT EXISTS idx_of_pages_deployments_upload_id
    ON of_pages_deployments (upload_id);

CREATE TABLE of_pages_projects_before_source (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    spa_fallback_enabled INTEGER NOT NULL DEFAULT 0,
    spa_fallback_path TEXT NOT NULL DEFAULT '/index.html',
    api_proxy_enabled INTEGER NOT NULL DEFAULT 0,
    api_proxy_path TEXT NOT NULL DEFAULT '',
    api_proxy_pass TEXT NOT NULL DEFAULT '',
    api_proxy_rewrite TEXT NOT NULL DEFAULT '',
    active_deployment_id INTEGER,
    root_dir TEXT NOT NULL DEFAULT '',
    entry_file TEXT NOT NULL DEFAULT 'index.html',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO of_pages_projects_before_source (
    id, name, slug, description, enabled, spa_fallback_enabled, spa_fallback_path,
    api_proxy_enabled, api_proxy_path, api_proxy_pass, api_proxy_rewrite,
    active_deployment_id, root_dir, entry_file, created_at, updated_at
)
SELECT
    id, name, slug, description, enabled, spa_fallback_enabled, spa_fallback_path,
    api_proxy_enabled, api_proxy_path, api_proxy_pass, api_proxy_rewrite,
    active_deployment_id, root_dir, entry_file, created_at, updated_at
FROM of_pages_projects;

DROP TABLE of_pages_projects;
ALTER TABLE of_pages_projects_before_source RENAME TO of_pages_projects;

CREATE UNIQUE INDEX IF NOT EXISTS idx_of_pages_projects_slug
    ON of_pages_projects (slug);
CREATE INDEX IF NOT EXISTS idx_of_pages_projects_active_deployment_id
    ON of_pages_projects (active_deployment_id);
