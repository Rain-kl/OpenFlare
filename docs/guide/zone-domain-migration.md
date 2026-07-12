# Zone 域名迁移与发布验收

从旧版 `managed_domains` / 反代路由内嵌域名列迁移到 Zone + Zone 域名模型时，按本指南操作。**导入报告存在冲突时禁止继续发布。**

## 前置条件

* 已备份 PostgreSQL / SQLite 数据库与当前激活配置版本（可导出管理端「配置版本」中的激活快照）。
* Server 二进制已升级到包含 `of_zones` / `of_zone_domains` 表迁移的版本。
* 维护窗口内可暂停非必要配置发布。

## 1. 备份

```bash
# PostgreSQL 示例
pg_dump "$DATABASE_URL" > openflare-pre-zone-$(date +%Y%m%d).sql

# 或复制备份卷 / 快照；SQLite 则直接复制 data 目录中的库文件
```

在管理端确认当前**激活版本号**并记下 checksum，便于回滚对比。

## 2. 执行历史导入

```bash
# 二进制名称为 wavelet（或你的部署包中的同名入口）
wavelet migrate-zones
```

命令会：

1. 先跑 goose 迁移（确保 Zone 表存在）。
2. 以事务从旧路由域名（及无路由域名时的 `of_managed_domains`）导入 Zone / Zone 域名。
3. 使用公共后缀列表解析注册根域；冲突时整单回滚并输出报告。

**成功标志：** 进程退出码 0，且日志/标准输出无「冲突」列表。

**失败时：** 阅读冲突项（无法解析的根域、通配符 FQDN、全局域名冲突、证书不存在等），修复源数据后重新执行。`migrate-zones` 幂等：已导入的域名会跳过，不会重复创建。

**有冲突时不要发布配置、不要执行第二阶段删列迁移。**

## 3. 导入后检查

1. 打开管理端 **网站** `/websites`：确认 Zone 根域与域名计数合理。
2. 进入各 Zone 详情：域名、证书绑定、关联路由 ID 是否正确。
3. 打开 **反代路由**：域名区应展示 Zone 域名绑定，而不是手写域名。

## 4. 配置预览与快照等价性

在升级前若已导出激活快照，导入后：

1. 在管理端打开配置差异 / 预览（或调用配置 diff / preview API）。
2. **逐路由**核对：
   * 明确 `server_name` 集合（全部 FQDN）
   * 证书支持文件路径 / 证书 ID 与域名对应关系
   * WAF 绑定的 Route ID（`site_name` 与路由 ID 不变）
   * Pages 项目引用
3. **允许**旧快照 JSON 中路由上的冗余 `domain` / `domains` / `cert_ids` 字段消失。
4. **不允许**数据面语义变化（域名集合、证书覆盖、上游、WAF、Pages 绑定）。

不一致时：停止发布，修正 Zone 域名/证书绑定后重新预览。

## 5. 发布与回滚

1. 预览通过后，在管理端执行**配置发布**，记录新版本号。
2. 用根域与各子域发起 HTTP(S) 请求，确认节点应用成功。
3. **回滚：** 在配置版本中重新激活导入前的版本；节点会拉取旧快照。数据库侧若需回退，使用升级前备份恢复（第二阶段删列后 Down 迁移不回填业务数据）。

## 6. 第二阶段：删除旧表与冗余列

仅在以下条件全部满足后执行：

* `migrate-zones` 无冲突
* 至少完成一次预览对比与发布（及必要时的回滚演练）
* 运维确认不再依赖 `of_managed_domains` 与 `of_proxy_routes` 上的 `domain` / `domains` / `cert_*` 列

然后升级到包含 `202607130001_drop_legacy_route_domain_columns` 的版本并启动 Server（自动 goose）。该迁移将：

* 删除 `of_proxy_routes` 的 `domain`、`domains`、`cert_id`、`cert_ids`、`domain_cert_ids`
* 删除表 `of_managed_domains`

**不可逆业务数据：** Down 仅在开发库重建空结构，不恢复历史域名行。

## 7. 命令速查

| 步骤 | 命令 / 操作 |
| --- | --- |
| 备份 | `pg_dump` / 复制 SQLite 文件 |
| 导入 | `wavelet migrate-zones` |
| 预览 | 管理端配置差异 / Preview API |
| 发布 | 管理端发布激活版本 |
| 回滚配置 | 管理端激活旧版本 |
| 回滚库 | 恢复备份（勿依赖 Down 填数） |

## 相关文档

* [Zone 与域名资源设计](../design/zone-design.md)
* [新建反代配置](./proxy-config.md)
* [发布第一份配置](./first-site.md)
