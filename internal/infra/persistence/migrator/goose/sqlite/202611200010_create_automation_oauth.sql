-- +goose Up
CREATE TABLE w_automation_codes (
    hash TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL,
    client_id TEXT NOT NULL,
    redirect_uri TEXT NOT NULL,
    resource TEXT NOT NULL,
    scope TEXT NOT NULL,
    challenge TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    used BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX idx_automation_codes_expires ON w_automation_codes (expires_at);
CREATE INDEX idx_automation_codes_user ON w_automation_codes (user_id);
CREATE TABLE w_automation_grants (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL,
    client_id TEXT NOT NULL,
    resource TEXT NOT NULL,
    scope TEXT NOT NULL,
    access_hash TEXT NOT NULL UNIQUE,
    refresh_hash TEXT NOT NULL UNIQUE,
    access_expires_at DATETIME NOT NULL,
    refresh_expires_at DATETIME NOT NULL
);
CREATE INDEX idx_automation_grants_expires ON w_automation_grants (refresh_expires_at);
CREATE INDEX idx_automation_grants_user ON w_automation_grants (user_id);

-- +goose Down
DROP TABLE w_automation_grants;
DROP TABLE w_automation_codes;
