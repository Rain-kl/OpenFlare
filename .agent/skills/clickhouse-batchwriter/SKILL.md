---
name: "clickhouse-batchwriter"
description: "Wavelet 项目专用：当新增或修改 ClickHouse 批量写入、接入 internal/infra/persistence/batchwriter、将业务域异步 flush 到分析表、迁移 risk_control/节点访问日志/可观测时序写入、或评估 async_insert 与背压策略时必须使用。本技能指导分层职责、各域独立 Writer 实例、repository 批量 API 与禁止写法。"
---

# ClickHouse 批量写入开发

开始前阅读根目录 `AGENTS.md`。ClickHouse 是辅助 OLAP 存储，**厌恶高频单条写入**（过多小 part）；写入路径必须优先批量或异步聚合。

DDL 与表结构变更见 `database-migration` 技能；本技能只覆盖**运行时写入架构**。

## 分层职责

| 层级 | 路径 | 职责 |
| :--- | :--- | :--- |
| 连接 | `internal/infra/persistence/clickhouse.go` | `ChConn`（原生批量写）、`ChDB`（GORM 查询）；禁止在业务包直接 `clickhouse.Open` |
| 批量框架 | `internal/infra/persistence/batchwriter/` | 泛型队列 + 按条数/时间 flush + 非阻塞入队 + 优雅停机；**各业务域独立实例** |
| Model | `internal/model/analytics/` | 列定义、`TableName()`、`BatchInsertSQL()`（及可选 `InsertColumns()`） |
| Repository | `internal/repository/analytics/` | `BatchInsert*` / `BatchInsertNodeAccessLogs` 等；`PrepareBatch` + 多行 `Append` + 一次 `Send` |
| Apps | `internal/apps/<domain>/` | 采集、入队、背压；`FlushFunc` 只调 repository，不写 SQL、不 `PrepareBatch` |
| 装配 | `internal/platform/bootstrap/bootstrap.go` | 进程启动时调用 `Writer.Start`；初始化时需调用 `lifecycle.OnShutdown` 挂载停机钩子 |
| 生命周期 | `internal/platform/lifecycle/lifecycle.go` | 统一协调全局并发优雅停机，业务包无需在 `bootstrap.go` 中硬编码 `Stop` 逻辑 |

**禁止**在 Handler / middleware 内直接 `db.ChConn.PrepareBatch`；**禁止**在 repository 内启动 goroutine 或维护全局 channel（队列生命周期由 apps + bootstrap 或专用 writer 包负责）。

## batchwriter 框架契约

```go
writer, err := batchwriter.New[YourType](cfg, flushFunc, opts...)
writer.Start(ctx)
writer.TryEnqueue(item)   // 非阻塞；满则 false
writer.IsFull()           // 背压探测
writer.Stop(stopCtx)      // close 队列 + drain + 最终 flush
```

### Config 默认值（`batchwriter.DefaultConfig()`）

- `QueueSize`: 10_000
- `MaxBatchSize`: 1_000
- `FlushInterval`: 1s

各域可独立覆盖；可观测低频指标可用更小 `MaxBatchSize`（如 100）与更长 `FlushInterval`（如 2–5s），但**不要**退化为逐条 `Send`。

### 可选回调

- `WithFlushErrorHandler[T]`：flush 失败时记录日志；批次丢弃后 worker 继续
- `WithDropHandler[T]`：队列满或未 `Start` 时丢弃项

### FlushFunc 规范

- 签名：`func(ctx context.Context, items []T) error`
- 内部调用 `internal/repository/analytics` 的 `BatchInsert*`（传入 `[]analyticsmodel.X`）
- 在 flush 边界记录一次错误日志，不要把 DB 驱动错误直接暴露给 HTTP 客户端
- `Start` 使用 `context.WithoutCancel(parent)`，避免请求 ctx 取消中断后台 flush

## 各域独立实例（不共享队列）

每个业务域拥有自己的 `Writer`、配置与 `FlushFunc`：

| 域 | 表 | 现状 | 目标形态 |
| :--- | :--- | :--- | :--- |
| 管理端审计 | `w_user_access_logs` | `risk_control` → `batchwriter` + `analyticsrepo.BatchInsert` | 已接入 |
| 边缘访问日志 | `of_node_access_logs` | `openflare/chwriter` 异步 flush | 已接入 |
| 可观测时序 | `of_node_metric_snapshots` 等 5 表 | `openflare/chwriter` 五表独立 writer + 进程内短 TTL 去重 | 已接入 |

**不要**把 audit、access log、observability 并入同一 channel。

## 新增 ClickHouse 写入工作流

1. **Model**：在 `internal/model/analytics/` 定义 struct 与 `BatchInsertSQL()`（列顺序与 goose DDL 一致）。
2. **Goose DDL**：在 `internal/infra/persistence/migrator/goose/clickhouse/` 新增迁移（见 `database-migration`）。
3. **Repository**：实现 `BatchInsertX(ctx, []analyticsmodel.X) error`：
   - `len(items)==0` 直接返回
   - `db.ChConn == nil` 返回明确错误
   - 一次 `PrepareBatch` → 循环 `Append` → 一次 `Send`
4. **Writer 胶水**（`internal/apps/<domain>/` 或 `internal/repository/analytics/<domain>_writer.go`）：
    - `New` + `Start`，并在初始化逻辑内通过 `lifecycle.OnShutdown("your_writer_name", Stop)` 注册停机回调
    - 业务路径 `TryEnqueue`；HTTP 背压用 `IsFull()`
5. **测试**：
   - repository：mock `ChConn` 验证 `BatchInsertSQL` 与 append 列数
   - batchwriter：`go test ./internal/infra/persistence/batchwriter`
6. 运行 `make code-check`；有 API 变更时 `make swagger`。

## 背压与丢弃策略

| 场景 | 推荐策略 |
| :--- | :--- |
| 管理端 API 审计 | 队列满 → `IsFull()` 触发 429（见 `risk_control` middleware） |
| Agent 心跳指标 | 队列满 → `WithDropHandler` 记 warn；不阻塞心跳响应 |
| 边缘 access log | 优先扩大队列与 batch；必要时丢弃最旧或采样 |

## 禁止写法

```go
// ❌ 单条伪批量：每条都 PrepareBatch + Send
batch.Append(oneRow)
batch.Send()

// ❌ 写前 OLTP 式去重（高 RTT + 仍产生小 part）
SELECT count() FROM ... WHERE node_id = ? AND captured_at = ?

// ❌ Handler 内直接写 ClickHouse
db.ChConn.PrepareBatch(...)

// ❌ 全局单队列承载所有分析表
var globalChan chan any
```

去重应使用：`ReplacingMergeTree`、查询侧 `argMax`、或进程内短 TTL 去重缓存——**不要**在每次 insert 前 `SELECT count()`。

## async_insert（补充，非主方案）

可在 `internal/infra/persistence/clickhouse.go` 的 `Settings` 增加服务端异步写入作为第二层防护：

```go
"async_insert": 1,
"wait_for_async_insert": 1,
```

**不能替代**应用层批量；接入前需评估丢失可观测性与服务端负载。优先完成 `batchwriter` 接入后再考虑。

## Bootstrap 装配示例

```go
// internal/platform/bootstrap/bootstrap.go（示意）
var userAccessLogWriter *batchwriter.Writer[*analytics.UserAccessLog]

func RegisterAPI(ctx context.Context) {
    // ...
    if config.Config.ClickHouse.Enabled {
        initUserAccessLogWriter(ctx) // Start writer
        risk_control.BindWriter(userAccessLogWriter) // 或逐步替换 InitLogWriter
    }
}
```

- `RegisterAPI` / `RegisterAll`：`Start`
- 进程优雅停机：业务模块在初始化时调用 `lifecycle.OnShutdown` 注册，由 `bootstrap.Stop()` 代理 `lifecycle.Stop()` 并发停机。
- 使用 `sync.Once` 保证幂等

## 验证清单

```bash
go test ./internal/infra/persistence/batchwriter
go test ./internal/repository/analytics
make code-check
```

- flush 按 `MaxBatchSize` 与 `FlushInterval` 触发
- `Stop` 能 drain 队列内剩余项
- repository 层无 goroutine、无 channel
- `clickhouse.enabled: false` 时不 `Start` writer、不入队

## 相关文件速查

- 框架：`internal/infra/persistence/batchwriter/{config,writer,errs}.go`
- 连接：`internal/infra/persistence/clickhouse.go`
- 审计写入：`internal/apps/risk_control/logics.go`
- OpenFlare 写入胶水：`internal/apps/openflare/chwriter/writer.go`
- 节点访问日志 repository：`internal/repository/analytics/node_access_log_writer.go`
- 可观测 repository：`internal/repository/analytics/node_observability_writer.go`
- 生命周期管理器：`internal/platform/lifecycle/lifecycle.go`
- Bootstrap：`internal/platform/bootstrap/bootstrap.go`