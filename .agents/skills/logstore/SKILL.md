---
name: "logstore"
description: "OpenFlare / Wavelet：当新增或修改日志/分析用途表（节点访问日志、用户访问日志、可观测时序）、接入 internal/repository/logstore、切换日志主库、实现 PG/SQLite 回落，或判断一张表该走业务主库还是日志库时必须使用。"
---

# 日志用途表开发

开始前阅读根目录 `AGENTS.md`。DDL 用 `database-migration`；高频写入队列用 `clickhouse-batchwriter`；切换任务用 `new-async-task`。本技能只回答：**这张表是不是日志表，以及如何接入可切换的日志主库。**

设计背景见 [日志存储解耦](../../../docs/design/logstore.md)。

## 先判定

日志表同时满足：

- 追加写入、几乎不更新单行
- 按时间查询/聚合，允许按保留天数删除
- 关闭 ClickHouse 后仍要能写、能查
- 不参与网站/节点/证书等事务一致性

**不要**做成日志表：Zone、节点、配置版本、任务执行、上传元数据。这些走主库 `repository`。

当前日志域：

| 域 | 接口 | 表 |
| :--- | :--- | :--- |
| 节点访问日志 | `AccessLogStore` | `of_node_access_logs` |
| 可观测 | `ObservabilityStore` | `of_node_metric_snapshots` / `of_node_edge_health` / `of_node_obs_frps` / `of_node_obs_frpc` |
| 用户访问审计 | `UserAccessLogStore` | `w_user_access_logs` |

## 分层

| 层级 | 路径 | 职责 |
| :--- | :--- | :--- |
| 抽象 | `internal/repository/logstore` | 接口 + `Active`/`BuildForMigration`；apps **只**面向这里或 `repository` 门面 |
| CH 实现 | `logstore/clickhouse_store.go` 委托 `analytics` | 原生批量 + 现有聚合 SQL |
| 主库实现 | `logstore/postgres_store.go` | PG（按月分区）与 SQLite（普通表）共用 GORM |
| Model | `internal/model/analytics` | 实体与批量 SQL，无 IO |
| 入队 | `chwriter` / `risk_control` + `batchwriter` | flush 调 logstore `BatchInsert*`；CH 入队经 hooks |
| 切换 | `of_log_db_switch` | 冻结 → `chwriter.Drain` → 逐表复制 → 翻转 |
| 约束 | `logstore/imports_test.go` | apps 禁止 import `repository/analytics` |

`log_database` 只能是「随主库」或 `clickhouse`。`log_database` / `log_db_migration` 受保护。

## 新增一张日志表

1. **Model**（`internal/model/analytics`）：`TableName` + `InsertColumns` / `BatchInsertSQL`。
2. **三套 DDL**：CH `MergeTree` + `toYYYYMM`；PG `PARTITION BY RANGE(时间列)`（主键含分区键）；SQLite 普通表。不要在主库建 CH 物化视图，聚合实时算。
3. **挂到已有域或新接口**：能进 `AccessLogStore` / `ObservabilityStore` / `UserAccessLogStore` 就不要再拆包。新域才新增接口并放进 `Store`。
4. **方法最少集**：`BatchInsert`（含 `ensureWritable`）、业务查询、`ListForMigration`、`MigrationRange`、`DeleteAll`、`DeleteBefore`、`EnsurePartitions`（仅 PG 预建）。
5. **双实现**：CH 委托 `analyticsrepo`；GORM 共用一套，方言 SQL 放 `dialect_*.go`。零值 id 用 `idgen.NextUint64ID()`。
6. **`buildStore`**：CH / GORM 两分支都挂上。
7. **写入**：独立 `batchwriter`；`FlushFunc` → `logstore.Active`。节点日志/可观测走 `SetAccessLogHooks` / `SetObservabilityHooks`，不要让 apps 碰 `ChConn`。
8. **切换任务**：`clearTarget` + `copy*` 增加该表；源数据不删，失败不翻转。
9. **清理**：访问类走 `log_retention_days_*`；性能指标走 `metric_retention_days`。不要擅自共用错误的 TTL。
10. **import-lint**：apps 新增对 `analytics` 或 `infra/persistence`（`batchwriter`/`idgen` 除外）的 import 必须失败。

## 禁止

- apps 直连 `analyticsrepo` / `db.ChConn` / `db.ChDB` 做日志读写
- 只建 CH、不建主库回落
- Handler 内逐条 `PrepareBatch`
- 业务表塞进 logstore
- 管理端改 `log_database` / `log_db_migration`

## 验证

```bash
go test ./internal/repository/logstore ./internal/repository/analytics
go test ./internal/apps/openflare/... ./internal/apps/admin/logs ./internal/apps/admin/status
make swagger
make code-check
```

对照：`of_node_access_logs` 或 `w_user_access_logs` 的 model、三库 goose、`logstore` 双实现、`chwriter`/`risk_control` flush、`LogDBSwitchHandler`。
