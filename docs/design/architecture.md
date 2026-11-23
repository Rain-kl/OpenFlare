# 系统架构

OpenFlare 由 Server 控制面、OpenFlare Edge 数据面、原生 Rust 穿透组件 `openflare-relay` 与 `openflared`，以及管理端前端组成。边缘同步、校验和运行时应用流程见 [边缘节点设计](./pingora-edge-design.md)。

---

## 流量路径概览

根据不同的网站上游类型与协议形态，OpenFlare 支持四类核心数据面流量路径：

### 1. 标准反向代理流量路径
```text
Browser
  |
  | HTTPS/HTTP request
  v
OpenFlare Edge (WAF, TLS, Rate Limit, 可选源站错误页)
  |
  | reverse proxy (反向代理)
  v
Origin Server (直连公网/局域网上游)
```

源站或网关返回配置列表内错误状态码时，可返回全局自定义/默认 HTML，且保持真实 HTTP 状态码；详见 [源站错误页设计](./origin-error-page.md)。

### 2. L7 内网穿透流量路径（OpenFlare Edge + Relay Gateway）
适用于受 NAT、防火墙或无公网 IP 限制的内网 Web 与 WebSocket 服务接入：
```text
Browser
  |
  | HTTPS/HTTP / WS request
  v
OpenFlare Edge (同步 tunnel-routing.v1, 执行 TLS / WAF / PoW)
  |
  | HTTP/WS forward (附带 X-OpenFlare-Gateway-Auth, 就近选择健康 Relay)
  v
OpenFlare Relay Gateway (:8443)      <-- 独立中继节点或与 Edge 同机部署
  |
  | 经本机内嵌 rathole server 动态转发，支持多 Connector 副本自动故障转移
  | (底层隧道传输支持 Noise :7443 / WebSocket :7444 / TCP)
  v
OpenFlared 内嵌 rathole client      <-- 内网环境（可部署多副本高可用）
  |
  | 内嵌 L7 Ingress（执行 allow/block CIDR SSRF 校验）
  v
Internal Service (HTTP / HTTPS / WS 内网源站)
```

### 3. L4 TCP/UDP 内网穿透流量路径
适用于内网 SSH、数据库或私有二进制协议端口映射：
```text
TCP / UDP Client
  |
  v
openflare-relay (监听指定的公开 TCP/UDP 服务端口)
  |
  | 内嵌 rathole 加密长连接 (Noise / WebSocket / TCP)
  v
openflared (执行 allow/block CIDR SSRF 校验)
  |
  v
Internal TCP / UDP Target
```

### 4. Pages 静态托管流量路径
适用于预构建的单页应用（SPA）或静态网站托管：
```text
Browser
  |
  | HTTPS/HTTP request
  v
OpenFlare Edge (Agent, TLS/WAF)
  |
  +---> [静态服务] 读取部署文件 ---> Agent 本地 Pages 部署目录
  |
  +---> [API 反代] 反向代理 ---> 后端 API 服务 (如果启用了 API 代理)
```

---

## 组件职责

| 组件 | 职责 | 详细设计参考 |
| --- | --- | --- |
| **Server** | 管理端 UI/API、控制面状态持久化、站点与 `tunnel-routing.v1` 配置编译同步、中继拓扑调度、Pages 部署包存储、Cloudflare A 记录指向、访问日志入库与业务流量聚合、Uptime Kuma 监控同步与登录验证码防护 | [边缘节点设计](./pingora-edge-design.md) / [内网穿透设计](./tunnel-design.md) / [Cloudflare DNS 指向设计](./cloudflare-pointing.md) / [边缘可观测与业务流量统计](./observability-design.md) / [Uptime Kuma 监控同步设计](./kuma-design.md) |
| **OpenFlare Edge** | 接收真实流量，执行 WAF 过滤、PoW 防护、Basic Auth 认证、静态/直连反向代理与 `tunnel-routing.v1` 穿透回源调度 | [WAF 设计](./waf-design.md) / [Pages 设计](./pages-design.md) / [源站错误页设计](./origin-error-page.md) |
| **OpenFlare Relay** | 部署于公网中继池的原生 Rust 守护进程，内嵌 `rathole` server 与 Relay Gateway（`:8443`），支持同 Edge 共置、排水迁移、RTT 探活与双槽位自更新 | [内网穿透设计](./tunnel-design.md) |
| **OpenFlared** | 部署于内网的原生 Rust 连接器，自动发现跨 `zone_tag` 最优中继、执行 Make-Before-Break 重平衡、内嵌 `rathole` client 与带 CIDR SSRF 防护的 L4/L7 转发器 | [内网穿透设计](./tunnel-design.md) |

---

## 组件架构与分工

### 1. Server (控制面)
仓库根目录的 Go 后端（模块 `github.com/Rain-kl/Wavelet`）是 OpenFlare 控制面，基于 Wavelet 全栈脚手架构建：
* 提供管理端 REST API（`/api/v1/d/*`），通过 **Session Cookie** 鉴权，可选 `X-Access-Token` 访问令牌。
* 边缘节点与穿透协议分别挂载于 `/api/v1/agent/*`（`X-Agent-Token`）、`/api/v1/relay/*`（`X-Relay-Bootstrap-Token` / `X-Relay-Token`）与 `/api/v1/tunnel/*`（`X-Tunnel-Token`）。
* 包含配置编译器（Compiler），将数据库中的规则、证书与全局参数统一编译为不可变的 `sites.v1` 快照，同时面向 OpenFlare Edge 下发 `tunnel-routing.v1` 动态隧道网关路由清单。
* 统一接收 Pages 本地上传、Remote URL 与公开 GitHub Release 预构建产物，完成来源检查、受限下载、归档校验和不可变 deployment。
* 提供可选的 Cloudflare DNS 指向控制面：以 ZoneDomain 为成员维护分组期望状态，通过 Asynq 将单条 A 记录幂等同步到当前生效节点 IPv4。
* 后台集成 Uptime Kuma 监控同步服务，自动为可用站点维护 HTTP 探测任务。
* 启动入口为 `cmd/server/main.go` + `cmd/server/cli/`（`api` / `worker` / `scheduler` / `all`）；OpenFlare 业务在 `internal/apps/openflare/`，穿透控制面域在 `internal/apps/openflare/tunnel/`，边缘 Data Plane 源码分别位于 `edge/pingora/`（Edge 网关）与 `tunnel/`（Relay 与 OpenFlared 工作区）。
* *详细设计请参阅：[边缘节点设计](./pingora-edge-design.md) 以及 [内网穿透设计](./tunnel-design.md)*

### 2. OpenFlare Edge（数据面）
`openflare-edge` 管理监听器、快照、Pages 资源及 `tunnel-routing.v1` 穿透路由清单，在同一运行时执行 HTTP 代理、HTTP/2、TLS、WAF、PoW、站点身份验证与缓存。节点通过 WebSocket 接收配置变更事件，再按清单拉取站点快照、IP 组、性能策略与隧道中继列表。

### 3. OpenFlare Relay 与 OpenFlared（Rust 穿透数据面）
位于 `tunnel/` Cargo 工作区（包含 `openflare-tunnel-common`、`openflare-relay` 与 `openflared` 三个 crate）：
* `openflare-relay` 内嵌 `rathole` 服务端与 HTTP/WS Relay Gateway，管理 L4/L7 隧道会话与健康检查。
* `openflared` 支持多连接器并发接入同一隧道，持续评估池内中继 RTT 与容量，采用 Make-Before-Break 策略维持高可用穿透隧道，并在本地内嵌执行 L7 HTTP/WS 与 L4 TCP/UDP 转发及 CIDR 防护。
* *详细设计请参阅：[内网穿透隧道设计文档](./tunnel-design.md)*

---

## 数据与请求流概览

### 1. 配置保存与同步流程
```text
管理端保存配置 -> 自动编译并更新站点快照 (sites.v1) / 隧道拓扑与网关清单 (tunnel-routing.v1)
                                 |
              +------------------+------------------+
              | (WebSocket 推送或周期 Heartbeat)      |
              v                                     v
     [OpenFlare Edge 节点]                     [Relay / OpenFlared]
  按摘要拉取站点快照与隧道网关清单             自动发现池内最优中继与服务清单
  校验并原子切换内存路由快照                 进程内热更新 rathole server/client 配置
  上报应用状态与可观测明细日志               上报连接器租约、RTT 与活跃流数量
```
* *同步与自愈的精细时序及回滚模型详见：[边缘节点设计](./pingora-edge-design.md)*

### 2. 静态托管与 API 代理流
* 静态资源解压落地于 Agent 节点的 `projects/{project_id}/current` 下（按项目 latest 拉取，仅保留最新包），OpenFlare Edge 从本地部署文件提供静态服务。
* 当启用 API 代理时，OpenFlare Edge 根据站点配置的 `api_proxy_path`（如 `/api`）将 API 请求重写并转发（`反向代理`）给后端动态接口。
* *部署包校验、解压逃逸防御及请求路由详见：[Pages 静态托管设计文档](./pages-design.md)*

### 3. WAF 安全过滤流
* WAF 引擎在 OpenFlare Edge 请求阶段运行。
* WAF 规则由控制面以可视化 DAG 编排，发布时编译为运行态图；节点载入后通过内存对象执行。
* *IP 组来源与同步机制详见：[WAF 设计文档](./waf-design.md)；图模型、执行语义与发布约束详见：[WAF 可编排规则设计](./waf-orchestration-design.md)。*

### 4. 边缘可观测与业务流量统计流
* 业务流量唯一真相为 OpenFlare Edge 访问日志；节点上报明细与主机采样后由 Server 经 `internal/repository/logstore` 统一入库与聚合。
* *传输模型、示例与采集频率详见：[观测数据传输模型](./observability-transport-model.md)；字段收敛与迁移详见：[边缘可观测与业务流量统计](./observability-design.md)*

### 5. Cloudflare DNS 指向流
* 管理员配置连接/分组/成员后，Server 持久化期望状态并通过 Asynq 幂等维护 Cloudflare 单条 A 记录。
* *连接、模型、幂等同步与分期边界详见：[Cloudflare DNS 指向设计](./cloudflare-pointing.md)。*

---

## 核心对象

当前系统核心实体包括：

* **反向代理与配置**：`zones` (根域管理边界), `zone_domains` (明确域名与证书/路由关联), `proxy_routes` (路由策略), `origins` (源站), `config_versions` (配置版本), `tls_certificates` (证书). 详见 [Zone 与域名资源设计](./zone-design.md)。
* **Cloudflare DNS 指向**：`of_cf_connections` (全局连接), `of_cf_pointing_groups` (主/备/生效节点与默认橙云), `of_cf_pointing_members` (ZoneDomain 成员、记录缓存与同步状态). 详见 [Cloudflare DNS 指向设计](./cloudflare-pointing.md)。
* **Pages 静态托管**：`of_pages_projects` (Pages 项目), `of_pages_project_sources` / `of_pages_project_source_runtime` (可变来源配置与运行态), `of_pages_deployments` (不可变部署), `of_pages_deployment_files` (部署文件清单).
* **边缘节点与内网穿透**：`nodes` (Edge 节点), `w_tunnel_relay_pools` (中继池), `w_tunnel_relays` (中继实例), `w_tunnels` (逻辑隧道), `w_tunnel_connectors` (内网连接器副本), `w_tunnel_attachments` (隧道-中继附着拓扑), `w_tunnel_services` (L4/L7 转发服务).
* **WAF 与安全**：`waf_rule_groups` (WAF 规则组), `waf_ip_groups` (WAF IP 组), `waf_rule_group_bindings` (网站 WAF 绑定).
* **系统与账号**：`acme_accounts` (ACME 账户), `dns_accounts` (DNS 账户), `geoip_update_configs` (GeoIP 更新配置).

---

## 关键设计决策

| 决策 | 原因 |
| --- | --- |
| 完整站点快照 | 为校验、原子切换和本地恢复提供一致的边界 |
| Agent 主动拉取 | Server 不需要 SSH 权限，降低安全风险；支持 HTTP 与 WebSocket 双协议灵活切换 |
| 全局单激活版本 | 降低控制面复杂度，保证所有节点默认一致；提供一键秒级回滚的稳定机制 |
| Zone 域名与路由策略分离 | Zone 提供根域入口与域名边界；路由仍可复用同一套站点级策略并按域名绑定证书 |
| Cloudflare 指向独立于 Zone 核心 | ZoneDomain 只提供明确 FQDN；Cloudflare 模块以库表期望状态驱动单 A 记录，不扩大 Zone 为通用 DNS 控制面 |
| 原生 Rust 工作区内嵌 rathole 隧道 | 彻底废除外部 `frp` 二进制包装，原生支持 Noise/WebSocket/TCP 传输回退、跨区 Closest-K 容灾、CIDR SSRF 防护与 L4/L7 双协议转发 |
| 运行时配置与控制库解耦 | WAF 规则发布时编译并随站点快照原子加载；动态 IP 组与 `tunnel-routing.v1` 通过独立清单和摘要同步刷新 |
| 业务流量以访问日志为唯一真相 | Agent 禁止业务预聚合；看板与 Zone 共用 Server 侧聚合 |
| Pages artifact 与仓库构建分离 | 现有来源只导入预构建产物；Agent 不执行第三方构建 |
