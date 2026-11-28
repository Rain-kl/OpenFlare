# CLI 与 MCP 自动化

通过 OAuth 或个人访问令牌接入 `openflare` CLI 与 MCP Server，自动化管理站点、WAF、节点与监控任务。

## 1. 启用 MCP 连接

进入 **系统管理** > **系统设置** > **MCP**，保存客户端可访问的 HTTP/HTTPS Server 地址（公网建议 HTTPS；清空保存即停用）并选择一种认证方式：

| 方式 | 配置与权限规则 |
| :--- | :--- |
| OAuth（推荐） | 客户端需支持 RFC 7591 动态注册与 S256 PKCE；首次由管理员授权并勾选 Read/Write 权限组（访问令牌 15 分钟有效、刷新令牌 30 天，可点击 **撤销所有 OAuth 授权** 立即失效） |
| 个人访问令牌 | 在 **个人设置** > **访问令牌** 创建管理员令牌并附带 `Authorization: Bearer TOKEN`，默认拥有全部 MCP 工具权限 |

调用 `openflare_connection_info` 可校验实例 ID 与当前权限。反向代理须将 `/mcp`、`/.well-known/`、`/oauth/automation/`、`/automation/` 及 API 前缀转发至控制面并保留 `Host` 头；多副本部署须共享数据库与至少 32 字节的 `session_secret`（详见 [轮换 Session Secret](./session-secret-rotation.md)）。

## 2. 安装并登录 CLI

从 [OpenFlare Releases](https://github.com/Rain-kl/OpenFlare/releases) 下载 `openflare_cli_*` 加入 `PATH`（或运行 `go build -o bin/openflare ./cmd/openflare`；Linux 桌面依赖 Secret Service keyring）：

```sh
openflare auth login --server https://openflare.example.com
openflare connection info
openflare operations list
openflare site create --help
```

多环境可通过 `--profile <name>` 隔离凭据；对象或数组参数传入 JSON，ID 字段一律使用字符串。

## 3. 常用任务速查

| 任务 | CLI 命令（MCP 工具统一前缀 `openflare_`） |
| :--- | :--- |
| 创建站点 | 先查 `zone list`/`zone create` 与 `domain create`，再执行 `site create`（HTTPS 需先在 `certificate list` 确认匹配证书） |
| 绑定附加域名 | 查询 `site list` 与 `domain list`/`domain create` 后执行 `site bind-domain` |
| 更新直连回源 | 用 `site set-upstreams` 传入完整上游列表（省略 `origin_host` 保留原值） |
| 局部更新路由配置 | 先执行 `site get-settings`，再用 `site patch-settings` 增量修改开关、TLS、跳转、缓存、请求头或限流 |
| 绑定 WAF 规则流 | 执行 `waf list --site-id SITE_ID` 后使用 `site set-waf` 绑定或解绑 |
| 配置 PoW 与站点认证 | 用 `site get-pow`/`site set-pow` 管理 PoW；用 `site set-auth` 切换 `off` / `basic` / `site`（`site` 需 HTTPS） |
| 管理攻击规则组 | 执行 `attack-group list` / `create` / `update` / `delete`（已被 WAF 流引用的规则组不可删除） |
| 查看 IP 组 | `ip-group list` 检查数量与同步状态，`ip-group get` 读取成员明细 |
| 站点流量总览 | `site overview` 查看头部站点、请求量与状态码分布（如 `--hours 168` 查近 7 天） |
| 查日志与节点监控 | 执行 `logs list`、`metrics traffic`（`--with-trends=false` 仅返回汇总）、`node health`、`node metrics` 与 `node ping` |
| 管理延迟探测任务 | 执行 `ping list` / `create` / `update` / `delete` / `reorder`（需 `ping:read` / `ping:write`） |
| 边缘节点远程执行 | 按 [服务端部署](../deployment/server.md) 开启配置开关后使用 `exec run` / `exec get` / `exec list`（需 `exec:write` / `exec:read`） |
| 调优性能策略 | 执行 `performance list` 查看策略并由 `performance set` 更新参数 |

## 4. 关键参数与权限提示

```sh
openflare site list --search www.example.com
openflare waf list --site-id "$SITE_ID"
openflare site set-waf --site-id "$SITE_ID" --flow-id "$FLOW_ID" --bound=true
openflare site set-pow --site-id "$SITE_ID" --enabled=true
openflare site patch-settings --site-id "$SITE_ID" --limits '{"limit_req_per_ip":"20r/s","limit_conn_per_ip":10}'
openflare node health --node-type edge_node --status online
```

- 站点写入不会修改公网 DNS 或自动签发证书；可在 `node get` 对比 `current_version` 与 `desired_site_version`（或 `node health` 的 `config_sync: synced`）确认快照已应用。
- `site set-waf` 保留现有流顺序且不改动启用状态；`attack-group update` 省略 `rule_overrides` 时保留原覆盖配置，传 `{}` 则清空。
- `site patch-settings` 的 `limits` 中传 `0` 或 `""` 表示继承默认阈值，传 `-1` / `"-1"` 表示关闭该限流；`site set-auth` 支持 `--password-stdin`，切回 `off` 会清除口令。
- WAF、攻击组、IP 组与 PoW 分别依赖 `security:read` / `security:write` 作用域；升级后已连接 OAuth 须重新授权生效。
- `site overview` 默认返回近 24 小时前 50 个站点（大结果按 `until` + `next_offset` 翻页），并可通过 `unmatched_requests` 排查未绑定域名的访问。
