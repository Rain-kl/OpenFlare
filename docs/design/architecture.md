# 系统架构

OpenFlare 由 Server 控制面、Pingora Edge 数据面、穿透组件 Relay 与 OpenFlared，以及管理端前端组成。边缘同步、校验和运行时应用流程见 [Pingora 边缘节点设计](./pingora-edge-design.md)。

---

## 流量路径概览

根据不同的网站上游类型，OpenFlare 支持三种不同的数据面流量路径：

### 1. 标准反代流量路径
```text
Browser
  |
  | HTTPS/HTTP request
  v
Pingora Edge (WAF, TLS, Rate Limit, 可选源站错误页)
  |
  | reverse proxy (反向代理)
  v
Origin Server (直连公网/局域网上游)
```

源站或网关返回配置列表内错误状态码时，可返回全局自定义/默认 HTML，且保持真实 HTTP 状态码；详见 [源站错误页设计](./origin-error-page.md)。

### 2. 内网穿透流量路径
适用于内网受限服务器上的源站服务接入：
```text
Browser
  |
  | HTTPS/HTTP request
  v
Pingora Edge (Agent 宿主机, TLS/WAF)
  |
  | 反向代理 http://localhost:vhost_port (Host header preserved)
  v
OpenFlareRelay (frps)              <-- 与 Agent 同机部署，提供中继
  |
  | frp tunnel protocol (Host header routing)
  v
OpenFlared (frpc)                  <-- 内网受限服务器
  |
  | HTTP/HTTPS forward
  v
Internal Service (192.168.x.x)
```

### 3. Pages 静态托管流量路径
适用于预构建的单页应用（SPA）或静态网站托管：
```text
Browser
  |
  | HTTPS/HTTP request
  v
Pingora Edge (Agent, TLS/WAF)
  |
  +---> [静态服务] 读取部署文件 ---> Agent 本地 Pages 部署目录
  |
  +---> [API 反代] 反向代理 ---> 后端 API 服务 (如果启用了 API 代理)
```

---

## 组件职责

| 组件            | 职责                                                                   | 详细设计参考 |
| --------------- | ---------------------------------------------------------------------- | ------------ |
| **Server**      | 管理端 UI/API、控制面状态持久化、配置编译渲染、发布版本控制、Pages 部署包存储、Cloudflare A 记录指向、访问日志入库与业务流量聚合、Uptime Kuma 监控同步与登录验证码防护 | [Agent 与发布模型](./agent-design.md) / [Cloudflare DNS 指向设计](./cloudflare-pointing.md) / [边缘可观测与业务流量统计](./observability-design.md) / [Uptime Kuma 监控同步设计](./kuma-design.md) / [登录验证码设计](./login-captcha.md) |
| **Pingora Edge**   | 接收真实流量，执行 WAF 过滤、PoW 防护、Basic Auth 认证、静态/反代服务与可选源站错误页 | [WAF 设计](./waf-design.md) / [Pages 设计](./pages-design.md) / [源站错误页设计](./origin-error-page.md) |
| **Relay**       | 部署于边缘节点，管理 `frps` 守护进程生命周期，接受心跳派发的穿透中继配置 | [内网穿透设计](./tunnel-design.md) |
| **OpenFlared**  | 部署于内网，管理 `frpc` 进程组，向多个 Relay 建立反向隧道，上报连接状态 | [内网穿透设计](./tunnel-design.md) |

---

## 组件架构与分工

### 1. Server (控制面)
仓库根目录的 Go 后端（模块 `github.com/Rain-kl/Wavelet`）是 OpenFlare 控制面，基于 Wavelet 全栈脚手架构建：
* 提供管理端 REST API（`/api/v1/d/*`），通过 **Session Cookie** 鉴权，可选 `X-Access-Token` 访问令牌。
* 边缘节点协议走 `/api/v1/agent|relay|tunnel/*`，分别使用 `X-Agent-Token` / `X-Tunnel-Token` 鉴权。
* 包含配置编译器（Compiler），将数据库中的规则、证书与全局参数统一编译为不可变的配置快照与所需资源。
* 统一接收 Pages 本地上传、Remote URL 与公开 GitHub Release 预构建产物，完成来源检查、受限下载、归档校验和不可变 deployment；manual 上传生成待显式激活的 candidate，持久来源 sync 才 create-or-load 并原子激活。Server 向 Agent 提供受控的 latest 下载接口；内部 scanner 负责 GitHub latest 的限量检查、租约恢复、可选自动发布与孤儿上传记录补偿，通用任务管理入口不能修改该排程。未来仓库源码构建由独立 Server build executor 扩展，Agent 不执行第三方拉取或构建命令。
* 提供可选的 Cloudflare DNS 指向控制面：以 ZoneDomain 为成员维护分组期望状态，通过 Asynq 将单条 A 记录幂等同步到当前生效节点 IPv4；节点 IP 变化只做 best-effort 入队，一期不执行自动故障切换。
* 后台集成 Uptime Kuma 监控同步服务，自动为可用站点维护 HTTP 探测任务。
* 启动入口为 `cmd/server/main.go` + `cmd/server/cli/`（`api` / `worker` / `scheduler` / `all`）；OpenFlare 业务在 `internal/apps/openflare/`，边缘协议处理在 `internal/apps/openflare/{agent,relay,flared}/`，边缘端侧运行时在 `edge/pingora/` 与 `internal/edge/{common,relay,flared}/`。
* *详细设计请参阅：[Agent 与发布模型设计](./agent-design.md) 以及 [Uptime Kuma 监控同步设计](./kuma-design.md)*

### 2. Pingora Edge（数据面）

`openflare-edge` 管理监听器、快照和 Pages 资源，在同一运行时执行 HTTP 代理、HTTP/2、TLS、WAF、PoW、站点身份验证和缓存。节点通过 WebSocket 得知变更，再以心跳清单同步站点快照、IP 组和性能策略。

### 3. Relay 与 OpenFlared (穿透组件)
扩展数据面反穿透能力：
* `openflare-relay` 守护本地 `frps`，接受 Server 的配置派发，自动更新中继端口。
* `openflared` 在内网守护一组 `frpc` 客户端进程，实现多中继就近建连与高可用容灾。
* *详细设计请参阅：[内网穿透隧道设计文档](./tunnel-design.md)*

---

## 数据与请求流概览

### 1. 配置发布与同步流
```text
管理端修改配置 -> 发布新版本 -> 生成全局唯一 Checksum 激活版本
                                 |
              +------------------+------------------+
              | (WebSocket 广播或周期 Heartbeat)      |
              v                                     v
       [边缘节点 Agent]                        [内网 OpenFlared]
  按哈希拉取站点快照与证书             拉取最新 Tunnel 映射配置
  增量拉取/解压 Pages 静态部署包            生成/重写 frpc.toml
  校验并原子切换内存快照          平滑重载或拉起 frpc 进程
  上报应用状态 (Success / Error)           上报隧道连接状态与活跃指标
```
* *同步与自愈的精细时序及回滚模型详见：[Agent 与发布模型设计](./agent-design.md)*

### 2. 静态托管与 API 代理流
* 静态资源解压落地于 Agent 节点的 `projects/{project_id}/current` 下（按项目 latest 拉取，仅保留最新包），Pingora 从本地部署文件提供静态服务。
* 当启用 API 代理时，Pingora 根据站点配置的 `api_proxy_path`（如 `/api`）将 API 请求重写并转发（`反向代理`）给后端动态接口。
* 管理员操作和内部 scanner 都只生成受约束的 artifact candidate，并复用统一 inspect、`upload.Ingest` 与 deployment pipeline。manual 上传创建新的未激活 candidate；持久来源 sync/scanner 才 create-or-load 并原子激活。未来 repository build executor 也只能向同一 artifact pipeline 输出产物；Agent 始终只是 active deployment 消费者。
* *部署包校验、解压逃逸防御及请求路由详见：[Pages 静态托管设计文档](./pages-design.md)*

### 3. WAF 安全过滤流
* WAF 引擎在 Pingora 请求阶段运行。
* WAF 规则由控制面以可视化 DAG 编排，发布时编译为运行态图；节点载入后通过内存对象执行。
* 全局规则固定前置，路由绑定规则按显式顺序执行；当前规则抵达“通过”后继续下一条，抵达“阻止”则立即返回该节点配置的拦截响应。
* IP 组成员独立热更新：控制端通知节点重新核对哈希，节点只更新有变化的 IP 组。
* *IP 组来源与同步机制详见：[WAF 设计文档](./waf-design.md)；图模型、执行语义与发布约束详见：[WAF 可编排规则设计](./waf-orchestration-design.md)。*

### 4. 边缘可观测与业务流量统计流
```text
Pingora 访问日志（业务事实）
        |
        | 节点通过心跳上报日志明细
        v
Server 经 logstore 入库（当前日志主库：PostgreSQL / SQLite / ClickHouse）
        |
        +---> 全局聚合 --> 看板「已提供数据 / 请求 / UV」
        +---> host∈Zone --> Zone「已提供数据」等（同一套语义）
        +---> node_id 过滤 --> 节点业务量

主机 /proc 网卡与 CPU 等 --> Agent 读数快照 --> 宿主机资源趋势（与业务交付分开展示）
Pingora 健康与连接数 --> 边缘健康（瞬时，不作 24h 业务总量）
```
* **原则**：Agent 只上报事实，Server 解释事实；业务流量唯一真相为访问日志。`openresty_tx` 与「已提供数据」不得双轨并存。
* *传输模型、示例与采集频率详见：[观测数据传输模型](./observability-transport-model.md)；字段收敛与迁移详见：[边缘可观测与业务流量统计](./observability-design.md)*

### 5. Cloudflare DNS 指向流

```text
管理员配置连接/分组/成员 -> Server 持久化期望状态 -> Asynq 同步任务
                                                        |
                                                        v
                                              Cloudflare Zone / DNS API
                                                        |
                                                        v
                                    单条 A 记录 -> active_node IPv4

节点 IP 手动更新或 Agent 心跳变化 --------------------> 按节点 best-effort 入队
```

* Cloudflare 模块只管理其缓存或接管的唯一同名 A 记录，不把 Zone 核心扩展为权威 DNS 控制面；同名多 A 时停止同步并要求管理员先在 Cloudflare 清理。
* 分组备用节点与生效节点为后续故障切换预留，一期固定使用主节点，不根据心跳离线状态自动切换。
* *连接、模型、幂等同步与分期边界详见：[Cloudflare DNS 指向设计](./cloudflare-pointing.md)。*

---

## 核心对象

当前系统核心实体包括：

* **反代与配置**：`zones` (根域管理边界), `zone_domains` (明确域名与证书/路由关联), `proxy_routes` (路由策略), `origins` (源站), `config_versions` (配置版本), `tls_certificates` (证书). 详见 [Zone 与域名资源设计](./zone-design.md)。
* **Cloudflare DNS 指向**：`of_cf_connections` (全局连接), `of_cf_pointing_groups` (主/备/生效节点与默认橙云), `of_cf_pointing_members` (ZoneDomain 成员、记录缓存与同步状态). 详见 [Cloudflare DNS 指向设计](./cloudflare-pointing.md)。
* **Pages 静态托管**：`of_pages_projects` (Pages项目), `of_pages_project_sources` / `of_pages_project_source_runtime` (可变来源配置与运行态), `of_pages_deployments` (不可变部署), `of_pages_deployment_files` (部署文件清单).
* **节点与穿透**：`nodes` (节点), `tunnels` (隧道客户端), `node_system_profiles` (系统概况), `apply_logs` (应用日志).
* **WAF 与安全**：`waf_rule_groups` (WAF规则组), `waf_ip_groups` (WAF IP组), `waf_rule_group_bindings` (网站WAF绑定).
* **系统与账号**：`acme_accounts` (ACME账户), `dns_accounts` (DNS账户), `geoip_update_configs` (GeoIP更新配置).

---

## 关键设计决策

| 决策                           | 原因                                                                        |
| ------------------------------ | --------------------------------------------------------------------------- |
| 完整配置版本，而不是在线 patch | 让预览、激活、历史和回滚有稳定边界，保证节点状态一致                        |
| Agent 主动拉取                 | Server 不需要 SSH 权限，降低安全风险；支持 HTTP 与 WebSocket 双协议灵活切换 |
| 全局单激活版本                 | 降低控制面复杂度，保证所有节点默认一致；提供一键秒级回滚的稳定机制           |
| Zone 域名与路由策略分离        | Zone 提供根域入口与域名边界；路由仍可复用同一套站点级策略并按域名绑定证书        |
| Cloudflare 指向独立于 Zone 核心 | ZoneDomain 只提供明确 FQDN；Cloudflare 模块以库表期望状态驱动单 A 记录，不扩大 Zone 为通用 DNS 控制面 |
| 内网穿透基于 frp 整合          | 复用成熟隧道协议，避免自研隧道引起稳定性风险；其 Vhost 机制天然适配反代路由 |
| 运行时配置与控制库解耦         | WAF 规则发布时编译并随站点快照原子加载；动态 IP 组通过 checksum 驱动的内存快照独立刷新 |
| 业务流量以访问日志为唯一真相   | Agent 禁止业务预聚合；看板与 Zone 共用 Server 侧聚合，避免 openresty_tx 与 bytes_sent 双轨 |
| 业务交付 / 边缘健康 / 主机资源分层 | 已提供数据≠宿主机网卡出站≠Pingora 连接数，UI 与 API 分名分区 |
| Pages artifact 与仓库构建分离 | 现有来源只导入预构建产物；未来 checkout/build 由 Server 隔离 executor 完成并复用 artifact pipeline，Agent 不执行第三方构建 |

---
