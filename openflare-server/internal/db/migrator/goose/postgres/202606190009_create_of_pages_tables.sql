-- +goose Up
CREATE TABLE IF NOT EXISTS of_pages_projects (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(128) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    spa_fallback_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    spa_fallback_path VARCHAR(512) NOT NULL DEFAULT '/index.html',
    api_proxy_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    api_proxy_path VARCHAR(255) NOT NULL DEFAULT '',
    api_proxy_pass VARCHAR(2048) NOT NULL DEFAULT '',
    api_proxy_rewrite VARCHAR(255) NOT NULL DEFAULT '',
    active_deployment_id BIGINT,
    root_dir VARCHAR(512) NOT NULL DEFAULT '',
    entry_file VARCHAR(512) NOT NULL DEFAULT 'index.html',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_pages_projects_slug ON of_pages_projects (slug);
CREATE INDEX IF NOT EXISTS idx_of_pages_projects_active_deployment_id ON of_pages_projects (active_deployment_id);

CREATE TABLE IF NOT EXISTS of_pages_deployments (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL,
    deployment_number INTEGER NOT NULL,
    checksum VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'uploaded',
    artifact_path VARCHAR(2048) NOT NULL,
    file_count INTEGER NOT NULL DEFAULT 0,
    total_size BIGINT NOT NULL DEFAULT 0,
    created_by VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    activated_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_of_pages_deployments_project_id ON of_pages_deployments (project_id);
CREATE INDEX IF NOT EXISTS idx_of_pages_deployments_checksum ON of_pages_deployments (checksum);
CREATE INDEX IF NOT EXISTS idx_of_pages_deployments_status ON of_pages_deployments (status);

CREATE TABLE IF NOT EXISTS of_pages_deployment_files (
    id BIGSERIAL PRIMARY KEY,
    deployment_id BIGINT NOT NULL,
    path VARCHAR(2048) NOT NULL,
    size BIGINT NOT NULL DEFAULT 0,
    checksum VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_pages_deployment_files_deployment_id ON of_pages_deployment_files (deployment_id);

-- +goose Down
DROP TABLE IF EXISTS of_pages_deployment_files;
DROP TABLE IF EXISTS of_pages_deployments;
DROP TABLE IF EXISTS of_pages_projects;