-- +goose Up
CREATE TABLE w_automation_sessions (
    kind TEXT NOT NULL,
    signature TEXT NOT NULL,
    request_id TEXT NOT NULL,
    user_id BIGINT NOT NULL,
    data TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (kind, signature)
);
CREATE INDEX idx_automation_sessions_request ON w_automation_sessions (request_id, kind);
CREATE UNIQUE INDEX idx_automation_sessions_code_request ON w_automation_sessions (request_id) WHERE kind = 'code';
CREATE INDEX idx_automation_sessions_user ON w_automation_sessions (user_id);
CREATE INDEX idx_automation_sessions_expires ON w_automation_sessions (expires_at);

-- +goose Down
DROP TABLE w_automation_sessions;
