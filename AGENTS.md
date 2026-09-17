# AGENTS.md

Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.

## Skills（匹配任务时必读）

| Skill | 何时使用 |
| :--- | :--- |
| `new-api` | 业务 API、Handler、服务层、路由注册 |
| `new-async-task` | Asynq 任务、定时任务、TaskHandler、任务元数据 |
| `new-setting` | 系统/业务/公开设置、`/admin/system`、`/admin/settings` |
| `database-migration` | 表结构、goose 迁移（PG/SQLite/ClickHouse）、seed |
| `logstore` | 日志/分析用途表、`backend/internal/repository/logstore`、切换日志主库、PG/SQLite 回落 |
| `clickhouse-batchwriter` | CH 批量写入、batchwriter、分析表 flush/背压 |
| `file-upload` | 上传/摄取、`upload.Ingest`、文件访问、`w_uploads` |
| `cache-framework` | 业务缓存（RAM/Redis/DB）、失效、多节点同步 |
| `push-notification` | 通知推送事件、统一触发器、带推送的业务 |
| `release-guide` | Version Bump 提交信息（触发双语 Release） |
| `shadcn` | 添加/修改/组合 shadcn/ui 组件 |

## 硬性约束

### 上游/下游改动归属（Cordis）

- 触碰框架目录 `backend/{core,pkg,plugins}` 前，先判断能力归属：
  - **通用能力**（与 OpenFlare 业务无关、任何下游都用得上）→ 必须同步在 **Wavelet 上游**完成修改，
    本仓库通过 `git fetch wavelet && git merge wavelet/main` 取得，不得长期持有本地补丁。
  - **非通用能力**（OpenFlare 业务特有）→ 在自己的插件内（`backend/openflare/plugins/<name>/`）实现，
    或新建一个下游插件，禁止塞进上游目录。
- 开发下游功能优先**复用上游已有能力**（`core/contracts`、`backend/plugins/*`、`backend/pkg/*`）；
  发现上游已提供而下游仍保留本地副本的，删除本地副本改为复用，或把差量回流上游。
- 上游暂缺而确属通用能力时，可先在本仓库实现并登记到 `backend/openflare/upstream-patches.md`
  （merge 上游后请确认补丁仍在），回流 Wavelet 后删除登记并重新 merge。

### Cordis 架构核心防线与分层规范
- **微内核 (`backend/core/`)**：
  - 上下文总线（`Context`）、泛型依赖注入（`Container`）、生命周期编排（`Lifecycle`）、扩展点定义（`extpoints/`）与领域事件总线（`EventBus`）。
  - **严禁**包含任何具体业务逻辑，**严禁** import `gin`、`gorm`、`asynq` 等具体运行时依赖。
- **服务契约 (`backend/core/contracts/`)**：
  - 跨插件通信的统一公开 Go Interface（如 `AuthService`、`UserService`、`CacheService`、`DBService`、`StorageService`）与公共 DTO。
  - **严禁**包含任何具体业务实现或 SQL 操作。
- **自包含插件 (`backend/plugins/`)**：
  - 所有业务功能与驱动实现均以插件形式存在（`backend/plugins/drivers/`、`backend/plugins/infra/`、`backend/plugins/domain/` 或下游 `backend/openflare/plugins/`）。
  - 每个插件实现 `core.Plugin`（`Name() string` 与 `Apply(ctx *core.Context) error`）。
  - **统一插件分层架构与标准模板**：
    - **开发模板唯一基准**：所有插件统一以 `backend/downstream/plugins/custom_example` 为基准模板构建。
    - **物理子包隔离规范**：统一采用物理子包结构（`plugin.go`, `consts/`, `controller/`, `service/`, `dao/`, `model/` [含 `entity/`, `do/`], `migrations/` [含 `postgres/`, `sqlite/`]）。**严禁在根包平铺 `handlers_*`、`service_*`、`dao_*` 等前缀文件**，子包内文件直接按业务实体命名（如 `hello.go`, `user.go`），严格约束 `controller -> service -> dao -> model` 单向依赖。
- **插件通信与依赖隔离**：
  - **严禁跨包 import internal/私有实现**：插件之间严禁直接 import 对方具体实现包代码。
  - **单向服务契约调用**：调用方仅面向 `backend/core/contracts` 编程，在 `Apply` 中通过 `core.Provide[contracts.XxxService](ctx, svc)` 注册服务，通过 `core.Inject[contracts.XxxService](ctx)` 或 `ctx.Using(func(svc contracts.XxxService) { ... })` 声明式解析。
  - **事件总线广播**：状态联动与解耦通信统一通过强类型事件 `ctx.Events().Emit()` 广播，由感兴趣的插件通过 `ctx.Events().On()` 订阅，消除双向依赖与循环引用。
- **扩展点自包含注册**：
  - **HTTP 路由与白名单机制**：
    - 插件自包含在 `Apply` 中通过 `ctx.Router().Group(...)` 挂载路由与中间件，禁止跨插件散落注册。
    - **白名单机制**：`driver_http` 与微内核扩展点提供路由白名单支持（`ctx.Router().RegisterWhitelist(patterns...)`），支持精确路径与通配符（如 `/api/v1/oauth/*`）。
    - **所有权主动声明**：认证域（`auth` 插件）与各业务插件必须在 `Apply` 中主动注册其公开/免鉴权接口（如 `/api/v1/user/login`、`/api/v1/oauth/callback`、`/api/v1/cap/*` 等）。
    - **鉴权中间件放行防线**：`auth` 提供的登录鉴权中间件（`LoginRequired`）必须先执行白名单匹配并自动放行，彻底杜绝免鉴权接口被全局或组级鉴权中间件误拦截（返回 401 Unauthorized）。
  - **异步与定时任务**：插件自包含在 `Apply` 中通过 `ctx.Task().Register(...)` 与 `ctx.Schedule().RegisterCron(...)` 声明。
  - **静态启动配置**：插件自包含在 `Apply` 中通过 `ctx.Config().Bind("<prefix>", &cfg)` 读取**自己声明**的配置，字段以 tag 表达来源：`config`（yaml 路径）、`env`（覆盖变量名）、`default`、`autoEnable`（该变量存在即置真）、`secret`（导出脱敏）。需要在 `Apply` 之前被门禁求值的键，必须在 `DeclareConfig()` 中提前声明并实现 `core.ConfigGatedPlugin`。新增基础设施 key 保持顶层命名（`redis.*`），插件私有配置归 `plugins.<name>.*`。**严禁**再造全局配置单例或在 `backend/pkg/` 读取配置。
  - **动态设置**：插件自包含在 `Apply` 中通过 `ctx.Settings().Register(core.SettingSchema{...})` 声明可热更新的管理台设置模式（与上面的静态启动配置分属两层）。
  - **数据迁移**：插件自包含在内部维护 `migrations/*.sql`，通过 `//go:embed` 打包并在 `Apply` 中通过 `ctx.Migrations().Register(pluginID, embedFS)` 注入。
- **表单一所有者原则 (Single Owner Principle)**：
  - 每张数据表有且仅由一个所有者插件声明与维护（表名使用插件前缀如 `w_order_*`）。
  - 严禁插件 B 跨过所有者插件 A 直接 DDL/DML 旁路读写表 A，必须调用插件 A 暴露的 `contracts` 接口或订阅事件。
- **平台服务复用**：
  - 文件摄取统一使用 `upload.Ingest` / `contracts.StorageService`，禁止绕过存储域直接操作底层 Bucket 或直写文件表。
  - 业务缓存统一使用 `ctx.Cache()`（`contracts.CacheService`）或标准缓存框架，禁止自研不带失效广播的本地 map。
  - 数据库操作通过 `ctx.DB()`（`contracts.DBService`）获取受事务与 Trace 保护的连接。

- 禁止删除 `frontend/node_modules`。
- `backend/pkg/util/` 保持纯净：禁止导入 Gin、GORM、sessions 等 HTTP/Web/DB 框架（会话选项在 `backend/openflare/plugins/server/oauth/session.go`）。
- 测试临时目录只用 `t.TempDir()`，禁止硬编码相对路径写源码树。
- HTTP 路由只由插件在 `Apply` 中经 `ctx.Router()` 声明；`router.BuildEngine()` 只挂引擎级中间件与前端 SPA 兜底，禁止进程级初始化（如 `SyncEvents`、`InitLogWriter`）。
- API 变更后：`make swagger`；开发完成：`make code-check`；提交前：`make format`。
- 缓存/文件管理复用平台实现，业务包禁止自建缓存目录或旁路存储后端。
- 文件摄取走 `upload.Ingest`（`PolicyCreate` / `PolicyDedupNewRecord` / `PolicyResolveExisting`）；删除走 `upload.Remove` / `upload.RemoveOwned`。禁止业务直接 `repository.CreateUpload` / `SoftDeleteUpload` 或 `db.Create(&model.Upload{})`。
- **分层**：`apps → repository → model`，`repository → infra/persistence`；禁止 `model → repository`。
  - `model`：实体、表名、配置 key、查询 DTO、无 IO 规则。禁止 `db.DB` / Redis / CH；禁止 `import repository`。GORM hook 仅可 mutate 自身字段，禁止在 hook 内再查 DB/缓存。
  - `repository`：唯一持久化入口。apps/logics 禁止为业务 CRUD 直调 `db.DB`（管理端 SQL 控制台、infra 内部等例外保留）。禁止新增 `model.Get/List/Create/...` 类数据访问 API。
- 日志/分析表（节点访问日志、用户访问日志、可观测时序）走 `backend/openflare/plugins/server/kernel/repository/logstore`，禁止 apps 直连 `repository/analytics` 或 `db.ChConn`/`db.ChDB`。判定与接入步骤见 `logstore` skill。
- 跨模块集成（任务 Handler、推送事件、域监听、完成钩子）禁止 `init()` 注册；经 `backend/openflare/plugins/server/platform/bootstrap` 在 `backend/cmd` 入口显式装配。
- 核心业务（如 `oauth`、`user`）禁止直接 import push/custom_events；经 `backend/openflare/plugins/server/listener` 发域事件，push 在 bootstrap 订阅。
- 依赖任务/推送注册的测试须显式 `bootstrap.RegisterTasks()` / `RegisterPushDomainEvents()` 等，不依赖 `init()`。
- API 错误必须 `response.Abort*` + `ErrorHandlerMiddleware`；禁止 Handler 直接 `c.JSON(..., response.Err(...))` 或用 HTTP 200 表示失败。

### 文档与 Changelog

- 内容变更同步**中文文档**（不同步英文）。
- 代码/配置变更写入 [`docs/changelog/index.md`](./docs/changelog/index.md) 的 `[Unreleased]`；纯文档变更不写 changelog。
- Changelog：合并相近项；不记格式化/调试/无关重构；用户可读完整中文句；说明效果；不编造；不写密钥等敏感信息；空分类可省略。

## 技术栈

- **后端**：Go 1.25+、Gin、GORM、PostgreSQL、可选 ClickHouse、Redis、Asynq、Cobra、Viper、Swaggo、OTel、Zap、AWS SDK v2、Snowflake IDs
- **前端**：Next.js App Router、TypeScript、Tailwind、pnpm、shadcn/ui

## Git

Conventional Commits：`<type>(<scope>): <subject>`（例：`feat(auth): support email login`）。

---

## 后端

### 命名

| 类别 | 规则 | 例 |
|------|------|-----|
| 包/文件 | 小写蛇形 | `auth_source`、`postgres_logger.go` |
| 导出/未导出标识符 | PascalCase / camelCase | — |
| 请求/响应结构体 | camelCase + 后缀 | `listUsersRequest` |
| 错误文案常量 | camelCase 字符串 `const`（非包级 `error`） | `errBindParamsFailed` |
| YAML 键 | 小写蛇形 | — |

### Handler

- 命名：动词 + 名词（`ListUsers`）；绑定用 `ShouldBindQuery` / `ShouldBindJSON`。
- 每个 HTTP API 需完整 Swagger 注释；API 变更后 `make swagger`。
- Handler：绑定 → 调 logic → 映射为 `Abort*` 或 `response.OK`。
- `logics.go`：接受 `context.Context`，返回结果/error；**禁止**依赖 `*gin.Context`、调用 `Abort*` / `c.JSON`。参考 `backend/internal/apps/user/logics.go`。

### API 响应

信封：`{ "error_msg": "", "data": ... }`。成功 `error_msg` 空、`data` 为载荷；失败 `data` 为 `null`。分页：`data: { total, results }`。

**成功**（始终 HTTP 200）：

```go
c.JSON(http.StatusOK, response.OK(data))
c.JSON(http.StatusOK, response.OKNil())
```

**失败**：仅用 `response.Abort*`（挂 `c.Errors` 并 `Abort`，由 `ErrorHandlerMiddleware` 统一写出并记 OTel）,阅读/internal/shared/response/abort.go使用已有函数

中间件同规则（`oauth.LoginRequired` → Unauthorized；`admin.LoginAdminRequired` → NotFound；`cap.VerifyMiddleware` → Unauthorized）。

- 用户可见错误：模块内 `errs.go` 的 camelCase 字符串常量；禁止向客户端暴露驱动错误/堆栈。
- `response.Err` 仅供中间件构造 JSON，业务禁止用于 `c.JSON`。

**禁止**：`c.JSON(200, response.Err(...))`；Handler 直接 `c.JSON(4xx/5xx, response.Err(...))`；手写 `gin.H` 错误体；在 `logics.go` 里 `Abort*`。

Swagger：`@Success 200` 用具体类型或 `response.Any`；每个可能 Abort 状态声明 `@Failure`。

### 日志

- 运行时错误（DB/Redis/第三方/IO）在 Handler 或 logic 边界用 `backend/pkg/logger`（带 `ctx`）记录，再返回安全 Abort/业务错误。
- 吞错、转通用响应、worker 忽略前必须先记日志。
- 禁止 `_ = err` 静默丢弃重要错误；best-effort 可忽略时加简短注释。
- 只在处理/抑制边界记一次，避免重复刷日志。

### 路由与装配

- `router.go` 只做高层分发，禁止直接挂业务 Handler。归属与开发步骤见 `new-api` skill。
- 跨模块副作用：在 `bootstrap` 增 `Register*`，于对应 `backend/internal/cmd/*.go` 调用（`RegisterAPI` / `RegisterWorker` / `RegisterAll`）。
- API/`all` 模式：`bootstrap.Init` 须在 `RegisterPushDomainEvents()` **之后**调用，保证 `SyncEvents` 同步内置推送元数据。

### 中间件

- 全局：`gin.Recovery()`、`otelgin`、日志、session。
- 登录组：`oauth.LoginRequired()`；管理组：`admin.LoginAdminRequired()`。

### 配置

- 运行时只读 `config.Config`，禁止 `os.Getenv()`。
- 新增配置同步 `config.example.yaml` 与 `backend/internal/infra/config/model.go`。

### 数据库

- 持久化只经 `repository`（或 analytics）；复杂查询不进 Handler；编排在 logics。
- repository 内用 `db.DB(ctx)`（链路追踪）。
- 迁移：`backend/internal/infra/persistence/migrator/goose/` SQL；禁止 GORM AutoMigrate。
- 不建物理外键，关系字段加显式索引。
- 列默认值与 Go 零值（`nil`/`0`/`false`/`""`）一致。

---

## 前端

- Next.js：以 `node_modules/next/dist/docs/` 为准（训练数据可能过时）。
- 示例：`frontend/app/(main)/admin/demo`。

### 样式

- shadcn 用 `variant` + CSS 变量；业务 `className` 不硬编码颜色/背景/阴影。
- 变体不足时扩展组件 variant，不写一次性颜色。

### 页面结构

- 根容器全宽 `w-full`；禁止页面级 `max-w-*`（主布局负责宽度）。
- 外层间距：`py-6` 或 `py-6 px-1`。
- 标题行：`flex items-center gap-2`（有右侧操作则加 `justify-between`）。
- 图标：Lucide 直接放标题容器，`size-5 text-primary`；禁止背景卡片/边框包裹。
- 标题：仅 `h1 className="text-2xl font-semibold tracking-tight"`。
- 多 Tab：各 Tab 独立文件；`page.tsx` 只管 Tabs 状态与触发器；禁止 `page.tsx` 仅转发同名空壳。
- 单文件 > ~600 行或状态过重时拆局部 `components/`；跨页复用放 `frontend/components/common/`。标杆：`/admin/database`。

### 组件放置

| 类型 | 路径 |
|------|------|
| 跨页业务 | `frontend/components/common/` |
| shadcn 原语 | `frontend/components/ui/` |
| 路由专属 | 邻近 feature 目录 |

### Services

```text
frontend/lib/services/<name>/
  types.ts
  <name>.service.ts
  index.ts
```

- 继承 `BaseService`，定义 `basePath`，有类型静态方法；在 `frontend/lib/services/index.ts` 注册。
- 回调/`mutationFn`/`queryFn` **禁止**直接传静态方法引用（丢 `this`）；用箭头：`(p) => XxxService.create(p)`。

### 国际化 (i18n)

- 使用 `next-intl`（无 URL locale 前缀 / provider 模式），兼容 `NEXT_STANDALONE_EXPORT`。
- 语言：`zh-CN`、`en`；默认 `zh-CN`。优先级：cookie `NEXT_LOCALE` → 浏览器语言 → 默认。
- 文案放在 `frontend/messages/fragments`。参考已有代码，按模块拆文件夹，en.json 和 zh-CN.json 是 ci 生成的(node scripts/merge-i18n-fragments.mjs)，禁止手动修改。
- 禁止在页面/组件里直接写文案，文案必须支持 i18
