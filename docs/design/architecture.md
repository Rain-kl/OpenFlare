# 系统架构

你会学到：OpenFlare 的整体架构、Server、Agent、OpenResty 与管理端前端的职责边界，以及一次配置发布从管理端到节点生效的请求流。

OpenFlare 由 Server、Agent、节点本地 OpenResty 和管理端前端组成。Server 是控制面，Agent 是节点侧唯一受控落地入口，OpenResty 是实际数据面。内网穿透场景中，Relay（frps 管理器）和 OpenFlared（frpc 管理器）扩展了数据面流量路径。

### 标准反代流量路径

```text
Browser
  |
  | Management UI / API
  v
OpenFlare Server (Gin + GORM + SQLite/PostgreSQL)
  |
  | Agent API / heartbeat / config pull
  v
OpenFlare Agent
  |
  | write config / openresty -t / reload / rollback
  v
OpenResty binary
  |
  | reverse proxy
  v
Origin
```

### 内网穿透流量路径

```text
Browser
  |
  | HTTPS request
  v
OpenResty (Agent, TLS/WAF)         <-- TunnelRelay 节点
  |
  | proxy_pass http://localhost:vhost_port (Host header preserved)
  v
OpenFlareRelay (frps)              <-- TunnelRelay 节点，与 Agent 同机部署
  |
  | frp tunnel protocol (HTTP Vhost routing by Host header)
  v
OpenFlared (frpc)                  <-- 内网服务器
  |
  | HTTP/HTTPS forward
  v
Internal Service (192.168.x.x)
```

## 组件职责

| 组件            | 职责                                                                   |
| --------------- | ---------------------------------------------------------------------- |
| Server          | 管理端 UI、管理 API、Agent/Relay/Client API、配置渲染、版本发布、数据存储与聚合查询 |
| Agent           | 注册、心跳、同步、写入文件、校验、reload、失败回滚、自更新与轻量采集   |
| OpenResty       | 接收真实流量，按 OpenFlare 渲染的配置执行 WAF、PoW、认证与反向代理     |
| OpenFlareRelay  | 管理 frps 进程生命周期，提供隧道中继服务，通过心跳接收 frps 配置       |
| OpenFlared      | 管理 frpc 进程（可多个），连接 Relay 中继，将流量转发到内网服务        |
| Frontend        | 管理网站配置、WAF、源站、证书、节点、Tunnel、版本、用户、设置与观测页面 |

## Server

`openflare_server` 是单体控制面：

* Gin 提供 HTTP 服务。
* GORM 访问 SQLite 或 PostgreSQL。
* 现有登录体系提供管理端 Session。
* 认证源与外部账号绑定支持 GitHub OAuth 和标准 OIDC。
* Go Server 托管 `openflare_server/web` 静态构建产物。

Server 不直接 SSH 到节点，也不在线修改节点文件。它只保存控制面状态、生成完整配置版本，并通过 Agent API 让节点主动拉取。

## Agent

`openflare_agent` 是 Go 单体程序：

* 单二进制运行在节点侧。
* 启动后读取或生成本地节点信息。
* 周期性 heartbeat，上报状态并获取激活版本摘要。
* 发现新版本后拉取配置、备份旧文件、写入新文件、校验并 reload。
* 应用失败时尝试恢复运行并回滚。
* 维护 WAF GeoIP mmdb，启动时写入内置初始库，并按配置定期更新。

Agent 通过 `openresty_path` 指向的 OpenResty 二进制统一执行校验、reload、启动与重启；未配置时默认调用 `openresty`。Docker 部署时，Agent 镜像内置 OpenResty 二进制，仍走同一套二进制控制逻辑。

节点 IP 默认由 Agent 注册和心跳上报维护；如果管理端锁定节点 IP，Server 只更新运行状态、版本、观测等运行态字段，不再接受 Agent 上报覆盖该 IP。

## Frontend

`openflare_server/web` 是正式管理端前端：

* Next.js 15 App Router。
* React 19。
* TypeScript。
* Tailwind CSS。
* TanStack Query 管理服务端状态。

前端采用静态导出模式（`output: 'export'`），导出后由 Go Server 通过 `embed.FS` 托管。所有 API 请求应统一经过 `lib/api/`，并处理 `success/message/data` 响应结构。

Server 集成以下安全特性：
* CORS 中间件：跨域请求保护。
* 速率限制：全局与关键接口限流。
* 会话管理：基于 Cookie/Redis 的会话存储。

## 数据与请求流

### 管理端请求流

```text
Browser -> Frontend -> /api/* -> controller -> service -> model -> database
```

管理端变更类接口使用 `POST`，只读接口使用 `GET`。成功与失败都返回清晰的 `message`。

### Agent 同步流

```text
Agent HTTP heartbeat -> Server 返回激活版本摘要
Agent 发现新版本 -> 拉取配置详情
Agent 写入主配置 / 路由配置 / 证书 / Lua 资源 / WAF 运行时配置
Agent 执行 OpenResty 校验与 reload
Agent 上报应用结果
```

### Relay 同步流

Relay（OpenFlareRelay 进程）运行在 TunnelRelay 节点上，与 Agent 共享同一 `agent_token`：

```text
Relay HTTP heartbeat -> Server 返回 frps 基础配置 (bindPort, vhostHTTPPort, auth_token)
Relay 生成 frps.toml 并启动或更新 frps 进程
Relay 定期上报 frps 健康状态与连接统计
Relay 尝试升级 WebSocket 连接以支持实时配置推送
```

frps 配置相对静态（端口、认证 Token），通过心跳下发，**不纳入版本化发布流**。Relay 需要监听 frps 进程异常并自动恢复。认证方式：`X-Agent-Token` + API 路径前缀 `/api/relay/*`，Server 通过 `node_type = tunnel_relay` 区分。

### OpenFlared 同步流

OpenFlared（客户端）运行在内网服务器，使用独立的 `tunnel_token` 认证：

```text
Client HTTP heartbeat -> Server 返回 tunnel 配置版本摘要 (version, checksum)
Client 发现新版本 -> 拉取完整 tunnel 路由配置 (relay 列表 + frpc proxy 定义)
Client 为每个 Relay 生成独立的 frpc.toml 配置文件
Client 为新 Relay 启动 frpc 进程，或为已有 Relay 执行热重载 (frpc reload)
Client 上报应用结果 (成功/失败原因)
```

OpenFlared 通过 `/api/flared/*` 端点与 Server 通信，认证使用 `X-Tunnel-Token`。Tunnel 路由配置随发布流程版本化同步，所有配置变更通过单一版本号关联并一致性发布到 Agent 和 Client。

**WebSocket 升级流程**（可选，通过 `AgentWebsocketUpgradeEnabled` 选项控制）：

当启用 WebSocket 升级时：
1. Agent 通过 HTTP heartbeat 获取运行配置与设置。
2. Agent 尝试升级连接到 `GET /api/agent/ws`（WebSocket）。
3. WS 连接成功后，周期性状态上报和实时消息由 WebSocket 承载，降低延迟。
4. Server 发布或激活版本后，可向已连接 Agent 立即广播激活版本摘要，使 Agent 立即进入同步流程。
5. 若 WebSocket 断开或建立失败，Agent 自动降级回 HTTP heartbeat，保证可用性。

通过 `OpenRestyWebsocketEnabled` 选项，可在 OpenResty 层面启用或禁用 WebSocket 反向代理支持。

### 反向代理流

```text
Client -> OpenResty server block -> WAF Lua -> named upstream -> Origin
```

网站配置是反向代理聚合边界。一条网站配置可绑定多个域名，并共享站点级流量限制、反向代理和缓存配置。

WAF 在 OpenResty `access_by_lua_file` 阶段执行。规则来自当前激活版本携带的 `waf_config.json`，全局规则组默认生效，网站可叠加自定义规则组。`waf_config.json` 只保存规则组直接 IP 和 IP 组引用 ID；IP 组成员由 Agent 独立同步到本地 `waf_ip_groups.json`，OpenResty Lua 按引用 ID 合并判断。

WAF IP 组由 Server 管理。手动 IP 组直接保存 IP/IP 段列表；自动 IP 组由 Server 定时任务读取请求日志、按单个 IP 聚合指标并执行 Expr 规则；订阅 IP 组由 Server 定时任务同步远程文本或 JSON 源。Agent 心跳会上报本地 IP 组 checksum，Server 只返回不一致的 IP 组；Server 侧 IP 组更新时会通过 Agent WebSocket 广播变更组。OpenResty Lua 只读取 Agent 落地的运行时 JSON，不直接访问 Server 数据库、请求日志或远程订阅源。

## 核心对象

当前有效实体包括：

* `proxy_routes`
* `origins`
* `config_versions`
* `nodes`
* `tunnels`
* `auth_sources`
* `external_accounts`
* `node_system_profiles`
* `apply_logs`
* `tls_certificates`
* `managed_domains`
* `node_request_reports`
* `node_access_logs`
* `node_metric_snapshots`
* `traffic_analytics_rollups`
* `node_health_events`
* `waf_rule_groups`
* `waf_ip_groups`
* `waf_rule_group_bindings`
* `acme_accounts`
* `dns_accounts`
* `geoip_update_configs`

## 关键设计决策

| 决策                           | 原因                                                                        |
| ------------------------------ | --------------------------------------------------------------------------- |
| 完整配置版本，而不是在线 patch | 让预览、激活、历史和回滚有稳定边界                                          |
| Agent 主动拉取                 | Server 不需要 SSH 权限，也不暴露远程命令入口；支持 HTTP 与 WebSocket 双协议 |
| 全局单激活版本                 | 降低 MVP 复杂度，保证所有节点默认一致；支持版本预览、历史查询与一键回滚     |
| 网站配置聚合多域名             | 支持一个业务站点共享站点级策略，同时允许按域名绑定证书                      |
| 观测数据服务端聚合             | 避免前端临时统计造成口径不一致                                              |
| 内网穿透基于 frp 整合          | 复用成熟隧道协议，避免自研隧道的稳定性风险；frps HTTP Vhost 路由天然适配    |
| Relay/Client 独立二进制         | 职责分离，Relay 管理 frps，Client 管理 frpc，各自独立升级和部署             |
| Tunnel 与 Node 体系分离         | Tunnel 客户端在内网运行，与公网节点概念不同，使用独立的注册和认证体系       |

## 贡献者阅读建议

如果要修改架构相关代码，先阅读：

1. [产品边界](./index.md)
2. [发布模型](./release-model.md)
3. [开发约束](../guildline/development-constraints.md)
4. [仓库结构](./repository.md)
