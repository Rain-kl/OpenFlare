---
title: CLI 与 MCP
description: 使用 OpenFlare CLI 或 MCP 为 AI 授权日常网站运营与故障排查。
---

# CLI 与 MCP

## 开通连接

在控制台打开「系统设置 → MCP」，填入客户端可访问的 HTTP 或 HTTPS 地址并保存。公网部署建议使用 HTTPS。该标签页显示 MCP 地址、实例 ID 和两种连接方式。清空地址并保存可暂停本实例的 CLI 和 MCP 入口。部分客户端要求 MCP 地址使用 HTTPS。

- **OAuth**：复制连接说明，或在客户端添加 Streamable HTTP 连接并选择 OAuth。客户端须支持动态客户端注册和 S256 PKCE。首次连接时，在浏览器使用管理员账户登录。授权页从上到下显示「读取权限」和「写入权限」。各组默认关闭，开启一组并确认后，将授予该组列出的全部权限。仅需查询时，开启「读取权限」即可。
- **Access Token**：在「个人访问令牌」设置页创建并管理现有的个人访问令牌，在客户端的自定义请求头中设置 `Authorization: Bearer TOKEN`。内置 MCP 仅接受已启用管理员权限的个人令牌；该令牌可调用全部 MCP 工具。

连接后可调用 `openflare_connection_info` 核对实例 ID 和已授权权限。OAuth 访问令牌有效期为 15 分钟，客户端可自动刷新，授权最长持续 30 天。「撤销全部 OAuth 授权」会使当前账户授予 CLI 和 MCP 客户端的 OAuth 授权立即失效。个人访问令牌沿用系统现有的轮换与撤销管理。

若页面提示会话密钥不足 32 字节，请检查 `app.session_secret` 或 `APP_SESSION_SECRET`。使用 [会话密钥更换指南](./session-secret-rotation.md) 检查存量凭据并完成迁移。

反向代理需将 `/mcp`、`/.well-known/`、`/oauth/automation/`、`/automation/` 和现有 API 前缀转发至同一控制端，并保留原始 `Host` 请求头。多实例部署时，各控制端应共用数据库及会话密钥。

## 使用 CLI

从 [OpenFlare Releases](https://github.com/Rain-kl/OpenFlare/releases) 下载对应系统的 `openflare_cli_*`，解压后将 `openflare`（Windows 为 `openflare.exe`）加入 `PATH`。也可在源码目录运行 `go build -o bin/openflare ./cmd/openflare`。在 Linux 上，CLI 需要可用的 Secret Service 凭据库；macOS 和 Windows 使用系统凭据库。

```sh
openflare auth login --server https://openflare.example.com
openflare connection info
openflare operations list
openflare site create --help
```

登录时指定服务器地址，后续读写命令均使用保存的默认连接。管理多个实例时，可用 `openflare auth login --profile staging --server https://staging.example.com` 保存额外连接，访问该实例的命令再加 `--profile staging`。

对象和数组参数使用 JSON。记录 ID 即使由数字组成，也应作为字符串提供。浏览器侧是否启用 HTTPS 与回源 URL 的协议分别设置。

## 常用操作

| 任务 | CLI / MCP |
| --- | --- |
| 创建网站 | 使用 `zone list` 或 `zone create` 取得 `zone_id`。用 `domain create` 登记域名并取得 `domain_record_id`，再用 `site create` 指定这些记录、上游 URL 和浏览器侧 HTTPS 开关。使用 HTTPS 时，先通过 `certificate list` 找到覆盖对应域名的现有证书。 |
| 新增站点别名 | 用 `site list` 找到 `site_id`，用 `domain create` 或 `domain list` 取得域名记录 ID，再执行 `site bind-domain`。该操作保留原有域名。 |
| 更换站点上游 | 先查询 `site list`，再使用 `site set-upstreams` 提交目标站点的完整直连上游列表。省略 `origin_host` 保留现有 Host 覆盖值。 |
| 修改反向代理与限流 | 使用 `site get-settings` 查看现有配置，再用 `site patch-settings` 部分更新站点启停、TLS、HTTP/2、重定向、缓存、响应头或限流。 |
| 绑定 WAF 流程 | 使用 `waf list --site-id SITE_ID` 取得流程 ID 和现有绑定，再用 `site set-waf` 添加或解除指定流程。 |
| 配置 PoW 或认证 | 使用 `site get-pow`、`site set-pow` 管理独立挑战；用 `site set-auth` 切换 `off`、`basic` 或 `site` 模式。`site` 模式要求 HTTPS。 |
| 管理攻击防护规则组 | 使用 `attack-group list`、`create`、`update` 和 `delete`。被任意 WAF 流程引用的组无法删除。 |
| 查询 IP 组 | 使用 `ip-group list` 查看条目数和同步状态，使用 `ip-group get` 读取指定组的 IP、CIDR 和已同步条目。 |
| 查看当前网站和近期访问 | 使用 `site overview`（MCP 工具 `openflare_site_overview`），一次返回站点清单、请求量排行、实例总流量和常见状态码。可用 `--hours 168` 查看近 7 天。 |
| 排查流量与节点 | 使用 `logs list` 按主机、节点、状态码和时间查询；`metrics traffic` 查询流量趋势，只需总量和状态码时设置 `--with-trends=false`。使用 `node health` 一次检查一页节点的在线状态、运行健康、活跃告警和站点快照同步状态；使用 `node metrics` 查看资源指标。 |
| 调整性能策略参数 | 先用 `performance list` 查看现有策略、受影响节点和参数范围，再使用 `performance set` 修改一个参数。修改默认策略影响未显式分配策略的 Pingora 节点。 |

MCP 工具使用相同的对象和动作，例如 `openflare_site_set_upstreams` 对应 `openflare site set-upstreams`。建站和绑定不会修改公网 DNS 或签发证书。控制端保存后由现有配置同步链路分发；通过 `node get` 比较 `current_version` 与 `desired_site_version`，核对节点是否已应用控制端当前的站点快照。

### 配置站点安全和限流

```sh
openflare site list --search www.example.com
openflare waf list --site-id "$SITE_ID"
openflare site set-waf --site-id "$SITE_ID" --flow-id "$FLOW_ID" --bound=true
openflare site set-pow --site-id "$SITE_ID" --enabled=true
openflare site patch-settings --site-id "$SITE_ID" --limits '{"limit_req_per_ip":"20r/s","limit_conn_per_ip":10}'
openflare node health --node-type edge_node --status online
```

`site set-waf` 保留其他流程及其执行顺序。全局流程独立生效；站点绑定不会启用已停用的流程。攻击防护组由流程中的安全检查节点引用，绑定时应使用 `flow_id`。`attack-group update` 替换组名和类别等级；省略 `rule_overrides` 保留逐条配置，传入 `{}` 清空。

`site patch-settings` 保留未提供的字段。`limits` 包含站点并发数 `limit_conn_per_server`、单 IP 并发数 `limit_conn_per_ip`、响应速率 `limit_rate` 和单 IP 请求速率 `limit_req_per_ip`。并发数 `0` 或速率空字符串继承实例默认值，`-1` 关闭对应限制（速率字段用字符串 `"-1"`）。`site set-pow` 省略 `config` 时保留原参数，首次设置使用默认值；传入 `config` 时替换整组参数。`site set-auth` 首次启用时需要用户名和密码，CLI 可通过 `--password-stdin` 从标准输入读取密码。切换至 `off` 会清除凭据。查询不会返回密码、签名密钥或自定义响应头的值。

WAF、攻击防护组、IP 组和 PoW 查询需要 `security:read`；安全配置和攻击防护组写入需要 `security:write`。已有 OAuth 连接需重新授权获取新增权限。

`node health` 的 `node.status` 表示心跳状态，`runtime_health` 为节点最后报告的运行状态。对边缘节点，`config_sync: synced` 表示已报告应用控制端当前的站点快照；离线节点的同步状态与运行状态均为最近一次上报的信息。

### 站点流量统计

`site overview` 需要 `sites:read` 和 `metrics:read` 权限，默认查询最近 24 小时并返回请求量最高的 50 个站点。停用和零请求站点也参与排序。分页时沿用首次返回的 `until`、原查询条件和 `next_offset`。

日志按查询时的域名绑定归属站点。同站点的多个域名合计请求量，并在站点内按来源 IP 去重。精确域名优先于单级通配域名。无法匹配当前绑定的请求记入 `unmatched_requests`，同时计入实例总流量。请求数包含自动探测、接口调用和浏览器访问。
