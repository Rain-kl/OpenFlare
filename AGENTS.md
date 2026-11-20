# AGENTS.md

## Skills（匹配任务时必读）

| Skill | 何时使用 |
| :--- | :--- |
| `new-api` | 业务 API、Handler、服务层、路由注册 |
| `new-async-task` | Asynq 任务、定时任务、TaskHandler、任务元数据 |
| `new-setting` | 系统/业务/公开设置、`/admin/system`、`/admin/settings` |
| `database-migration` | 表结构、goose 迁移（PG/SQLite/ClickHouse）、seed |
| `logstore` | 日志/分析用途表、`internal/repository/logstore`、切换日志主库、PG/SQLite 回落 |
| `clickhouse-batchwriter` | CH 批量写入、batchwriter、分析表 flush/背压 |
| `file-upload` | 上传/摄取、`upload.Ingest`、文件访问、`w_uploads` |
| `cache-framework` | 业务缓存（RAM/Redis/DB）、失效、多节点同步 |
| `push-notification` | 通知推送事件、统一触发器、带推送的业务 |
| `release-guide` | Version Bump 双语提交信息（中英文以 `---` 分隔） |
| `shadcn` | 添加/修改/组合 shadcn/ui 组件 |

## 硬性约束

- `pkg/util/` 保持纯净：禁止导入 Gin、GORM、sessions 等 HTTP/Web/DB 框架（会话选项在 `internal/apps/oauth/session.go`）。
- 测试临时目录只用 `t.TempDir()`，禁止硬编码相对路径写源码树。
- API 变更后：`make swagger`；开发完成：`make code-check`；提交前：`make format`。
- 缓存/文件管理复用平台实现，业务包禁止自建缓存目录或旁路存储后端。
- 文件摄取走 `upload.Ingest`（`PolicyCreate` / `PolicyDedupNewRecord` / `PolicyResolveExisting`）；删除走 `upload.Remove` / `upload.RemoveOwned`。禁止业务直接 `repository.CreateUpload` / `SoftDeleteUpload` 或 `db.Create(&model.Upload{})`。
- **分层**：`apps → repository → model`，`repository → infra/persistence`；禁止 `model → repository`。
  - `model`：实体、表名、配置 key、查询 DTO、无 IO 规则。禁止 `db.DB` / Redis / CH；禁止 `import repository`。GORM hook 仅可 mutate 自身字段，禁止在 hook 内再查 DB/缓存。
  - `repository`：唯一持久化入口。apps/logics 禁止为业务 CRUD 直调 `db.DB`（管理端 SQL 控制台、infra 内部等例外保留）。禁止新增 `model.Get/List/Create/...` 类数据访问 API。
- 日志/分析表（节点访问日志、用户访问日志、可观测时序）走 `internal/repository/logstore`，禁止 apps 直连 ClickHouse 驱动或 `db.ChConn`/`db.ChDB`（ClickHouse 日志与分析访问已并入 `logstore`）。判定与接入步骤见 `logstore` skill。
- 跨模块集成（任务 Handler、推送事件、域监听、完成钩子）禁止 `init()` 注册；经 `internal/platform/bootstrap` 在 `cmd/server/cli` 入口显式装配。
- 核心业务（如 `oauth`、`user`）禁止直接 import push/custom_events；经 `internal/listener` 发域事件，push 在 bootstrap 订阅。
- 依赖任务/推送注册的测试须显式 `bootstrap.RegisterTasks()` / `RegisterPushDomainEvents()` 等，不依赖 `init()`。
- API 错误必须 `response.Abort*` + `ErrorHandlerMiddleware`；禁止 Handler 直接 `c.JSON(..., response.Err(...))` 或用 HTTP 200 表示失败。
- 根据单次任务工作量动态调整测试力度,不要改个文案就去跑全量测试。

### 文档与 Changelog

- 内容变更同步**中文文档**。
- 代码/配置变更写入 [`docs/changelog/index.md`](./docs/changelog/index.md) 的 `[Unreleased]`；纯文档变更不写 changelog。
- Changelog：合并相近项；不记格式化/调试/无关重构；用户可读完整中文句；说明效果；不编造；不写密钥等敏感信息；空分类可省略。

## 技术栈

- **后端**：Go 1.25+、Gin、GORM、PostgreSQL、可选 ClickHouse、Redis、Asynq、Cobra、Viper、Swaggo、OTel、Zap、AWS SDK v2、Snowflake IDs
- **前端**：Next.js App Router、TypeScript、Tailwind、pnpm、shadcn/ui


## Git 提交规范

每次完成一个功能点开发或修复一个问题后，务必提交 Git commit , 禁止推送远程仓库。
遵循 Conventional Commits：`<type>(<scope>): <subject>`（例：`feat(auth): support email login`）。
若涉及非琐碎改动（技术选型/架构重构/核心改动/踩坑复盘），必须将对应的 Agent Note（`.agents/notes/...`）与代码同批原子提交。
- **改动范围约束**：仅允许提交自己明确修改的文件，严禁提交非本次任务修改的文件（杜绝盲目 `git add .` 或 `git commit -a` 卷入无关变动）。
- **工作区保护**：严禁随意还原、丢弃或覆盖他人或当前工作区正在施工的文件。
- **测试容错与告知**：若工作区正在施工的文件导致全局测试失败，仅验证本次修改相关的测试用例，跳过受无关施工影响的测试并明确告知跳过原因。

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
- 跨模块副作用：在 `bootstrap` 增 `Register*`，于对应 `cmd/server/cli/*.go` 调用（`RegisterAPI` / `RegisterWorker` / `RegisterAll`）。
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
- 卡片与内容分区统一采用**虚线外框、无阴影扁平设计**（`Pages` 与 `/admin/demo` 风格）：`frontend/components/ui/card.tsx` 已默认 `border-dashed shadow-none`，业务直接使用 `<Card>` 即可；**禁止**在卡片或列表项容器上添加 `shadow-sm`/`shadow-md` 等投影、`backdrop-blur-*` 毛玻璃或实线立体边框（`Dialog`、`Sheet`、`Popover` 等浮层除外）。自定义列表项/空状态/分块容器同样使用 `border border-dashed` 且不加阴影。

### 页面结构

- 根容器全宽 `w-full`；禁止页面级 `max-w-*`（主布局负责宽度）。
- 外层间距：`py-6` 或 `py-6 px-1`。
- 标题行：`flex items-center gap-2`（有右侧操作则加 `justify-between`）。
- 图标：Lucide 直接放标题容器，`size-5 text-primary`；禁止背景卡片/边框包裹。
- 标题：仅 `h1 className="text-2xl font-semibold tracking-tight"`。
- 多 Tab：各 Tab 独立文件；`page.tsx` 只管 Tabs 状态与触发器；禁止 `page.tsx` 仅转发同名空壳。
- 单文件 > ~600 行或状态过重时拆局部 `components/`；跨页复用放 `frontend/components/common/`。目录拆分参考：`/admin/database`；视觉与卡片风格以 `Pages` 与 `/admin/demo` 为准。

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
- 语言：`zh-CN`、`en`、`ru`；默认 `en`。优先级：cookie `NEXT_LOCALE` → 浏览器语言 → 默认。
- 文案放在 `frontend/messages/fragments`。参考已有代码，按模块拆文件夹，`en.json`、`zh-CN.json` 和 `ru.json` 是通过 `node scripts/merge-i18n-fragments.mjs` 生成的，禁止手动修改。
- 禁止在页面/组件里直接写文案，文案必须支持 i18n。

### 文案规范

- 展示于网站界面、文档页及内置 HTML 模板的所有前端文案须采用**正式、客观、克制的专业书面语**，禁止口语化表达。
- **禁用写法**：
  - 口语词、语气助词、网络梗与随意调侃（如「试试」「吧」「呢」「呀」「哦」「这个人很懒」「迷失在数字星云中」「你知道吗」「五秒盾」「过盾」「站长」「兜底」）。
  - 随意缩写、非正式简称与开发俚语（如「反代」→「反向代理」、「控台」→「控制台」、「退登」→「退出登录」、「追平」→「同步」、「报错」→「错误」、「串给」→「分发给」、「灰云/橙云」→「仅 DNS / Cloudflare 代理」）。
  - 情绪化感叹号（`！`/`!` 仅用于高危警告）与对话腔/随意第二人称（如「换个词试试？」「介绍一下自己吧」「发给你的」「这里可以」「请看」；改用客观陈述或「在此」「当前页面」「请查看」）。
  - 残缺省略提示与错别字/异体字（如「空关闭」「0 关闭」→「留空表示关闭」「0 表示关闭」；统一使用「其他」而非「其它」、「模板」而非「模版」、「登录」而非「登陆」、「连接」而非「链接」）。
- **多语言一致性**：修改文案语义时须同步校对 `zh-CN`、`en`、`ru` 三个 fragment 文件；英文与俄文同样须采用正式产品用语（避免 `you've`、`hasn't` 等非正式缩写及 `Did you know:`、`Try a different term?` 等对话腔），并保持占位符与富文本标签一致。
