# 日志数据库解耦设计（ClickHouse 可选化）

> 状态：已与用户逐段确认，待用户复核。
> 日期：2026-08-08

## 1. 背景与目标

当前系统日志/分析（访问日志、可观测时序）完全绑定 ClickHouse：`internal/repository/analytics` 直接操作 `db.ChConn`/`db.ChDB`，apps 层（`chwriter`、`risk_control`、`admin/logs`、`admin/status`）依赖 `config.ClickHouse.Enabled` 判断可用性。业务流量小、主机性能低时 ClickHouse 负担大。

目标：

1. **解耦**：ClickHouse 变为可选项；不启用时，主库（PostgreSQL；禁用时 SQLite）完整承接全部日志功能（写入、查询、聚合、清理）。
2. **代码级约束**：上层应用写日志不能直接调用底层库（`analyticsrepo` / `db.ChConn`），用接口 + import-lint 测试保证，而非 AGENTS.md 口头约束。
3. **可迁移**：提供用户触发的「切换日志数据库」任务，支持 PostgreSQL/SQLite ↔ ClickHouse 数据迁移。
4. **表结构**：CH 日志表迁入 PG/SQLite；CH 保持只有日志表的 SQL 脚本；PG/SQLite 包含全部表。

## 2. 现状要点

- 连接：`internal/infra/persistence/clickhouse.go`（`ChConn` 原生批量写 + `ChDB` GORM 查询），`init()` 依据 `clickhouse.enabled`。
- 分析域：`internal/repository/analytics/` 直接读写 CH；apps 通过 `batchwriter` 异步 flush（`chwriter`、`risk_control`）。
- 已有抽象雏形：`internal/repository/openflare_access_log_store.go` / `openflare_observability_store.go` 中的未导出 `accessLogStore` / `observabilityStore` 接口，默认 `clickhouseAccessLogStore{}`，测试可换 memory 实现——默认写死 CH、不可配置切换、接口未导出。
- 迁移：主库 goose（`goose/postgres` + `goose/sqlite` 双方言）与 CH 单方言（`goose/clickhouse`）分离。
- 历史：PG/SQLite 曾有过 `of_node_metric_snapshots`、`of_node_access_logs` 等观测表（`202606190010_create_of_observability_tables.sql`），后由 `202606200005_drop_of_node_observability_timeseries.sql` 删除（迁去 CH）。**旧 DDL 可复活改造**。
- 任务：Asynq + `task.RegisterHandler`/`RegisterTaskMeta`；`system_cleanup`（系统垃圾清理）每日任务已存在；`of_database_auto_cleanup`（可观测清理，schedule id=102）存在。
- 系统配置：`system_configs` 表（key/type/visibility），现有 `database_auto_cleanup_enabled` / `database_auto_cleanup_retention_days`（business）。

## 3. 已确认的核心决策

| # | 决策 |
|---|---|
| 1 | 范围：CH 不启用时，PG（或 SQLite）承担**全部**日志功能；聚合在 PG/SQLite 查询时实时计算，不物理建 MV 同构表。 |
| 2 | 实现：接口定义在 repository 层；PG 用 GORM 全新实现；CH 保留现有原生批量优化（`PrepareBatch`）包进同一接口。 |
| 3 | SQLite 是一等公民：`log_database` ∈ {`postgres`, `sqlite`, `clickhouse`}；迁移方向 PG→CH、SQLite→CH、CH→PG、CH→SQLite。 |
| 4 | 日志库只有两种合法状态：**随主库**（`database.enabled` → postgres，否则 sqlite）或 **clickhouse**；不存在主库 PG + 日志 SQLite 的组合。 |
| 5 | 迁移任务「切换日志数据库」：纯复制、**源数据不删除**、可重试；迁移期间**冻结日志写入**（拒绝，不排队积压）；全部成功才翻转主库标记。 |
| 6 | 清理统一到 `system_cleanup`（每日一次，日志过期无需实时）；保留时间按**存储库**配置（`type=business`）。 |

## 4. 包结构与接口（方案一）

新增 `internal/repository/logstore/`，职责唯一：日志存储抽象。

```
internal/repository/logstore/
├── logstore.go         # 导出接口：AccessLogStore / ObservabilityStore / UserAccessLogStore / CleanupStore / StatusStore
├── provider.go         # Open(ctx) 按当前日志主库返回实现；ActiveDatabase() 供状态/UI；测试可注入
├── postgres_store.go   # GORM 实现（PG 与 SQLite 共用一套，方言差异只在 goose DDL + dialect_* 小文件）
├── dialect_postgres.go # PG 方言 SQL 片段（date_trunc / FILTER / 分区清理）
├── dialect_sqlite.go   # SQLite 方言 SQL 片段（strftime / unixepoch）
└── clickhouse_store.go # 把现有 analyticsrepo 原生批量 + GORM 查询包进接口（零性能损耗）
```

- **接口划分**（避免 40+ 方法巨型接口，合成 `logstore.Store` 结构体持有）：
  - `AccessLogStore`：节点访问日志的 InsertBatch / List / Count / RegionCounts / BucketAggregates / CountBuckets / BucketDimensions / IPAggregates / IPSummaries / CountIPSummaries / WAFIPAggregates / IPTrend / TrafficSummary / ValueCounts / NodeAggregates / DeleteAll / DeleteBefore / DeleteByNodeBefore。
  - `ObservabilityStore`：4 表（metric snapshots / edge health / frps / frpc）的 Insert / List / Delete。
  - `UserAccessLogStore`：`w_user_access_logs` 的 BatchInsert / Count / List / 统计（DailyTrend / BrowserDistribution / TopActiveUsers 等）。
  - `CleanupStore`：按保留天数清理过期数据（PG=分区 DROP + 分批 DELETE；SQLite=分批 DELETE；CH=MODIFY TTL + materialize）。
  - `StatusStore`：当前库状态、CH 运行指标（激活时）、GORM 写入器状态。
- **消费面**：`internal/repository` 现有公开函数（`ListOpenFlareAccessLogs`、`InsertOpenFlareAccessLogsBatch`、`InsertOpenFlareMetricSnapshot` 等）**保留签名、改为一行委托 `logstore`**，apps 调用面几乎不动；apps 里现有 `analyticsrepo` 直连（`risk_control`、`chwriter`、`tasks/database_cleanup.go`、`observability/access_log_logics.go`、`admin/logs`、`admin/status`）全部改走 repository/logstore。
- **import-lint 测试**：新增 `go test`，扫描 `internal/apps/**` 的 import，发现 `internal/repository/analytics` 或 `internal/infra/persistence`（`batchwriter` 白名单除外）即失败。这是「代码层面规避」的验收。
- `analyticsrepo` 保留，仅被 `logstore/clickhouse_store.go` 引用（CH 实现细节）。

### 主库标记与启动校验

- `system_configs` 新增内部 key：
  - `log_database`（`postgres`/`sqlite`/`clickhouse`）：当前日志主库，仅迁移任务写入。
  - `log_db_migration`（`"migrating"`/空）：迁移冻结标记，仅迁移任务写入。
- **首次 seed**（bootstrap Go 侧，因依赖运行时主库选择）：key 缺失时，`clickhouse.enabled` → `clickhouse`（保持现状、不丢现有 CH 数据）；否则 → 当前主库（`database.enabled` → `postgres`，否则 `sqlite`）。
- **启动校验**（bootstrap）：
  - `log_database=clickhouse` 但 `clickhouse.enabled=false` → 启动报错：「当前日志主库为 ClickHouse 但 ClickHouse 未启用。请先重新启用 ClickHouse 配置并启动，在任务管理运行『切换日志数据库』迁移到 PostgreSQL/SQLite 后再禁用 ClickHouse」。
  - `log_database=postgres` 但 `database.enabled=false`，或 `log_database=sqlite` 但 `database.enabled=true` → 启动报错（违反「随主库或随 CH」规则）。
- **key 保护**：`log_database`、`log_db_migration` 在配置更新接口（admin system-configs / option 校验）拒绝修改；仅迁移任务可写；启动校验兜底被篡改组合。
- **热切换**：`logstore` 通过系统配置缓存（Redis，更新即失效）读取 `log_database`；翻转后 API 进程自动切到新实现，无需自定义跨进程协议。

## 5. PG/SQLite 表结构与优化

**新建原始日志表（PG + SQLite 双方言 goose，同版本号）**——只建原始表，**不建** CH 物化视图/聚合表（`of_access_log_hourly`、`of_node_metric_capacity_hourly` 等），PG/SQLite 查询时实时聚合：

| 表 | 说明 |
|---|---|
| `w_user_access_logs` | 用户访问日志 |
| `of_node_access_logs` | 节点访问日志（含 user_agent/cache_status/bytes_sent/request_length/request_time_ms 现行列） |
| `of_node_metric_snapshots` | 资源指标 |
| `of_node_edge_health` | 边缘健康 |
| `of_node_obs_frps` | FRPS 观测 |
| `of_node_obs_frpc` | FRPC 观测 |

- **ID**：沿用 snowflake uint64（DDL 用 BIGINT，与 CH UInt64 对齐）；不换自增，保证迁移 ID 原样保留、无冲突。
- **时间**：PG `TIMESTAMPTZ`；SQLite `DATETIME`。
- **复合主键**：分区表主键 `(id, 时间列)`（满足 PG 分区键进唯一索引要求）。

### PG 优化

1. **分区**：仅 `of_node_access_logs`、`w_user_access_logs` 两个高频表用 PG 原生 `PARTITION BY RANGE` **按月分区**；可观测 4 表数据量小，普通表 + 索引。SQLite 无原生分区 → 普通表 + 组合索引（方言差异只留在 goose DDL，运行时 GORM 代码共用）。
2. **批量写入**：PG/SQLite 统一 GORM `CreateInBatches`（批次 500–1000）；CH 维持原生 `PrepareBatch`。
3. **索引**：
   - `of_node_access_logs`：`(logged_at DESC)`、`(node_id, logged_at DESC)`、`(host, logged_at DESC)`；
   - `w_user_access_logs`：`(created_at DESC)`、`(user_id, created_at DESC)`；
   - 可观测表：`(node_id, captured_at DESC)`。
4. **聚合查询重写**：PG 用 `date_trunc` / `count(DISTINCT)` / `FILTER (WHERE ...)` 等价替换 CH 的 `toStartOfHour` / `uniqExact` / `countIf`；SQLite 用 `strftime` / `unixepoch`。时间分桶等少量方言 SQL 拆到 `dialect_postgres.go` / `dialect_sqlite.go`，store 主体方言中立。

### goose 迁移

- PG/SQLite 各新增一组建表迁移（复活并改造 `202606190010` 旧 DDL，按 database-migration 技能双方言、同版本号规则）。
- CH 目录不动（本来就只有日志表脚本，满足「CH 保持只有日志表 SQL」）。

## 6. 清理（并入 system_cleanup）

- 日志过期清理并入 `system_cleanup`（系统垃圾清理）每日任务；`of_database_auto_cleanup` 专用 schedule（id=102）与任务下线。
- 新增 `type=business` 配置（替换旧 `database_auto_cleanup_enabled` / `database_auto_cleanup_retention_days`）：
  - `log_retention_days_postgres`（默认 90）
  - `log_retention_days_sqlite`（默认 90）
  - `log_retention_days_clickhouse`（默认 90）
- `CleanupStore` 按当前生效库读取对应值执行：
  - PG：分区 DROP（整月）+ 分批 DELETE（不满月）；
  - SQLite：分批 DELETE；
  - CH：`ALTER TABLE ... MODIFY TTL toDateTime(...) + INTERVAL N DAY` + materialize（保留期由配置驱动，不再依赖 DDL 写死）。
- 旧 key `database_auto_cleanup_*` 由 goose 迁移删除，前端同步清理。

## 7. 迁移任务「切换日志数据库」

**元数据**：Asynq `openflare:log_db_switch`，管理类型 `of_log_db_switch`，名称「切换日志数据库」，参数 `target`（`postgres`/`sqlite`/`clickhouse`），`Retryable: true`。UI 按当前日志主库只展示合法目标（当前=CH → 「主库」；当前=主库 → 「ClickHouse」）。

**执行流程（worker 进程）**：

1. **校验**：`target == 当前主库` → 拒绝；`target=clickhouse` 但 CH 未启用 / `target=postgres` 但 `database.enabled=false` / `target=sqlite` 但 `database.enabled=true` → 拒绝。
2. **写冻结**：写 `log_db_migration = "migrating"`；先让 batchwriter 把在途批次 flush 完；此后 API 进程所有日志写入路径（`risk_control`、`chwriter` 队列、agent 上报落库）检查该 key → 返回明确错误（HTTP 503「日志数据库迁移中，暂不可写」），不排队积压。
3. **复制**：6 张原始日志表逐表、按 id 分批（每批 ~1000）读源 → 写目标（CH→主库用 GORM `CreateInBatches`；主库→CH 用原生 `PrepareBatch`）；ID 原样保留；每表/每批 `task.AppendLog` 进度。
   - **幂等前提**：开始复制前**清空目标库日志表**（任务参数「覆盖目标库已有日志」默认开启；目标库通常为空，仅「切回去」场景有旧数据）——保证失败重试可重跑不重复。
4. **翻转**：全部成功 → 更新 `log_database = target`、清除迁移标记 → `logstore` 缓存失效自动切到新实现 → 写入恢复（走新库）。
5. **失败**：返回错误触发 Asynq 重试；**失败时清除迁移标记**，写入继续走源库（不丢功能）；重试时重新清空目标 + 复制。

**双进程一致性**：迁移标记与主库标记落在 `system_configs`（Redis 缓存，worker 更新后 API 进程自动失效重读）。

## 8. API 与前端

**后端**：

- `GET /api/v1/admin/status/log-database`（改造现有 `/clickhouse` 状态端点）：返回当前日志主库、迁移状态（`idle`/`migrating`）、各库保留天数、当前合法迁移目标；CH 为主时附带现有 CH 运行指标，主库为主时附带 GORM 写入器状态。
- 任务「切换日志数据库」走现有任务管理通用派发 API（`RegisterTaskMeta` + Params），无需新派发接口；执行记录/进度复用任务框架。
- 系统配置：新增 3 个 `log_retention_days_*`（business）图形化 + 参数表可见；新增内部 `log_database`、`log_db_migration`（system、visibility=0、受保护）；下线 `database_auto_cleanup_*`。

**前端**：

- 任务管理页：出现「切换日志数据库」，参数下拉只显示合法目标；页面展示当前日志主库与迁移状态。
- `/admin/settings` 业务配置：新增「日志保留时间」分组（PG/SQLite/CH 三个数字输入）。
- 状态/仪表盘：日志库状态卡片（当前库 + 迁移中提示）。

## 9. 测试与验证

- **import-lint 测试**：`internal/apps/**` 不得 import `internal/repository/analytics`、`internal/infra/persistence`（`batchwriter` 白名单除外），违规即失败。
- **logstore 单测**：GORM 实现用 SQLite 全量跑；PG 专属（分区 DROP 等）走既有集成测试路径；CH 实现复用现有 analyticsrepo 测试。
- **迁移任务测试**：目标/组合校验、批处理与 ID 保留、清空目标、翻转标记、失败清标记回退、冻结期写入拒绝——用 memory/sqlite 双端模拟，不依赖真实 CH。
- **清理测试**：`system_cleanup` 日志清理步骤（PG 分区 DROP / SQLite 分批 DELETE / CH TTL 修改）与保留配置读取。
- **迁移验证**：goose 空库 Up 全量（PG/SQLite/CH 三套）、`go test ./...`、`make swagger`（API 变更）、`make code-check`、`make format`。

## 10. 非目标（YAGNI）

- 不在 PG/SQLite 物理建聚合/物化视图表（查询实时聚合）。
- 不做 PG ↔ SQLite 日志互迁（非法组合，启动校验拒绝）。
- 迁移成功不自动删除源库数据（保留，后续提供手动清理入口）。
- 不引入 PG COPY 协议（GORM `CreateInBatches` 对低流量足够）。
- 不引入自定义跨进程迁移协议（`system_configs` + Redis 缓存即可）。

## 11. 里程碑建议（供实现计划分解）

1. **M1 抽象与改造**：`logstore` 接口 + PG/SQLite 实现 + `clickhouse_store` 包装 + import-lint 测试 + repository 委托改造 + apps 直连改造 + `log_database`/`log_db_migration` key 与启动校验。
2. **M2 表与清理**：goose 双方言建表迁移 + 保留配置 key + `system_cleanup` 日志清理步骤 + 下线 `of_database_auto_cleanup` 与旧配置。
3. **M3 迁移任务与展示**：迁移任务 Handler + 状态端点 + 任务管理页/业务配置前端 + 日志库状态卡片。
4. **M4 收尾**：全量验证（goose 三套、单测、`make code-check`/`swagger`/`format`）、文档同步（中文）、changelog `[Unreleased]`。

## 12. 实现归档说明（Task 18，2026-08-08）

- 设计稿第 4 节 provider 入口写作 `Open(ctx)`，实现命名为 `Active(ctx)`（按 `log_database` 解析并缓存，配置翻转后重建），另导出 `Build(ctx, database)` / `BuildForMigration(ctx, database)` 供迁移任务构造目标库 store；`ActiveDatabase(ctx)` 供状态端点。
- 设计稿第 4 节列出的 `CleanupStore` 接口未单独落地：清理实现为包级 `CleanupExpired(ctx)`（按当前激活库保留天数删除过期日志并预建 PG 分区），由 `system_cleanup` 每日任务调用。
- 设计稿第 4 节列举的 `tasks/database_cleanup.go` 已随 M2 下线（`of_database_auto_cleanup` 配置与前端 UI 一并移除），日志清理职责并入 `system_cleanup`。
- 迁移复制按 id 升序分页，`copyObservability` 以每批最后一条 id 作为下一批游标（修正计划中 `lastID += n` 的近似写法）；失败回退由 `defer setMigrationFlag("")` 保证源库恢复可写，重试前先清空目标库保证幂等。
- 其余实现决策（`SetConfigReader` 注入、`ensureWritable` 统一冻结、解析 helper 迁至 `model/analytics` 等）见计划「自检记录」，与本文档一致。
