# AGENTS.md

本文件是 OpenFlare 的 AI 接手入口，不承载详细设计、规范和计划。接手项目时，请根据以下分层文档指引进行阅读与开发：

### 1. 开发指导规范 (AI & Developer Guidelines)

* **必须阅读**：
  * **[docs/plan/index.md](./docs/plan/index.md)**：查看正在进行的开发实现计划（Implementation Plan）与 AI 代理交接文档（Handover），接手项目时优先检查。

### 2. 系统设计与架构 (Design Docs)

* **[docs/design/index.md](./docs/design/index.md)**：理解产品范围、系统边界、核心对象及长期约束，以及[仓库结构](./docs/design/index.md#仓库结构)。
* **[docs/design/architecture.md](./docs/design/architecture.md)**：理解 Server、Agent、OpenResty 与前端的职责边界与网络拓扑。
* **[docs/design/agent-design.md](./docs/design/agent-design.md)**：理解 Agent 设计原则、与 Server 交互时序、OpenResty 管控与配置发布回滚模型。

---

## Git 提交规范指南

### 提交信息基本格式

每次提交更改时，应当使用以下提交格式:

```text
<type>(<scope>): <subject>

<body>
```

* **Type**: 提交类型（例如 `feat`, `fix`, `refactor`, `perf`, `docs`, `chore` 等）。
* **Scope** (可选): 影响的范围（例如 `api`, `frontend`, `auth`, `mcp` 等）。
* **Subject**: 简短的一句话描述变更。
* **Body** (可选): 详细的说明，多行叙述。

## 务必阅读匹配的 Skill

| Skill | 何时使用 |
| :--- | :--- |
| `new-api` | 添加或修改自定义业务 API、Handler、服务层逻辑、自定义路由注册 |
| `new-async-task` | 添加或修改 Asynq 任务、定时任务、TaskHandler、任务元数据 |
| `new-setting` | 添加或修改系统/业务/公开设置、`/admin/system` 参数或 `/admin/settings` 图形化设置 |
| `database-migration` | 数据库表结构变更、goose SQL 迁移（PG/SQLite/ClickHouse）、seed 数据 |
| `clickhouse-batchwriter` | ClickHouse 批量写入、`internal/infra/persistence/batchwriter` 接入、分析表异步 flush、背压与写入路径改造 |
| `file-upload` | 业务上传文件、Worker 程序化摄取、`upload.Ingest` 策略选型、文件访问与 `w_uploads` / 统计排查 |
| `cache-framework` | 新增或修改业务缓存（RAM/Redis/DB 三层读路径）、缓存失效、多节点 pub/sub 同步、评估高频读是否应接入缓存 |
| `push-notification` | 系统通知推送事件、统一触发器投递、带消息推送的业务功能 |
| `release-guide` | 根据自上一正式版本 Tag 以来的提交整理 Version Bump 提交信息以触发双语 Release |
| `shadcn` | 添加、修改或组合 shadcn/ui 组件 |


## 严格遵循事项 (Guardrails)

- 切勿删除 `frontend/node_modules`
- 保持 `pkg/util/` 绝对纯净且不引入任何框架。禁止从 `pkg/util/` 及其子包中导入 Gin、GORM、sessions 等 HTTP/Web/数据库相关框架包（例如，Web 会话选项已收敛至 `internal/apps/oauth/session.go`）。
- 编写测试用例时，禁止使用硬编码的相对路径（如 `"uploads/test_cache"`）在源码目录下创建临时测试目录，必须统一使用 Go 内置的 `t.TempDir()` 以避免污染源码目录。
- 所有 HTTP 路由仅在 `internal/router/router.go` 中注册。
- 当 API Handler 发生变化时，更新 Swagger 文档（运行 `make swagger`）。
- 在完成代码开发后必须运行 `make code-check`, 并修复报错。
- 在完成代码开发后或者 git 提交前必须运行 `make format` 格式化代码。
- 需要缓存或文件管理能力时，必须复用现有平台实现，禁止在业务包中自行创建缓存目录、直接管理缓存文件或重复封装存储后端。
- 文件摄取必须通过 `upload.Ingest`（`upload.PolicyCreate` / `PolicyDedupNewRecord` / `PolicyResolveExisting`）；删除必须通过 `upload.Remove` 或 `upload.RemoveOwned`。禁止业务模块直接调用 `repository.CreateUpload` / `repository.SoftDeleteUpload`，禁止 `db.Create(&model.Upload{})` 旁路写 `w_uploads`。
- **`internal/model` 与 `internal/repository` 分层（硬规则）**：
  - `internal/model/`：仅 GORM 实体、表名、配置 key、查询 DTO、无 IO 领域规则（如密码哈希校验、字段规范化）。**禁止**在 model 中调用 `db.DB` / Redis / ClickHouse，**禁止** `import internal/repository`。
  - 实体上允许仅 mutate 自身字段的 GORM hook（如 `AfterFind(*gorm.DB)`），**禁止**在 hook 内再发起 DB/缓存查询。
  - `internal/repository/`：唯一持久化入口（CRUD、事务、缓存、分析查询）。apps / logics / task 框架通过 repository 访问数据，**禁止**在 Handler 内直接写复杂 SQL。
  - apps 不得为业务 CRUD 直接调用 `db.DB`；必须走 repository（管理端 SQL 控制台、infra 内部实现等例外可保留）。
  - 依赖方向只能是 `apps → repository → model` 与 `repository → infra/persistence`；**禁止** `model → repository`。
  - 新增代码不得再增加 `model.Get/List/Create/Update/Delete*(ctx…)` 类数据访问 API；存量迁移按域收敛至 repository。
- 禁止在 `init()` 中注册跨模块集成（任务 Handler、推送内置事件、域事件监听器、任务完成钩子）。统一通过 `internal/platform/bootstrap` 在 `internal/cmd` 入口显式装配。
- `internal/router/router.go` 的 `Serve()` 仅负责 HTTP 路由与中间件，禁止在其中执行 `SyncEvents`、`InitLogWriter` 等进程级运行时初始化。
- 核心业务模块（如 `oauth`、`user`）禁止直接 `import` `internal/apps/admin/push` 或 `custom_events` 触发通知；应通过 `internal/listener` 发射域事件，由 push 模块在 bootstrap 阶段订阅。
- 编写依赖任务注册或推送事件同步的测试时，必须在测试 setup 中显式调用 `bootstrap.RegisterTasks()`、`bootstrap.RegisterPushDomainEvents()` 等，不得依赖 `init()` 副作用。
- API 错误响应必须通过 `response.Abort*` 中断请求，由 `ErrorHandlerMiddleware` 统一写出 JSON；禁止 `c.JSON(http.StatusOK, response.Err(...))` 及 Handler 直接 `c.JSON(status, response.Err(...))`。
1. **设计先行**：
    * 开发新功能或重要特性时，必须在 `docs/design/` 下创建/更新对应的设计文档，理清架构与核心决策。
    * 新增的设计文档应同步更新至 `docs/design/architecture.md` 及在 `docs/config.ts` 中注册侧边栏路由。
    * 若实现内容超出产品边界，必须先修改设计文档，再编码实现。
3. **开发计划与交接**：
    * 正在进行的开发计划或 AI 接手交接发生变化时，在 `docs/plan/` 下更新对应的开发计划或接手文档，并使用相应模板初始化。
4. **文档与变更日志**：
	* 当相关内容发生变化时，同步更新对应的**中文文档**（不要同步英文文档）。
	* 代码或配置变更完成后，必须在 [`docs/changelog/index.md`](./docs/changelog/index.md) 的 `[Unreleased]` 区块补充对应变更条目。
	* **纯文档变更（如 `docs/` 下的 Markdown 文档、README 等）不需要写入 changelog。**
	* 更新 changelog 时遵循以下书写规则：
	  1. 合并重复或相近的变更，不按提交逐条罗列。
	  2. 不记录格式化、临时调试、无关重构等对用户无意义的变更。
	  3. 使用用户可理解的表述，不描述内部实现细节。
	  4. 每条均使用完整中文句子，并尽量说明修复或优化的内容及其带来的效果。
	  5. 仅基于实际变更撰写，不编造提交或代码中不存在的信息。
	  6. 不记录 Token、密钥、私有地址等敏感信息；没有内容的分类可以省略。

## 项目介绍

### 技术栈

- 后端：Go 1.25+、Gin、GORM、PostgreSQL、可选 ClickHouse、Redis、Asynq、Cobra、Viper、Swaggo、OpenTelemetry、Zap、AWS SDK v2、Snowflake IDs。
- 前端：Next.js App Router、TypeScript、Tailwind CSS、pnpm、shadcn/ui。

### 目录结构与平台能力

顶层目录：

- `main.go`：程序入口，委派给 `internal/cmd`。
- `config.example.yaml`：已提交的配置模板。在添加配置字段时保持更新。
- `config.yaml`：本地运行时的配置文件。不要将其作为已提交的源码提交。
- `docker/`：集成的、仅前端的和仅后端的 Dockerfile。
- `docs/`：自动生成的 Swagger 文档。请勿手动编辑生成的文件。
- `frontend/`：Next.js 应用。
- `internal/`：私有 Go 后端代码。
- `pkg/`：公共 Go 库/工具包（留作扩展或存放不依赖特定业务的通用代码）。
- `scripts/`：本地和 CI 辅助脚本。
- `support-files/`：部署 and 数据库辅助文件。
- `bin/`：本地编译生成的二进制可执行文件。
- `data/`：本地运行时数据文件目录（如 PostgreSQL、Redis 数据等）。
- `uploads/`：本地文件上传存储目录。

后端目录：

- `internal/cmd/`：用于 API、worker、scheduler、root init 的 Cobra 命令。进程启动时在此调用 `bootstrap.Register*` 与 `bootstrap.Init`，再启动 router / worker / scheduler。
- `internal/platform/bootstrap/`：应用装配根（composition root）。集中注册任务 Handler、推送域事件订阅、任务完成监听器，并执行 `SyncEvents`、ClickHouse 访问日志写入等进程级初始化；所有注册函数使用 `sync.Once` 保证幂等。
- `internal/infra/config/`：Viper 加载和配置结构体。运行时代码应使用 `config.Config.<Section>.<Field>`。
- `internal/router/`：唯一的 HTTP 路由注册点。
- `internal/apps/`：按功能（Feature-based）组织的 HTTP Handler、中间件、内部服务与模块逻辑。移除全局 service 层，模块内部业务逻辑（如验证码业务逻辑管理器 `internal/apps/cap/manager.go`）均收敛于各自模块中；管理端模块位于 `internal/apps/admin/`。
- `internal/apps/upload/`：上传记录、文件访问控制、本地/S3 文件响应、下载及图片 WebP 压缩。业务应复用 `upload.Ingest` / `upload.Remove` 与 `GET /f/:id` 文件服务，不直接操作底层 storage 或旁路写 `w_uploads`。
- `internal/model/`：GORM 实体、表映射、配置 key、查询 DTO 与无 IO 领域规则；不含数据库访问。
- `internal/repository/`：数据访问层（平台与业务域 CRUD、缓存、ClickHouse 分析读写）；唯一持久化入口。
- `internal/infra/persistence/`：PostgreSQL、Redis、ClickHouse、GORM 日志、ID 生成和 goose SQL 迁移的布线。
- `internal/infra/diskcache/`：平台级磁盘字节缓存，通过 `diskcache.GetGlobalCache()` 提供 TTL、最大空间限制、LRU 淘汰、清空、状态统计和配置热更新。写入时使用 `DefaultExpiration`（全局默认 TTL）、正数 `time.Duration`（业务 TTL）或 `NoExpiration`（无 TTL，仍受空间限制和 LRU 淘汰）。
- `internal/infra/objectstore/`：S3 兼容对象存储适配，提供对象上传、读取、删除、CDN/代理读取及远端对象本地缓存。
- `internal/infra/task/`：Asynq 任务框架；参见 `new-async-task` 了解变更。
- `internal/shared/`：共享的通用模型及响应（如 `internal/shared/response`）、绑定（bind）、常量以及通用错误。
- `pkg/util/`：纯底层无副作用的系统工具（Crypto/Password/UUID、格式化、网络、版本比较等）。
- `internal/listener/`：域事件分发层。核心域（auth、user 等）在此定义并发射事件（如 `EmitAdminLoggedIn`）；运维模块（push、webhook 等）在 bootstrap 阶段订阅，实现跨模块解耦。
- `internal/otel_trace/`：链路追踪（tracing）助手。
- `internal/testhelper/`：后端测试共享辅助能力。
- `internal/buildinfo/`：暴露在发布/构建工作流中注入的元数据（如版本号、编译时间等）。

公共底层包 (`pkg/`)：
- `pkg/cache/disk/`：纯底层的通用本地磁盘缓存引擎。
- `pkg/cap/`：底层的通用验证码验证和生成库。
- `pkg/httppool/`：管理全局共享且经过优化的 HTTP 传输客户端及连接池，集成 OTel 链路追踪。
- `pkg/logger/`：Zap 和 OTel 日志助手。
- `pkg/push/`：推送渠道客户端集成（Lark/Telegram/Email）。
- `pkg/mail/`：邮件发送客户端。
- `pkg/trace/`：OpenTelemetry 链路追踪配置。

前端目录：

- `frontend/app/`：App Router 页面、路由组、根布局、全局配置。
- `frontend/components/ui/`：shadcn/ui 基础组件。
- `frontend/components/common/`：跨页面的业务组件。
- `frontend/components/layout/`：Header、Sidebar、Footer 等应用布局组件。
- `frontend/components/auth/`、`home/`、`animate-ui/`、`providers/`：特定作用域的 UI 组件。
- `frontend/lib/services/`：基于 `BaseService` 的类型化 API 服务，按业务域拆分并由 `services` 对象统一导出。
- `frontend/contexts/`、`hooks/`、`lib/`、`types/`、`public/`：共享状态、Hook、客户端与实用工具、TypeScript 类型、静态资产。
- `frontend/scripts/`：前端构建和维护脚本。
- `frontend/.next/`、`frontend/out/`、`frontend/node_modules/`：本地生成或安装的产物，不作为业务源码编辑。


## 开发要求

### 后端规则

命名规范：

- Go 包和文件使用小写蛇形命名（lowercase snake case）：如 `auth_source`、`postgres_logger.go`。
- 导出的 Go 标识符使用 PascalCase；未导出的标识符使用 camelCase。
- 请求/响应结构体使用 camelCase 并带有后缀，例如 `listUsersRequest` 和 `listUsersResponse`。
- 错误消息常量是 camelCase 字符串 `const`值，而不是包级别的 `error` 值。
- YAML 配置键使用小写蛇形命名（lowercase snake case）。

Handler 规范：

- Handler 命名为 动词 + 名词，例如 `ListUsers`。
- 使用 `ShouldBindQuery` 或 `ShouldBindJSON` 进行绑定。
- 每个 HTTP API 都需要有完整的 Swagger 注释；在 API 变更后运行 `make swagger`。

#### API 响应信封（统一格式）

所有 JSON API 响应的外层结构**必须**为：

```json
{ "error_msg": "", "data": ... }
```

- 成功时：`error_msg` 为空字符串，`data` 承载业务载荷。
- 失败时：`data` 为 `null`，`error_msg` 为用户可见的错误说明。
- 分页响应在 `data` 下使用 `{ "total": 0, "results": [] }`。

#### 成功响应（唯一写法）

成功时**始终**使用 HTTP `200`，由 Handler 直接写出 JSON：

```go
import (
    "net/http"
    "github.com/Rain-kl/Wavelet/internal/shared/response"
    "github.com/gin-gonic/gin"
)

// 有数据
c.JSON(http.StatusOK, response.OK(data))

// 无数据（data 为 null）
c.JSON(http.StatusOK, response.OKNil())
```

#### 失败响应（中断请求，禁止直接写错误 JSON）

失败时**禁止**在 Handler / 中间件中直接调用 `c.JSON(..., response.Err(msg))`，也**禁止**用 HTTP `200` 携带非空 `error_msg` 表示失败。

统一通过 `internal/shared/response` 的 **Abort 系列函数**中断请求。这些函数会将 `*response.APIError` 挂载到 Gin 的 `c.Errors` 链并 `c.Abort()`；请求结束后由全局 `response.ErrorHandlerMiddleware()`（在 `internal/router/middlewares.go` 中注册）统一写出 JSON，并记录到 OpenTelemetry Trace/Jaeger。

**推荐使用的便捷函数（优先于手写状态码）：**

| 函数 | HTTP 状态码 | 典型场景 |
|------|-------------|----------|
| `response.AbortBadRequest(c, msg)` | 400 | 参数绑定失败、字段校验、业务规则拒绝（如密码错误、重复注册） |
| `response.AbortUnauthorized(c, msg)` | 401 | 未登录、Session/Token 失效（`oauth.LoginRequired()`） |
| `response.AbortForbidden(c, msg)` | 403 | 已登录但无权访问（如 Token 不允许访问的端点） |
| `response.AbortNotFound(c, msg)` | 404 | 资源不存在；管理员中间件对非管理员隐藏端点时也使用此码 |
| `response.AbortConflict(c, msg)` | 409 | 资源冲突（如唯一键重复） |
| `response.AbortTooManyRequests(c, msg)` | 429 | 限流、频率限制 |
| `response.AbortInternal(c, msg)` | 500 | 对用户返回通用提示；底层错误须先记录日志 |
| `response.AbortWithError(c, code, msg)` | 自定义 | 上表未覆盖的状态码时使用 |

**标准 Handler 模板：**

```go
func CreateWidget(c *gin.Context) {
    var req createWidgetRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        response.AbortBadRequest(c, errBindParamsFailed)
        return
    }

    widget, err := createWidgetLogic(c.Request.Context(), req)
    if err != nil {
        // 底层错误已记录日志时，向用户返回安全文案
        response.AbortBadRequest(c, err.Error()) // 或按语义选用 AbortConflict / AbortInternal 等
        return
    }

    c.JSON(http.StatusOK, response.OK(widget))
}
```

**中间件**与 Handler 遵循同一规则。参考 `oauth.LoginRequired()` → `AbortUnauthorized`，`admin.LoginAdminRequired()` → `AbortNotFound`，`cap.VerifyMiddleware` → `AbortUnauthorized`。

#### 错误消息定义

- 面向用户的错误文案定义为模块内 **camelCase 字符串常量**（放在 `errs.go`），例如 `errBindParamsFailed = "参数绑定失败"`。
- Handler / 中间件向 Abort 函数传入这些常量或经校验的安全字符串；**禁止**将数据库驱动错误、堆栈信息等内部细节直接暴露给客户端。
- `response.Err(msg)` 仅供 `ErrorHandlerMiddleware` 内部构造 JSON，**业务代码不得直接用于 `c.JSON`**。

#### `logics.go` 与 Handler 的分工

- `logics.go` 接受 `context.Context`，返回 `(result, error)` 或带状态的业务结果结构体（参考 `internal/apps/user/logics.go` 的 `LoginEmailVerificationResult`）。
- `logics.go` **不得**依赖 `*gin.Context`，**不得**调用 `response.Abort*` 或 `c.JSON`。
- Handler 负责：绑定参数 → 调用 logic → 将 logic 错误/状态映射为对应的 `Abort*` 或 `response.OK`。

#### 日志与内部错误

- 数据库、Redis、第三方 API、文件 I/O 等**运行时错误**：在 Handler 或 logic 边界用 `pkg/logger` 记录（带 `ctx`），再向用户返回安全的 `AbortInternal` 或语义匹配的业务错误常量。
- 任何关键错误在被吞掉、转换为通用响应，或由后台 worker 忽略之前，都必须通过 `pkg/logger` 打印日志。
- 禁止用 `_ = ...` 静默丢弃重要错误。如果某个错误因为 best-effort 操作或确认无害而需要忽略，必须添加简短注释说明原因。
- 避免重复刷日志：在真正处理或抑制错误的边界记录一次，然后 `Abort*` 或成功返回。

#### 禁止写法（反模式）

```go
// ❌ 禁止：HTTP 200 表示失败
c.JSON(http.StatusOK, response.Err("密码错误"))

// ❌ 禁止：Handler 直接写错误 JSON，绕过 ErrorHandlerMiddleware 与 OTel 记录
c.JSON(http.StatusBadRequest, response.Err("参数错误"))

// ❌ 禁止：gin.H 手写错误体
c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error_msg": "...", "data": nil})

// ❌ 禁止：logics.go 中中断 HTTP 请求
func doSomething(c *gin.Context) { response.AbortBadRequest(c, "...") }
```

#### Swagger 注释约定

- `@Success 200` 的 `data` 使用具体类型或 `response.Any`。
- 对每个可能返回的 Abort 状态码声明 `@Failure`，例如 `@Failure 400 {object} response.Any "参数错误"`、`@Failure 401 {object} response.Any "未登录"`。

路由与模块：

- 仅在 `internal/router/router.go` 中作为统一高层入口进行路由分发委派，不允许在 `router.go` 中直接挂载业务 Handler。
- 关于所有的路由归属划分、接口开发隔离防线以及详细的注册和开发步骤，请直接阅读并严格遵循 [new-api](.agent/skills/new-api/SKILL.md) 技能。

应用装配与跨模块集成：

- 新增跨模块副作用（任务注册、推送订阅、后台监听器）时，在 `internal/platform/bootstrap/bootstrap.go` 增加 `Register*` 函数，并在对应 `internal/cmd/*.go` 入口调用；参考现有 `RegisterAPI` / `RegisterWorker` / `RegisterAll` 分工。
- `bootstrap.Init` 必须在 `RegisterPushDomainEvents()` 之后调用（API/`all` 模式），以确保 `SyncEvents` 能同步内置推送事件元数据。
- Handler 与业务逻辑分离：HTTP Handler 负责绑定与响应；可复用逻辑放入 `logics.go`（接受 `context.Context`，不依赖 `*gin.Context`），便于 Worker 与单元测试复用。参考 `internal/apps/user/logics.go`。

中间件：

- 全局中间件属于路由设置：`gin.Recovery()`、`otelgin.Middleware()`、日志中间件 and session 中间件。
- 对于登录路由组，使用 `oauth.LoginRequired()`。
- 对于管理路由组，使用 `admin.LoginAdminRequired()`。

配置管理：

- 运行时代码从 `config.Config` 中读取配置，绝对不要直接从 `os.Getenv()` 中读取。
- 当添加配置时，同时更新 `config.example.yaml` and `internal/infra/config/model.go`。

数据库操作：

- **持久化只通过 `internal/repository`**（或 analytics 子包）。apps / logics 不要直接 `db.DB(ctx).Where...` 拼复杂查询；简单事务编排可在 logics 中调用多个 repository 方法。
- repository 内管理员/业务查询应使用 `db.DB(ctx)` 以获得链路追踪感知的 DB 访问。
- 不要在 Handler 中放置 SQL；复杂查询放 `internal/repository/`，业务编排放 `internal/apps/<module>/logics.go`（或 `service.go`）。
- `internal/model` 只定义实体与无 IO 规则，不访问数据库。
- 在 `internal/infra/persistence/migrator/goose/` 下使用 goose SQL 迁移；不要添加基于 GORM AutoMigrate 的 Schema 升级。
- 不要创建物理数据库外键。改为关系字段添加显式索引。
- 数据库默认值必须与 Go 模型零值（`nil`、`0`、`false`、`""`）匹配，以避免意外的插入。

### 前端规则

在进行任何 Next.js 工作之前，请在 `node_modules/next/dist/docs/` 中找到并阅读相关文档。您的训练数据已过时 —— 这些文档是唯一的真理来源。

请直接查看并参考项目提供的示例和 Demo 代码：[frontend/app/(main)/admin/demo](frontend/app/(main)/admin/demo)。

样式规范：

- shadcn/ui 基础组件应该使用它们的 `variant` 系统和全局 CSS 变量。当组件的变体（variant）应该拥有某种外观时，不要在业务 `className` 中硬编码颜色、背景或阴影。
- 如果现有的变体不足以满足需求，请扩展 shadcn/ui 组件的变体，而不是硬编码一次性的颜色。

页面标题栏规范 (新人开发与重构必读)：

- **容器与对齐机制**：
    - 标题容器统一使用 `flex items-center gap-2`。如果右侧有操作按钮（如“新增”、“刷新”），请使用 `justify-between` 布局让操作区与标题双向分布。
    - 为了确保所有页面在进入/切换时，顶部的呼吸感和视觉高度完全一致，页面最外层容器**必须**统一使用 `py-6 px-1` 或 `py-6` 进行上边距对齐。
- **图标标准**：图标作为视觉辅助点缀，**必须**直接嵌套在标题容器中，直接使用 Lucide 图标组件，样式大小限制为 `size-5 text-primary`。**严禁**为图标包裹任何背景小卡片、圆角边框或额外的修饰容器。
- **标题文字标准**：标题文字使用且仅使用 `h1 className="text-2xl font-semibold tracking-tight"`。不要自行定义字号、字量（如使用 `font-bold`）或添加任何渐变色，保持整个系统的字形规范化。
- **Tabs 模块化与文件拆分规范**：凡是带有多个 Tab 页切换的复杂页面，**禁止**将所有 Tab 的渲染逻辑堆积在同一个主文件内。每个 Tab 的具体渲染内容必须单独拆分为独立的 React 组件文件（如 `tabs/events-tab.tsx`）。主页面文件应该仅用于导入子组件、注册 Tabs 触发器以及管理 Tab 的切换激活状态。这有利于防止单文件过大（避免单文件行数超过 600 行限制），并大幅度提高代码的可读性与编译维护效率。
- **扁平化结构与避免冗余中间件**：为了消除无意义的“中间代理文件”，所有作为路由物理入口的 Tabs 状态维护、骨架及外层布局代码，**必须**直接定义在 Next.js 的 `app/` 页面文件（即 `page.tsx`）中。禁止在 `page.tsx` 中仅写一个单纯的 `<AnotherComponent />` 转发，而在外部新建一个同名中转容器。
- **复杂度驱动的组件拆分规范**：组件的拆分不应局限于“跨页面复用”。当一个路由页面的复杂度变高时（如渲染逻辑膨胀、存在大型嵌套弹窗或多层状态管理，如单文件代码行数超过 600 行），必须主动将其拆分为子组件以维持单文件的高可读性与低耦合度。拆分时遵循就近原则：特定于该路由且不复用的子组件应放置在最邻近该路由的特征目录（Feature Folder，如 `components/` 局部文件夹）中；只有真正具备跨页面复用价值的通用业务/基础 UI 组件才应存放在全局 `components/` 共享目录下。
    - **最佳实践标杆案例（数据管理 `/admin/database`）**：
      该页面由于整合了“运行状态概览”、“物理表网格浏览器”、“磁盘缓存管理”和“SQL 交互控台”多个复杂大区块，重构前单文件接近 1000 行。
      重构后，主页面 `page.tsx` 仅做高级页面骨架与排版排布，维护全局刷新机制与终端视图切换；而“数据表浏览器 (`table-browser.tsx`)”、“缓存管理 (`cache-manager.tsx`)”与“SQL 终端 (`sql-console.tsx`)”等独立高状态密度区块均被抽离为局部子组件，存放在 `frontend/app/(main)/admin/database/components/`。这保证了代码结构层次清晰、单文件小巧好维护。所有复杂页面的新开发和重构必须遵循此模式。

页面宽度：

- 页面根容器必须支持全宽。使用 `w-full`。
- 不要硬编码页面级的最大宽度，如 `max-w-6xl` 或 `max-w-4xl`；主布局（main layout）拥有正常/全宽的限制。

组件放置：

- 跨页面的业务组件属于 `frontend/components/common/`。
- shadcn/ui 原生组件（primitives）属于 `frontend/components/ui/`。
- 特定于路由/页面的组件放在最邻近的特征（feature）目录中。

服务类（Services）：

- 前端 API 访问通过服务类和导出的 `services` 对象进行。
- 新增服务结构如下：

```text
frontend/lib/services/<service-name>/
  types.ts
  <service-name>.service.ts
  index.ts
```

- 服务类继承 `BaseService`，定义 `basePath`，并暴露有类型的静态方法。
- **防止回调 `this` 上下文丢失（核心规范）**：在传递服务类的静态方法作为组件事件回调（如 `onClick`）或 React Query 的 `mutationFn`/`queryFn` 时，**禁止直接传递静态方法引用**（如 `mutationFn: DnsAccountService.create`），必须使用箭头函数包裹以防止 `this` 上下文丢失导致运行时崩溃（如 `mutationFn: (payload) => DnsAccountService.create(payload)`）。
- 在 `frontend/lib/services/index.ts` 中注册新服务。
