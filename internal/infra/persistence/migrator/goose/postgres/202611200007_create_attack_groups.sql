-- +goose Up
CREATE TABLE of_attack_groups (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL DEFAULT '',
    levels TEXT NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER dirty_edge_attack_groups AFTER INSERT OR UPDATE ON of_attack_groups
FOR EACH STATEMENT EXECUTE FUNCTION openflare_dirty_edge_snapshot();

-- +goose Down
DROP TABLE of_attack_groups;
