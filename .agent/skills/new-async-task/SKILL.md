---
name: "new-async-task"
description: "Wavelet 项目专用：新增或修改 Asynq 异步任务、后台任务、定时任务、任务元数据、TaskHandler、TaskParam、PayloadValidator、AppendLog、任务重试、任务执行记录或 Admin 任务 API 时必须使用。"
---

# 异步任务开发

开始前阅读根目录 `AGENTS.md`。只修改任务相关链路，遵守项目路由、日志、数据库迁移和质量门禁要求。

## 开始前

按任务范围检查当前实现：

- `internal/infra/task/handler.go`：`TaskHandler`、`TaskResult`、`PayloadValidator`
- `internal/infra/task/meta.go`：`TaskMeta`、`TaskParam`
- `internal/infra/task/executor.go`：下发、执行、日志、重试、`OnTaskCompleted` 订阅
- `internal/infra/task/handlers/register.go`：Handler 和元数据注册（由 bootstrap 调用）
- `internal/platform/bootstrap/bootstrap.go`：任务注册与进程级装配入口
- `internal/infra/task/worker/worker.go`：Worker 路由和队列
- `internal/infra/task/scheduler/scheduler.go`：定时调度
- `internal/apps/admin/task/routers.go`：Admin 任务 API
- `internal/model/task_execution.go`：执行记录实体与 DTO
- `internal/repository/task_execution.go`：执行记录和日志持久化

需要模板时阅读 [references/CODE-EXAMPLES.md](references/CODE-EXAMPLES.md)。

## 实现要求

### 任务定义

- 在 `internal/apps/<module>/tasks.go` 定义任务类型、Admin 任务类型和 `TaskMeta`。
- Asynq 任务类型使用 `<module>:<action>` 格式。
- 完整设置 `Type`、`AsynqTask`、`Name`、`Description`、`MaxRetry`、`Queue`、`Retryable`。
- 有参数任务必须定义 payload struct。
- `TaskParam.Name` 必须与 payload JSON tag 一致。
- `TaskParam` 只描述前端表单，不代替服务端校验。

### Handler

- Handler 必须实现 `task.TaskHandler`。
- 有参数任务必须实现 `task.PayloadValidator`，负责校验和标准化 Admin 下发参数。
- `Execute` 必须再次解析 payload；不要假设入口一定经过 Admin 校验。
- 成功返回 `&task.TaskResult{Message: ..., Detail: ...}`。
- 失败返回 error，由任务框架处理状态和重试。
- 不要吞掉关键错误。
- 持久化只通过 `internal/repository/`（唯一入口）；业务编排放模块内 `logics.go` / `service.go`。`internal/model` 仅实体/DTO，禁止 CRUD 与 DB 访问。

### 注册

- 在 `internal/infra/task/handlers/register.go` 同时注册 Handler 和 `TaskMeta`。
- 不要在其他位置单独注册任务。
- **禁止**在业务包 `routers.go` 或 `init()` 中调用 `task.RegisterHandler`；统一由 `bootstrap.RegisterTasks()` → `taskhandlers.Register()` 在进程启动时装配。
- 任务完成钩子（如 push 通知）通过 `task.OnTaskCompleted` 注册，在 `bootstrap.RegisterTaskListeners()` 中装配（Worker/`all` 进程）。

### 进程装配分工

| 进程 | 注册入口 |
| :--- | :--- |
| `api` | `cmd/api.go` → `bootstrap.RegisterAPI()`（含 `RegisterTasks`） |
| `worker` | `worker.StartWorker()` → `bootstrap.RegisterWorker()`（含 `RegisterTasks` + `RegisterTaskListeners`） |
| `scheduler` | `scheduler.StartScheduler()` → `bootstrap.RegisterScheduler()` |
| `all` | `cmd/all.go` → `bootstrap.RegisterAll()` |

所有 `Register*` 使用 `sync.Once`，重复调用安全。

### 测试

- 依赖已注册任务类型或 Handler 的测试（如 `internal/apps/admin/task/routers_test.go`），必须在 setup 中显式调用 `bootstrap.RegisterTasks()`。
- 不得依赖 `init()` 副作用或 import 链触发注册。

## 日志要求

- 在 `TaskHandler.Execute` 中使用 `task.AppendLog(ctx, format, args...)`。
- 记录任务开始、参数摘要、批次进度、关键状态、可继续错误和完成摘要。
- 批量处理按批次记录；禁止为大循环中的每条数据写日志。
- 不要直接修改任务日志的 Redis key 或 `w_task_executions.log`。

日志框架约束：

- 执行状态实时写入数据库：`pending`、`running`、`succeeded`、`failed`。
- 实时日志写入 Redis，每个任务最多保留最近 1000 行。
- Redis 日志 TTL 为 24 小时，每次追加时刷新。
- 查询时优先返回 Redis 日志，Redis 不存在时读取数据库。
- 任务成功或自动重试耗尽后，将日志写入数据库并删除 Redis 缓冲。
- 自动重试期间保留同一 taskID 的 Redis 日志。

## 重试要求

- Handler 返回 error 以触发 Asynq 自动重试。
- 不要在 Handler 内自行实现重复重试循环。
- Admin 手动重试只允许：
  - 原任务状态为 `failed`
  - `Retryable=true`
  - `RetryCount < MaxRetry`
- 修改重试行为时同时检查：
  - `internal/infra/task/executor.go`
  - `internal/model/task_execution.go`
  - `internal/apps/admin/task/routers.go`
  - 前端任务执行列表

## 定时任务

- 默认定时任务必须通过 Goose SQL 迁移写入 `schedules`。
- PostgreSQL 和 SQLite 迁移必须同时提供。
- 初始化 SQL 必须幂等。
- 涉及迁移时使用 `database-migration` skill。

## Admin API

- Handler 放在现有 Admin task 模块或 `internal/apps/admin/<module>/`。
- 路由只在 `internal/router/router.go` 注册。
- 响应保持 `{ "error_msg": "", "data": ... }`。
- 分页数据保持 `{ "total": 0, "results": [] }`。
- Swagger 注释必须完整；API 变化后运行 `make swagger`。

## 前端

- 仅任务元数据变化时，优先复用现有动态任务表单，不新增页面。
- API 调用必须通过 `frontend/lib/services/`。
- 修改 shadcn/ui 时使用 `shadcn` skill。
- 不使用 `any`。
- 页面根容器使用 `w-full`，不添加页面级 `max-w-*`。
