-- +goose Up
DELETE FROM of_options WHERE key = 'Notice';

-- +goose Down
INSERT OR IGNORE INTO of_options (key, value) VALUES ('Notice', '');
