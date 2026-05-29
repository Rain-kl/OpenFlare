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

1. 是否符合 [产品边界](./)。
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

## Server 分层

| 目录 | 职责 |
| --- | --- |
| `controller/` | 参数解析、调用 service、返回响应 |
| `service/` | 业务逻辑、校验、事务编排、渲染 |
| `model/` | 模型定义与持久化 |
| `router/` | 路由注册 |
| `middleware/` | 认证、鉴权、限流等横切逻辑 |
| `common/` | 配置、全局状态与初始化入口 |
| `utils/` | 纯工具函数与通用 helper |

禁止在 `controller/` 堆积业务逻辑，禁止在 `middleware/` 实现业务流程，禁止为简单需求新增平台层抽象。

## Agent 分层

Agent 保持现有模块边界：

* `config`
* `heartbeat`
* `sync`
* `openresty` / `nginx`
* `state`
* `httpclient`
* `protocol`
* `internal/updater`

要求：

* 每个模块职责单一。
* 外部命令调用集中封装。
* 状态落盘与配置落盘分离。

## Frontend 分层

推荐目录：

```text
app/
components/
features/
lib/
hooks/
store/
types/
styles/
tests/
```

职责约束：

* `app/`：路由、布局、页面组装。
* `features/`：按业务域组织模块。
* `components/`：跨 feature 复用组件。
* `lib/`：请求客户端、环境变量、工具函数、常量。
* `store/`：少量跨页面 UI 状态。
* `types/`：共享类型定义。

页面文件只负责获取路由参数、组织页面结构、调用 feature 组件；不应手写复杂 API 细节、复杂表单校验逻辑或维护大量彼此耦合的局部状态。

## 数据模型规范

当前有效实体：

* `proxy_routes`
* `origins`
* `config_versions`
* `nodes`
* `auth_sources`
* `external_accounts`
* `node_system_profiles`
* `apply_logs`
* `tls_certificates`
* `managed_domains`
* `node_request_reports`
* `node_access_logs`
* `node_metric_snapshots`
* `traffic_analytics_rollups`
* `node_health_events`
* `options`

通用约束：

* 不新增平台化对象，除非设计文档明确要求。
* `origins` 仅作为可复用源站地址目录，字段保持轻量。
* `proxy_routes` 以“网站配置”作为聚合边界，必须包含唯一 `site_name` 与非空 `domains` 列表。
* `proxy_routes.domains` 中的每个域名都必须全局唯一，列表第一项视为主域名。
* `proxy_routes` 继续允许保存一个或多个上游地址用于负载均衡，但不引入独立 `origin_pool`。
* 遗留 `domain` 字段只能作为 `domains[0]` 的兼容镜像；新代码不得继续以该字段作为唯一业务输入。
* `proxy_routes` 如关联 `origins`，必须同时保存可直接渲染的 `origin_url`。
* 上游统一使用 named `upstream` + keepalive；单上游如带 base path 或 query，应在 `proxy_pass` 上补回 URI，多上游仅允许纯 `scheme://host[:port]`。
* 流量限制、反向代理与缓存配置当前都归属站点级 `proxy_routes`。
* HTTPS 证书绑定必须通过与 `domains` 平行的 `domain_cert_ids` 逐域名保存；未绑定证书的域名不得参与 HTTPS 渲染。
* `config_versions` 必须保存完整快照与渲染结果。
* 全局同时只能有一个激活版本。
* 回滚通过重新激活旧版本实现。
* `nodes` 只保留控制面状态与低频摘要。
* 观测数据必须按节点与时间窗口关联，快照与聚合结果采用追加式模型。
* 原始访问明细必须有受控保留策略。
* `auth_sources` 仅保存管理端第三方登录源配置，当前支持 `github` 与 `oidc`。
* `external_accounts` 是第三方账号与本地用户的唯一绑定来源；旧 `users.github_id` 仅用于兼容迁移，不得作为新登录流程的业务输入。

## 数据库迁移

任何涉及表结构、索引、列类型、分表规则或内部持久化元数据的修改，都必须同步提升数据库版本号。

数据库版本号定义在 `openflare_server/model`，不得只依赖 `AutoMigrate` 隐式升级存量数据库。

每次提升数据库版本号时，必须补充从上一版本升级到新版本的显式迁移方法。迁移方法必须包含升级后的校验逻辑；只有校验通过，才能写入新的数据库版本记录。

新包启动后必须先检查数据库当前版本，再按顺序逐步升级到目标版本；禁止跳过中间升级步骤直接写目标版本。

空库初始化可以直接建立当前版本结构，但初始化完成后仍必须执行同版本校验，并落库当前数据库版本。

如果迁移失败或校验失败，启动流程必须中止，且不得提升数据库版本记录。涉及数据库版本变更的提交，必须补充对应的迁移测试或等效回归测试。

## API 与鉴权

管理端与 Agent API 统一使用 JSON。成功与失败都必须返回清晰 `message`：

```json
{
  "success": true,
  "message": "",
  "data": {}
}
```

约定：

* Agent API 固定放在 `/api/agent/*`。
* 总览与节点详情优先使用专用聚合接口。
* 管理端变更类接口统一使用 `POST`；只读接口使用 `GET`。
* 管理端继续复用现有登录、角色与 Session。
* 第三方登录统一通过认证源 API 进入，认证源管理接口必须要求 Root Session。
* `/api/status` 只能返回已启用认证源的公开字段，不得返回 Client Secret。
* 第三方账号未绑定且注册关闭时，应提供绑定已有账号流程，不得自动创建用户。
* Agent 正式请求统一使用节点专属 `agent_token`。
* 首次接入可使用全局 `discovery_token`。
* Agent 请求头统一使用 `X-Agent-Token`。

禁止暴露远程 shell 或任意命令执行入口，禁止在日志中打印完整 Token，禁止绕过占位符约束保存不可渲染的主配置模板。

## 发布与运行

发布逻辑必须保持：

* 发布时读取全部启用的 `proxy_routes`。
* 同时读取 OpenResty 主配置参数、反代性能参数与缓存参数。
* 生成完整 OpenResty 配置。
* 计算 `checksum`。
* 写入 `config_versions`。
* 通过切换 `is_active` 激活版本。

版本约束：

* 版本号格式固定为 `YYYYMMDD-NNN`。
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
* 写入新配置后执行 `openresty -t -c <main_config_path>`，再 reload；reload 发现运行时未启动时允许直接启动 OpenResty。
* 周期性运行时健康检查不得调用 `openresty -t`，避免健康探针触发 upstream 域名同步解析；应优先请求本地 `openresty_observability_port` 上的 `/openflare/stub_status`，以 HTTP `200 OK` 作为 OpenResty 主进程和 worker 正在提供服务的判断依据。
* 新配置激活失败时必须先尝试用目标配置恢复运行，再回滚到旧配置并重新拉起 OpenResty。
* 回滚后 OpenResty 恢复正常时上报警告；如果本地没有历史主配置可恢复，必须允许写入内置安全兜底配置并拉起对外只监听 `80` 端口、统一返回 `503` 的 OpenResty 运行态；兜底配置仍需保留本地 `stub_status` 健康检查入口。
* 兜底运行态不得清除失败目标的阻断状态；应用记录必须能体现目标版本失败但 fallback runtime 已启动。存在历史主配置但回滚后仍无法恢复运行时上报失败。
* 某个目标 `version + checksum` 一旦应用失败并回退，Agent 必须在本地状态中阻断该目标的重复应用。

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

* 产品边界变动：更新 [产品边界](./)。
* 工程约束变动：更新本文档。
* 部署与配置变动：更新 [部署说明](../guide/deployment.md)、[配置项](../reference/configuration.md) 与 README。

如果未来出现明确的新阶段目标，再单独新增专项计划文档；不要把已完成的历史计划继续堆回本文档。

当前专项“网站级规则与配置界面改造”的模型边界已纳入 [产品边界](./)，执行时仍按数据模型、接口、前端页面、迁移测试与文档联动的顺序推进。
