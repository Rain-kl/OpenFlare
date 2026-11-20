# Zone 域名迁移与发布验收

从旧版 `managed_domains` / 反代路由内嵌域名列迁移到 Zone + Zone 域名模型时，数据导入与表结构升级均由 **Server 启动时的 goose 自动迁移**完成，无需单独执行导入命令。

## 升级时发生了什么

启动（或滚动升级）包含 Zone 改造的 Server 版本时，**无需手动命令**，`migrator.Migrate()` 自动：

1. 应用 goose SQL：创建 `of_zones` / `of_zone_domains`（若尚未存在）。
2. **自动导入**旧路由域名列（及无路由域名时的 `of_managed_domains`）为 Zone / Zone 域名，并绑定 `proxy_route_id` / `cert_id`（公共后缀列表解析注册根域）。
3. 继续 goose SQL：删除 `of_managed_domains` 与 `of_proxy_routes` 冗余域名/证书列。

导入幂等：已存在的域名会跳过或补绑路由。

**若历史数据无法解析（冲突域名、无效根域、证书不存在等），启动失败。** 修复数据或恢复备份后再次启动即可重试。

## 建议操作

### 1. 升级前备份

```bash
# PostgreSQL 示例
pg_dump "$DATABASE_URL" > openflare-pre-zone-$(date +%Y%m%d).sql

# 或复制备份卷 / 快照；SQLite 则复制 data 目录中的库文件
```

可在节点详情记录当前配置修订和最近一次成功的应用记录，作为升级后核对依据。

### 2. 升级并启动 Server

部署新版本并启动即可。观察启动日志中的 goose 成功信息；若出现「迁移 Zone 失败（N 个冲突）」则按日志中的冲突项修复源数据后重启。

### 3. 升级后检查

1. 管理端 **网站** `/websites`：Zone 根域与域名计数是否合理。
2. Zone 详情：域名、证书、关联路由 ID。
3. **反代路由**：域名绑定来自 Zone 域名，而非旧手写字段。

### 4. 验证节点同步

控制端启动后会从已提交的域名、证书和路由生成站点快照。检查节点「应用记录」，并验证域名匹配、证书、WAF 和 Pages 的实际行为。数据库回退需使用升级前的备份。

## 相关文档

* [Zone 与域名资源设计](../design/zone-design.md)
* [新建反代配置](./proxy-config.md)
* [创建第一条反向代理规则](./first-site.md)
