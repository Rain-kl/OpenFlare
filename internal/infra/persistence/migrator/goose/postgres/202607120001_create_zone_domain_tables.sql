-- +goose Up
CREATE TABLE IF NOT EXISTS of_zones (
    id BIGSERIAL PRIMARY KEY,
    domain VARCHAR(255) NOT NULL,
    remark VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_zones_domain ON of_zones (domain);

CREATE TABLE IF NOT EXISTS of_zone_domains (
    id BIGSERIAL PRIMARY KEY,
    zone_id BIGINT NOT NULL,
    proxy_route_id BIGINT,
    domain VARCHAR(255) NOT NULL,
    cert_id BIGINT,
    remark VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_of_zone_domains_domain ON of_zone_domains (domain);
CREATE INDEX IF NOT EXISTS idx_of_zone_domains_zone_id ON of_zone_domains (zone_id);
CREATE INDEX IF NOT EXISTS idx_of_zone_domains_proxy_route_id ON of_zone_domains (proxy_route_id);
CREATE INDEX IF NOT EXISTS idx_of_zone_domains_cert_id ON of_zone_domains (cert_id);

-- +goose Down
DROP TABLE IF EXISTS of_zone_domains;
DROP TABLE IF EXISTS of_zones;
