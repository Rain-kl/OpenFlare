-- +goose Up
ALTER TABLE of_dns_accounts ADD COLUMN scopes TEXT NOT NULL DEFAULT '[]';
ALTER TABLE of_dns_accounts ADD COLUMN verification_status VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE of_dns_accounts ADD COLUMN last_checked_at TIMESTAMPTZ;
ALTER TABLE of_cf_pointing_members ADD COLUMN dns_account_id BIGINT;
ALTER TABLE of_cf_pointing_members ALTER COLUMN cf_zone_id TYPE TEXT;
CREATE INDEX idx_of_cf_pointing_members_dns_account ON of_cf_pointing_members (dns_account_id);

-- +goose Down
DROP INDEX idx_of_cf_pointing_members_dns_account;
ALTER TABLE of_cf_pointing_members DROP COLUMN dns_account_id;
ALTER TABLE of_dns_accounts DROP COLUMN last_checked_at;
ALTER TABLE of_dns_accounts DROP COLUMN verification_status;
ALTER TABLE of_dns_accounts DROP COLUMN scopes;
