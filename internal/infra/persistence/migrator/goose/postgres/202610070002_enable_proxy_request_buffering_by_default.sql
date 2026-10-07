-- +goose Up
UPDATE w_system_configs
SET value = 'true', updated_at = CURRENT_TIMESTAMP
WHERE key = 'openresty_proxy_request_buffering_enabled' AND value = 'false';

-- +goose Down
-- Intentionally left unchanged: restoring the old value could overwrite a later user choice.
