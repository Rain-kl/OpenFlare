-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id BIGINT PRIMARY KEY,
    username VARCHAR(64) UNIQUE,
    password VARCHAR(255),
    nickname VARCHAR(255),
    email VARCHAR(255),
    avatar_url VARCHAR(255),
    is_active BOOLEAN DEFAULT TRUE,
    is_admin BOOLEAN DEFAULT FALSE,
    bio VARCHAR(500),
    phone VARCHAR(32),
    gender VARCHAR(16),
    website VARCHAR(255),
    location VARCHAR(255),
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);
CREATE INDEX IF NOT EXISTS idx_users_is_active ON users (is_active);
CREATE INDEX IF NOT EXISTS idx_users_last_login_at ON users (last_login_at);
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users (created_at);

CREATE TABLE IF NOT EXISTS auth_sources (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(80) NOT NULL UNIQUE,
    type VARCHAR(20) NOT NULL,
    display_name VARCHAR(100),
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    client_id VARCHAR(255),
    client_secret VARCHAR(1024),
    openid_discovery_url VARCHAR(1024),
    scopes VARCHAR(255),
    icon_url VARCHAR(1024),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_auth_sources_is_active ON auth_sources (is_active);

CREATE TABLE IF NOT EXISTS external_accounts (
    id BIGSERIAL PRIMARY KEY,
    auth_source_id BIGINT,
    user_id BIGINT NOT NULL,
    external_id VARCHAR(255) NOT NULL,
    external_username VARCHAR(255),
    email VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_external_accounts_auth_source_id ON external_accounts (auth_source_id);
CREATE INDEX IF NOT EXISTS idx_external_accounts_user_id ON external_accounts (user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_external_accounts_source_external ON external_accounts (auth_source_id, external_id);

CREATE TABLE IF NOT EXISTS system_configs (
    key VARCHAR(64) PRIMARY KEY,
    value TEXT NOT NULL,
    type VARCHAR(32) NOT NULL DEFAULT 'system',
    visibility INTEGER NOT NULL DEFAULT 0,
    description VARCHAR(255),
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS uploads (
    id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    file_size BIGINT NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    extension VARCHAR(50) NOT NULL,
    hash VARCHAR(64),
    storage_driver VARCHAR(50) NOT NULL,
    type VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_uploads_user_id ON uploads (user_id);
CREATE INDEX IF NOT EXISTS idx_uploads_file_path ON uploads (file_path);
CREATE INDEX IF NOT EXISTS idx_uploads_hash ON uploads (hash);
CREATE INDEX IF NOT EXISTS idx_uploads_type ON uploads (type);

CREATE TABLE IF NOT EXISTS access_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    name VARCHAR(128) NOT NULL,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    masked_token VARCHAR(64) NOT NULL,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_access_tokens_user_id ON access_tokens (user_id);

CREATE TABLE IF NOT EXISTS task_executions (
    id BIGINT PRIMARY KEY,
    task_id VARCHAR(128) NOT NULL UNIQUE,
    task_type VARCHAR(64) NOT NULL,
    task_name VARCHAR(128),
    status VARCHAR(32) NOT NULL,
    retryable BOOLEAN NOT NULL DEFAULT FALSE,
    max_retry INTEGER NOT NULL DEFAULT 0,
    retry_count INTEGER NOT NULL DEFAULT 0,
    log TEXT,
    error_message TEXT,
    result TEXT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    duration BIGINT,
    payload TEXT,
    triggered_by VARCHAR(32) NOT NULL DEFAULT 'system',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_task_executions_task_type ON task_executions (task_type);
CREATE INDEX IF NOT EXISTS idx_task_executions_status ON task_executions (status);
CREATE INDEX IF NOT EXISTS idx_task_executions_started_at ON task_executions (started_at);
CREATE INDEX IF NOT EXISTS idx_task_executions_created_at ON task_executions (created_at);

CREATE TABLE IF NOT EXISTS templates (
    id BIGSERIAL PRIMARY KEY,
    key VARCHAR(80) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(20) NOT NULL DEFAULT 'email',
    subject VARCHAR(255),
    content TEXT NOT NULL,
    description VARCHAR(255),
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_templates_is_system ON templates (is_system);
CREATE INDEX IF NOT EXISTS idx_templates_created_at ON templates (created_at);
CREATE INDEX IF NOT EXISTS idx_templates_updated_at ON templates (updated_at);

INSERT INTO system_configs (key, value, type, visibility, description, created_at, updated_at) VALUES
    ('cap_login_enabled', 'true', 'system', 1, '是否启用登录人机验证（true/false）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('cap_auto_solve', 'true', 'system', 1, '打开页面后是否自动开始计算，关闭则需用户手动点击触发', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('cap_challenge_count', '1', 'system', 0, '客户端需求解的 PoW 难题总数，默认 1，推荐 1～5', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('cap_challenge_size', '32', 'system', 0, '人机验证盐值长度', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('cap_challenge_difficulty', '4', 'system', 0, '人机验证 PoW 难度（目标前缀长度）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('cap_challenge_ttl_seconds', '600', 'system', 0, '人机验证难题有效时间（秒）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('cap_token_ttl_seconds', '1200', 'system', 0, '人机验证兑换凭证有效时间（秒）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('server_address', '', 'system', 0, '服务器地址（用于跨域源控制，不设定则允许任意源）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('smtp_host', '', 'system', 0, 'SMTP 服务器地址（例如 smtp.example.com）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('smtp_port', '587', 'system', 0, 'SMTP 端口（例如 587 或 465）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('smtp_username', '', 'system', 0, 'SMTP 账户（如 sender@example.com）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('smtp_password', '', 'system', 0, 'SMTP 访问凭证（授权码/密码）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('upload_allowed_extensions', 'jpg,png,webp', 'system', 1, '允许上传的图片扩展名（逗号分隔）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('site_name', 'OpenFlare', 'system', 1, '系统平台的展示名称', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('password_login_enabled', 'true', 'system', 1, '是否允许使用账号密码登录', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('registration_enabled', 'false', 'system', 1, '控制普通用户是否可以自主注册（true/false）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('password_register_enabled', 'false', 'system', 1, '是否允许通过密码创建本地账号', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('oidc_login_enabled', 'true', 'system', 1, '是否允许使用第三方 OIDC 认证源登录', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('max_api_keys_per_user', '5', 'business', 1, '限制每个普通用户可以创建的 API Key 最大数量', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('email_login_verification_enabled', 'false', 'system', 1, '是否开启邮箱登录验证（true/false）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('email_register_verification_enabled', 'false', 'system', 1, '是否开启邮箱注册验证（true/false）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('menu_display_config', '{}', 'system', 1, '目录显示配置（JSON 字符串，格式为 {url: enabled}）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('search_engine_indexing_enabled', 'false', 'system', 1, '是否允许搜索引擎爬取/检索该站点（true/false）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('update_upstream_repository', 'Rain-kl/OpenFlare', 'system', 0, 'GitHub Actions Release 上游仓库（owner/repo 或 GitHub 仓库地址）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('storage_config', '{"driver":"local","local":{"root":"."},"s3":{"region":"us-east-1"},"r2":{"region":"auto"},"minio":{"region":"us-east-1","path_style":true},"oss":{},"webdav":{}}', 'system', 0, '文件存储驱动及连接配置（JSON）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

INSERT INTO users (id, username, password, nickname, avatar_url, is_active, is_admin, last_login_at, created_at, updated_at)
VALUES (1, 'admin', '12345678', 'Administrator', '', TRUE, TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (username) DO NOTHING;

INSERT INTO templates (key, name, type, subject, content, description, is_system, created_at, updated_at) VALUES
    ('login_email', '登录验证码邮件', 'email', 'OpenFlare 登录验证码', '<h3>OpenFlare 登录验证</h3><p>您的登录验证码为：<strong>{{.Code}}</strong>，5分钟内有效，请勿将验证码泄露给他人。</p>', '用户密码登录时发送的验证码邮件模板，支持变量：{{.Code}}', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('register_email', '注册验证码邮件', 'email', 'OpenFlare 注册验证码', '<h3>OpenFlare 注册验证</h3><p>您的注册验证码为：<strong>{{.Code}}</strong>，5分钟内有效，请勿泄露给他人。</p>', '用户注册时发送的验证码邮件模板，支持变量：{{.Code}}', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS templates;
DROP TABLE IF EXISTS task_executions;
DROP TABLE IF EXISTS access_tokens;
DROP TABLE IF EXISTS uploads;
DROP TABLE IF EXISTS system_configs;
DROP TABLE IF EXISTS external_accounts;
DROP TABLE IF EXISTS auth_sources;
DROP TABLE IF EXISTS users;
