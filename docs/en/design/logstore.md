# Log Store Decoupling

OpenFlare isolates all append-only, time-retention analytics and telemetry tables behind `internal/repository/logstore`. When ClickHouse is enabled, high-volume writes batch into columnar ClickHouse storage; when ClickHouse is disabled, PostgreSQL or SQLite handles all ingestion, real-time aggregation, and retention cleanup seamlessly, and operators can migrate live between backends.

See [OpenFlare Edge Reporting Protocol & Observability Data Model](./observability-data-model.md) for field contracts and schemas.

---

## 1. Design Goals

* **Optional ClickHouse**: Disabling ClickHouse leaves PostgreSQL (or SQLite when standalone) fully responsible for writes, queries, aggregations, and retention pruning.
* **Driver isolation**: Packages under `internal/apps/*` interact solely with `internal/repository/logstore` (or `repository` facades). Raw `db.ChConn`, `db.ChDB`, and ClickHouse native batches are confined inside `logstore`.
* **Online database switching**: The `of_log_db_switch` ("Switch Log Database") task copies historical rows between PostgreSQL/SQLite and ClickHouse and flips `log_database` atomically without deleting source data.

---

## 2. Classification Criteria and Active Domains

A table belongs in `logstore` only when all four conditions hold:

* Append-mostly workload with negligible per-row updates;
* Time-bounded queries/aggregations and retention-based pruning;
* Must remain readable and writable when ClickHouse is disabled;
* Excluded from transactional consistency of websites, nodes, or certificates.

Core entities (Zones, nodes, config versions, task executions, upload records) stay on the business primary DB (`repository`). Active `logstore` interfaces (`internal/repository/logstore/logstore.go`):

| Domain | Interface | Physical Tables |
| :--- | :--- | :--- |
| Edge node access logs | `AccessLogStore` | `of_node_access_logs` |
| Node metrics, probes, network quality, DNS metrics | `ObservabilityStore` | `of_node_metric_snapshots`, `of_node_ping_records`, `of_network_quality_samples`, `of_dns_metric_snapshots` |
| User access audit | `UserAccessLogStore` | `w_user_access_logs` |

ClickHouse hourly materialized views (`of_access_log_hourly`, `of_node_metric_capacity_hourly`) serve purely as acceleration layers; PostgreSQL and SQLite aggregate directly from raw partitioned/indexed tables. Legacy `of_node_edge_health`, `of_node_obs_frps`, and `of_node_obs_frpc` tables have been removed.

---

## 3. Layering

| Layer | Location | Responsibility |
| :--- | :--- | :--- |
| Facade & interfaces | `internal/repository/logstore` | Interfaces + `Active` / `BuildForMigration` routing by `log_database` |
| ClickHouse store | `logstore/clickhouse_store.go`, `logstore/ch_*.go` | Native `PrepareBatch` / `Append` / `Send` and analytical queries |
| Primary DB store | `logstore/postgres_store.go` | Shared GORM store for monthly partitioned PostgreSQL and plain SQLite tables |
| Analytics models | `internal/model/analytics` | Pure structs and insert column definitions, zero I/O |
| Async batching | `openflare/logwriter`, `risk_control` + `batchwriter` | `FlushFunc` calls `logstore.Active`; logs and metrics enqueue via hooks |
| Architecture guard | `logstore/imports_test.go` | Blocks `apps` packages from importing ClickHouse drivers, `migrator`, or raw DB connections |

`log_database` accepts only **the active business primary DB** (`postgres` or `sqlite`) or **`clickhouse`**. Both `log_database` and `log_db_migration` are protected keys blocked from manual option edits. Startup fails fast if `log_database=clickhouse` while ClickHouse is disabled.

---

## 4. Online Switch Protocol (`of_log_db_switch`)

1. Validate target backend differs from the active `log_database`.
2. Persist `log_db_migration=migrating` and call `Drain` (never `Stop`) on active `batchwriter` instances; incoming writes return HTTP 503 (`ErrMigrating`) during the freeze.
3. Truncate target tables, call `EnsurePartitions` when copying into PostgreSQL, and paginate by ascending Snowflake `id` across all `logstore` tables (`of_node_access_logs`, `of_node_metric_snapshots`, `of_node_ping_records`, `of_network_quality_samples`, `of_dns_metric_snapshots`, `w_user_access_logs`).
4. Flip `log_database=target` and clear `log_db_migration` only after full completion; failures clear the lock and resume writes against the original backend while keeping source rows intact for idempotent retries.

---

## 5. Adding a New Log Table

* **Identical schemas**: Keep column names identical across `internal/infra/persistence/migrator/goose/{clickhouse,postgres,sqlite}` and preserve Snowflake `uint64` IDs verbatim during migration.
* **High-volume partitioning**: ClickHouse uses `MergeTree` + `toYYYYMM` + native `TTL`; PostgreSQL uses `PARTITION BY RANGE` with the partition key included in the primary key; SQLite uses plain indexed tables.
* **No bypass writes**: Route async inserts through a dedicated `batchwriter` calling `logstore.Active`, and register the table in both switch-task `copy*` paths and `cleanup.go`.

Runtime settings: [Configuration Reference · Log Storage](../reference/configuration.md#8-log-database).
