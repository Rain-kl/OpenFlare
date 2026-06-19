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

-- Rename indexes for consistency
ALTER INDEX IF EXISTS idx_users_email RENAME TO idx_w_users_email;
ALTER INDEX IF EXISTS idx_users_is_active RENAME TO idx_w_users_is_active;
ALTER INDEX IF EXISTS idx_users_last_login_at RENAME TO idx_w_users_last_login_at;
ALTER INDEX IF EXISTS idx_users_created_at RENAME TO idx_w_users_created_at;
ALTER INDEX IF EXISTS idx_auth_sources_is_active RENAME TO idx_w_auth_sources_is_active;
ALTER INDEX IF EXISTS idx_external_accounts_auth_source_id RENAME TO idx_w_external_accounts_auth_source_id;
ALTER INDEX IF EXISTS idx_external_accounts_user_id RENAME TO idx_w_external_accounts_user_id;
ALTER INDEX IF EXISTS idx_external_accounts_source_external RENAME TO idx_w_external_accounts_source_external;
ALTER INDEX IF EXISTS idx_uploads_user_id RENAME TO idx_w_uploads_user_id;
ALTER INDEX IF EXISTS idx_uploads_file_path RENAME TO idx_w_uploads_file_path;
ALTER INDEX IF EXISTS idx_uploads_hash RENAME TO idx_w_uploads_hash;
ALTER INDEX IF EXISTS idx_uploads_type RENAME TO idx_w_uploads_type;
ALTER INDEX IF EXISTS idx_access_tokens_user_id RENAME TO idx_w_access_tokens_user_id;
ALTER INDEX IF EXISTS idx_task_executions_task_type RENAME TO idx_w_task_executions_task_type;
ALTER INDEX IF EXISTS idx_task_executions_status RENAME TO idx_w_task_executions_status;
ALTER INDEX IF EXISTS idx_task_executions_started_at RENAME TO idx_w_task_executions_started_at;
ALTER INDEX IF EXISTS idx_task_executions_created_at RENAME TO idx_w_task_executions_created_at;
ALTER INDEX IF EXISTS idx_templates_is_system RENAME TO idx_w_templates_is_system;
ALTER INDEX IF EXISTS idx_templates_created_at RENAME TO idx_w_templates_created_at;
ALTER INDEX IF EXISTS idx_templates_updated_at RENAME TO idx_w_templates_updated_at;
ALTER INDEX IF EXISTS idx_schedules_is_active RENAME TO idx_w_schedules_is_active;

-- +goose Down
ALTER INDEX IF EXISTS idx_w_schedules_is_active RENAME TO idx_schedules_is_active;
ALTER INDEX IF EXISTS idx_w_templates_updated_at RENAME TO idx_templates_updated_at;
ALTER INDEX IF EXISTS idx_w_templates_created_at RENAME TO idx_templates_created_at;
ALTER INDEX IF EXISTS idx_w_templates_is_system RENAME TO idx_templates_is_system;
ALTER INDEX IF EXISTS idx_w_task_executions_created_at RENAME TO idx_task_executions_created_at;
ALTER INDEX IF EXISTS idx_w_task_executions_started_at RENAME TO idx_task_executions_started_at;
ALTER INDEX IF EXISTS idx_w_task_executions_status RENAME TO idx_task_executions_status;
ALTER INDEX IF EXISTS idx_w_task_executions_task_type RENAME TO idx_task_executions_task_type;
ALTER INDEX IF EXISTS idx_w_access_tokens_user_id RENAME TO idx_access_tokens_user_id;
ALTER INDEX IF EXISTS idx_w_uploads_type RENAME TO idx_uploads_type;
ALTER INDEX IF EXISTS idx_w_uploads_hash RENAME TO idx_uploads_hash;
ALTER INDEX IF EXISTS idx_w_uploads_file_path RENAME TO idx_uploads_file_path;
ALTER INDEX IF EXISTS idx_w_uploads_user_id RENAME TO idx_uploads_user_id;
ALTER INDEX IF EXISTS idx_w_external_accounts_source_external RENAME TO idx_external_accounts_source_external;
ALTER INDEX IF EXISTS idx_w_external_accounts_user_id RENAME TO idx_external_accounts_user_id;
ALTER INDEX IF EXISTS idx_w_external_accounts_auth_source_id RENAME TO idx_external_accounts_auth_source_id;
ALTER INDEX IF EXISTS idx_w_auth_sources_is_active RENAME TO idx_auth_sources_is_active;
ALTER INDEX IF EXISTS idx_w_users_created_at RENAME TO idx_users_created_at;
ALTER INDEX IF EXISTS idx_w_users_last_login_at RENAME TO idx_users_last_login_at;
ALTER INDEX IF EXISTS idx_w_users_is_active RENAME TO idx_users_is_active;
ALTER INDEX IF EXISTS idx_w_users_email RENAME TO idx_users_email;

ALTER TABLE w_schedules RENAME TO schedules;
ALTER TABLE w_templates RENAME TO templates;
ALTER TABLE w_task_executions RENAME TO task_executions;
ALTER TABLE w_access_tokens RENAME TO access_tokens;
ALTER TABLE w_uploads RENAME TO uploads;
ALTER TABLE w_system_configs RENAME TO system_configs;
ALTER TABLE w_external_accounts RENAME TO external_accounts;
ALTER TABLE w_auth_sources RENAME TO auth_sources;
ALTER TABLE w_users RENAME TO users;
