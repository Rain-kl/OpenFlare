-- +goose Up
CREATE TABLE of_waf_request_whitelist (
    id BIGSERIAL PRIMARY KEY,
    domain TEXT NOT NULL,
    path_match TEXT NOT NULL,
    path TEXT NOT NULL
);

CREATE TRIGGER dirty_edge_waf_request_whitelist AFTER INSERT OR UPDATE OR DELETE ON of_waf_request_whitelist
FOR EACH STATEMENT EXECUTE FUNCTION openflare_dirty_edge_snapshot();

-- +goose Down
DROP TABLE of_waf_request_whitelist;
