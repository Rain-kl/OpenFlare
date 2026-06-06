# 开发约束

OpenFlare 代码修改的准入标准、后端/Agent/前端分层约束、数据模型边界、API 约定、数据库迁移要求和测试交付基线。

## 变更准入

新需求进入实现前，按以下顺序判断：

1. 是否符合 [产品边界](../design/index.md)。
2. 是否符合本文档的后端、Agent 与前端约束。
3. 是否会破坏现有发布、同步、回滚或升级主链路。
4. 是否需要同步更新部署、配置、README 或文档站页面。

如果需求超出边界或引入新基础设施，应先更新设计文档，再开始实现。

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

各组件和模块（Server、Agent、Frontend）的物理目录分层职责详见 [仓库结构](../design/index.md#仓库结构)。在此结构下，开发必须遵守以下核心分层规则：

* **Server 开发规则**：
  * 禁止在 `controller/` 堆积业务逻辑，禁止在 `middleware/` 实现业务流程，禁止为简单需求新增平台层抽象。
  * **定时任务开发规则**：禁止将不同业务模块（如 Uptime Kuma 整合、WAF IP 同步等）的定时任务具体执行逻辑与状态堆积在单个 `cron.go` 文件中。各模块对应的定时任务结构体和运行逻辑必须在独立的 Go 文件中定义，`cron.go` 只允许承担统一注册、初始化与调度器启停的职责。
* **Agent 开发规则**：每个模块职责单一，外部命令调用集中封装，状态落盘与配置落盘分离。
* **Frontend 开发规则**：页面文件只负责获取路由参数、组织页面结构、调用 feature 组件；不应手写复杂 API 细节、复杂表单校验逻辑或维护大量彼此耦合的局部状态。

## 数据模型规范

在定义和修改 Go/GORM 模型实体时，所有模型的业务边界与设计约束必须严格符合 [产品边界](../design/index.md)。

### 1. 当前有效实体
* **核心配置与反代**：`proxy_routes` (网站配置), `origins` (源站), `config_versions` (配置版本), `tls_certificates` (证书), `managed_domains` (托管域名).
* **Pages 静态托管**：`pages_projects` (Pages 项目), `pages_deployments` (不可变部署), `pages_deployment_files` (部署文件清单).
* **节点与状态**：`nodes` (节点), `node_system_profiles` (系统概况), `apply_logs` (应用日志).
* **内网穿透**：`tunnels` (隧道客户端), `tunnel_tokens` (隧道认证令牌，可选持久化).
* **观测与分析**：`node_request_reports` (请求上报), `node_access_logs` (访问明细), `node_metric_snapshots` (指标快照), `traffic_analytics_rollups` (流量聚合), `node_health_events` (健康事件).
* **系统配置与第三方登录**：`options` (全局参数), `auth_sources` (第三方认证源), `external_accounts` (外部绑定账号).
* **安全与 WAF**：`waf_rule_groups` (WAF规则组), `waf_ip_groups` (WAF IP组), `waf_rule_group_bindings` (网站WAF绑定).

### 2. 底层数据库技术约束

在编写或修改模型时，必须严格遵守以下持久化与数据库设计准则：

* **禁止随意引入平台化新实体**：除非 [产品边界](../design/index.md) 设计发生调整并经评审。

## 数据库迁移

任何涉及表结构、索引、列类型、分表规则或内部持久化元数据的修改，都必须同步提升数据库版本号。

数据库版本号定义在 `openflare-server/model`，不得只依赖 `AutoMigrate` 隐式升级存量数据库。

每次提升数据库版本号时，必须补充从上一版本升级到新版本的显式迁移方法。迁移方法必须包含升级后的校验逻辑；只有校验通过，才能写入新的数据库版本记录。

数据库升级统一使用 goose。新的 goose provider、桥接逻辑、注册入口和具体迁移文件必须全部放在 `openflare-server/model/goose` 包下，`openflare-server/model` 根包只保留纯净实体类、旧框架兼容适配和必要的上下文注入。每次新增数据库升级都必须新建一个单独的 Go 文件，文件名使用 `openflare-server/model/goose/goose_<timestamp>_<description>.go`，例如 `openflare-server/model/goose/goose_202606020001_add_node_capabilities_json.go`。迁移文件必须同时包含该版本的 goose migration 构造函数、升级逻辑和校验逻辑；`model/goose/migrations.go` 只能作为注册入口和公共构造工具，禁止把具体迁移逻辑集中堆放在该文件中。

执行数据库升级时必须按以下步骤完成：

1. 判断是否需要升级数据库版本：凡是新增/删除/重命名表、字段、索引、约束、列类型、分表规则，或改变持久化数据语义，都必须升级。
2. 新增 `openflare-server/model/goose/goose_<timestamp>_<description>.go`，其中 `<timestamp>` 为 goose 版本号。文件头部或迁移构造函数附近必须包含注释，说明本次升级了什么内容，以及为什么需要升级。
3. 在该文件中实现独立迁移构造函数，并返回通过 `newGORMMigration(...)` 创建的 migration；随后只在 `openflare-server/model/goose/migrations.go` 的 `registeredMigrations(...)` 中新增一条注册项。
4. 在同一个单独迁移文件中写入升级逻辑。可通过 goose `Context` 调用 `ApplyCurrentSchema`、历史 backfill、默认数据初始化等公共能力；复杂数据修复必须显式处理，不得只依赖 `AutoMigrate`。
5. 在同一个单独迁移文件中写入升级后的校验逻辑。校验至少要覆盖新增表/字段/索引是否存在、关键默认数据是否存在、必要的数据回填是否成功。
6. 如果新迁移需要新的公共 backfill 或校验辅助函数，优先放在该迁移文件中；只有多个迁移共同复用时，才放到 `openflare-server/model/goose` 包内的公共文件中。不要把新 goose 框架代码放回 `openflare-server/model` 根包。
7. 补充迁移测试：至少覆盖从旧框架终点或上一 goose 版本升级后 schema version、字段/表结构、关键数据回填和校验结果。还应保留旧库从 v15/v17 桥接到 goose 的回归覆盖。

新包启动后必须先检查数据库当前版本，再按顺序逐步升级到目标版本；禁止跳过中间升级步骤直接写目标版本。

空库初始化可以直接建立当前版本结构，但初始化完成后仍必须执行同版本校验，并落库当前数据库版本。

如果迁移失败或校验失败，启动流程必须中止，确保数据库能够回滚。涉及数据库版本变更的提交，必须补充对应的迁移测试或等效回归测试。

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
* **Tunnel Client API** 固定放在 `/api/flared/*`，使用 `X-Tunnel-Token` 认证（独立的 tunnel_token）。
  - OpenFlared 使用 `tunnel_token` 与 Server 通信，独立于 Agent 认证体系。
* **Admin Tunnel 管理 API** - `/api/tunnels/*`。
  - CRUD tunnel 实体（创建、查询、更新、删除）。
  - Token 管理（生成、轮换）。
  - 强制同步（触发 Client 立即拉取新配置）。
* **Admin Pages 管理 API** - `/api/pages/*`。
  - CRUD Pages 项目，包括 SPA fallback 启用状态与回退路径。
  - 上传 zip 部署包、查看部署历史、激活部署、删除非激活部署。
* **Agent Pages 下载 API** - `/api/agent/pages/*`，使用 `X-Agent-Token` 认证。
  - Agent 仅能按激活配置引用的部署 ID 拉取静态部署包，不提供任意文件读取或远程命令入口。
* 管理端变更类接口统一使用 `POST`；只读接口使用 `GET`。
* 管理端登录成功后返回用户 token；管理端 API 只允许从 `OPENFLARE_TOKEN` 请求头读取登录凭证，不得通过 Cookie Session 放行。
* `/api/status` 只能返回已启用认证源的公开字段，不得返回 Client Secret。
* 系统仅单租户使用, 不得创建用户。
* Agent/Relay/Client 正式请求统一使用对应的专属 token（`agent_token` / `relay_token`（即 agent_token） / `tunnel_token`）。
* 首次接入 Agent 可使用全局 `discovery_token`；首次接入 Client 由 Server 生成 tunnel_token，直接用于部署命令。
* Agent/Relay 请求头统一使用 `X-Agent-Token`；Client 请求头统一使用 `X-Tunnel-Token`。

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
* 部署与配置变动：更新 [部署说明](../deployment/deployment.md)、[配置项](../reference/configuration.md) 与 README。

如果未来出现明确的新阶段目标，再单独新增专项计划文档；不要把已完成的历史计划继续堆回本文档。

当前专项“网站级规则与配置界面改造”的模型边界已纳入 [产品边界](../design/index.md)，执行时仍按数据模型、接口、前端页面、迁移测试与文档联动的顺序推进。
