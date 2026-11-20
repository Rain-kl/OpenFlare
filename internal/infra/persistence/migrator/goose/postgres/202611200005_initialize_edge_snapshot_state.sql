-- +goose Up
INSERT INTO of_edge_snapshot_state (id, source_revision) VALUES (1, 1);

-- +goose Down
DELETE FROM of_edge_snapshot_state WHERE id = 1;
