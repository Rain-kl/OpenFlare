-- +goose Up
DROP TABLE IF EXISTS of_node_edge_health;

-- +goose Down
SELECT 1;
