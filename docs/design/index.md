# 产品边界

你会学到：OpenFlare 是什么、当前稳定能力，以及开发时应遵守的核心产品边界与仓库结构目录分工。

OpenFlare 是一套自托管的 OpenResty 控制面，面向单团队或单组织内部运维场景。

---

## 项目定位

OpenFlare 适合需要统一管理多台 OpenResty 代理节点的团队，具备以下定位：
* **控制与落地分离**：Server 控制面不直接 SSH 到代理节点，而是通过 Agent 主动拉取版本并应用。
* **不可变配置发布**：采用完整的配置版本进行预览、发布、激活和一键回滚。
* **一体化网关托管**：在同一个控制面内集成网站反代、TLS 证书自动续期申请、WAF 防护拦截、内网穿透（Tunnel）以及 Pages 静态网站托管。

**非本产品定位**：多租户云平台、Kubernetes Ingress Controller、服务网格或通用日志平台。

---

## 当前能力

| 能力 | 说明 | 详细设计/使用指南 |
| --- | --- | --- |
| **反代配置管理** | 以网站规则（Proxy Route）为聚合边界，支持多域名与多上游负载均衡 | [新建反代配置](../guide/proxy-config.md) |
| **边缘缓存** | 单节点 OpenResty `proxy_cache`；默认 static 扩展名 + 源站头/Set-Cookie 闸门 + 默认 Edge TTL（对标 CF 默认模型） | [边缘缓存策略设计](./edge-cache-design.md) |
| **Zone 与域名管理** | 以可注册根域为管理入口，聚合明确域名、域名证书与反代路由 | [Zone 与域名资源设计](./zone-design.md) |
| **配置版本控制** | 支持全局单一激活版本的预览、发布、不可变快照历史与秒级一键回滚 | [Agent 与发布模型](./agent-design.md) |
| **WAF 安全防护** | 支持可视化 DAG 编排规则、手动/自动/订阅型 IP 组、GeoIP 匹配与 PoW CC 防护 | [WAF 设计](./waf-design.md) / [WAF 可编排规则设计](./waf-orchestration-design.md) / [WAF 使用指南](../guide/waf-usage.md) |
| **内网穿透** | 通过中继节点（Relay）与内网客户端（OpenFlared），反向穿透暴露内网 Web 服务 | [内网穿透设计](./tunnel-design.md) / [穿透使用指南](../guide/tunnel-usage.md) |
| **Pages 静态托管** | 支持上传或从 Remote URL、公开 GitHub Release 同步预构建产物；GitHub latest 可定时检查并可选自动发布。不可变部署由边缘节点拉取并由 OpenResty 本地服务，支持回滚、API 反代与 SPA Fallback | [Pages 静态托管设计](./pages-design.md) / [Pages 使用指南](../guide/pages-usage.md) |
| **TLS 证书自动续期** | 将证书显式绑定到 Zone 域名，并通过 ACME 协议向 Let's Encrypt 申请/续期证书 | [Zone 与域名资源设计](./zone-design.md) |
| **多节点监控与观测** | 访问日志为业务流量唯一真相；Agent 只上报明细与主机读数，Server 统一聚合；与 Zone/看板对账 | [观测数据传输模型](./observability-transport-model.md) / [边缘可观测与业务流量统计](./observability-design.md) / [上报协议与表结构](./observability-data-model.md) / [系统架构](./architecture.md) |

---

## 核心产品边界与约束

在开发与贡献代码时，**必须严格遵守**以下业务边界与技术约束，禁止为了临时需求而绕过限制：

### 1. 网站配置与上游约束
* **单站点域名共享策略**：一条路由规则对应一个网站，该站点下的多域名共享限流、缓存与反代上游等配置，不支持在同一规则内为不同域名做差异化服务配置。
* **上游类型互斥**：上游必须是直连地址（`direct`）、内网穿透（`tunnel`）或 Pages 静态托管（`pages`）三者之一，不允许在同一规则中混用。
* **直连类型限制**：直连上游可以是纯 `http://` 或 `https://` 的单个或多个地址（多地址仅支持纯 `scheme://host[:port]`），不支持非 HTTP 协议（如 TCP/UDP）上游。

### 2. WAF 安全边界
* **白名单优先原则**：白名单拥有绝对匹配权。若未命中白名单规则，才依次触发全局和自定义黑名单过滤。
* **GeoIP 弱依赖性**：地域准入解析完全依赖节点本地 MaxMind 库。当 GeoIP 异常或解析失败时，系统必须自动忽略地域规则，**绝对不能**破坏 IP 组过滤和反代主链路的可用性。
* **运行时数据解耦**：OpenResty 拦截时仅读取 Agent 同步至本地的 JSON，不与 Server 数据库通信。IP 组成员同步与版本发布解耦，通过 Checksum 差分拉取以实现零重载平滑生效。

### 3. 内网穿透边界
* **仅限 HTTP 流量**：穿透组件仅支持 HTTP/HTTPS 协议（底层依靠 frp 虚拟主机 Vhost 机制实现单端口域名路由复用），暂不支持单独的 TCP/UDP 端口分配。
* **中继配置动态化控制**：中继节点（Relay）在连接至 Server 后，可通过心跳周期性动态拉取并同步全局系统配置（例如是否开启内嵌 FRPS Web UI 及其监听端口），但不直接纳入控制面的不可变配置版本发布体系。
* **Tunnel 与 Node 体系隔离**：Tunnel 客户端在内网发起出向建连，与控制面托管的边缘 Node（公网节点）是独立的实体，使用专属的 `tunnel_token` 进行鉴权。

### 4. Pages 静态托管边界
* **预构建产物来源**：项目可保持手动上传，或配置一个 Remote URL / 公开 GitHub Release asset 来源。Remote 与固定 tag 只支持手动操作；只有 GitHub latest 进入定时检查并可选择自动更新。来源可切换，但不可变 deployment 与当前生产版本不会随 source 编辑或删除而丢失。
* **归档与资源上限**：支持 `zip`、`tar.gz` / `tgz`、`tar.xz` / `txz`、`tar.bz2` / `tbz2`、`tar`、`7z`。压缩包上限由 `pages_max_package_size_mb` 控制（默认 100 MiB，范围 1～2048）；展开后的单文件和总量上限为包上限的 4 倍且最低 100 MiB，最多 1,000 个常规文件。Server 与 Agent 都校验实际字节，并拒绝路径逃逸、软/硬链接与特殊文件。
* **构建与运行时边界**：当前不从外部 Git 仓库拉取源码或执行构建，也不提供边缘 Serverless、动态 SSR 或二级预览域名。未来仓库集成必须使用独立 `git_repository` Provider 与 Server 侧隔离 build executor，只向统一 artifact 管线输出受限产物；Agent 不接收仓库凭据、外部 URL 或 clone/install/build 命令。

### 5. 系统与版本边界
* **全局单一激活版本**：所有节点拉取并消费同一份全局激活配置。不进行按节点分组的差异化配置发布。
* **单租户架构**：OpenFlare 仅供单团队在受信任的内部网络部署使用。采用单租户设计，不支持细粒度的多用户角色或多租户资源隔离。
* **外部基础设施依赖性**：Server 虽支持 SQLite 作为本地轻量关系数据库，但**系统必须强制依赖外部 Redis（或 Valkey）及 ClickHouse 实例**。Redis 用于处理分布式协调、后台异步队列（Asynq 框架）及系统级全局缓存；ClickHouse 用于接收海量节点访问日志与基础观测的异步 Flush。系统不支持完全脱离这两个组件运行。

---

## 仓库结构

OpenFlare 已收敛为**单 monorepo**（Go 模块 `github.com/Rain-kl/Wavelet`）。控制面 Server 与边缘组件（Agent、Relay、OpenFlared）共享同一仓库，业务代码按 Wavelet `internal/apps/` 领域模块组织。

在贡献代码时，请严格遵守以下物理分层与目录分工：

| 路径 | 职责 |
| --- | --- |
| `main.go` | Server 唯一入口，委派给 `internal/cmd/` |
| `cmd/agent`、`cmd/relay`、`cmd/flared` | 边缘组件 CLI 入口（**不含** Server） |
| `internal/` | 控制面与边缘运行时实现 |
| `frontend/` | Next.js 管理端，构建产物嵌入 Go Server |
| `pkg/` | 跨组件共享库（协议、渲染、GeoIP 等） |
| `scripts/` | Swagger 生成、安装脚本等 |
| `docs/` | VitePress 文档站与设计基线 |
| `docker/` | 各组件 Dockerfile |
| `uploads/`、`data/` | 运行时上传目录与静态数据（`.gitignore` 忽略） |

### 1. Server 分层（`main.go` + `internal/`）

| 目录 | 职责 |
| --- | --- |
| `main.go` | Server 启动入口 |
| `internal/cmd/` | Cobra 子命令：`api`、`worker`、`scheduler`、`all`（默认融合模式） |
| `internal/bootstrap/` | 跨模块装配：任务 Handler、推送域事件、进程级初始化 |
| `internal/router/` | HTTP 路由注册与全局中间件 |
| `internal/router/v1/openflare/` | OpenFlare 路由注册器（`register_*.go`） |
| `internal/apps/openflare/` | OpenFlare 控制面业务域（`routers.go` + `logics.go`） |
| `internal/apps/{admin,user,oauth,upload,cap,...}/` | Wavelet 平台能力（用户、认证、任务、推送等） |
| `internal/apps/openflare/{agent,relay,flared}/` | **Server 侧**边缘协议处理器（鉴权、心跳、WS） |
| `internal/model/` | GORM 实体（`openflare_*.go` + 平台模型） |
| `internal/db/migrator/goose/` | goose SQL 迁移（PostgreSQL / SQLite / ClickHouse） |
| `internal/repository/` | 平台域数据访问层 |
| `internal/task/` | Asynq 异步任务（Worker + Scheduler） |
| `internal/config/` | Viper 配置加载 |
| `internal/common/` | 统一 API 响应封装（`response/`） |
| `pkg/protocol/` | Relay / Tunnel 共享 HTTP/WS 协议结构 |
| `pkg/render/`、`pkg/geoip/`、`pkg/wsclient/` | OpenResty 配置渲染、GeoIP、WebSocket 客户端 |

**API 路由前缀：**

| 前缀 | 用途 | 鉴权 |
| --- | --- | --- |
| `/api/v1/d/*` | OpenFlare 管理控制台 API | Session Cookie + 可选 `X-Access-Token` |
| `/api/v1/agent/*` | Agent 节点协议 | `X-Agent-Token` |
| `/api/v1/relay/*` | Relay 中继协议 | `X-Agent-Token` |
| `/api/v1/tunnel/*` | Tunnel 客户端协议 | `X-Tunnel-Token` |
| `/api/v1/admin/*` | Wavelet 平台管理 API | 管理员 Session |

### 2. Agent 模块 (`internal/apps/agent/` / `cmd/agent/`)

| 目录/模块                     | 职责                                         |
| ----------------------------- | -------------------------------------------- |
| `cmd/agent/`                  | Agent 命令行启动入口及主函数                 |
| `internal/apps/agent/config/`      | 配置读取与默认值                             |
| `internal/apps/agent/heartbeat/`   | 心跳与版本摘要判断                           |
| `internal/apps/agent/sync/`        | 配置拉取与应用编排                           |
| `internal/apps/agent/nginx/`       | OpenResty 文件写入、校验、reload、启动与回滚 |
| `internal/apps/agent/state/`       | 本地状态与观测补报缓冲                       |
| `internal/apps/agent/httpclient/`  | Server 通信                                  |
| `internal/apps/agent/wsclient/`    | WebSocket 客户端通信                         |
| `internal/apps/agent/protocol/`    | Agent API 协议类型                           |
| `internal/apps/agent/updater/`     | Agent 自更新逻辑                             |
| `internal/apps/agent/logging/`     | 日志处理                                     |
| `internal/apps/agent/observability/`| 可观测性（指标、链路等）                     |
| `internal/apps/agent/geoipdata/`   | GeoIP 数据处理                               |
| `internal/apps/agent/geoipupdate/` | GeoIP 数据更新                               |
| `internal/apps/agent/agent/`       | 核心 Agent 逻辑与生命周期                    |

### 3. Frontend 分层 (`frontend/`)

基于 Wavelet Next.js 脚手架，OpenFlare 业务 UI 以路由共置方式组织在 `app/(main)/` 下。

| 目录 | 职责 |
| --- | --- |
| `app/` | Next.js App Router；`(main)` 控制台、`(auth)` 认证、`(docs)` 文档页 |
| `app/(main)/<domain>/` | 业务页面与域内组件（路由共置） |
| `components/` | 跨域复用 UI（`ui/`、`layout/`、`common/` 等） |
| `lib/services/` | API 服务层：`core/` 基类 + `openflare/` 业务 API |
| `lib/navigation/` | OpenFlare 侧栏导航配置（`openflare-nav.ts`） |
| `lib/theme/` | 主题解析与切换 |
| `contexts/` | 跨页面 UI 状态（用户、通知等） |
| `hooks/`、`lib/hooks/` | 可复用 React Hooks |
| `public/` | 静态资源与主题 CSS |
| `scripts/` | 构建辅助脚本 |
| `proxy.ts` | 开发/生产代理：API 限流与页面鉴权 |

**API 约定**：OpenFlare 业务接口统一前缀 `/api/v1/d/*`，通过 `OpenFlareBaseService` 封装；页面数据获取使用 `@tanstack/react-query`。

### 4. Relay 模块 (`internal/apps/relay/` / `cmd/relay/`)

| 模块             | 职责                                             |
| ---------------- | ------------------------------------------------ |
| `cmd/relay/`     | Relay 命令行启动入口及初始化主函数               |
| `internal/apps/relay/config/`| 本地配置文件解析与默认参数初始化                 |
| `internal/apps/relay/frps/` | 管理 frps 进程生命周期、端口与 Token 并监控运行   |
| `internal/apps/relay/heartbeat/`| 周期性 HTTP 心跳通信、上报状态并获取更新请求  |
| `internal/apps/relay/httpclient/`| Server 的通用 API 客户端调用工具类              |
| `internal/apps/relay/observability/`| 采集本地宿主机、frps 的基础运行指标并进行预聚合 |
| `internal/apps/relay/relay/` | 协调中继的核心生命周期、初始化与清理             |
| `internal/apps/relay/state/` | 本地运行时状态、错误记录与持久化缓存             |
| `internal/apps/relay/updater/`| Relay 升级检查、下载安装与重启机制               |
| `internal/apps/relay/wsclient/`| 与 Server 保持的长连接 WebSocket 双向通信管道     |

### 5. OpenFlared (Client) 模块 (`internal/apps/flared/` / `cmd/flared/`)

| 模块             | 职责                                             |
| ---------------- | ------------------------------------------------ |
| `cmd/flared/`    | Client 命令行启动入口及初始化主函数              |
| `internal/apps/flared/config/`| 本地客户端配置加载与解析                         |
| `internal/apps/flared/flared/`| 内网穿透客户端的核心调度与状态管理机制           |
| `internal/apps/flared/frpc/` | 热重载/动态生成多 Relay 的 `frpc_{relayNodeID}.toml` 并监控 frpc |
| `internal/apps/flared/heartbeat/`| 与控制面进行的心跳通信，包含 Token 校验机制       |
| `internal/apps/flared/httpclient/`| 客户端通用 API 通信（`/api/v1/tunnel/*`）       |
| `internal/apps/flared/sync/`  | 增量拉取最新 Tunnel 路由绑定关系、生成快照并应用  |
| `internal/apps/flared/updater/`| 客户端自更新、新版检查与更新落地逻辑             |
| `internal/apps/flared/wsclient/`| 用于实时监听 Server 端隧道配置变更推送的 WS 信道  |

> **说明**：OpenFlared 无独立 `state/` 包；版本与 checksum 由 `frpc/manager.go` 持久化到 `flared-state.json`。

---

## 文档维护原则

* 产品范围或系统边界变化：更新本文档（[产品边界](./index.md)）。
* 系统结构、组件分工变化：更新 [系统架构](./architecture.md)。
* 发布、同步、回滚与 Agent 模型变化：更新 [Agent 与发布模型](./agent-design.md)。
* 部署方式变化：更新 [部署说明](../deployment/deployment.md) 与 README。
* 配置项变化：更新 [配置项参考](../reference/configuration.md)。
