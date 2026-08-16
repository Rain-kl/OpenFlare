# 日志存储解耦

你会学到：哪些表属于日志用途、为什么不能绑死 ClickHouse，以及新增一张日志表时必须走哪条代码路径。

观测字段与上报协议仍以 [观测上报协议与表结构](./observability-data-model.md) 为准；本文只约定**存到哪、怎么切库**。

---

## 1. 目标

* **ClickHouse 可选**：不启用时，PostgreSQL（或关闭主库时的 SQLite）完整承接写入、查询、聚合与清理。
* **上层不碰底层库**：apps 只面向 `internal/repository/logstore`（或 `repository` 门面）。`repository/analytics` 与 `db.ChConn` / `db.ChDB` 仅供 logstore 的 ClickHouse 实现使用。
* **可切换**：任务管理里的「切换日志数据库」在 PostgreSQL/SQLite 与 ClickHouse 之间复制数据并翻转主库；迁移期间冻结写入，成功才切换，源数据不删。

---

## 2. 什么算日志表

同时满足才进 logstore：

* 追加写入，几乎不更新单行
* 按时间查询或聚合，允许按保留天数删除
* 关闭 ClickHouse 后仍要能写、能查
* 不参与网站 / 节点 / 证书等事务一致性

**不要**做成日志表：Zone、节点、配置版本、任务执行、上传元数据。这些走业务主库 `repository`。

当前日志域：

| 域 | 接口 | 表 |
| --- | --- | --- |
| 节点访问日志 | `AccessLogStore` | `of_node_access_logs` |
| 可观测时序 | `ObservabilityStore` | `of_node_metric_snapshots` / `of_node_edge_health` / `of_node_obs_frps` / `of_node_obs_frpc` |
| 用户访问审计 | `UserAccessLogStore` | `w_user_access_logs` |

ClickHouse 上的小时级物化视图（如 `of_access_log_hourly`）只服务 CH 查询加速。PostgreSQL / SQLite **不建**同构聚合表，查询时从原始日志实时聚合。

---

## 3. 分层

| 层级 | 路径 | 职责 |
| --- | --- | --- |
| 抽象 | `internal/repository/logstore` | 接口 + `Active` / `BuildForMigration`；按 `log_database` 选实现 |
| CH 实现 | `logstore/clickhouse_store.go` | 委托 `repository/analytics`（原生批量 + 现有聚合 SQL） |
| 主库实现 | `logstore/postgres_store.go` | PostgreSQL（高频表按月分区）与 SQLite（普通表）共用 GORM |
| Model | `internal/model/analytics` | 实体与批量 SQL，无 IO |
| 入队 | `chwriter` / `risk_control` + `batchwriter` | `FlushFunc` 调 `logstore.Active`；节点日志 / 可观测经 hooks 入队 |
| 约束 | `logstore/imports_test.go` | apps 禁止 import `repository/analytics` |

`log_database` 只有两种合法状态：**随业务主库**（`postgres` 或 `sqlite`）或 **`clickhouse`**。不存在「主库 PostgreSQL + 日志 SQLite」。`log_database` / `log_db_migration` 受保护，管理端不可改。

启动时：`log_database=clickhouse` 但 ClickHouse 未启用会拒绝启动，须先重新启用 ClickHouse 并切回主库后再关掉。

---

## 4. 切换协议

任务类型 `of_log_db_switch`（管理端名称「切换日志数据库」），参数 `target`。

1. 校验目标合法且不等于当前库。
2. 写 `log_db_migration=migrating`，排空在途 batchwriter（`Drain`，不要 `Stop` writer）。此后写入返回明确错误（HTTP 503），不排队积压。
3. 清空目标日志表后按 id 分页复制；复制前对 PostgreSQL 目标 `EnsurePartitions`。
4. 全部成功才写 `log_database=target` 并清除迁移标记；失败清除标记，写入继续走源库。
5. 源数据不删；重试前重新清空目标以保证幂等。

不要另起切换协议，也不要在任务里直连 `analyticsrepo`。

---

## 5. 新增一张日志表

列名必须在 ClickHouse / PostgreSQL / SQLite 三套 goose 迁移中一致。要点：

* 高频表：CH 用 `MergeTree` + `toYYYYMM`；PG 用 `PARTITION BY RANGE(时间列)`，主键含分区键；SQLite 普通表 + 索引。
* ID 用 snowflake `uint64`，迁移时原样保留。
* 写入走独立 `batchwriter`；flush 调 `logstore.Active`，不要 `analyticsrepo.BatchInsert`。
* 切换任务的 `copy*` 必须覆盖新表；清理走已有 `log_retention_days_*` 或 `metric_retention_days`，不要用错 TTL。

运行时配置见 [配置项参考 · 日志存储](../reference/configuration.md#8-日志存储log-database)。

---

## 6. 相关文档

* 观测字段与上报协议：[观测上报协议与表结构](./observability-data-model.md)
