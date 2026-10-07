-- +goose Up
ALTER TABLE of_proxy_routes ADD COLUMN enable_http2 BOOLEAN NOT NULL DEFAULT TRUE;

-- +goose Down
ALTER TABLE of_proxy_routes DROP COLUMN enable_http2;
