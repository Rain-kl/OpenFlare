-- +goose Up
-- 移除业务实体上未再使用的备注列（证书与源站备注保留）。
-- SQLite 3.35+ 支持 DROP COLUMN；goose 仅在列存在的既有库上执行。

ALTER TABLE of_zones DROP COLUMN remark;
ALTER TABLE of_zone_domains DROP COLUMN remark;
ALTER TABLE of_proxy_routes DROP COLUMN remark;
ALTER TABLE of_waf_rule_groups DROP COLUMN remark;
ALTER TABLE of_waf_ip_groups DROP COLUMN remark;

-- +goose Down

ALTER TABLE of_zones ADD COLUMN remark TEXT NOT NULL DEFAULT '';
ALTER TABLE of_zone_domains ADD COLUMN remark TEXT NOT NULL DEFAULT '';
ALTER TABLE of_proxy_routes ADD COLUMN remark TEXT NOT NULL DEFAULT '';
ALTER TABLE of_waf_rule_groups ADD COLUMN remark TEXT NOT NULL DEFAULT '';
ALTER TABLE of_waf_ip_groups ADD COLUMN remark TEXT NOT NULL DEFAULT '';
