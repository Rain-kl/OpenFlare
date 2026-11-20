-- +goose Up
CREATE TABLE of_performance_policies (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    preset_key VARCHAR(16) UNIQUE,
    settings_json TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
ALTER TABLE of_nodes ADD COLUMN performance_policy_id BIGINT;
CREATE INDEX idx_of_nodes_performance_policy_id ON of_nodes (performance_policy_id);

-- +goose Down
DROP INDEX idx_of_nodes_performance_policy_id;
ALTER TABLE of_nodes DROP COLUMN performance_policy_id;
DROP TABLE of_performance_policies;
