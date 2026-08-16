# Log Store Decoupling

You will learn: which tables are log-purpose, why they must not be pinned to ClickHouse, and which code path a new log table must follow.

Observability fields and the reporting protocol are still governed by [Observability Protocol & Tables](./observability-data-model.md); this document only defines **where data is stored and how to switch databases**.

---

## 1. Goals

* **ClickHouse optional**: when not enabled, PostgreSQL (or SQLite when the primary DB is off) fully takes over writes, queries, aggregation, and cleanup.
* **Upper layers don't touch the underlying DB**: apps only face `internal/repository/logstore` (or the `repository` facade). `repository/analytics` and `db.ChConn` / `db.ChDB` are only used by logstore's ClickHouse implementation.
* **Switchable**: 「Switch Log Database」in Task Management copies data between PostgreSQL/SQLite and ClickHouse and flips the primary; writes are frozen during migration, the switch only happens on success, and source data is not deleted.

---

## 2. What Counts as a Log Table

A table enters logstore only if it meets all of:

* Append-only writes, almost no row updates
* Query or aggregate by time, deletable by retention days
* Must still support writes and queries when ClickHouse is off
* Does not participate in transactional consistency for websites / nodes / certificates, etc.

**Don't** make these log tables: Zones, nodes, config versions, task executions, upload metadata. These go through the business primary DB `repository`.

Current log domains:

| Domain | Interface | Tables |
| --- | --- | --- |
| Node access logs | `AccessLogStore` | `of_node_access_logs` |
| Observability time series | `ObservabilityStore` | `of_node_metric_snapshots` / `of_node_edge_health` / `of_node_obs_frps` / `of_node_obs_frpc` |
| User access audit | `UserAccessLogStore` | `w_user_access_logs` |

Hourly materialized views on ClickHouse (e.g. `of_access_log_hourly`) only serve CH query acceleration. PostgreSQL / SQLite **do not** build isomorphic aggregation tables; queries aggregate in real time from raw logs.

---

## 3. Layering

| Layer | Path | Responsibility |
| --- | --- | --- |
| Abstraction | `internal/repository/logstore` | Interfaces + `Active` / `BuildForMigration`; selects implementation by `log_database` |
| CH implementation | `logstore/clickhouse_store.go` | Delegates to `repository/analytics` (native batch + existing aggregation SQL) |
| Primary DB implementation | `logstore/postgres_store.go` | PostgreSQL (high-frequency tables partitioned monthly) and SQLite (plain tables) share GORM |
| Model | `internal/model/analytics` | Entities and batch SQL, no IO |
| Enqueue | `chwriter` / `risk_control` + `batchwriter` | `FlushFunc` calls `logstore.Active`; node logs / observability enqueue via hooks |
| Constraint | `logstore/imports_test.go` | apps are forbidden from importing `repository/analytics` |

`log_database` has only two legal states: **follow the business primary DB** (`postgres` or `sqlite`) or **`clickhouse`**. "Primary PostgreSQL + log SQLite" does not exist. `log_database` / `log_db_migration` are protected and cannot be changed from the admin panel.

At startup: `log_database=clickhouse` but ClickHouse not enabled → startup is refused; you must re-enable ClickHouse, switch back to the primary DB, and only then turn it off.

---

## 4. Switch Protocol

Task type `of_log_db_switch` (admin name 「Switch Log Database」), parameter `target`.

1. Validate the target is legal and not the current DB.
2. Write `log_db_migration=migrating`, drain in-flight batchwriter (`Drain`, not `Stop` writer). Writes return a clear error afterward (HTTP 503), not queued backlog.
3. Clear the target log tables, then copy by id in pages; call `EnsurePartitions` on the PostgreSQL target before copying.
4. Only on full success write `log_database=target` and clear the migration marker; on failure clear the marker and writes continue on the source DB.
5. Source data is not deleted; re-clear the target before retry to guarantee idempotency.

Don't invent another switch protocol, and don't connect `analyticsrepo` directly inside tasks.

---

## 5. Adding a New Log Table

Column names must be identical across the three goose migrations (ClickHouse / PostgreSQL / SQLite). Key points:

* High-frequency tables: CH uses `MergeTree` + `toYYYYMM`; PG uses `PARTITION BY RANGE(time column)` with the partition key in the primary key; SQLite uses a plain table + indexes.
* IDs use snowflake `uint64`, preserved as-is during migration.
* Writes go through a dedicated `batchwriter`; flush calls `logstore.Active`, not `analyticsrepo.BatchInsert`.
* The switch task's `copy*` must cover the new table; cleanup uses existing `log_retention_days_*` or `metric_retention_days`, don't use the wrong TTL.

Runtime config: [Configuration Reference · Log Storage](../reference/configuration.md#8-日志存储log-database).

---

## 6. Related Docs

* Observability fields and reporting protocol: [Observability Protocol & Tables](./observability-data-model.md)
