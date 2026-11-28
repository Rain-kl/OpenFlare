# 产品边界与仓库结构

OpenFlare 是面向多服务器集群与单团队运维场景的分布式反向代理、边缘安全网关与智能权威 DNS 平台。

## 项目定位

OpenFlare 适用于需要集中管理多台 OpenFlare Edge 代理节点与 OpenFlare DNS 权威节点、自建边缘网络或为源站提供统一防护网关的团队：

* **控制与执行分离**：Server 控制面不通过 SSH 操作节点，边缘与 DNS 节点主动通过心跳与 WebSocket 拉取配置快照。
* **零重载动态同步**：`openflare-edge` 通过内存快照原子替换与增量清单同步生效，避免进程重载导致的连接中断。
* **一体化安全与调度**：同一控制面集成反向代理、SmartDNS 智能权威解析、多服务商 DNS 账号管理、TLS 证书自动续期、WAF 防护、PoW 人机验证、内网穿透与 Pages 静态托管。
* **双重部署模式**：可作为独立私有边缘网关运行，也可部署在 Cloudflare 等 CDN 之后作为源站防护网关（Origin Shield）。

**非本产品定位**：多租户公有云平台、Kubernetes Ingress Controller、服务网格或通用日志分析平台。

## 当前能力

| 能力 | 说明 | 设计与指南 |
| --- | --- | --- |
| **反向代理管理** | 以代理路由（`of_proxy_routes`）为聚合边界，支持多域名与多上游负载均衡 | [配置反向代理](../guide/proxy-config.md) |
| **源站错误页** | 源站或网关命中指定状态码时返回内置或自定义 HTML，保持原始 HTTP 状态码 | [源站错误页设计](./origin-error-page.md) |
| **边缘缓存** | 节点级 TinyUFO 内存/磁盘缓存，支持默认静态扩展名与源站头闸门 | [边缘缓存策略设计](./edge-cache-design.md) |
| **Zone 与域名管理** | 以可注册根域为管理入口，统一关联域名、TLS 证书与反代路由 | [Zone 与域名资源设计](./zone-design.md) |
| **多云 DNS 指向与 SmartDNS** | 统一管理 Cloudflare、阿里云 DNS 与腾讯云 DNSPod 账号，支持节点故障切换与独立 `openflare-dns` 智能解析 | [多云 DNS 指向与故障切换设计](./cloudflare-pointing.md) / [SmartDNS 系统架构设计](./smartdns-design.md) |
| **网络质量与主动探测** | 融合多源网络情报、被动 `TCP_INFO`、受控探测、自适应质量矩阵与 5 级预测器 | [网络情报与链路质量感知架构](./network-quality-design.md) |
| **边缘配置同步** | 保存即编译版本化快照，节点校验后原子切换并保留最后一份健康快照 | [OpenFlare Edge 架构设计](./pingora-edge-design.md) |
| **WAF 与 PoW 防护** | 可视化 DAG 规则、OWASP CRS 攻击检测、多种 IP 组、GeoIP 匹配与客户端 PoW 验证 | [WAF 可编排规则设计](./waf-orchestration-design.md) / [PoW 防护设计](./pow-design.md) |
| **内网穿透** | 原生 Rust 中继（`openflare-relay`）与连接器（`openflared`），支持跨可用区调度与 L4/L7 转发 | [内网穿透隧道设计](./tunnel-design.md) |
| **Pages 静态托管** | 导入或从 Remote URL / GitHub Release 同步预构建归档，在边缘提供静态服务与 API 反代 | [Pages 静态托管设计](./pages-design.md) |
| **观测与日志存储** | 访问日志为业务流量唯一真相，可观测数据经 `logstore` 落盘（PostgreSQL / SQLite / ClickHouse） | [边缘可观测性与流量统计架构](./observability-design.md) / [日志存储解耦](./logstore.md) |
| **运行语言与国际化** | 前端支持 `zh-CN` / `en` / `ru`（`NEXT_LOCALE` 优先），后端运维日志固定英文 | [运行时语言与国际化](./log-language.md) |

## 核心产品边界与约束

开发与贡献代码时须遵守以下业务边界与技术约束：

### 1. 网站配置与上游约束
* **单站点共享策略**：一条路由规则对应一个站点，其下所有域名共享上游、限流与缓存配置，不支持单规则内按域名差异化配置。
* **上游类型互斥**：路由上游只能选择直连上游（`direct`）、内网穿透（`tunnel`）或 Pages 静态托管（`pages`）之一。
* **直连地址限制**：直连上游仅支持 `http://` 或 `https://` 地址，多上游仅支持纯 `scheme://host[:port]`。

### 2. WAF 安全边界
* **白名单优先**：请求白名单匹配后跳过 WAF 规则流与请求体检测；其后依次执行全局与站点级规则组。
* **GeoIP 弱依赖**：地域匹配依赖节点本地 MaxMind MMDB。数据库缺失或无法解析时自动按未命中处理，不阻断 IP 组与反代主链路。
* **运行时解耦**：边缘节点仅消费内存快照，不直接连接 Server 数据库；IP 组成员通过独立清单增量刷新。

### 3. 内网穿透边界
* **L4 与 L7 双协议**：L7 HTTP/WebSocket 由 OpenFlare Edge 调度至 Relay Gateway；L4 TCP/UDP 由 `openflare-relay` 直接监听端口并通过内嵌 `rathole` 转发。
* **域隔离与鉴权**：中继池、中继、隧道、连接器与服务归属独立 `/tunnels` 控制域，使用 `oftok1_` 引导令牌、`X-Relay-Token` 与 `X-Tunnel-Token` 鉴权，采用独立 `tunnel-v*` 版本线发布。
* **回源防线**：`openflared` 在发起内网建连前对解析后的目标 IP 执行允许/禁止 CIDR 校验（默认封锁 `169.254.169.254/32`）。

### 4. Pages 静态托管边界
* **预构建产物导入**：支持手动上传或配置 Remote URL / 公开 GitHub Release 来源。变更或移除来源不删除已有不可变部署与现役生产版本。
* **归档与容量限制**：支持 `zip`、`tar.gz`/`tgz`、`tar.xz`/`txz`、`tar.bz2`/`tbz2`、`tar`、`7z`。包体积由 `pages_max_package_size_mb` 控制（默认 `100 MiB`，范围 `1–2048`）；展开后单文件及总大小上限为包上限的 `4` 倍（最低 `100 MiB`），最多 `1,000` 个常规文件。
* **不执行源码构建**：系统不拉取外部 Git 仓库或运行构建脚本，边缘节点仅消费已校验的产物包。

### 5. 系统与运行边界
* **全局站点快照**：所有边缘节点同步同一份站点快照，并按各自绑定的性能策略运行。
* **单租户架构**：OpenFlare 面向单团队使用，不提供多租户资源隔离或细粒度权限体系。
* **基础设施依赖**：Server 必需 Redis（或 Valkey）支撑 Asynq 队列与系统缓存；关系主库为 PostgreSQL（或关闭 `database.enabled` 时使用 SQLite）；ClickHouse 为可选项，见 [日志存储解耦](./logstore.md)。

## 仓库结构

OpenFlare 采用单 monorepo（Go 模块 `github.com/Rain-kl/Wavelet`，边缘网关在 `edge/`，SmartDNS 在 `dns/`，内网穿透工作区在 `tunnel/`）：

| 路径 | 职责 |
| --- | --- |
| `cmd/server/`、`cmd/openflare/` | Server 控制面启动入口（`main.go` + `cli/`）与 `openflare` 管理 CLI |
| `edge/` | Rust OpenFlare Edge 网关源码与集成测试 |
| `dns/` | Rust OpenFlare DNS 权威解析节点源码 |
| `tunnel/` | Rust 内网穿透工作区（`common/`、`relay/`、`flared/`） |
| `shared/` | 跨 Rust 组件共享的探测隔离库（`probe/`）与静态页面模板（`edgeassets/`） |
| `internal/` | 控制面业务域、持久化仓库与跨端协议 |
| `frontend/` | Next.js 管理端，构建后嵌入 Go Server |
| `pkg/` | 通用基础设施库（`cache/`、`cap/`、`geoip/`、`httppool/`、`i18n/`、`logger/`、`mail/`、`netintel/`、`push/`、`trace/`、`util/`） |
| `scripts/` | Swagger 生成及各组件安装、更新与卸载脚本 |
| `docs/` | Fumadocs 文档站与设计文档 |
| `docker/` | 各组件 Dockerfile 与入口脚本 |
| `tests/integration/` | 跨模块集成测试 |

### Server 控制面分层（`cmd/server/` + `internal/`）

| 目录 | 职责 |
| --- | --- |
| `internal/platform/` | 跨模块装配与生命周期（`bootstrap/`、`buildinfo/`、`lifecycle/`） |
| `internal/router/` | HTTP 路由与中间件（挂载于 `v1/openflare_*.go`） |
| `internal/apps/openflare/` | OpenFlare 业务域（`agent/`、`cloudflare/`、`dnsaccount/`、`netintel/`、`netquality/`、`node/`、`observability/`、`pages/`、`pow/`、`proxy_route/`、`smartdns/`、`tunnel/`、`waf/`、`zone/` 等） |
| `internal/openflare/` | 共享域包：`dnsprovider/`、`edgeconfig/`、`pagesarchive/`、`protocol/`、`routeidentity/` |
| `internal/model/` | GORM 实体、配置键常量与无 IO 规则 |
| `internal/infra/persistence/migrator/goose/` | PostgreSQL、SQLite 与 ClickHouse 的 SQL 迁移 |
| `internal/repository/` | 唯一持久化入口（含业务 CRUD、缓存与 `logstore` 双库层） |
| `internal/infra/task/`、`internal/infra/config/` | Asynq 异步任务与 Viper 配置加载 |
| `internal/shared/` | 统一 API 响应（`response/`）与绑定助手（`httpx/`） |

**API 路由前缀**：

| 前缀 | 用途 | 鉴权方式 |
| --- | --- | --- |
| `/api/v1/d/*` | OpenFlare 控制台 API | Session Cookie + 可选 `X-Access-Token` |
| `/api/v1/agent/*` | OpenFlare Edge 协议 | `X-Agent-Token` |
| `/api/v1/dns-agent/*` | OpenFlare DNS 协议 | `X-DNS-Discovery-Token` / `X-DNS-Token` |
| `/api/v1/relay/*` | Relay 中继协议 | `X-Relay-Bootstrap-Token` / `X-Relay-Token` |
| `/api/v1/tunnel/*` | OpenFlared 连接器协议 | `X-Tunnel-Token` |
| `/api/v1/admin/*` | 平台管理 API | 管理员 Session |
