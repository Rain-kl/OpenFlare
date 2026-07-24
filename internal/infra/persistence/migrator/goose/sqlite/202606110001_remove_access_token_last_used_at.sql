-- +goose Up
-- +goose StatementBegin
ALTER TABLE access_tokens DROP COLUMN last_used_at;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE access_tokens ADD COLUMN last_used_at DATETIME;
-- +goose StatementEnd
