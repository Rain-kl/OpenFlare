-- +goose Up
CREATE TABLE of_attack_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(100) NOT NULL DEFAULT '',
    levels TEXT NOT NULL DEFAULT '{}',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_attack_groups_insert AFTER INSERT ON of_attack_groups
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_attack_groups_update AFTER UPDATE ON of_attack_groups
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose Down
DROP TABLE of_attack_groups;
