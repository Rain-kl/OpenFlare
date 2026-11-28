# 日志存储解耦

OpenFlare 将所有追加写、可按时间保留清理的分析与观测表统一收拢于 `internal/repository/logstore` 抽象层。启用 ClickHouse 时写入高性能列式库，未启用时由业务主库（PostgreSQL 或 SQLite）完整承接读写、实时聚合与清理，并支持在不丢失源数据的前提下在线切换日志主库。

字段语义与完整表结构参见 [OpenFlare Edge 上报协议与观测数据模型](./observability-data-model.md)；本文约定存储边界与切换协议。

---

## 1. 目标与选型边界

* **ClickHouse 可选**：不启用 ClickHouse 时，PostgreSQL（或关闭主库时的 SQLite）完整承接写入、查询、聚合与过期清理。
* **上层不碰底层分析库**：`internal/apps/*` 只调用 `internal/repository/logstore`（或 `repository` 门面）。`db.ChConn`、`db.ChDB` 与原生 ClickHouse 批量读写仅限 `logstore` 内部使用。
* **在线切库**：任务管理中的「切换日志数据库」（`of_log_db_switch`）支持在主库与 ClickHouse 之间分页搬迁并原子切换 `log_database`。

---

## 2. 日志表准入标准与当前域

同时满足以下四项条件的数据表才进入 `logstore`：

* 以追加写入为主，极少执行单行更新；
* 按时间范围查询或聚合，可按保留天数过期删除；
* 在关闭 ClickHouse 后仍必须可写、可查；
* 不参与站点、节点、证书等核心资源的事务一致性。

Zone、节点、配置版本、任务执行记录与上传元数据必须走业务主库 `repository`，严禁放入日志库。当前已接入的日志域与物理表（`internal/repository/logstore/logstore.go`）：

| 域 | 接口 | 物理表 |
| :--- | :--- | :--- |
| 节点访问日志 | `AccessLogStore` | `of_node_access_logs` |
| 节点可观测、延迟探测、网络质量与 DNS 指标 | `ObservabilityStore` | `of_node_metric_snapshots`、`of_node_ping_records`、`of_network_quality_samples`、`of_dns_metric_snapshots` |
| 用户访问审计 | `UserAccessLogStore` | `w_user_access_logs` |

ClickHouse 上的小时级物化视图（`of_access_log_hourly`、`of_node_metric_capacity_hourly`）仅作为 CH 查询加速层；PostgreSQL 与 SQLite 不建同构汇总表，查询时从原始分区表或普通表实时聚合。历史遗留表 `of_node_edge_health`、`of_node_obs_frps` 与 `of_node_obs_frpc` 已随架构演进删除。

---

## 3. 分层架构

| 层级 | 路径 | 职责 |
| :--- | :--- | :--- |
| 抽象门面 | `internal/repository/logstore` | 接口定义与 `Active` / `BuildForMigration`，按 `log_database` 选择后端 |
| ClickHouse 实现 | `logstore/clickhouse_store.go`、`logstore/ch_*.go` | 原生 `PrepareBatch` / `Append` / `Send` 批量写及聚合 SQL |
| 业务主库实现 | `logstore/postgres_store.go` | PostgreSQL（高频表按月范围分区）与 SQLite（普通表 + 索引）共享 GORM 查询逻辑 |
| 分析实体模型 | `internal/model/analytics` | 结构体映射与批量 SQL 定义，无 IO |
| 异步入队与背压 | `openflare/logwriter`、`risk_control` + `batchwriter` | `FlushFunc` 调用 `logstore.Active`；节点日志与可观测通过 hooks 异步刷新 |
| 依赖守卫 | `logstore/imports_test.go` | 禁止 `apps` 层直接导入 ClickHouse 驱动、`migrator` 或内部连接句柄 |

`log_database` 仅存在两种合法状态：**跟随业务主库**（`postgres` 或 `sqlite`）或 **`clickhouse`**；`log_database` 与 `log_db_migration` 为受保护配置键，不允许通过常规设置接口直接修改。启动阶段若检测到 `log_database=clickhouse` 但未启用 ClickHouse，控制面将拒绝启动。

---

## 4. 在线切库协议 (`of_log_db_switch`)

1. 校验目标库合法且不同于当前 `log_database`。
2. 写入 `log_db_migration=migrating`，调用在途 `batchwriter` 的 `Drain`（而非 `Stop`）排空队列；迁移冻结期间所有新写入直接返回明确错误（HTTP 503 `ErrMigrating`）。
3. 清空目标日志表（若目标为 PostgreSQL，先调用 `EnsurePartitions` 预建对应月份分区），随后按 `id` 升序分页搬迁全量表（`of_node_access_logs`、`of_node_metric_snapshots`、`of_node_ping_records`、`of_network_quality_samples`、`of_dns_metric_snapshots`、`w_user_access_logs`）。
4. 仅当全部表复制成功后，原子更新 `log_database=target` 并清除 `log_db_migration`；任意步骤失败则清除迁移标记并恢复写入原库，源数据不删除，重试时重清目标表保证幂等。

---

## 5. 新增日志表接入约定

* **三方言列名一致**：在 `internal/infra/persistence/migrator/goose/{clickhouse,postgres,sqlite}` 保持完全一致的列名与 Snowflake `uint64` 主键。
* **高频表分区规划**：ClickHouse 使用 `MergeTree` + `toYYYYMM` + 原生 TTL；PostgreSQL 使用 `PARTITION BY RANGE(时间列)` 并在主键中包含分区键；SQLite 使用普通表加组合索引。
* **禁止旁路写入**：异步写入必须通过独立 `batchwriter` 并经 `logstore.Active` 落盘，切换任务 `copy*` 与定期清理（`cleanup.go`）须同步覆盖新表。

运行时配置见 [配置项参考 · 日志存储](../reference/configuration.md#8-日志存储log-database)。
