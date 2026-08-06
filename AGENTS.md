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
| `clickhouse-batchwriter` | CH 批量写入、batchwriter、分析表 flush/背压 |
| `file-upload` | 上传/摄取、`upload.Ingest`、文件访问、`w_uploads` |
| `cache-framework` | 业务缓存（RAM/Redis/DB）、失效、多节点同步 |
| `push-notification` | 通知推送事件、统一触发器、带推送的业务 |
| `release-guide` | Version Bump 提交信息（触发双语 Release） |
| `shadcn` | 添加/修改/组合 shadcn/ui 组件 |

## 硬性约束

- 禁止删除 `frontend/node_modules`。
- `pkg/util/` 保持纯净：禁止导入 Gin、GORM、sessions 等 HTTP/Web/DB 框架（会话选项在 `internal/apps/oauth/session.go`）。
- 测试临时目录只用 `t.TempDir()`，禁止硬编码相对路径写源码树。
- HTTP 路由仅在 `internal/router/router.go` 注册；`Serve()` 只挂路由与中间件，禁止进程级初始化（如 `SyncEvents`、`InitLogWriter`）。
- API 变更后：`make swagger`；开发完成：`make code-check`；提交前：`make format`。
- 缓存/文件管理复用平台实现，业务包禁止自建缓存目录或旁路存储后端。
- 文件摄取走 `upload.Ingest`（`PolicyCreate` / `PolicyDedupNewRecord` / `PolicyResolveExisting`）；删除走 `upload.Remove` / `upload.RemoveOwned`。禁止业务直接 `repository.CreateUpload` / `SoftDeleteUpload` 或 `db.Create(&model.Upload{})`。
- **分层**：`apps → repository → model`，`repository → infra/persistence`；禁止 `model → repository`。
  - `model`：实体、表名、配置 key、查询 DTO、无 IO 规则。禁止 `db.DB` / Redis / CH；禁止 `import repository`。GORM hook 仅可 mutate 自身字段，禁止在 hook 内再查 DB/缓存。
  - `repository`：唯一持久化入口。apps/logics 禁止为业务 CRUD 直调 `db.DB`（管理端 SQL 控制台、infra 内部等例外保留）。禁止新增 `model.Get/List/Create/...` 类数据访问 API。
- 跨模块集成（任务 Handler、推送事件、域监听、完成钩子）禁止 `init()` 注册；经 `internal/platform/bootstrap` 在 `internal/cmd` 入口显式装配。
- 核心业务（如 `oauth`、`user`）禁止直接 import push/custom_events；经 `internal/listener` 发域事件，push 在 bootstrap 订阅。
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
- `logics.go`：接受 `context.Context`，返回结果/error；**禁止**依赖 `*gin.Context`、调用 `Abort*` / `c.JSON`。参考 `internal/apps/user/logics.go`。

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

- 运行时错误（DB/Redis/第三方/IO）在 Handler 或 logic 边界用 `pkg/logger`（带 `ctx`）记录，再返回安全 Abort/业务错误。
- 吞错、转通用响应、worker 忽略前必须先记日志。
- 禁止 `_ = err` 静默丢弃重要错误；best-effort 可忽略时加简短注释。
- 只在处理/抑制边界记一次，避免重复刷日志。

### 路由与装配

- `router.go` 只做高层分发，禁止直接挂业务 Handler。归属与开发步骤见 `new-api` skill。
- 跨模块副作用：在 `bootstrap` 增 `Register*`，于对应 `internal/cmd/*.go` 调用（`RegisterAPI` / `RegisterWorker` / `RegisterAll`）。
- API/`all` 模式：`bootstrap.Init` 须在 `RegisterPushDomainEvents()` **之后**调用，保证 `SyncEvents` 同步内置推送元数据。

### 中间件

- 全局：`gin.Recovery()`、`otelgin`、日志、session。
- 登录组：`oauth.LoginRequired()`；管理组：`admin.LoginAdminRequired()`。

### 配置

- 运行时只读 `config.Config`，禁止 `os.Getenv()`。
- 新增配置同步 `config.example.yaml` 与 `internal/infra/config/model.go`。

### 数据库

- 持久化只经 `repository`（或 analytics）；复杂查询不进 Handler；编排在 logics。
- repository 内用 `db.DB(ctx)`（链路追踪）。
- 迁移：`internal/infra/persistence/migrator/goose/` SQL；禁止 GORM AutoMigrate。
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
