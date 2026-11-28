-- +goose Up
ALTER TABLE of_dns_instances ADD COLUMN remote_update_supported BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE of_dns_instances ADD COLUMN update_request_id VARCHAR(32) NOT NULL DEFAULT '';
ALTER TABLE of_dns_instances ADD COLUMN update_channel VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE of_dns_instances ADD COLUMN update_status VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE of_dns_instances ADD COLUMN update_requested_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE of_dns_instances DROP COLUMN update_requested_at;
ALTER TABLE of_dns_instances DROP COLUMN update_status;
ALTER TABLE of_dns_instances DROP COLUMN update_channel;
ALTER TABLE of_dns_instances DROP COLUMN update_request_id;
ALTER TABLE of_dns_instances DROP COLUMN remote_update_supported;
