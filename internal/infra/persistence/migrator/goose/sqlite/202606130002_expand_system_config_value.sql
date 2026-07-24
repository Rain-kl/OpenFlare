-- +goose Up
-- SQLite stores VARCHAR and TEXT with the same TEXT affinity.
SELECT 1;

-- +goose Down
SELECT 1;
