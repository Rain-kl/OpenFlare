# 开发约束

你会学到：OpenFlare 代码修改的准入标准、后端/Agent/前端分层约束、数据模型边界、API 约定、数据库迁移要求和测试交付基线。

本文档融合原开发规范、前端规范与开发计划，是 OpenFlare `1.0.0` 之后的工程约束入口。

## 当前结论

* 第一版至第六版的主线能力已经全部完成。
* `1.0.0` 是当前正式基线。
* 已完成阶段的过程性任务以代码、测试与 Git 历史为准。
* 新工作优先以缺陷修复、可维护性改进、文档与测试补强为主。

当前开发优先级：

1. 稳定性。
2. 升级与回滚链路可靠性。
3. 文档准确性。
4. 测试覆盖补强。
5. 在既有边界内的小步迭代。

## 变更准入

新需求进入实现前，按以下顺序判断：

1. 是否符合 [产品边界](../design/index.md)。
2. 是否符合本文档的后端、Agent 与前端约束。
3. 是否会破坏现有发布、同步、回滚或升级主链路。
4. 是否需要同步更新部署、配置、README 或文档站页面。

如果需求超出边界或引入新基础设施，应先更新设计文档，再开始实现。

任何合入正式基线的改动，至少应满足：

* 不破坏 Agent 心跳、同步、发布与回滚主链路。
* 不破坏现有 OpenResty 主配置托管模型。
* 不降低总览、节点详情与访问分析的既有可用性。
* 有与风险相称的测试或联调验证。
* 文档与代码保持一致。

## 技术基线

Server：

* Go 1.25+
* Gin
* GORM
* SQLite / PostgreSQL
* 现有登录体系

Agent：

* 单二进制
* 节点本地执行
* 通过 `openresty_path` 或默认 `openresty` 控制 OpenResty 二进制
* Docker 部署使用内置 OpenResty 的 Agent 镜像，不由 Agent 再控制独立 OpenResty 容器

Frontend：

* Next.js 15 App Router
* React 19
* TypeScript 5
* Tailwind CSS 4
* TanStack Query
* React Hook Form + Zod
* Zustand 仅用于轻量客户端状态
* ESLint + Prettier
* Vitest + Testing Library + Playwright
* pnpm

## 工程分层约束

各组件和模块（Server、Agent、Frontend）的物理目录分层职责详见 [仓库结构](../design/repository.md)。在此结构下，开发必须遵守以下核心分层规则：

* **Server 开发规则**：禁止在 `controller/` 堆积业务逻辑，禁止在 `middleware/` 实现业务流程，禁止为简单需求新增平台层抽象。
* **Agent 开发规则**：每个模块职责单一，外部命令调用集中封装，状态落盘与配置落盘分离。
* **Frontend 开发规则**：页面文件只负责获取路由参数、组织页面结构、调用 feature 组件；不应手写复杂 API 细节、复杂表单校验逻辑或维护大量彼此耦合的局部状态。

## 数据模型规范

在定义和修改 Go/GORM 模型实体时，所有模型的业务边界与设计约束必须严格符合 [产品边界](../design/index.md)。

### 1. 当前有效实体
* **核心配置与反代**：`proxy_routes` (网站配置), `origins` (源站), `config_versions` (配置版本), `tls_certificates` (证书), `managed_domains` (托管域名).
* **节点与状态**：`nodes` (节点), `node_system_profiles` (系统概况), `apply_logs` (应用日志).
* **内网穿透**：`tunnels` (隧道客户端), `tunnel_tokens` (隧道认证令牌，可选持久化).
* **观测与分析**：`node_request_reports` (请求上报), `node_access_logs` (访问明细), `node_metric_snapshots` (指标快照), `traffic_analytics_rollups` (流量聚合), `node_health_events` (健康事件).
* **系统配置与第三方登录**：`options` (全局参数), `auth_sources` (第三方认证源), `external_accounts` (外部绑定账号).
* **安全与 WAF**：`waf_rule_groups` (WAF规则组), `waf_ip_groups` (WAF IP组), `waf_rule_group_bindings` (网站WAF绑定).

### 2. 底层数据库技术约束

在编写或修改模型时，必须严格遵守以下持久化与数据库设计准则：

* **禁止随意引入平台化新实体**：除非 [产品边界](../design/index.md) 设计发生调整并经评审。

* **业务唯一性保障**：
  * `proxy_routes.site_name` 作为业务唯一主标识。
  * `proxy_routes.domains` 中的各域名必须全局唯一，不可跨站点冲突，列表第一项视为主域名。
  * `nodes.node_id` 唯一标识节点（自动生成或由用户指定）。
  * `tunnels.tunnel_id` 唯一标识内网穿透客户端（格式 `tun-<32hex>`，自动生成）。

* **兼容字段处理**：遗留的 `proxy_routes.domain` 只能作为 `domains[0]` 的只读/兼容镜像，新代码不得以该字段为唯一业务输入。

* **多上游及 Keepalive**：单上游时应支持 base path/query 并在 `proxy_pass` 中正确补齐 URI；多上游负载均衡时仅允许纯 `scheme://host[:port]`。

* **证书映射**：证书绑定必须通过逐域名平行的 `domain_cert_ids` 字段精确保存，未绑定证书的域名不得参与 HTTPS 渲染。

* **版本快照一致性**：`config_versions` 必须保存版本发布时的完整快照及 checksum 校验码，确保渲染结果不可变且全局单激活版本。

* **外部账户唯一绑定**：第三方登录必须通过 `external_accounts` 映射至本地唯一用户，原 `users.github_id` 仅用于向后兼容迁移，任何新登录流程禁止以此为业务输入。

* **Tunnel 与上游关联**：
  * `proxy_routes.upstream_type = 'tunnel'` 时，必须指定 `tunnel_id`（关联到 `tunnels` 表）。
  * 必须指定 `tunnel_target_addr`（内网目标地址，如 `192.168.1.100:8080`）和 `tunnel_target_protocol`（`http` 或 `https`）。
  * 发布配置时，Server 自动将此上游渲染为 `http://127.0.0.1:{relay_vhost_port}`，Agent 依据 Host 头由 frps 路由。

* **TunnelRelay 节点配置**：
  * `nodes.node_type = 'tunnel_relay'` 时，新增字段 `relay_bind_port`、`relay_vhost_http_port`、`relay_auth_token` 必须有合理默认值。
  * `relay_bind_port` 默认 7000，`relay_vhost_http_port` 默认 8080。
  * `relay_auth_token` 由 Server 自动生成（32 位随机字符串），不由用户输入。
  * 相对静态配置（如 `relay_agent_access_addr`、`relay_client_access_addr`）由 Relay 心跳下发，Server 可记录但不纳入版本化流。

* **Tunnel 客户端状态**：
  * `tunnels.status` 记录客户端在线/离线/待激活状态。
  * `tunnels.current_version` / `tunnels.current_checksum` 记录当前已应用的配置版本。
  * `tunnels.connected_relays` 以 JSON 数组形式存储已连接 Relay 的信息（relay_node_id、连接状态等）。
  * `last_seen_at`、`last_error` 用于调试和可观测性。

## 数据库迁移

任何涉及表结构、索引、列类型、分表规则或内部持久化元数据的修改，都必须同步提升数据库版本号。

数据库版本号定义在 `openflare_server/model`，不得只依赖 `AutoMigrate` 隐式升级存量数据库。

每次提升数据库版本号时，必须补充从上一版本升级到新版本的显式迁移方法。迁移方法必须包含升级后的校验逻辑；只有校验通过，才能写入新的数据库版本记录。

v1-v7 视为历史初始基线，不再维护逐版本升级文件。从 v8 起，数据库迁移必须放在 `openflare_server/model/migrate` 目录中，并以目标版本命名文件，例如 `v16.go`。每个版本文件通过 `init()` 注册自己的迁移，当前数据库版本取已注册迁移的最大目标版本。不得为了整理文件而改变已发布 v8+ 迁移的语义。

执行数据库升级时必须按以下步骤完成：

1. 判断是否需要升级数据库版本：凡是新增/删除/重命名表、字段、索引、约束、列类型、分表规则，或改变持久化数据语义，都必须升级。
2. 新增 `openflare_server/model/migrate/vN.go`，其中 `N` 为目标版本号。文件头部必须包含注释，说明本次升级了什么内容，以及为什么需要升级。
3. 在 `vN.go` 中实现 `VN()`，并在 `init()` 中调用 `Register(VN())`。`FromVersion` 必须等于 `N-1`，`ToVersion` 必须等于 `N`。
4. 在 `migrateVN` 中写入升级逻辑。可通过 `Context` 调用 `ApplyCurrentSchema`、历史 backfill、默认数据初始化等公共能力；复杂数据修复必须显式处理，不得只依赖 `AutoMigrate`。
5. 在 `validateVN` 中写入升级后的校验逻辑。校验至少要覆盖新增表/字段/索引是否存在、关键默认数据是否存在、必要的数据回填是否成功。
6. 如果新迁移需要新的公共 backfill 或校验辅助函数，将其放在 `openflare_server/model/migrations.go` 或更合适的 model 文件中，并通过 `Context` 暴露给 `model/migrate`，避免子包反向 import `model` 造成循环依赖。
7. 补充迁移测试：至少覆盖从 `N-1` 老库升级到 `N` 后 schema version、字段/表结构、关键数据回填和校验结果。注册表连续性由 `model/migrate` 测试兜底，但具体业务迁移仍必须有测试。
8. 同步更新设计/开发文档；如果管理端 API、配置项或用户可见行为变化，还要同步更新对应指南、配置参考和 Swagger 文档。

新包启动后必须先检查数据库当前版本，再按顺序逐步升级到目标版本；禁止跳过中间升级步骤直接写目标版本。

空库初始化可以直接建立当前版本结构，但初始化完成后仍必须执行同版本校验，并落库当前数据库版本。

如果迁移失败或校验失败，启动流程必须中止，且不得提升数据库版本记录。涉及数据库版本变更的提交，必须补充对应的迁移测试或等效回归测试。

## API 与鉴权

管理端与 Agent/Relay/Client API 统一使用 JSON。成功与失败都必须返回清晰 `message`：

```json
{
  "success": true,
  "message": "",
  "data": {}
}
```

约定：

* Agent API 固定放在 `/api/agent/*`，使用 `X-Agent-Token` 认证（节点专属 token）。
* **Relay API** 固定放在 `/api/relay/*`，使用 `X-Agent-Token` 认证（同 TunnelRelay 节点）。
  - Server 通过 token + `/api/relay/*` 路径区分 Relay 请求。
  - Relay 心跳返回 frps 配置（bindPort、vhostHTTPPort、authToken）。
  - Relay 上报进程状态、连接数、proxy 列表等指标。
* **Tunnel Client API** 固定放在 `/api/flared/*`，使用 `X-Tunnel-Token` 认证（独立的 tunnel_token）。
  - OpenFlared 使用 `tunnel_token` 与 Server 通信，独立于 Agent 认证体系。
  - Client 心跳返回 tunnel 配置版本摘要。
  - Client 可拉取完整配置（relay 列表 + frpc 代理定义）。
  - Client 上报配置应用结果。
* **Admin Tunnel 管理 API** - `/api/tunnels/*`，要求 Admin Session。
  - CRUD tunnel 实体（创建、查询、更新、删除）。
  - Token 管理（生成、轮换）。
  - 强制同步（触发 Client 立即拉取新配置）。
* 总览与节点详情优先使用专用聚合接口。
* 管理端变更类接口统一使用 `POST`；只读接口使用 `GET`。
* 管理端继续复用现有登录、角色与 Session。
* 第三方登录统一通过认证源 API 进入，认证源管理接口必须要求 Root Session。
* `/api/status` 只能返回已启用认证源的公开字段，不得返回 Client Secret。
* 第三方账号未绑定且注册关闭时，应提供绑定已有账号流程，不得自动创建用户。
* Agent/Relay/Client 正式请求统一使用对应的专属 token（`agent_token` / `relay_token`（即 agent_token） / `tunnel_token`）。
* 首次接入 Agent 可使用全局 `discovery_token`；首次接入 Client 由 Server 生成 tunnel_token，直接用于部署命令。
* Agent/Relay 请求头统一使用 `X-Agent-Token`；Client 请求头统一使用 `X-Tunnel-Token`。

禁止暴露远程 shell 或任意命令执行入口，禁止在日志中打印完整 Token，禁止绕过占位符约束保存不可渲染的主配置模板。

## 发布与运行

发布逻辑必须保持：

* 发布时读取全部启用的 `proxy_routes`。
* 同时读取 OpenResty 主配置参数、反代性能参数与缓存参数。
* 读取 WAF 规则组、规则组引用的 IP 组与网站绑定关系，并在发布快照中保存可回放数据。
* 自动型 WAF IP 组只能由 Server 定时任务读取请求日志并执行 Expr 布尔规则，OpenResty Lua 与 Agent 不得直接访问请求日志库或执行自动挖掘逻辑。
* 发布版本不得展开 WAF IP 组成员；Agent 必须通过独立的 IP 组 checksum 差异同步和 WebSocket 增量广播维护本地 `waf_ip_groups.json`。
* **内网穿透配置扩展**：区分上游类型，为 `upstream_type = 'tunnel'` 的代理规则生成独立的 tunnel 配置数据。
  * OpenResty 侧：将 tunnel 上游自动渲染为 `http://127.0.0.1:{relay_vhost_port}`，必须保留原始 `Host` 请求头。
  * Tunnel 侧：为每个 Client 生成完整的 relay 列表与 frpc 代理定义（frpc proxy 配置）。
* 生成完整 OpenResty 配置。
* 计算 `checksum`。
* 写入 `config_versions`（OpenResty 部分）+ 生成或更新 tunnel 配置版本数据。
* 通过切换 `is_active` 激活版本。

版本约束：

* 版本号格式固定为 `YYYYMMDD-NNN`。
* 同一版本号同时关联 OpenResty 配置与 Tunnel 配置，保证一致性。
* 不在线修改历史版本。
* 不做按节点分组的差异化版本。
* 预览与 diff 是只读能力，不产生发布记录。

Agent 必须满足：

* 启动后读取或生成本地 `node_id`。
* 周期性心跳与同步。
* 常规同步优先依据 heartbeat 返回的版本摘要判断。
* WS 连接升级开启且连接成功时，Agent 可通过 WS 接收激活版本摘要并立即同步；WS 失败或断开必须退回 HTTP heartbeat。
* 发现新版本时先备份旧文件。
* 写入主配置、路由配置与必要证书文件。
* 写入 WAF/PoW 运行时配置，并确保 WAF Lua 资源由 Agent 统一管理。
* WAF IP 组同步必须按组增量更新，不得在每次心跳或每次同步中传输全部 IP 组。
* 写入新配置后执行 `openresty -t -c <main_config_path>`，再 reload；reload 发现运行时未启动时允许直接启动 OpenResty。
* 周期性运行时健康检查不得调用 `openresty -t`，避免健康探针触发 upstream 域名同步解析；应优先请求本地 `openresty_observability_port` 上的 `/openflare/stub_status`，以 HTTP `200 OK` 作为 OpenResty 主进程和 worker 正在提供服务的判断依据。
* 新配置激活失败时必须先尝试用目标配置恢复运行，再回滚到旧配置并重新拉起 OpenResty。
* 回滚后 OpenResty 恢复正常时上报警告；如果本地没有历史主配置可恢复，必须允许写入内置安全兜底配置并拉起对外只监听 `80` 端口、统一返回 `503` 的 OpenResty 运行态；兜底配置仍需保留本地 `stub_status` 健康检查入口。
* 兜底运行态不得清除失败目标的阻断状态；应用记录必须能体现目标版本失败但 fallback runtime 已启动。存在历史主配置但回滚后仍无法恢复运行时上报失败。
* 某个目标 `version + checksum` 一旦应用失败并回退，Agent 必须在本地状态中阻断该目标的重复应用。
* Agent 维护本地 MaxMind mmdb 时，下载或刷新失败只能记录警告，不得阻断心跳、同步、配置应用或 OpenResty 健康检查。

OpenFlareRelay 必须满足：

* 启动后从 config 读取 Server 地址和 `agent_token`。
* 周期性向 Server 发送心跳，获取 frps 配置（bindPort、vhostHTTPPort、authToken）。
* 根据心跳响应生成 frps.toml，启动或更新 frps 进程。
* 上报 frps 进程健康状态、连接数、proxy 数等指标。
* frps 进程异常时自动重启，并上报失败信息。
* 可选支持 WebSocket 升级连接，接收实时配置推送。

OpenFlared 必须满足：

* 启动后从 config 读取 Server 地址和 `tunnel_token`。
* 周期性向 Server 发送心跳，获取 tunnel 配置版本摘要。
* 发现新版本后拉取完整 tunnel 配置（relay 列表 + frpc 代理定义）。
* 为每个 relay 生成独立 frpc.toml，启动新 frpc 进程或对已有进程执行热重载。
* 上报每个 frpc 进程的健康状态与连接情况。
* 配置应用失败时记录错误并上报，支持重试。
* 可选支持 WebSocket 升级连接，接收实时配置变更通知。

## 前端请求、状态与类型

所有 API 请求必须统一经过 `lib/api/`：

* 统一处理 `success/message/data` 响应结构。
* 统一处理鉴权失效、网络异常和通用错误消息。
* 统一维护资源接口与请求路径。

状态分层：

* 服务端状态：TanStack Query。
* 页面临时状态：组件内部 `useState`。
* 跨页面 UI 状态：Zustand。

要求开启 TypeScript 严格模式，禁止滥用 `any`，API 响应、表单输入、业务实体必须有明确类型。

## 表单、交互、样式与主题

表单统一使用 React Hook Form 与 Zod。

高风险操作必须二次确认、展示操作对象名称，并明确成功与失败反馈。

样式原则：

* 统一使用 Tailwind CSS 与现有 token 体系。
* 优先复用已有基础组件与布局组件。
* 保持视觉层级、留白与语义颜色一致。

主题要求：

* 同时支持 `light`、`dark`、`system`。
* 用户选择必须持久化。
* 首屏尽量避免主题闪烁。

## 测试与交付

* 关键业务逻辑必须有单元测试或等效回归测试。
* Agent 主链路修改必须验证同步、应用与回滚。
* 前端页面至少覆盖加载态、空态、错误态与成功反馈。
* Go 版本调整时，同步检查 `go.mod`、Dockerfile 与 CI 工作流。

## 后续维护方式

后续规划不再按“大版本阶段文档”维护，而采用以下方式：

* 产品边界变动：更新 [产品边界](../design/index.md)。
* 工程约束变动：更新本文档。
* 部署与配置变动：更新 [部署说明](../reference/deployment.md)、[配置项](../reference/configuration.md) 与 README。

如果未来出现明确的新阶段目标，再单独新增专项计划文档；不要把已完成的历史计划继续堆回本文档。

当前专项“网站级规则与配置界面改造”的模型边界已纳入 [产品边界](../design/index.md)，执行时仍按数据模型、接口、前端页面、迁移测试与文档联动的顺序推进。
