# 产品边界

你会学到：OpenFlare 是什么、当前稳定能力，以及开发时应遵守的核心产品边界与仓库结构目录分工。

OpenFlare 是一套新一代分布式反向代理与边缘安全网关平台，面向多服务器集群与单团队运维场景。

---

## 项目定位

OpenFlare 适合需要统一管理多台 Pingora 代理节点、构建自建高可用边缘网络，或为源站搭建坚固防护网关的团队，具备以下定位：
* **控制与落地分离**：Server 控制面不直接 SSH 到代理节点，而是通过 Agent 主动拉取版本并应用。
* **Zero-Reload 动态配置同步**：基于 Rust Pingora 边缘网关运行，通过内存快照原子替换与增量配置同步生效，避免传统网关进程重载（reload）导致的连接抖动。
* **一体化边缘安全网关**：在同一个控制面内集成反向代理、TLS 证书自动续期、WAF 防护拦截、Turnstile-like 客户端 PoW 防 CC 盾、内网穿透（Tunnel）以及 Pages 静态网站托管。
* **双重部署生态适应性**：既支持作为独立私有边缘 CDN 运行，亦可作为 Cloudflare 背后坚固的源站防护网关（Origin Shield），防止源站真实 IP 泄露并沉淀全量私有审计日志。

**非本产品定位**：多租户公有云平台、Kubernetes Ingress Controller、服务网格或通用日志平台。

---

## 当前能力

| 能力 | 说明 | 详细设计/使用指南 |
| --- | --- | --- |
| **反代配置管理** | 以网站规则（Proxy Route）为聚合边界，支持多域名与多上游负载均衡 | [新建反代配置](../guide/proxy-config.md) |
| **源站错误页** | 全局可配置：源站/网关匹配状态码时返回 OpenFlare 默认或自定义 HTML，HTTP 状态码保持原值 | [源站错误页设计](./origin-error-page.md) |
| **边缘缓存** | Pingora 节点内存缓存；默认 static 扩展名 + 源站头/Set-Cookie 闸门 + 默认 Edge TTL（对标 CF 默认模型） | [边缘缓存策略设计](./edge-cache-design.md) |
| **Zone 与域名管理** | 以可注册根域为管理入口，聚合明确域名、域名证书与反代路由 | [Zone 与域名资源设计](./zone-design.md) |
| **Cloudflare DNS 指向** | 以 ZoneDomain 为粒度，将单条 Cloudflare A 记录幂等指向边缘节点 IPv4；支持连接配置、分组、成员橙云与异步同步，一期不含自动故障切换 | [Cloudflare DNS 指向设计](./cloudflare-pointing.md) |
| **边缘配置同步** | 保存后自动同步，节点原子应用并保留最后一份健康快照 | [Pingora 边缘节点设计](./pingora-edge-design.md) |
| **WAF 安全防护** | 支持可视化 DAG 编排规则、手动/自动/订阅型 IP 组、GeoIP 匹配与 PoW CC 防护 | [WAF 设计](./waf-design.md) / [WAF 可编排规则设计](./waf-orchestration-design.md) / [WAF 使用指南](../guide/waf-usage.md) |
| **内网穿透** | 通过中继节点（Relay）与内网客户端（OpenFlared），反向穿透暴露内网 Web 服务 | [内网穿透设计](./tunnel-design.md) / [穿透使用指南](../guide/tunnel-usage.md) |
| **Pages 静态托管** | 支持上传或从 Remote URL、公开 GitHub Release 同步预构建产物；GitHub latest 可定时检查并可选自动发布。不可变部署由边缘节点拉取并由 Pingora 本地服务，支持回滚、API 反代与 SPA Fallback | [Pages 静态托管设计](./pages-design.md) / [Pages 使用指南](../guide/pages-usage.md) |
| **TLS 证书自动续期** | 将证书显式绑定到 Zone 域名，并通过 ACME 协议向 Let's Encrypt 申请/续期证书 | [Zone 与域名资源设计](./zone-design.md) |
| **多节点监控与观测** | 访问日志为业务流量唯一真相；Agent 只上报明细与主机读数，Server 统一聚合；与 Zone/看板对账 | [观测数据传输模型](./observability-transport-model.md) / [边缘可观测与业务流量统计](./observability-design.md) / [上报协议与表结构](./observability-data-model.md) / [系统架构](./architecture.md) |
| **日志存储** | 访问日志与可观测时序走可切换日志主库（随业务主库或 ClickHouse）；关闭 ClickHouse 后仍可写可查 | [日志存储解耦](./logstore.md) |
| **控制台双语与国际化** | 无 URL 前缀的 zh-CN / en / ru，cookie `NEXT_LOCALE` 优先，兼容静态导出 | [运行时语言与国际化](./log-language.md) |

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
* **运行时数据解耦**：Pingora 通过内存快照执行规则，不与 Server 数据库通信。IP 组成员可独立于站点快照更新，通过 Checksum 差分拉取以实现零重载平滑生效。

### 3. 内网穿透边界
* **仅限 HTTP 流量**：穿透组件仅支持 HTTP/HTTPS 协议（底层依靠 frp 虚拟主机 Vhost 机制实现单端口域名路由复用），暂不支持单独的 TCP/UDP 端口分配。
* **中继配置动态化控制**：中继节点（Relay）在连接至 Server 后，可通过心跳周期性动态拉取并同步全局系统配置（例如是否开启内嵌 FRPS Web UI 及其监听端口），随独立的中继配置协议下发。
* **Tunnel 与 Node 体系隔离**：Tunnel 客户端在内网发起出向建连，与控制面托管的边缘 Node（公网节点）是独立的实体，使用专属的 `tunnel_token` 进行鉴权。

### 4. Pages 静态托管边界
* **预构建产物来源**：项目可保持手动上传，或配置一个 Remote URL / 公开 GitHub Release asset 来源。Remote 与固定 tag 只支持手动操作；只有 GitHub latest 进入定时检查并可选择自动更新。来源可切换，但不可变 deployment 与当前生产版本不会随 source 编辑或删除而丢失。
* **归档与资源上限**：支持 `zip`、`tar.gz` / `tgz`、`tar.xz` / `txz`、`tar.bz2` / `tbz2`、`tar`、`7z`。压缩包上限由 `pages_max_package_size_mb` 控制（默认 100 MiB，范围 1～2048）；展开后的单文件和总量上限为包上限的 4 倍且最低 100 MiB，最多 1,000 个常规文件。Server 与 Agent 都校验实际字节，并拒绝路径逃逸、软/硬链接与特殊文件。
* **构建与运行时边界**：当前不从外部 Git 仓库拉取源码或执行构建，也不提供边缘 Serverless、动态 SSR 或二级预览域名。未来仓库集成必须使用独立 `git_repository` Provider 与 Server 侧隔离 build executor，只向统一 artifact 管线输出受限产物；Agent 不接收仓库凭据、外部 URL 或 clone/install/build 命令。

### 5. 系统与版本边界
* **站点快照**：所有边缘节点同步同一份站点快照，各节点按分配的性能策略运行。
* **单租户架构**：OpenFlare 仅供单团队在受信任的内部网络部署使用。采用单租户设计，不支持细粒度的多用户角色或多租户资源隔离。
* **外部基础设施依赖性**：Server **必须依赖**外部 Redis（或 Valkey），用于分布式协调、Asynq 队列与系统缓存。关系库为 PostgreSQL，或关闭 `database.enabled` 时使用 SQLite。ClickHouse **可选**：不启用时，访问日志与可观测时序由当前日志主库（随业务主库）承接；启用后可通过「切换日志数据库」任务迁到 ClickHouse。系统不支持脱离 Redis 运行。详情见 [日志存储解耦](./logstore.md)。

---

## 仓库结构

OpenFlare 已收敛为**单 monorepo**（Go 模块 `github.com/Rain-kl/Wavelet`）。控制面 Server 与边缘组件（Agent、Relay、OpenFlared）共享同一仓库，业务代码按 Wavelet `internal/apps/` 领域模块组织。

在贡献代码时，请严格遵守以下物理分层与目录分工：

| 路径 | 职责 |
| --- | --- |
| `cmd/server`、`cmd/relay`、`cmd/flared` | 控制面 Server、中继 Relay 与客户端 OpenFlared 的 CLI 启动入口（Server 在 `cmd/server/main.go` + `cmd/server/cli/`） |
| `edge/pingora/` | Rust Pingora 边缘网关（`openflare-edge`）源码与构建目录 |
| `internal/` | 控制面与边缘运行时实现 |
| `frontend/` | Next.js 管理端，构建产物嵌入 Go Server |
| `pkg/` | 通用基础设施共享库（GeoIP、缓存、邮件、推送、工具函数等） |
| `scripts/` | Swagger 生成、安装脚本等 |
| `docs/` | Fumadocs 文档内容、Next.js 静态站点与设计基线 |
| `docker/` | 各组件 Dockerfile |
| `tests/integration/` | 跨模块集成测试 |
| `uploads/`、`data/` | 运行时上传目录与静态数据（`.gitignore` 忽略） |

### 1. Server 与共享域分层（`cmd/server/` + `internal/`）

| 目录 | 职责 |
| --- | --- |
| `cmd/server/` | Server 启动入口（`main.go`）与 Cobra 子命令 `cli/`（`api`、`worker`、`scheduler`、`all`） |
| `internal/platform/` | 跨模块装配与生命周期：`bootstrap/`、`buildinfo/`、`lifecycle/` |
| `internal/router/` | HTTP 路由注册与全局中间件（OpenFlare 挂载于 `v1/openflare_console.go` 与 `v1/openflare_edge.go`） |
| `internal/apps/openflare/` | OpenFlare 控制面业务域（`routers.go` + `logics.go`、`tasks/`、`logwriter/`） |
| `internal/apps/{admin,user,oauth,upload,cap,...}/` | Wavelet 平台能力（用户、认证、上传、任务、推送等） |
| `internal/apps/openflare/{agent,relay,flared}/` | **Server 侧**边缘协议处理器（鉴权、心跳、WS） |
| `internal/openflare/` | OpenFlare 跨端共享域模块：`protocol/`、`edgeconfig/`、`pagesarchive/`、`routeidentity/` |
| `internal/edge/` | 边缘运行时共享基础与端侧实现：`common/`、`relay/`、`flared/` |
| `internal/model/` | GORM 实体 / DTO / 无 IO 领域规则（`openflare_*.go` + 平台模型）；**不含** DB 访问 |
| `internal/infra/persistence/migrator/goose/` | goose SQL 迁移（PostgreSQL / SQLite / ClickHouse） |
| `internal/repository/` | 数据访问层（平台 + OpenFlare 业务 CRUD、缓存、`logstore` 日志/分析读写）；**唯一**持久化入口 |
| `internal/infra/task/` | Asynq 异步任务（Worker + Scheduler） |
| `internal/infra/config/` | Viper 配置加载 |
| `internal/shared/` | 统一 API 响应封装（`response/`）与 HTTP 绑定助手（`httpx/`） |
| `pkg/` | 通用基础设施库（`cache/`、`cap/`、`geoip/`、`httppool/`、`i18n/`、`logger/`、`mail/`、`push/`、`trace/`、`util/`、`wsclient/`） |

**API 路由前缀：**

| 前缀 | 用途 | 鉴权 |
| --- | --- | --- |
| `/api/v1/d/*` | OpenFlare 管理控制台 API | Session Cookie + 可选 `X-Access-Token` |
| `/api/v1/agent/*` | Agent 节点协议 | `X-Agent-Token` |
| `/api/v1/relay/*` | Relay 中继协议 | `X-Agent-Token` |
| `/api/v1/tunnel/*` | Tunnel 客户端协议 | `X-Tunnel-Token` |
| `/api/v1/admin/*` | Wavelet 平台管理 API | 管理员 Session |

### 2. Pingora 边缘节点

节点运行时位于 `edge/pingora/`。控制端通过配置清单分发站点、WAF IP 组和性能策略，节点校验后更新本地数据面。详见 [Pingora 边缘节点设计](./pingora-edge-design.md)。

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

### 4. Relay 模块 (`internal/edge/relay/` / `cmd/relay/`)

| 模块 | 职责 |
| --- | --- |
| `cmd/relay/` | Relay 命令行启动入口及初始化主函数 |
| `internal/edge/relay/config/` | 本地配置文件解析与默认参数初始化 |
| `internal/edge/relay/frps/` | 管理 frps 进程生命周期、端口与 Token 并监控运行 |
| `internal/edge/relay/heartbeat/` | 周期性 HTTP 心跳通信、上报状态并获取更新请求 |
| `internal/edge/relay/httpclient/` | Server 的通用 API 客户端调用工具 |
| `internal/edge/relay/observability/` | 采集本地宿主机、frps 的基础运行指标并进行预聚合 |
| `internal/edge/relay/relay/` | 协调中继的核心生命周期、初始化与清理 |
| `internal/edge/relay/state/` | 本地运行时状态、错误记录与持久化缓存 |

### 5. OpenFlared (Client) 模块 (`internal/edge/flared/` / `cmd/flared/`)

| 模块 | 职责 |
| --- | --- |
| `cmd/flared/` | Client 命令行启动入口及初始化主函数 |
| `internal/edge/flared/config/` | 本地客户端配置加载与解析 |
| `internal/edge/flared/flared/` | 内网穿透客户端的核心调度与状态管理机制 |
| `internal/edge/flared/frpc/` | 热重载/动态生成多 Relay 的 `frpc_{relayNodeID}.toml` 并监控 frpc |
| `internal/edge/flared/heartbeat/` | 与控制面进行的心跳通信，包含 Token 校验机制 |
| `internal/edge/flared/httpclient/` | 客户端通用 API 通信（`/api/v1/tunnel/*`） |
| `internal/edge/flared/sync/` | 增量拉取最新 Tunnel 路由绑定关系、生成快照并应用 |

> **说明**：OpenFlared 无独立 `state/` 包；版本与 checksum 由 `frpc/manager.go` 持久化到 `flared-state.json`。

---

## 文档维护原则

* 产品范围或系统边界变化：更新本文档（[产品边界](./index.md)）。
* 日志存储、日志表判定或切换协议变化：更新 [日志存储解耦](./logstore.md)。
* 系统结构、组件分工变化：更新 [系统架构](./architecture.md)。
* 配置同步和节点恢复逻辑记录于 [Pingora 边缘节点设计](./pingora-edge-design.md)。
* 部署方式变化：更新 [部署说明](../deployment/deployment.md) 与 README。
* 配置项变化：更新 [配置项参考](../reference/configuration.md)。
