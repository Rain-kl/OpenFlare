# 产品边界

你会学到：OpenFlare 是什么、当前稳定能力，以及开发时应遵守的核心产品边界与仓库结构目录分工。

OpenFlare 是一套新一代分布式反向代理、边缘安全网关与智能权威 DNS 平台，面向多服务器集群与单团队运维场景。

---

## 项目定位

OpenFlare 适合需要统一管理多台 OpenFlare Edge 代理节点与 OpenFlare DNS 权威节点、构建自建高可用边缘网络，或为源站搭建坚固防护网关的团队，具备以下定位：
* **控制与落地分离**：Server 控制面不直接 SSH 到代理节点，而是通过 Agent 主动拉取版本并应用。
* **Zero-Reload 动态配置同步**：基于 Rust OpenFlare Edge 边缘网关运行，通过内存快照原子替换与增量配置同步生效，避免传统网关进程重载（reload）导致的连接抖动。
* **一体化边缘安全与智能调度**：在同一个控制面内集成反向代理、SmartDNS 智能权威解析、多服务商 DNS 账号管理、TLS 证书自动续期、WAF 防护拦截、客户端 PoW 防 CC 验证、高可用内网穿透（Tunnel）以及 Pages 静态网站托管。
* **双重部署生态适应性**：既支持作为独立私有边缘 CDN 运行，亦可作为 Cloudflare 背后坚固的源站防护网关（Origin Shield），防止源站真实 IP 泄露并沉淀全量私有审计日志。

**非本产品定位**：多租户公有云平台、Kubernetes Ingress Controller、服务网格或通用日志平台。

---

## 当前能力

| 能力 | 说明 | 详细设计/使用指南 |
| --- | --- | --- |
| **反向代理配置管理** | 以网站规则（Proxy Route）为聚合边界，支持多域名与多上游负载均衡 | [新建反代配置](../guide/proxy-config.md) |
| **源站错误页** | 全局可配置：源站/网关匹配状态码时返回 OpenFlare 默认或自定义 HTML，HTTP 状态码保持原值 | [源站错误页设计](./origin-error-page.md) |
| **边缘缓存** | 边缘节点内存缓存；默认 static 扩展名 + 源站头/Set-Cookie 闸门 + 默认 Edge TTL | [边缘缓存策略设计](./edge-cache-design.md) |
| **Zone 与域名管理** | 以可注册根域为管理入口，聚合明确域名、域名证书与反向代理路由 | [Zone 与域名资源设计](./zone-design.md) |
| **多服务商 DNS 与 SmartDNS 智能权威解析** | 统一管理 Cloudflare、阿里云 DNS 与腾讯云 DNSPod 账号；提供独立高性能 `OpenFlare DNS` 权威节点、可视化调度策略编辑器、多时间窗口网络质量矩阵与 ECS 感知分流 | [SmartDNS 设计](./smartdns-design.md) / [SmartDNS 使用指南](../guide/smartdns.md) / [OpenFlare DNS 部署](../deployment/openflare-dns.md) |
| **智能 DNS 接入契约（Phase 2）** | 基于 ECS / 访客 IP 解析与 Phase 1.5 5 级预测器融合的调度输入、离散度惩罚与回退契约 | [SmartDNS Phase 2 ECS 接入契约](./smartdns-phase2-ecs-contract.md) |
| **边缘配置同步** | 保存后自动同步，节点原子应用并保留最后一份健康快照 | [边缘节点设计](./pingora-edge-design.md) |
| **WAF 安全防护** | 支持可视化 DAG 编排规则、手动/自动/订阅型 IP 组、GeoIP 匹配与 PoW CC 防护 | [WAF 设计](./waf-design.md) / [WAF 可编排规则设计](./waf-orchestration-design.md) / [WAF 使用指南](../guide/waf-usage.md) |
| **内网穿透** | 基于原生 Rust 中继（`openflare-relay`）与连接器（`openflared`），支持跨可用区多中继高可用调度与 L4/L7 双协议内网穿透 | [内网穿透设计](./tunnel-design.md) / [穿透使用指南](../guide/tunnel-usage.md) |
| **Pages 静态托管** | 支持上传或从 Remote URL、公开 GitHub Release 同步预构建产物，由 OpenFlare Edge 本地服务并支持回滚、API 反向代理与 SPA Fallback | [Pages 静态托管设计](./pages-design.md) / [Pages 使用指南](../guide/pages-usage.md) |
| **TLS 证书自动续期** | 将证书显式绑定到 Zone 域名，并通过 ACME 协议向 Let's Encrypt 申请/续期证书 | [Zone 与域名资源设计](./zone-design.md) |
| **多节点监控与观测** | 访问日志为业务流量唯一真相；Agent 只上报明细与主机读数，Server 统一聚合 | [观测数据传输模型](./observability-transport-model.md) / [边缘可观测与业务流量统计](./observability-design.md) / [上报协议与表结构](./observability-data-model.md) / [系统架构](./architecture.md) |
| **日志存储** | 访问日志与可观测时序走可切换日志主库（随业务主库或 ClickHouse）；关闭 ClickHouse 后仍可写可查 | [日志存储解耦](./logstore.md) |
| **控制台多语与国际化** | 无 URL 前缀的 zh-CN / en / ru，cookie `NEXT_LOCALE` 优先，兼容静态导出 | [运行时语言与国际化](./log-language.md) |

---

## 核心产品边界与约束

在开发与贡献代码时，**必须严格遵守**以下业务边界与技术约束，禁止为了临时需求而绕过限制：

### 1. 网站配置与上游约束
* **单站点域名共享策略**：一条路由规则对应一个网站，该站点下的多域名共享限流、缓存与反向代理上游等配置，不支持在同一规则内为不同域名做差异化服务配置。
* **上游类型互斥**：上游必须是直连地址（`direct`）、内网穿透（`tunnel`）或 Pages 静态托管（`pages`）三者之一，不允许在同一规则中混用。
* **直连类型限制**：直连上游可以是纯 `http://` 或 `https://` 的单个或多个地址（多地址仅支持纯 `scheme://host[:port]`），不支持非 HTTP 协议（如 TCP/UDP）上游。

### 2. WAF 安全边界
* **白名单优先原则**：白名单拥有绝对匹配权。若未命中白名单规则，才依次触发全局和自定义黑名单过滤。
* **GeoIP 弱依赖性**：地域准入解析完全依赖节点本地 MaxMind 库。当 GeoIP 异常或解析失败时，系统必须自动忽略地域规则，不能破坏 IP 组过滤和反向代理主链路的可用性。
* **运行时数据解耦**：OpenFlare Edge 通过内存快照执行规则，不与 Server 数据库通信。IP 组成员可独立于站点快照更新，通过 Checksum 差分拉取实现平滑生效。

### 3. 内网穿透边界
* **L4 与 L7 双协议支持**：L7 HTTP/WebSocket 服务通过 `/proxy-routes` 绑定 `tunnel_id` 经由 OpenFlare Edge 与 Relay Gateway 自动调度；L4 TCP/UDP 端口转发通过隧道服务（`w_tunnel_services`）在所选 Relay 上监听公开端口并由内嵌 `rathole` 通道转发。
* **穿透与边缘节点域完全解耦**：内网穿透资源（中继池、中继节点、隧道、连接器、L4 服务）归属独立的 `/tunnels` 控制域，不再混入 Edge 节点列表；中继与连接器通过 `oftok1_` 引导令牌、`X-Relay-Token` 与 `X-Tunnel-Token` 独立鉴权，采用独立 `tunnel-v*` 版本线发布与自更新。
* **回源安全防线**：所有经 `openflared` 发往内网的 L7 与 L4 连接必须先通过目标 IP 解析后的允许/禁止 CIDR 规则校验（默认封锁 `169.254.169.254/32`）。

### 4. Pages 静态托管边界
* **预构建产物来源**：项目可保持手动上传，或配置一个 Remote URL / 公开 GitHub Release asset 来源。来源可切换，但不可变 deployment 与当前生产版本不会随 source 编辑或删除而丢失。
* **归档与资源上限**：支持 `zip`、`tar.gz` / `tgz`、`tar.xz` / `txz`、`tar.bz2` / `tbz2`、`tar`、`7z`。压缩包上限由 `pages_max_package_size_mb` 控制（默认 100 MiB，范围 1～2048）；展开后的单文件和总量上限为包上限的 4 倍且最低 100 MiB，最多 1,000 个常规文件。
* **构建与运行时边界**：当前不从外部 Git 仓库拉取源码或执行构建，Agent 不接收仓库凭据、外部 URL 或构建命令。

### 5. 系统与版本边界
* **站点快照**：所有边缘节点同步同一份站点快照，各节点按分配的性能策略运行。
* **单租户架构**：OpenFlare 仅供单团队在受信任的内部网络部署使用。采用单租户设计，不支持细粒度的多用户角色或多租户资源隔离。
* **外部基础设施依赖性**：Server 必须依赖外部 Redis（或 Valkey），用于分布式协调、Asynq 队列与系统缓存。关系库为 PostgreSQL，或关闭 `database.enabled` 时使用 SQLite。ClickHouse 为可选项。详情见 [日志存储解耦](./logstore.md)。

---

## 仓库结构

OpenFlare 采用单 monorepo（Go 模块 `github.com/Rain-kl/Wavelet`，Rust Edge 位于 `edge/`，Rust 隧道工作区位于 `tunnel/`）。控制面与边缘组件共享同一仓库，业务代码按 Wavelet `internal/apps/` 领域模块组织：

| 路径 | 职责 |
| --- | --- |
| `cmd/server/` | 控制面 Server 启动入口（`main.go` + Cobra 子命令 `cli/`） |
| `edge/` | Rust OpenFlare Edge 边缘网关（`openflare-edge`）源码与构建目录 |
| `dns/` | Rust OpenFlare DNS 权威解析节点（`openflare-dns`）源码与构建目录 |
| `tunnel/` | Rust 原生内网穿透工作区（`common/`、`relay/`、`flared/`） |
| `internal/` | 控制面域实现、持久化仓库与跨端共享协议 |
| `frontend/` | Next.js 管理端，构建产物嵌入 Go Server |
| `pkg/` | 通用基础设施共享库（GeoIP、缓存、邮件、推送、工具函数等） |
| `scripts/` | Swagger 生成、边缘与隧道安装/更新脚本等 |
| `docs/` | Fumadocs 文档内容、Next.js 静态站点与设计基线 |
| `docker/` | 各组件 Dockerfile（`Dockerfile`、`Dockerfile.edge`、`Dockerfile.relay`、`Dockerfile.flared`） |
| `tests/integration/` | 跨模块集成测试 |
| `uploads/`、`data/` | 运行时上传目录与静态数据（`.gitignore` 忽略） |

### 1. Server 与共享域分层（`cmd/server/` + `internal/`）

| 目录 | 职责 |
| --- | --- |
| `cmd/server/` | Server 启动入口（`main.go`）与 Cobra 子命令 `cli/`（`api`、`worker`、`scheduler`、`all`） |
| `internal/platform/` | 跨模块装配与生命周期：`bootstrap/`、`buildinfo/`、`lifecycle/` |
| `internal/router/` | HTTP 路由注册与全局中间件（OpenFlare 挂载于 `v1/openflare_console.go` 与 `v1/openflare_edge.go`） |
| `internal/apps/openflare/` | OpenFlare 控制面业务域（`routers.go` + `logics.go`、`agent/`、`tunnel/`、`tasks/`、`logwriter/` 等） |
| `internal/apps/{admin,user,oauth,upload,cap,...}/` | Wavelet 平台能力（用户、认证、上传、任务、推送等） |
| `internal/apps/openflare/tunnel/` | **Server 侧**内网穿透控制面与中继/连接器协议处理器（池、中继、隧道、服务管理，HTTP + WS） |
| `internal/openflare/` | OpenFlare 跨端共享域模块：`protocol/`、`edgeconfig/`、`pagesarchive/`、`routeidentity/` |
| `internal/model/` | GORM 实体 / DTO / 无 IO 领域规则（`openflare_*.go` + 平台模型）；不含 DB 访问 |
| `internal/infra/persistence/migrator/goose/` | goose SQL 迁移（PostgreSQL / SQLite / ClickHouse） |
| `internal/repository/` | 数据访问层（平台 + OpenFlare 业务 CRUD、缓存、`logstore` 日志/分析读写）；唯一持久化入口 |
| `internal/infra/task/` | Asynq 异步任务（Worker + Scheduler） |
| `internal/infra/config/` | Viper 配置加载 |
| `internal/shared/` | 统一 API 响应封装（`response/`）与 HTTP 绑定助手（`httpx/`） |
| `pkg/` | 通用基础设施库（`cache/`、`cap/`、`geoip/`、`httppool/`、`i18n/`、`logger/`、`mail/`、`push/`、`trace/`、`util/`） |

**API 路由前缀：**

| 前缀 | 用途 | 鉴权 |
| --- | --- | --- |
| `/api/v1/d/*` | OpenFlare 管理控制台 API | Session Cookie + 可选 `X-Access-Token` |
| `/api/v1/agent/*` | Edge 节点协议 | `X-Agent-Token` |
| `/api/v1/relay/*` | Relay 中继协议 | `X-Relay-Bootstrap-Token` / `X-Relay-Token` |
| `/api/v1/tunnel/*` | OpenFlared 连接器协议 | `X-Tunnel-Token` |
| `/api/v1/admin/*` | Wavelet 平台管理 API | 管理员 Session |

### 2. 边缘节点（`edge/`）

节点运行时位于 `edge/`。控制端通过配置清单分发站点（`sites.v1`）、隧道网关路由（`tunnel-routing.v1`）、WAF IP 组和性能策略，节点校验后更新本地数据面。详见 [边缘节点设计](./pingora-edge-design.md)。

### 3. Native Rust 内网穿透工作区（`tunnel/`）

| 包路径 | 职责 |
| --- | --- |
| `tunnel/common/` | 跨端共享契约：`oftok1_` 引导令牌解析、wire 协议类型、CIDR SSRF 校验、`0600` 凭据持久化、公网 IP 探测、双槽位 `tunnel-v*` 更新与 systemd `NOTIFY_SOCKET` 就绪通知 |
| `tunnel/relay/` | `openflare-relay` 二进制：自动发现注册、内嵌 `rathole` server 热加载管理、HTTP/WS Gateway（`:8443`）、与同机/远端 Edge 协同及本地探测 |
| `tunnel/flared/` | `openflared` 二进制：多中继发现、RTT/负载加权与跨 `zone_tag` Closest-K 选择、Make-Before-Break 迟滞重平衡、内嵌 `rathole` client 连接池及带 CIDR 校验的 L4/L7 转发器 |

### 4. Frontend 分层（`frontend/`）

基于 Wavelet Next.js 脚手架，OpenFlare 业务 UI 以路由共置方式组织在 `app/(main)/` 下（如 `/nodes`、`/tunnels`、`/proxy-routes`、`/zones` 等）。API 约定与前端约定见根目录 `AGENTS.md`。

---

## 文档维护原则

* 产品范围或系统边界变化：更新本文档（[产品边界](./index.md)）。
* 日志存储、日志表判定或切换协议变化：更新 [日志存储解耦](./logstore.md)。
* 系统结构、组件分工变化：更新 [系统架构](./architecture.md)。
* 配置同步和节点恢复逻辑记录于 [边缘节点设计](./pingora-edge-design.md)。
* 部署方式变化：更新 [部署说明](../deployment/deployment.md) 与组件部署文档。
* 配置项变化：更新 [配置项参考](../reference/configuration.md)。
