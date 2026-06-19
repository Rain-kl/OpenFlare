-- +goose Up
CREATE TABLE IF NOT EXISTS of_tls_certificates (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    cert_pem TEXT NOT NULL,
    key_pem TEXT NOT NULL,
    not_before TIMESTAMPTZ,
    not_after TIMESTAMPTZ,
    remark VARCHAR(255) NOT NULL DEFAULT '',
    provider VARCHAR(64) NOT NULL DEFAULT 'upload',
    acme_account_id BIGINT NOT NULL DEFAULT 0,
    dns_account_id BIGINT NOT NULL DEFAULT 0,
    key_algorithm VARCHAR(32) NOT NULL DEFAULT '',
    auto_renew BOOLEAN NOT NULL DEFAULT FALSE,
    primary_domain VARCHAR(255) NOT NULL DEFAULT '',
    other_domains TEXT NOT NULL DEFAULT '',
    disable_cname BOOLEAN NOT NULL DEFAULT FALSE,
    skip_dns BOOLEAN NOT NULL DEFAULT FALSE,
    dns1 VARCHAR(128) NOT NULL DEFAULT '',
    dns2 VARCHAR(128) NOT NULL DEFAULT '',
    apply_status VARCHAR(64) NOT NULL DEFAULT 'ready',
    apply_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_tls_certificates_name ON of_tls_certificates (name);

CREATE TABLE IF NOT EXISTS of_managed_domains (
    id BIGSERIAL PRIMARY KEY,
    domain VARCHAR(255) NOT NULL,
    cert_id BIGINT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    remark VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_managed_domains_domain ON of_managed_domains (domain);

CREATE TABLE IF NOT EXISTS of_dns_accounts (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(64) NOT NULL,
    "authorization" TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS of_acme_accounts (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL DEFAULT '',
    url VARCHAR(255) NOT NULL DEFAULT '',
    private_key TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS of_managed_domains;
DROP TABLE IF EXISTS of_tls_certificates;
DROP TABLE IF EXISTS of_dns_accounts;
DROP TABLE IF EXISTS of_acme_accounts;