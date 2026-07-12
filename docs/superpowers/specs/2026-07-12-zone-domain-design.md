# Zone 域名重构规格

已确认的设计：

* `/websites` 展示可注册根域 Zone；详情 URL 使用 `/websites/:zoneId`。
* `managed_domains` 将被彻底替换为 `of_zones` 与 `of_zone_domains`。
* Zone 域名是 `of_proxy_routes` 域名的规范化来源；一个域名至多连接一条路由，一条路由可含多个 Zone 的域名。
* Zone 域名只允许明确 FQDN；允许把含 `*.example.com` SAN 的 TLS 证书绑定到明确域名，但不允许通配符域名记录。
* 路由仍拥有上游、缓存、限流、WAF 与 Pages；Zone 只提供聚合管理和展示。
* 迁移先建新表、用 Public Suffix List 回填和验证，再在后续独立发布中移除旧表及冗余列。

完整设计、API、迁移与验证策略见 [Zone 与域名资源设计](../../design/zone-design.md)。
