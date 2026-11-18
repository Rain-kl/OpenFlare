-- +goose Up
ALTER TABLE of_proxy_routes ADD COLUMN site_auth_ttl_hours INTEGER NOT NULL DEFAULT 0;
ALTER TABLE of_proxy_routes ADD COLUMN site_auth_key VARCHAR(64) NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE of_proxy_routes DROP COLUMN site_auth_key;
ALTER TABLE of_proxy_routes DROP COLUMN site_auth_ttl_hours;
