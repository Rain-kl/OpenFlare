-- +goose Up
ALTER TABLE users RENAME TO w_users;
ALTER TABLE auth_sources RENAME TO w_auth_sources;
ALTER TABLE external_accounts RENAME TO w_external_accounts;
ALTER TABLE system_configs RENAME TO w_system_configs;
ALTER TABLE uploads RENAME TO w_uploads;
ALTER TABLE access_tokens RENAME TO w_access_tokens;
ALTER TABLE task_executions RENAME TO w_task_executions;
ALTER TABLE templates RENAME TO w_templates;
ALTER TABLE schedules RENAME TO w_schedules;

-- Drop and Recreate SQLite indexes
DROP INDEX IF EXISTS idx_users_email;
CREATE INDEX IF NOT EXISTS idx_w_users_email ON w_users (email);
DROP INDEX IF EXISTS idx_users_is_active;
CREATE INDEX IF NOT EXISTS idx_w_users_is_active ON w_users (is_active);
DROP INDEX IF EXISTS idx_users_last_login_at;
CREATE INDEX IF NOT EXISTS idx_w_users_last_login_at ON w_users (last_login_at);
DROP INDEX IF EXISTS idx_users_created_at;
CREATE INDEX IF NOT EXISTS idx_w_users_created_at ON w_users (created_at);

DROP INDEX IF EXISTS idx_auth_sources_is_active;
CREATE INDEX IF NOT EXISTS idx_w_auth_sources_is_active ON w_auth_sources (is_active);

DROP INDEX IF EXISTS idx_external_accounts_auth_source_id;
CREATE INDEX IF NOT EXISTS idx_w_external_accounts_auth_source_id ON w_external_accounts (auth_source_id);
DROP INDEX IF EXISTS idx_external_accounts_user_id;
CREATE INDEX IF NOT EXISTS idx_w_external_accounts_user_id ON w_external_accounts (user_id);
DROP INDEX IF EXISTS idx_external_accounts_source_external;
CREATE UNIQUE INDEX IF NOT EXISTS idx_w_external_accounts_source_external ON w_external_accounts (auth_source_id, external_id);

DROP INDEX IF EXISTS idx_uploads_user_id;
CREATE INDEX IF NOT EXISTS idx_w_uploads_user_id ON w_uploads (user_id);
DROP INDEX IF EXISTS idx_uploads_file_path;
CREATE INDEX IF NOT EXISTS idx_w_uploads_file_path ON w_uploads (file_path);
DROP INDEX IF EXISTS idx_uploads_hash;
CREATE INDEX IF NOT EXISTS idx_w_uploads_hash ON w_uploads (hash);
DROP INDEX IF EXISTS idx_uploads_type;
CREATE INDEX IF NOT EXISTS idx_w_uploads_type ON w_uploads (type);

DROP INDEX IF EXISTS idx_access_tokens_user_id;
CREATE INDEX IF NOT EXISTS idx_w_access_tokens_user_id ON w_access_tokens (user_id);

DROP INDEX IF EXISTS idx_task_executions_task_type;
CREATE INDEX IF NOT EXISTS idx_w_task_executions_task_type ON w_task_executions (task_type);
DROP INDEX IF EXISTS idx_task_executions_status;
CREATE INDEX IF NOT EXISTS idx_w_task_executions_status ON w_task_executions (status);
DROP INDEX IF EXISTS idx_task_executions_started_at;
CREATE INDEX IF NOT EXISTS idx_w_task_executions_started_at ON w_task_executions (started_at);
DROP INDEX IF EXISTS idx_task_executions_created_at;
CREATE INDEX IF NOT EXISTS idx_w_task_executions_created_at ON w_task_executions (created_at);

DROP INDEX IF EXISTS idx_templates_is_system;
CREATE INDEX IF NOT EXISTS idx_w_templates_is_system ON w_templates (is_system);
DROP INDEX IF EXISTS idx_templates_created_at;
CREATE INDEX IF NOT EXISTS idx_w_templates_created_at ON w_templates (created_at);
DROP INDEX IF EXISTS idx_templates_updated_at;
CREATE INDEX IF NOT EXISTS idx_w_templates_updated_at ON w_templates (updated_at);

DROP INDEX IF EXISTS idx_schedules_is_active;
CREATE INDEX IF NOT EXISTS idx_w_schedules_is_active ON w_schedules (is_active);

-- +goose Down
ALTER TABLE w_schedules RENAME TO schedules;
ALTER TABLE w_templates RENAME TO templates;
ALTER TABLE w_task_executions RENAME TO task_executions;
ALTER TABLE w_access_tokens RENAME TO access_tokens;
ALTER TABLE w_uploads RENAME TO uploads;
ALTER TABLE w_system_configs RENAME TO system_configs;
ALTER TABLE w_external_accounts RENAME TO external_accounts;
ALTER TABLE w_auth_sources RENAME TO auth_sources;
ALTER TABLE w_users RENAME TO users;

-- Revert indexes
DROP INDEX IF EXISTS idx_w_users_email;
CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);
DROP INDEX IF EXISTS idx_w_users_is_active;
CREATE INDEX IF NOT EXISTS idx_users_is_active ON users (is_active);
DROP INDEX IF EXISTS idx_w_users_last_login_at;
CREATE INDEX IF NOT EXISTS idx_users_last_login_at ON users (last_login_at);
DROP INDEX IF EXISTS idx_w_users_created_at;
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users (created_at);

DROP INDEX IF EXISTS idx_w_auth_sources_is_active;
CREATE INDEX IF NOT EXISTS idx_auth_sources_is_active ON auth_sources (is_active);

DROP INDEX IF EXISTS idx_w_external_accounts_auth_source_id;
CREATE INDEX IF NOT EXISTS idx_external_accounts_auth_source_id ON external_accounts (auth_source_id);
DROP INDEX IF EXISTS idx_w_external_accounts_user_id;
CREATE INDEX IF NOT EXISTS idx_external_accounts_user_id ON external_accounts (user_id);
DROP INDEX IF EXISTS idx_w_external_accounts_source_external;
CREATE UNIQUE INDEX IF NOT EXISTS idx_external_accounts_source_external ON external_accounts (auth_source_id, external_id);

DROP INDEX IF EXISTS idx_w_uploads_user_id;
CREATE INDEX IF NOT EXISTS idx_uploads_user_id ON uploads (user_id);
DROP INDEX IF EXISTS idx_w_uploads_file_path;
CREATE INDEX IF NOT EXISTS idx_uploads_file_path ON uploads (file_path);
DROP INDEX IF EXISTS idx_w_uploads_hash;
CREATE INDEX IF NOT EXISTS idx_uploads_hash ON uploads (hash);
DROP INDEX IF EXISTS idx_w_uploads_type;
CREATE INDEX IF NOT EXISTS idx_uploads_type ON uploads (type);

DROP INDEX IF EXISTS idx_w_access_tokens_user_id;
CREATE INDEX IF NOT EXISTS idx_access_tokens_user_id ON access_tokens (user_id);

DROP INDEX IF EXISTS idx_w_task_executions_task_type;
CREATE INDEX IF NOT EXISTS idx_task_executions_task_type ON task_executions (task_type);
DROP INDEX IF EXISTS idx_w_task_executions_status;
CREATE INDEX IF NOT EXISTS idx_task_executions_status ON task_executions (status);
DROP INDEX IF EXISTS idx_w_task_executions_started_at;
CREATE INDEX IF NOT EXISTS idx_task_executions_started_at ON task_executions (started_at);
DROP INDEX IF EXISTS idx_w_task_executions_created_at;
CREATE INDEX IF NOT EXISTS idx_task_executions_created_at ON task_executions (created_at);

DROP INDEX IF EXISTS idx_w_templates_is_system;
CREATE INDEX IF NOT EXISTS idx_templates_is_system ON templates (is_system);
DROP INDEX IF EXISTS idx_w_templates_created_at;
CREATE INDEX IF NOT EXISTS idx_templates_created_at ON templates (created_at);
DROP INDEX IF EXISTS idx_w_templates_updated_at;
CREATE INDEX IF NOT EXISTS idx_templates_updated_at ON templates (updated_at);

DROP INDEX IF EXISTS idx_w_schedules_is_active;
CREATE INDEX IF NOT EXISTS idx_schedules_is_active ON schedules (is_active);
