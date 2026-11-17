-- +goose Up
CREATE TABLE of_pow_policies (
    route_id BIGINT PRIMARY KEY,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    config TEXT NOT NULL DEFAULT '{}'
);

-- +goose Down
DROP TABLE of_pow_policies;
