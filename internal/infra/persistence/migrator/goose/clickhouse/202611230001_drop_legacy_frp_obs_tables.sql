-- +goose Up
DROP TABLE IF EXISTS of_node_obs_frps;
DROP TABLE IF EXISTS of_node_obs_frpc;

-- +goose Down
SELECT 1;
