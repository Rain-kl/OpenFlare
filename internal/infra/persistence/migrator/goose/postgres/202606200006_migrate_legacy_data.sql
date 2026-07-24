-- +goose Up
-- Historical copy from legacy_* tables into of_* / w_* (formerly Go migration).
-- Already-applied environments keep their data; new installs have no legacy_* sources.
SELECT 1;

-- +goose Down
SELECT 1;
