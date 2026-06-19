-- +goose Up
CREATE TABLE IF NOT EXISTS of_pages_projects (
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
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_pages_projects_slug ON of_pages_projects (slug);
CREATE INDEX IF NOT EXISTS idx_of_pages_projects_active_deployment_id ON of_pages_projects (active_deployment_id);

CREATE TABLE IF NOT EXISTS of_pages_deployments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL,
    deployment_number INTEGER NOT NULL,
    checksum TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'uploaded',
    artifact_path TEXT NOT NULL,
    file_count INTEGER NOT NULL DEFAULT 0,
    total_size INTEGER NOT NULL DEFAULT 0,
    created_by TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    activated_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_of_pages_deployments_project_id ON of_pages_deployments (project_id);
CREATE INDEX IF NOT EXISTS idx_of_pages_deployments_checksum ON of_pages_deployments (checksum);
CREATE INDEX IF NOT EXISTS idx_of_pages_deployments_status ON of_pages_deployments (status);

CREATE TABLE IF NOT EXISTS of_pages_deployment_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    deployment_id INTEGER NOT NULL,
    path TEXT NOT NULL,
    size INTEGER NOT NULL DEFAULT 0,
    checksum TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_pages_deployment_files_deployment_id ON of_pages_deployment_files (deployment_id);

-- +goose Down
DROP TABLE IF EXISTS of_pages_deployment_files;
DROP TABLE IF EXISTS of_pages_deployments;
DROP TABLE IF EXISTS of_pages_projects;