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
| **配置版本控制** | 支持全局单一激活版本的预览、发布、不可变快照历史与秒级一键回滚 | [Agent 与发布模型](./agent-design.md) |
| **WAF 安全防护** | 全局与自定义规则组，支持手动/自动/订阅型 IP 组，GeoIP 准入与 PoW CC 防护 | [WAF 设计](./waf-design.md) / [WAF 使用指南](../guide/waf-usage.md) |
| **内网穿透** | 通过中继节点（Relay）与内网客户端（OpenFlared），反向穿透暴露内网 Web 服务 | [内网穿透设计](./tunnel-design.md) / [穿透使用指南](../guide/tunnel-usage.md) |
| **Pages 静态托管** | 直接上传前端 zip 包，由边缘节点拉取并由 OpenResty 本地服务，支持 API 反代与 SPA Fallback | [Pages 静态托管设计](./pages-design.md) |
| **TLS 证书自动续期** | 绑定 managed_domains 并通过 ACME 协议向 Let's Encrypt 申请/续期证书 | [新建反代配置](../guide/proxy-config.md) |
| **多节点监控与观测** | 收集节点资源快照、健康事件，聚合请求指标与访问日志明细 | [系统架构](./architecture.md) |

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
* **运行时数据解耦**：OpenResty 拦截时仅读取 Agent 同步至本地的 JSON，不与 Server 数据库通信。IP 组成员同步与版本发布解耦，通过 Chestsum 差分拉取以实现零重载平滑生效。

### 3. 内网穿透边界
* **仅限 HTTP 流量**：穿透组件仅支持 HTTP/HTTPS 协议（底层依靠 frp 虚拟主机 Vhost 机制实现单端口域名路由复用），暂不支持单独的 TCP/UDP 端口分配。
* **中继配置静态化**：中继节点（Relay）配置相对静态，通过心跳被动获取，不纳入控制面的配置版本化管理体系。
* **Tunnel 与 Node 体系隔离**：Tunnel 客户端在内网发起出向建连，与控制面托管的边缘 Node（公网节点）是独立的实体，使用专属的 `tunnel_token` 进行鉴权。

### 4. Pages 静态托管边界
* **Direct Upload 托管模式**：仅支持直接上传预构建的 ZIP 静态资源包。不支持外部 Git 仓库自动构建、边缘 Serverless 函数、动态 SSR 服务或生成的二级预览域名。
* **包体硬上限限制**：为了保障边缘节点安全，ZIP 压缩包体最大 25 MiB，解压文件树不超过 1,000 个且总体积不超过 100 MiB。禁止上传含有任何软链接或目录跨越（Zip-Slip）的安全高危压缩包。

### 5. 系统与版本边界
* **全局单一激活版本**：所有节点拉取并消费同一份全局激活配置。不进行按节点分组的差异化配置发布。
* **单租户架构**：OpenFlare 仅供单团队在受信任的内部网络部署使用。采用单租户设计，不支持细粒度的多用户角色或多租户资源隔离。

---

## 仓库结构

在贡献代码时，请严格遵守以下物理分层与目录分工，保持代码结构清晰：

| 路径                   | 职责                                                 |
| ---------------------- | ---------------------------------------------------- |
| `openflare-server`     | Gin + GORM + SQLite/PostgreSQL 单体控制面            |
| `openflare-server/frontend` | Next.js App Router 管理端前端，由 Go Server 嵌入托管  |
| `pkg`                  | 跨组件复用的协议类型与通用工具包                     |
| `openflare-agent`      | Go 单体 Agent，运行在节点侧                          |
| `openflare-relay`      | Tunnel 中继代理，运行在公网边缘管理 frps 进程        |
| `openflared`           | Tunnel 客户端，运行在内网服务器侧管理 frpc 进程      |
| `scripts`              | 安装、自更新等系统辅助脚本                           |
| `docs`                 | VitePress 文档站、设计基线、开发规范、部署与配置文档 |
| `docs/en`              | 英文版文档                                           |

### 1. Server 分层 (`openflare-server/`)

| 目录                    | 职责                                             |
| ----------------------- | ------------------------------------------------ |
| `cmd/server/`           | Server 命令行启动入口及主函数                    |
| `internal/controller/`  | 参数解析、调用 service、返回响应                 |
| `internal/service/`     | 业务逻辑、校验、事务编排、配置渲染               |
| `internal/model/`       | 纯净实体模型类定义、旧迁移框架兼容与上下文注入   |
| `internal/model/goose/` | goose 迁移提供者、桥接逻辑、注册入口与具体迁移文件 |
| `internal/router/`      | 路由注册                                         |
| `internal/middleware/`  | 认证、鉴权、限流、CORS、Turnstile 验证等横切逻辑 |
| `internal/common/`      | 配置、全局状态与初始化入口                       |
| `internal/job/`         | 定时任务（各业务定时逻辑在独立文件中定义，cron.go 仅用于初始化调度） |
| `internal/utils/`       | 仅 Server 内部使用的基础能力包，如 ACME、限流、验证码、邮件、安全校验等 |
| `pkg/protocol/`         | Server、Relay、OpenFlared 之间共享的 HTTP/WS 协议结构 |
| `pkg/utils/`            | 跨组件可复用的纯工具函数                         |
| `pkg/geoip`、`pkg/render`、`pkg/wsclient` | 被多个组件复用的 GeoIP、OpenResty 配置渲染与 WebSocket 客户端能力 |
| `upload/`               | 运行时本地临时文件上传目录（在 .gitignore 中忽略） |
| `logs/`                 | 运行时本地日志输出目录（在 .gitignore 中忽略）   |
| `docs/`                 | API 文档（Swagger）                              |
| `data/`                 | 静态数据（如 GeoIP 数据库）                      |

### 2. Agent 模块 (`openflare-agent/`)

| 目录/模块                     | 职责                                         |
| ----------------------------- | -------------------------------------------- |
| `cmd/agent/`                  | Agent 命令行启动入口及主函数                 |
| `internal/config/`            | 配置读取与默认值                             |
| `internal/heartbeat/`         | 心跳与版本摘要判断                           |
| `internal/sync/`              | 配置拉取与应用编排                           |
| `internal/nginx/`             | OpenResty 文件写入、校验、reload、启动与回滚 |
| `internal/state/`             | 本地状态与观测补报缓冲                       |
| `internal/httpclient/`        | Server 通信                                  |
| `internal/wsclient/`          | WebSocket 客户端通信                         |
| `internal/protocol/`          | Agent API 协议类型                           |
| `internal/updater/`           | Agent 自更新逻辑                             |
| `internal/logging/`           | 日志处理                                     |
| `internal/observability/`     | 可观测性（指标、链路等）                     |
| `internal/geoipdata/`         | GeoIP 数据处理                               |
| `internal/geoipupdate/`       | GeoIP 数据更新                               |
| `internal/agent/`             | 核心 Agent 逻辑与生命周期                    |

### 3. Frontend 分层 (`openflare-server/web/`)

| 目录          | 职责                                         |
| ------------- | -------------------------------------------- |
| `app/`        | Next.js App Router 路由、布局、页面组装      |
| `features/`   | 按业务域组织的功能模块                       |
| `components/` | 跨 feature 复用的 UI 组件                    |
| `lib/`        | 请求客户端、环境变量、工具函数、常量         |
| `store/`      | 少量跨页面 UI 状态管理                       |
| `types/`      | 共享类型定义                                 |
| `styles/`     | 全局样式                                     |
| `tests/`      | 前端单元测试与集成测试（Vitest、Playwright） |
| `scripts/`    | 构建和部署相关脚本                           |
| `public/`     | 静态资源                                     |

### 4. Relay 模块 (`openflare-relay/`)

| 模块             | 职责                                             |
| ---------------- | ------------------------------------------------ |
| `cmd/`           | Relay 命令行启动入口及初始化主函数               |
| `internal/config/`| 本地配置文件解析与默认参数初始化                 |
| `internal/frps/` | 管理 frps 进程生命周期、端口与 Token 并监控运行   |
| `internal/heartbeat/`| 周期性 HTTP 心跳通信、上报状态并获取更新请求  |
| `internal/httpclient/`| Server 的通用 API 客户端调用工具类              |
| `internal/observability/`| 采集本地宿主机、frps 的基础运行指标并进行预聚合 |
| `internal/relay/` | 协调中继的核心生命周期、初始化与清理             |
| `internal/state/` | 本地运行时状态、错误记录与持久化缓存             |
| `internal/updater/`| Relay 升级检查、下载安装与重启机制               |
| `internal/wsclient/`| 与 Server 保持的长连接 WebSocket 双向通信管道     |

### 5. OpenFlared (Client) 模块 (`openflared/`)

| 模块             | 职责                                             |
| ---------------- | ------------------------------------------------ |
| `cmd/`           | Client 命令行启动入口及初始化主函数              |
| `internal/config/`| 本地客户端配置加载与解析                         |
| `internal/flared/`| 内网穿透客户端的核心调度与状态管理机制           |
| `internal/frpc/` | 热重载/动态生成多 Relay 的 `frpc.toml` 并监控 frpc |
| `internal/heartbeat/`| 与控制面进行的心跳通信，包含 Token 校验机制       |
| `internal/httpclient/`| 客户端通用 API 通信客户端                       |
| `internal/sync/`  | 增量拉取最新 Tunnel 路由绑定关系、生成快照并应用  |
| `internal/updater/`| 客户端自更新、新版检查与更新落地逻辑             |
| `internal/wsclient/`| 用于实时监听 Server 端隧道配置变更推送的 WS 信道  |

---

## 文档维护原则

* 产品范围或系统边界变化：更新本文档（[产品边界](./index.md)）。
* 系统结构、组件分工变化：更新 [系统架构](./architecture.md)。
* 发布、同步、回滚与 Agent 模型变化：更新 [Agent 与发布模型](./agent-design.md)。
* 开发约束、代码规范、接口约定变化：更新 [开发约束](../guideline/Constraints.md)。
* 部署方式变化：更新 [部署说明](../deployment/deployment.md) 与 README。
* 配置项变化：更新 [配置项参考](../reference/configuration.md)。
