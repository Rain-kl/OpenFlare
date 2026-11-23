-- +goose Up
CREATE TABLE of_waf_request_whitelist (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain TEXT NOT NULL,
    path_match TEXT NOT NULL,
    path TEXT NOT NULL
);

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_waf_request_whitelist_insert AFTER INSERT ON of_waf_request_whitelist
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_waf_request_whitelist_update AFTER UPDATE ON of_waf_request_whitelist
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER dirty_edge_waf_request_whitelist_delete AFTER DELETE ON of_waf_request_whitelist
BEGIN
    UPDATE of_edge_snapshot_state SET source_revision = source_revision + 1 WHERE id = 1;
END;
-- +goose StatementEnd

-- +goose Down
DROP TABLE of_waf_request_whitelist;
