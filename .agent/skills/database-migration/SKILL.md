---
name: "database-migration"
description: "Wavelet 项目专用：当新增或修改数据库表结构、索引、初始化数据、系统配置 seed、模板 seed、默认管理员、goose SQL 迁移、internal/infra/persistence/migrator、ClickHouse 分析库 DDL 或数据库升级流程时必须使用。本技能指导在 internal/infra/persistence/migrator/goose 下编写 PostgreSQL/SQLite 双方言 SQL 迁移，以及在 goose/clickhouse 下编写 ClickHouse 单方言分析表迁移，并完成验证。"
---

# Wavelet 数据库升级操作指南

Wavelet 使用 `github.com/pressly/goose/v3` 执行 SQL 迁移。迁移入口是 `internal/infra/persistence/migrator.Migrate()`，SQL 文件嵌入在二进制中。

## 基本规则

- SQL 迁移文件放在：
    - `internal/infra/persistence/migrator/goose/postgres/`
    - `internal/infra/persistence/migrator/goose/sqlite/`
- PostgreSQL 和 SQLite 必须使用同一个版本号、同一个语义文件名。
- 迁移文件使用 goose SQL 标记：

```sql
-- +goose Up
...

-- +goose Down
...
```

- 不要把表结构、默认系统配置、默认模板、默认管理员初始化写回 Go 代码。
- 编辑表结构（DDL）和插入表数据（DML/Seed）不要放在同一个 SQL 文件里，必须分成两个独立的 SQL 文件完成（例如，先通过一个文件修改表结构，再通过下一个递增版本号的文件插入/初始化数据）。
- 插入定时任务（schedules 表数据）时绝对不能指定 `id`，必须依靠数据库自增（Identity 或 AUTOINCREMENT）自动分配，防止与用户手动或后续插入的定时任务产生 ID 冲突。
- 不要添加物理外键；关系字段使用显式索引。
- 数据库默认值应匹配 Go model 零值或业务兜底值。
- 系统配置仍然保存字符串值；布尔值写 `"true"` / `"false"`，数字写十进制字符串，复杂结构写合法 JSON 字符串。

## 新增迁移流程

1. 先确认涉及的 Go model、读写路径和前端/接口消费方。
2. 选择下一个递增版本号，格式建议 `YYYYMMDDNNNN`，例如：

```text
202606090002_add_example_column.sql
```

3. 在 PostgreSQL 和 SQLite 目录各新增同名 SQL 文件。
4. 写 `Up`：
   - 表结构变更使用 SQL DDL。
   - 初始化/seed 数据使用 SQL `INSERT`。
   - 需要幂等时使用 `IF NOT EXISTS` 或 `ON CONFLICT ... DO NOTHING`。
5. 写 `Down`：
   - 能安全回滚的结构变更写反向 DDL。
   - seed 数据按 key/name 等稳定标识删除。
6. 如果变更 API handler，运行 `make swagger`。
7. 至少运行：

```bash
go test ./internal/infra/persistence/migrator
go test ./internal/model ./internal/apps/config ./internal/apps/admin/system_config
make code-check
```

## 方言注意事项

- PostgreSQL 自增主键用 `BIGSERIAL`；SQLite 自增主键用 `INTEGER PRIMARY KEY AUTOINCREMENT`。
- PostgreSQL 时间类型优先 `TIMESTAMPTZ`；SQLite 使用 `DATETIME`。
- PostgreSQL JSON 字段用 `JSONB`；SQLite 用 `JSON` 或 `TEXT`。
- 两个方言目录的字段名、索引名、seed 数据语义必须保持一致。

## 修改默认系统配置

- 新增或调整系统配置 seed 时，更新两个方言的 SQL 文件。
- `visibility` 使用常量语义：`0` 不公开，`1` 通过 `/api/v1/config/public` 返回。
- 公共配置 API 直接返回所有 `visibility = 1` 的配置键值，不要在 handler 中重新硬编码 key 列表。

## 验证重点

- goose 能在空库上完整执行。
- `system_configs`、默认 `admin`、内置模板能按预期初始化。
- 新增表/列与 Go model 的列名、类型和默认值兼容。
- 前端或接口消费的公共配置值仍按字符串解析。

## ClickHouse 分析库（辅助 OLAP）

ClickHouse 是**辅助 OLAP 存储**，与 PostgreSQL/SQLite 主库**完全独立**的迁移与访问管线：

- 主库（PG/SQLite）：业务事务数据、`goose_db_version`、双方言 SQL。
- 分析库（ClickHouse）：访问日志、统计聚合等分析型数据、`goose_clickhouse_version`、单方言 SQL。

**不要**把 ClickHouse 表结构混入 PG/SQLite 迁移目录，也**不要**在 `support-files/`、`internal/apps/` 或 `internal/repository/` 中手写 DDL。

### 目录与职责

| 路径 | 职责 |
| :--- | :--- |
| `internal/infra/persistence/migrator/goose/clickhouse/` | **唯一** ClickHouse DDL 来源（goose SQL，嵌入二进制） |
| `internal/model/analytics/` | 分析表 Go model，列名须与 goose DDL 一致 |
| `internal/repository/analytics/` | 所有 ClickHouse 读写（批量写入、查询、聚合） |
| `internal/infra/persistence/clickhouse.go` | 连接初始化（`ChConn` 原生批量、`ChDB` GORM 查询） |

### 迁移入口与版本表

- 入口：`migrator.MigrateClickHouse()`，在 `cmd/root.go` 的 `PreRun` 中于 `migrator.Migrate()` 之后调用。
- 仅当 `clickhouse.enabled: true` 时执行；禁用时直接跳过（见 `TestMigrateClickHouseSkipsWhenDisabled`）。
- 版本表：`goose_clickhouse_version`，与主库 `goose_db_version` **分离**，互不影响。
- 方言：仅 ClickHouse，**无** SQLite 镜像目录。

### ClickHouse 迁移规则

1. **DDL 只写 goose SQL**：`CREATE TABLE IF NOT EXISTS ...`，禁止 GORM `AutoMigrate`、禁止在 repository 或 handler 中建表。
2. **无事务**：ClickHouse 不支持 goose 事务包装；每个 `Up`/`Down` 语句独立提交。
3. **幂等 Up**：表用 `IF NOT EXISTS`；`Down` 用 `DROP TABLE IF EXISTS`。
4. **Down 谨慎**：MergeTree 等引擎上 `DROP TABLE` 会立即删除数据，生产环境通常只前滚；仅在开发/测试需要回滚时编写 `Down`。
5. **DDL 与 DML 分离**：与主库相同，表结构变更与数据初始化分文件、分版本号；分析表通常无 seed，批量写入由 repository 在运行时完成。
6. **引擎与排序键**：在 SQL 中显式声明 `ENGINE`、`PARTITION BY`、`ORDER BY` 等，与查询模式对齐（例如按 `created_at` 分区）。
7. **禁止重复 DDL**：不要在 `support-files/`、`apps` 初始化逻辑或 `repository/analytics` 中复制建表语句。

### 新增分析表工作流

按以下顺序落地，避免列名或类型漂移：

1. **Model**：在 `internal/model/analytics/` 定义 struct，`gorm:"column:..."` 与 DDL 列名一一对应；实现 `TableName()`，批量写入表可提供 `InsertColumns()` / `BatchInsertSQL()`。
2. **Goose SQL**：在 `internal/infra/persistence/migrator/goose/clickhouse/` 新增递增版本文件（格式同主库，如 `YYYYMMDDNNNN_create_xxx.sql`），编写 `-- +goose Up` / `-- +goose Down`。
3. **Repository**：在 `internal/repository/analytics/` 实现 `BatchInsert*`（`db.ChConn` 一次 `PrepareBatch` + 多行 `Append` + 一次 `Send`）与查询（`db.ChDB`）；连接未初始化时返回明确错误，**不要**在 handler 写 SQL，**不要**在 repository 内维护 channel/goroutine。
4. **Apps**：在 `internal/apps/<domain>/` 编排采集与入队；高频写入通过 `internal/infra/persistence/batchwriter` 各域独立实例异步 flush（详见 `clickhouse-batchwriter` 技能），`FlushFunc` 只调 repository `BatchInsert*`；管理端统计 API 只读 repository，不触达 DDL。

### ClickHouse 验证

至少运行：

```bash
go test ./internal/infra/persistence/migrator
go test ./internal/repository/analytics
make code-check
```

验证重点：

- goose 能在空 ClickHouse 实例上完整执行 `Up`。
- `internal/model/analytics` 列名、类型与 goose SQL 一致。
- repository 读写路径不依赖 handler 内联 SQL。
- `clickhouse.enabled: false` 时启动不报错、不执行迁移。
