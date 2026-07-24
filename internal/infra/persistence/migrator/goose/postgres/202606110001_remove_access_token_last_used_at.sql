-- +goose Up
-- +goose StatementBegin
ALTER TABLE access_tokens DROP COLUMN IF EXISTS last_used_at;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE access_tokens ADD COLUMN last_used_at TIMESTAMPTZ;
-- +goose StatementEnd
