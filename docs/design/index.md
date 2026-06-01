# 产品边界

你会学到：OpenFlare 是什么、解决什么问题、目标用户是谁、当前稳定能力有哪些，以及哪些设计边界在实现时不能被绕过。

OpenFlare 是一套自托管的 OpenResty 控制面，面向单团队或单组织内部运维场景。它解决反向代理配置、节点同步、证书托管、配置发布回滚与基础观测分散管理的问题。

## 项目定位

OpenFlare 适合需要统一管理多台 OpenResty 代理节点的团队：

* 希望用管理端维护反向代理网站配置。
* 希望每次配置变更都有完整版本、预览、激活与回滚。
* 希望节点主动同步配置，而不是由控制面 SSH 到节点执行命令。
* 希望在同一系统中管理 TLS 证书、域名资产、节点状态和基础访问分析。

OpenFlare 当前不定位为通用日志平台、服务网格、Kubernetes Ingress Controller 或多租户云平台。

## 当前能力

| 能力 | 说明 |
| --- | --- |
| 反代规则管理 | 以网站配置为聚合边界，支持多域名与源站配置 |
| 网站级配置 | 一条规则对应一个网站，可绑定一个或多个域名，并共享站点级配置 |
| 源站管理 | 维护轻量源站目录，并允许网站保存可渲染的源站快照 |
| 配置版本 | 支持预览、发布、激活、不可变历史与回滚 |
| Agent 同步 | 支持注册、心跳、同步、应用结果上报与自更新 |
| OpenResty 托管 | 管理主配置模板、性能参数、缓存参数与 Lua 资源 |
| HTTPS/TLS | 托管证书与域名资产，并按域名绑定证书 |
| WAF | 以全局规则组与网站自定义规则组维护 IP/IP 段、IP 组、国家级地域黑白名单 |
| 基础观测 | 聚合节点请求、资源快照、健康事件和访问分析 |
| 节点管理 | 节点状态、令牌体系、部署与更新链路 |
| 管理端前端 | 基于 Next.js 的正式管理端 |
| 认证源登录 | 支持以认证源形式配置 GitHub 与标准 OIDC 登录入口，并允许第三方账号绑定已有本地用户 |
| 内网穿透 | 通过 TunnelRelay 节点与 OpenFlared 客户端，将内网 HTTP 服务安全暴露到公网，复用 Agent 的 HTTPS/WAF 能力 |

默认工作方式：

* 所有节点消费同一份全局激活版本。
* Server 保存配置与状态，不直接 SSH 管理节点。
* Agent 是节点侧唯一受控落地入口。
* TunnelRelay 节点同时运行 Agent（OpenResty）和 Relay（frps），提供内网穿透中继。
* OpenFlared 客户端在内网运行，管理 frpc 进程连接 Relay，将流量转发到内网服务。

## 典型使用场景

| 场景 | 说明 |
| --- | --- |
| 内部服务统一入口 | 把多个内部 HTTP 服务通过统一域名和证书暴露 |
| 多节点反代配置同步 | 多台 OpenResty 节点消费同一份激活配置 |
| 配置变更审查 | 发布前查看预览或 diff，发布后保留不可变历史 |
| 快速回滚 | 重新激活旧版本，让 Agent 拉取并应用 |
| 证书托管 | 为不同域名绑定 TLS 证书 |
| 基础观测 | 查看节点状态、请求聚合、访问分析和健康事件 |
| 内网穿透 | 通过 Tunnel 将无法直接公网访问的内网 HTTP 服务暴露到互联网，享有 HTTPS、WAF 等全部防护能力 |


## 网站配置约束

`proxy_routes` 从“单域名规则”升级为“网站配置”聚合对象。一条记录对应一个网站，可绑定一个或多个域名，并共享一组站点级配置。

约束：

* `proxy_routes.site_name` 是网站的业务唯一标识。
* `proxy_routes.domains` 至少包含一个域名，且 `domains[0]` 作为主域名。
* 任一域名全局只能属于一个 `proxy_routes`。
* 迁移期可保留 `proxy_routes.domain` 作为 `domains[0]` 的镜像字段，但业务读写与后续扩展必须以 `site_name` + `domains` 为准。
* 网站级流量限制、反向代理与缓存配置当前按站点共享，不在同一网站内做域名级差异化配置。
* HTTPS 允许在同一站点内按域名绑定证书。

## 源站约束

`origins` 只保存源站地址、展示名与备注，不承载协议、端口、路径、权重或健康检查策略。

`proxy_routes` 可选关联一个 `origins` 记录，用于复用源站地址；规则仍保存完整 `origin_url` 快照以参与渲染与版本快照。

上游约束：

* `proxy_routes` 至少包含一个上游地址（直连类型），或关联一个 Tunnel（内网穿透类型）。
* `proxy_routes.upstream_type` 区分上游类型：`direct`（默认，直连）或 `tunnel`（内网穿透）。
* 为兼容历史数据保留 `origin_url` 主上游字段，也允许在同一规则内补充多个上游做负载均衡。
* 上游统一渲染为带 keepalive 的 named `upstream`。
* 单上游可附带 base path 或 query 并在 `proxy_pass` 中追加。
* 多上游限定为纯 `scheme://host[:port]`。
* `proxy_routes.origin_host` 为可选字段，用于回源时覆盖 `Host` 请求头。
* 所有直连类型上游地址都必须为合法 `http://` 或 `https://`。
* 内网穿透类型上游必须关联 `tunnel_id`，并指定内网目标地址与协议。

## 内网穿透约束

OpenFlare 通过 TunnelRelay 节点与 OpenFlared 客户端实现内网穿透，底层基于 frp（快速反向代理）构建。

### 节点与组件模型

**节点类型**：

* `nodes.node_type` 区分节点类型：`edge_node`（边缘节点，默认）和 `tunnel_relay`（隧道中继）。
* TunnelRelay 节点同时运行 Agent（OpenResty）和 Relay（frps 管理器），共享同一个 `agent_token`。
  - Agent 负责 HTTPS 终结、WAF 防护、缓存与流量限制等。
  - Relay 管理 frps 进程，为内网客户端提供隧道中继服务。
* TunnelRelay 节点新增字段：`node_type`、`relay_bind_port`（frpc 连接端口，默认 7000）、`relay_vhost_http_port`（HTTP Vhost 端口，默认 8080）、`relay_auth_token`（自动生成）、`relay_status` 等。

**Tunnel 客户端**：

* `tunnels` 表独立存储内网穿透客户端注册信息，与 `nodes` 体系无关。
* 每个 Tunnel 拥有唯一的 `tunnel_id`（格式 `tun-<32hex>`）和 `tunnel_token`（客户端认证凭据）。
* OpenFlared 客户端运行在内网，不对外暴露，使用 `tunnel_token` 认证，通过 `/api/flared/*` 端点与 Server 通信。
* 一个 OpenFlared 客户端可同时连接多个 Relay（为高可用）。

### 上游类型扩展

`proxy_routes` 的上游配置分为两种类型，通过 `upstream_type` 字段区分：

* **直连上游（`direct`，默认）**：直接将流量转发到源站地址，行为与现有完全一致。
* **内网穿透上游（`tunnel`）**：通过 TunnelRelay 节点将流量转发到内网服务。
  - 必须指定 `tunnel_id`（关联 `tunnels` 表）。
  - 必须指定 `tunnel_target_addr`（内网目标地址，如 `192.168.1.100:8080`）和 `tunnel_target_protocol`（`http` 或 `https`）。
  - 发布时，Server 自动将上游地址替换为 `http://127.0.0.1:{relay_vhost_http_port}`。

### 流量路径与协议

**完整数据面流量路径**：

```
浏览器 → OpenResty (Agent, TLS/WAF)         [TunnelRelay 节点]
       ↓
     frps (Relay, HTTP Vhost 路由)          [TunnelRelay 节点, 127.0.0.1:{vhost_port}]
       ↓
   frp 隧道协议 (Host 头路由)
       ↓
     frpc (Client, 多进程)                  [内网服务器]
       ↓
   内网服务 (192.168.x.x:port)
```

**关键特性**：

* frps 使用 HTTP Vhost 单端口复用机制，所有 HTTP 隧道共享一个 `vhost_port`，通过 Host 头自动路由到对应 frpc。
* Agent 保留原始 `Host` 请求头，frps 依据此头进行虚拟主机匹配。
* 每个隧道对应一条 `proxy_routes`，可绑定多个域名。
* OpenFlared 客户端为每个连接的 Relay 管理一个独立的 frpc 进程，通过单一 frp 隧道传输多个 HTTP 代理定义。

### 配置同步模型

发布流程同时生成两类配置版本数据，统一使用 `config_version` 版本号关联：

* **Agent 侧配置**：OpenResty 主配置 + 路由配置 + WAF 规则。包含 tunnel 上游时，自动渲染为 `http://127.0.0.1:{vhost_port}` 上游。
* **Tunnel 侧配置**：Relay 列表 + frpc 代理定义。随发布流程版本化，变更时优先使用 `frpc reload` 热重载。
* **Relay 配置**：通过心跳响应下发，相对静态，不纳入版本化流程。

### 当前阶段约束

* 仅支持 HTTP 协议隧道流量，保留未来 TCP/UDP 隧道扩展性。
* Tunnel 类型上游的域名 DNS 应仅解析到 TunnelRelay 节点；EdgeNode 上对应请求会因 frps 不可达返回 502。
* frp 版本使用 v0.61+（或更新稳定版），frp 二进制由部署脚本或 Docker 镜像提供。
* 暂不支持 TCP/UDP 端口分配；HTTP 单端口复用已满足 MVP 需求。


## HTTPS 约束

`proxy_routes.domain_cert_ids` 用于记录与 `domains` 平行的域名证书绑定；值为 `0` 表示该域名不启用 HTTPS，仅保留 HTTP。

发布渲染时：

* 带证书的域名按证书分组输出独立 `443 ssl` `server` 块。
* 未绑定证书的域名不得被自动带入 HTTPS。
* 必须将 `proxy_routes.domains` 中的全部域名一并纳入同一站点配置，避免同站点在版本快照中被拆散。

## WAF 约束

WAF 以规则组为配置边界。系统固定一个全局规则组，默认应用到所有网站；网站可叠加多个自定义规则组。

一期支持：

* IP / IP 段白名单与黑名单。
* IP 组引用，支持手动、自动、订阅三类 IP 组。
* 国家级地域白名单与黑名单。
* 规则组级拦截状态码与响应页面，默认 `418` 与空页面。

IP 组约束：

* 手动 IP 组由管理端直接维护 IP/IP 段列表。
* 自动 IP 组使用 Expr 语法保存自定义规则，由 Server 定时按单个 IP 聚合请求日志并更新 IP 列表。
* 订阅 IP 组由 Server 定时从 HTTP/HTTPS URL 同步，支持文本列表和 JSON 映射。
* WAF 运行时不访问数据库；发布版本只保存规则组引用的 IP 组 ID，不把 IP 组成员展开进版本快照。
* Agent 通过心跳上报本地 IP 组 checksum，Server 仅返回 checksum 不一致的 IP 组；Server 侧 IP 组更新时会通过 Agent WebSocket 主动广播变更组，使节点可在不重新发布配置版本的情况下更新 WAF IP 组内容。

自动 IP 组首批内置预设规则：

* 单个 IP 请求数大于 100，且 404 状态码占比不低于 80%：`request_count > 100 && status_404_ratio >= 0.8`
* 单个 IP 通过 IP 地址访问次数大于 50，且通过 IP 地址访问占比大于 50%：`ip_host_count > 50 && ip_host_ratio > 0.5`

判定顺序：

* 白名单是放行例外，任意启用规则组命中白名单即放行。
* 未命中白名单时继续判断黑名单。
* 多个黑名单命中时，全局规则组优先，其后按自定义规则组 ID 升序。

地域识别由 Agent 维护节点本地 MaxMind mmdb，OpenResty Lua 在请求路径中读取本地库。GeoIP 依赖不可用时只能跳过地域规则，不得影响 IP 规则与反向代理主链路。

## 认证源约束

`auth_sources` 是管理端第三方登录入口的配置对象，当前仅支持 `github` 与 `oidc` 两类。启用后的认证源会显示在登录页。

`external_accounts` 保存认证源外部账号与本地用户的绑定关系。第三方账号首次登录时：

* 已绑定本地用户则直接登录。
* 当前已有本地登录 Session 时，绑定到当前用户。
* 未绑定且允许注册时，自动创建普通用户并绑定。
* 未绑定且关闭注册时，只允许用户输入已有本地账号密码完成绑定。

旧 `users.github_id` 仅作为升级迁移来源，新的第三方账号登录与绑定关系必须以 `external_accounts` 为准。

## 版本与观测约束

* `config_versions` 必须保存完整快照、渲染结果与 `checksum`。
* 全局同时只能有一个激活版本。
* 回滚通过重新激活旧版本实现。
* `nodes` 只承载控制面状态与低频摘要，不承载高频观测事实。
* 指标、趋势和访问分析优先使用服务端聚合结果，而不是前端临时统计。
* 访问明细只保留受控时间窗口，不演变成通用日志平台。

## 文档维护原则

* 产品范围或系统边界变化时更新本文档。
* 系统结构或模块职责变化时更新 [系统架构](./architecture.md)。
* 发布、同步、回滚模型变化时更新 [发布模型](./release-model.md)。
* 开发约束、代码规范、接口约定变化时更新 [开发约束](../guildline/development-constraints.md)。
* 部署方式变化时更新 [部署说明](../reference/deployment.md) 与 README。
* 配置项变化时更新 [配置项参考](../reference/configuration.md)。
* 已完成阶段不再以“版本计划”形式回填。
* 新阶段开始前，先补设计，再进入实现。
