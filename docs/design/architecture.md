# 系统架构

你会学到：OpenFlare 的整体架构、各核心组件（Server, Agent, OpenResty, Relay, Client）的职责分工，以及主要数据与请求流的宏观流向。

OpenFlare 是一套自托管的 OpenResty 控制面。它在物理上由 Server（控制面）、Agent（配置落地端）、节点本地 OpenResty（数据面）、内网穿透组件（Relay 与 OpenFlared，数据面扩展）以及管理端前端组成。

---

## 流量路径概览

根据不同的网站上游类型，OpenFlare 支持三种不同的数据面流量路径：

### 1. 标准反代流量路径
```text
Browser
  |
  | HTTPS/HTTP request
  v
OpenResty (WAF, TLS, Rate Limit)
  |
  | reverse proxy (proxy_pass)
  v
Origin Server (直连公网/局域网上游)
```

### 2. 内网穿透流量路径
适用于内网受限服务器上的源站服务接入：
```text
Browser
  |
  | HTTPS/HTTP request
  v
OpenResty (Agent 宿主机, TLS/WAF)
  |
  | proxy_pass http://localhost:vhost_port (Host header preserved)
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
OpenResty (Agent, TLS/WAF)
  |
  +---> [静态服务] root/try_files ---> Agent 本地 Pages 部署目录
  |
  +---> [API 反代] proxy_pass ---> 后端 API 服务 (如果启用了 API 代理)
```

---

## 组件职责

| 组件            | 职责                                                                   | 详细设计参考 |
| --------------- | ---------------------------------------------------------------------- | ------------ |
| **Server**      | 管理端 UI/API、控制面状态持久化、配置编译渲染、发布版本控制、Pages 部署包存储、Uptime Kuma 监控同步与登录验证码防护 | [Agent 与发布模型](./agent-design.md) / [Uptime Kuma 监控同步设计](./kuma-design.md) / [登录验证码设计](./login-captcha.md) |
| **Agent**       | 周期心跳与 WS 同步、静态资源包拉取与解压、OpenResty 配置写入/校验/重载与自愈 | [Agent 与发布模型](./agent-design.md) |
| **OpenResty**   | 接收真实流量，执行 WAF 过滤、PoW 防护、Basic Auth 认证与静态/反代服务 | [WAF 设计](./waf-design.md) / [Pages 设计](./pages-design.md) |
| **Relay**       | 部署于边缘节点，管理 `frps` 守护进程生命周期，接受心跳派发的穿透中继配置 | [内网穿透设计](./tunnel-design.md) |
| **OpenFlared**  | 部署于内网，管理 `frpc` 进程组，向多个 Relay 建立反向隧道，上报连接状态 | [内网穿透设计](./tunnel-design.md) |

---

## 组件架构与分工

### 1. Server (控制面)
仓库根目录的 Go 后端（模块 `github.com/Rain-kl/Wavelet`）是 OpenFlare 控制面，基于 Wavelet 全栈脚手架构建：
* 提供管理端 REST API（`/api/v1/d/*`），通过 **Session Cookie** 鉴权，可选 `X-Access-Token` 访问令牌。
* 边缘节点协议走 `/api/v1/agent|relay|tunnel/*`，分别使用 `X-Agent-Token` / `X-Tunnel-Token` 鉴权。
* 包含配置编译器（Compiler），将数据库中的规则、证书与全局参数统一编译为不可变的配置快照及 OpenResty 物理配置文件文本。
* 存储 Pages 部署 ZIP 包于本地 Artifacts 目录，并向 Agent 提供受控的下载接口。
* 后台集成 Uptime Kuma 监控同步服务，自动为可用站点维护 HTTP 探测任务。
* 启动入口为根目录 `main.go` + `internal/cmd/`（`api` / `worker` / `scheduler` / `all`）；OpenFlare 业务在 `internal/apps/openflare/`，边缘协议处理在 `internal/apps/openflare/{agent,relay,flared}/`。
* *详细设计请参阅：[Agent 与发布模型设计](./agent-design.md) 以及 [Uptime Kuma 监控同步设计](./kuma-design.md)*

### 2. Agent (配置落地端)
`openflare-agent` 是运行在节点本地的守护进程：
* 启动后维持与控制面的周期性心跳，并通过可选的 WebSocket 接收实时的配置发布广播。
* 负责拉取最新激活版本的配置文件及证书，写入本地目录，并通过 `openresty -t` 执行安全校验后平滑重载 (`reload`)。
* 在本地处理 Pages 部署包的下载、SHA-256 校验与解压缩切换。
* *详细设计请参阅：[Agent 与发布模型设计](./agent-design.md)*

### 3. OpenResty (数据面)
接收访客流量并执行最终的业务落地：
* 流量入口，支持 HTTP/2、HTTP/3（QUIC）和 TLS 证书动态绑定。
* 嵌入 Lua 逻辑，在 `access_by_lua` 阶段高效过滤 WAF 规则、验证工作量证明 (PoW) 挑战，并在此之后执行连接数/速率限制及基础缓存。
* *详细设计请参阅：[WAF 设计文档](./waf-design.md) 与 [Pages 静态托管设计文档](./pages-design.md)*

### 4. Relay 与 OpenFlared (穿透组件)
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
  拉取最新 OpenResty 配置/证书             拉取最新 Tunnel 映射配置
  增量拉取/解压 Pages 静态部署包            生成/重写 frpc.toml
  Nginx 校验配置并平滑重载 (reload)          平滑重载或拉起 frpc 进程
  上报应用状态 (Success / Error)           上报隧道连接状态与活跃指标
```
* *同步与自愈的精细时序及回滚模型详见：[Agent 与发布模型设计](./agent-design.md)*

### 2. 静态托管与 API 代理流
* 静态资源解压落地于 Agent 节点的 `deployments/{id}/current` 下，OpenResty 通过 `root`/`index`/`try_files` 指令在边缘直接向访客提供极低延迟的静态资源服务。
* 当启用 API 代理时，OpenResty 自动根据站点配置的 `api_proxy_path`（如 `/api`）将 API 请求重写并转发（`proxy_pass`）给后端动态接口。
* *部署包校验、解压逃逸防御及 Nginx 规则渲染详见：[Pages 静态托管设计文档](./pages-design.md)*

### 3. WAF 安全过滤流
* WAF 引擎嵌入在 OpenResty 请求生命周期中。
* 过滤规则直接从 Agent 落地在节点本地的 `waf_config.json` 及 `waf_ip_groups.json` 读取，判决逻辑白名单优先、黑名单层层过滤，完全在本地内存中完成，不产生数据库或网络 I/O 损耗。
* *IP组增量同步、自动 IP 组计算与拦截响应机制详见：[WAF 设计文档](./waf-design.md)*

---

## 核心对象

当前系统核心实体包括：

* **反代与配置**：`zones` (根域管理边界), `zone_domains` (明确域名与证书/路由关联), `proxy_routes` (路由策略), `origins` (源站), `config_versions` (配置版本), `tls_certificates` (证书). 详见 [Zone 与域名资源设计](./zone-design.md)。
* **Pages 静态托管**：`pages_projects` (Pages项目), `pages_deployments` (不可变部署), `pages_deployment_files` (部署文件清单).
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
| 内网穿透基于 frp 整合          | 复用成熟隧道协议，避免自研隧道引起稳定性风险；其 Vhost 机制天然适配反代路由 |
| 运行时配置与控制库解耦         | 如 WAF 运行时只读取本地 JSON 规则包，配置变更通过差分广播或快速重载热生效    |

---

## 贡献者阅读建议

修改系统架构或开发新功能前，请按以下顺序阅读：

1. **[产品边界](./index.md)**：了解 OpenFlare 核心定位与不允许逾越的设计边界。
3. **[Agent 与发布模型](./agent-design.md)**：理解版本快照同步及失败回滚的安全兜底逻辑。
4. **细分领域设计**：
   * Zone 与域名相关开发：阅读 [Zone 与域名资源设计](./zone-design.md)。
   * 穿透相关开发：阅读 [内网穿透隧道设计](./tunnel-design.md)。
   * WAF 相关开发：阅读 [WAF 设计](./waf-design.md)。
   * Pages 托管开发：阅读 [Pages 静态托管设计](./pages-design.md)。
   * 监控同步开发：阅读 [Uptime Kuma 监控同步设计](./kuma-design.md)。
5. **[仓库结构](./index.md#仓库结构)**：明确各个物理目录分层职责，避免堆砌和重复开发。
