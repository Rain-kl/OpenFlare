# 日志数据库解耦（ClickHouse 可选化）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让日志/分析存储从 ClickHouse 解耦——新增 `internal/repository/logstore` 抽象（PG/SQLite 用 GORM、CH 用现有原生优化），ClickHouse 变为可选；提供「切换日志数据库」迁移任务与按库保留时间配置。

**Architecture:** repository 层导出接口 + 配置驱动 provider（`log_database` 系统配置决定激活实现）；apps 只面向 `logstore`/`repository` 公开函数，import-lint 测试强制约束；CH 实现包住现有 `analyticsrepo`（零性能损耗）；PG/SQLite 共用一套 GORM 实现（方言 SQL 拆 `dialect_*` 小文件）。

**Tech Stack:** Go 1.25+、GORM、PostgreSQL/SQLite（主库 goose 双方言）、ClickHouse（原生 driver + 单方言 goose）、Asynq 任务框架、Next.js/TypeScript/shadcn。

## Global Constraints

- 模块：`github.com/Rain-kl/Wavelet`；Go 1.25.7。
- 分层：`apps → repository → model`；`model` 禁止 import `repository`/`db`；`pkg/util/` 禁止 Gin/GORM/sessions。
- 路由仅注册于 `internal/router/router.go`；`Serve()` 禁止进程级初始化。
- 迁移：PG/SQLite 双方言同版本号 goose SQL（`internal/infra/persistence/migrator/goose/{postgres,sqlite}`）；CH 单方言（`goose/clickhouse`）；禁止 GORM AutoMigrate（**生产**；单测可用 sqlite AutoMigrate 建测试表）。
- 任务/推送注册：`bootstrap.RegisterTasks()` 等显式装配，禁止 `init()` 注册跨模块集成。
- API 错误：`response.Abort*` + `ErrorHandlerMiddleware`；禁止 Handler 直接 `c.JSON(..., response.Err(...))`。
- 系统配置：key 常量在 `internal/model/system_configs.go`；值存字符串；`type` ∈ {`system`,`business`}；`visibility` 0/1；goose 双方言 seed。
- 前端：shadcn `variant` + CSS 变量；页面根 `w-full`；标题 `h1 text-2xl font-semibold tracking-tight`；service 继承 `BaseService`，回调用箭头函数。
- 日志库合法状态：`log_database` ∈ {`postgres`,`sqlite`,`clickhouse`}，且 `postgres` 仅当 `database.enabled`、`sqlite` 仅当 `!database.enabled`、`clickhouse` 仅当 `clickhouse.enabled`。
- 完成标准：`go test ./...`、`make swagger`（API 变更时）、`make code-check`、`make format`；goose 三套空库 Up 全量通过。

---

## 里程碑与文件总览

| 文件 | 职责 |
|---|---|
| `internal/model/analytics/filter.go`（新） | 从 analyticsrepo 迁入的过滤/结果 DTO（纯数据） |
| `internal/model/system_configs.go` | 新增 `ConfigKeyLogDatabase`、`ConfigKeyLogDBMigration`、`ConfigKeyLogRetentionDaysPostgres/SQLite/ClickHouse` |
| `internal/repository/logstore/logstore.go`（新） | 导出接口 + `Store` 结构体 + `ErrMigrating` |
| `internal/repository/logstore/provider.go`（新） | `Init(ctx)`/`Active(ctx)`/`Migrating(ctx)`/`Reload`/测试注入 |
| `internal/repository/logstore/postgres_store.go`（新） | GORM 实现（PG/SQLite 共用） |
| `internal/repository/logstore/dialect_postgres.go`、`dialect_sqlite.go`（新） | 方言 SQL 片段 |
| `internal/repository/logstore/clickhouse_store.go`（新） | CH 实现（委托 analyticsrepo） |
| `internal/repository/logstore/hooks.go`（新） | `AccessLogInsertHooks`/`ObservabilityInsertHooks` 注册表（从 repository 迁入） |
| `internal/repository/logstore/imports_test.go`（新） | import-lint 测试 |
| `internal/repository/openflare_access_log_store.go`、`openflare_observability_store.go` | 删除（被 logstore 吸收） |
| `internal/repository/openflare_access_log.go`、`openflare_observability.go` | 改为一行委托 logstore |
| `internal/apps/risk_control/logics.go`、`internal/apps/openflare/chwriter/writer.go` | flush func 与入口改为 logstore；冻结检查 |
| `internal/apps/openflare/tasks/database_cleanup.go` | 清理逻辑迁入 `system_cleanup`；任务下线 |
| `internal/apps/admin/logs/routers.go`、`internal/apps/admin/status/clickhouse.go` | 改走 logstore；状态端点改造 |
| `internal/apps/upload/task/cleanup.go` | 新增日志清理步骤 |
| `internal/apps/openflare/async_tasks.go`、`internal/infra/task/handlers/register.go` | 注册「切换日志数据库」任务；下线清理任务 |
| `internal/apps/openflare/tasks/log_db_switch.go`（新） | 迁移任务 Handler |
| `internal/platform/bootstrap/bootstrap.go` | 启动校验 + logstore 初始化 |
| `internal/infra/config/model.go` | （无新启动配置；校验仅用现有字段） |
| goose：`postgres/20260808NNNN_create_log_tables.sql`、`sqlite/20260808NNNN_create_log_tables.sql` | 6 张原始日志表（PG 分区） |
| goose：`postgres/20260808NNNN_log_retention_configs.sql`、`sqlite/...` | 保留配置 + 旧 key 下线 |
| goose：`postgres/20260808NNNN_drop_database_cleanup_schedule.sql`、`sqlite/...` | 下线 `of_database_auto_cleanup` schedule |
| `internal/apps/admin/system_config/routers.go`、`internal/apps/openflare/option/validate.go` | `log_database`/`log_db_migration` key 保护 |
| `frontend/...` | 任务管理页日志库状态、业务配置「日志保留时间」分组 |
| `docs/changelog/index.md` | `[Unreleased]` 中文条目 |

---

## M1：抽象层与主库日志读写

### Task 1: DTO 类型迁入 model/analytics

**Files:**
- Create: `internal/model/analytics/filter.go`
- Modify: `internal/repository/analytics/access_log.go`、`node_access_log.go`、`node_observability.go`、`access_log_stats.go`、`node_access_log_stats.go`、`node_observability_delete.go` 等（删除本地类型定义，改 import model/analytics）
- Test: `internal/model/analytics/filter_test.go`

**Interfaces:**
- Consumes: 现有 analyticsrepo 包内类型定义位置。
- Produces: `analyticsmodel.AccessLogFilter`、`analyticsmodel.NodeAccessLogFilter`、`analyticsmodel.NodeObservabilityFilter`、`analyticsmodel.DailyTrend`、`analyticsmodel.BrowserShare`、`analyticsmodel.TopUser`、`analyticsmodel.NodeAccessLogRegionCount`、`analyticsmodel.NodeAccessLogTrafficSummary`、`analyticsmodel.NodeAccessLogValueCount`、`analyticsmodel.NodeAccessLogNodeAggregate`（字段逐一从 analyticsrepo 原定义复制）。

- [ ] **Step 1: 在 `internal/model/analytics/filter.go` 定义迁移类型**

```go
// Package analytics 定义分析域模型与查询 DTO（纯数据，无 IO）。
package analytics

import "time"

// AccessLogFilter 用户访问日志查询条件。
type AccessLogFilter struct {
    UserID   uint64
    Path     string
    Method   string
    IP       string
    Status   int32
    Since    time.Time
    Until    time.Time
    Page     int
    PageSize int
}

// NodeAccessLogFilter 节点访问日志查询条件。
type NodeAccessLogFilter struct {
    NodeID     string
    RemoteAddr string
    Host       string
    Hosts      []string
    Path       string
    Since      time.Time
    Until      time.Time
    Page       int
    PageSize   int
    SortBy     string
    SortOrder  string
}

// NodeObservabilityFilter 可观测查询条件。
type NodeObservabilityFilter struct {
    NodeID string
    Since  time.Time
    Limit  int
}

// DailyTrend 每日访问趋势。
type DailyTrend struct {
    Date string
    Cnt  uint64
}

// BrowserShare 浏览器占比。
type BrowserShare struct {
    Browser string
    Cnt     uint64
}

// TopUser 活跃用户排行。
type TopUser struct {
    UserID uint64
    Cnt    uint64
}

// NodeAccessLogRegionCount 地区访问计数。
type NodeAccessLogRegionCount struct {
    Region string
    Count  uint64
}

// NodeAccessLogTrafficSummary 流量汇总。
type NodeAccessLogTrafficSummary struct {
    RequestCount  uint64
    ErrorCount    uint64
    UniqueIPCount uint64
    BytesSent     uint64
    RequestLength uint64
    NodeCount     uint64
}

// NodeAccessLogValueCount 维度值计数。
type NodeAccessLogValueCount struct {
    Value string
    Count uint64
}

// NodeAccessLogNodeAggregate 按节点聚合。
type NodeAccessLogNodeAggregate struct {
    NodeID        string
    RequestCount  uint64
    ErrorCount    uint64
    UniqueIPCount uint64
}
```

> 注意：以上字段必须与 `internal/repository/analytics/` 中同名类型**逐字段一致**（比对 `access_log.go`、`node_access_log.go`、`node_access_log_stats.go`、`access_log_stats.go`）。若原类型字段与这里不同，以原类型为准修改本文件，保持语义不变。

- [ ] **Step 2: 让 analyticsrepo 使用新类型**——在每个原类型定义处删除定义，替换为类型别名，保证包内调用点零改动：

```go
// internal/repository/analytics/access_log.go 顶部
import analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"

type AccessLogFilter = analyticsmodel.AccessLogFilter
```

对 `NodeAccessLogFilter`、`NodeObservabilityFilter`、`DailyTrend`、`BrowserShare`、`TopUser`、`NodeAccessLogRegionCount`、`NodeAccessLogTrafficSummary`、`NodeAccessLogValueCount`、`NodeAccessLogNodeAggregate`、`ClickHouseOperationalStats`（及 `ClickHouseOperationalStats` 的字段结构体，含 `BatchWriters []batchwriter.Stats`）同样处理（原类型定义删除，替换为别名）。`ClickHouseOperationalStats` 迁入 `model/analytics` 后，logstore 状态接口可直接引用，CH 实现仍由 analyticsrepo 填充。

- [ ] **Step 3: 编译验证** 运行 `go build ./internal/...`，确认无重定义/未使用错误。
- [ ] **Step 4: 提交** `git add internal/model/analytics/filter.go internal/repository/analytics/ && git commit -m "refactor(analytics): move filter/result DTOs to model/analytics"`

### Task 2: logstore 接口与 provider 骨架

**Files:**
- Create: `internal/repository/logstore/logstore.go`
- Create: `internal/repository/logstore/provider.go`
- Create: `internal/repository/logstore/provider_test.go`

**Interfaces:**
- Consumes: `analyticsmodel.*` DTO（Task 1）、`model.ConfigKeyLogDatabase`/`ConfigKeyLogDBMigration`（Task 8 定义，本任务先用字符串常量占位并加注释）、`db.DB(ctx)`（`internal/infra/persistence` 的 GORM 句柄）、`repository.GetSystemConfigByKey`。
- Produces: 接口 `AccessLogStore`/`ObservabilityStore`/`UserAccessLogStore`、结构体 `Store`、`ErrMigrating`、`Init(ctx)`/`Active(ctx)`/`Migrating(ctx)`/`ResetForTest`。

- [ ] **Step 1: 写接口与 `Store` 结构体（logstore.go）**

```go
// Package logstore 提供日志/分析存储抽象：上层只面向本包接口，
// 禁止直接 import internal/repository/analytics 或触碰 db.ChConn/db.ChDB。
package logstore

import (
    "context"
    "errors"
    "time"

    "github.com/Rain-kl/Wavelet/internal/model"
    analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

// ErrMigrating 表示日志数据库正在迁移，当前禁止写入。
var ErrMigrating = errors.New("log database is migrating, writes are disabled")

// AccessLogStore 节点访问日志（of_node_access_logs）。
type AccessLogStore interface {
    // InsertBatch 为写入入口：冻结检查 + 经 hook 入队（异步），不直接落库。
    InsertBatch(ctx context.Context, records []*model.OpenFlareAccessLog) error
    // BatchInsertNodeAccessLogs 为 batchwriter flush 目标：直接批量写入当前存储。
    BatchInsertNodeAccessLogs(ctx context.Context, rows []analyticsmodel.NodeAccessLog) error

    List(ctx context.Context, query model.OpenFlareAccessLogQuery) ([]*model.OpenFlareAccessLog, error)
    Count(ctx context.Context, query model.OpenFlareAccessLogQuery) (int64, int64, int64, error)
    RegionCounts(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareAccessLogRegionCount, error)
    BucketAggregates(ctx context.Context, filter model.OpenFlareAccessLogQuery, bucketSeconds int64) ([]analyticsmodel.NodeAccessLogBucketAggregate, error)
    CountBuckets(ctx context.Context, filter model.OpenFlareAccessLogQuery, bucketSeconds int64) (int64, error)
    BucketDimensions(ctx context.Context, filter model.OpenFlareAccessLogQuery, column string, bucketSeconds int64) ([]analyticsmodel.NodeAccessLogBucketDimension, error)
    IPAggregates(ctx context.Context, filter model.OpenFlareAccessLogQuery, exactRemoteAddr bool) ([]analyticsmodel.NodeAccessLogIPAggregate, error)
    IPSummaries(ctx context.Context, filter model.OpenFlareAccessLogQuery, recentSince time.Time) ([]analyticsmodel.NodeAccessLogIPSummary, error)
    CountIPSummaries(ctx context.Context, filter model.OpenFlareAccessLogQuery) (int64, error)
    WAFIPAggregates(ctx context.Context, filter model.OpenFlareAccessLogQuery) ([]analyticsmodel.NodeAccessLogWAFIPAggregate, error)
    IPTrend(ctx context.Context, filter model.OpenFlareAccessLogQuery, bucketSeconds int64) ([]analyticsmodel.NodeAccessLogIPTrend, error)
    TrafficSummary(ctx context.Context, filter model.OpenFlareAccessLogQuery) (model.OpenFlareAccessLogTrafficSummary, error)
    ValueCounts(ctx context.Context, filter model.OpenFlareAccessLogQuery, column string, limit int) ([]model.OpenFlareAccessLogValueCount, error)
    NodeAggregates(ctx context.Context, filter model.OpenFlareAccessLogQuery) ([]model.OpenFlareAccessLogNodeAggregate, error)
    DeleteAll(ctx context.Context) (int64, error)
    DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error)
    DeleteByNodeBefore(ctx context.Context, nodeID string, before time.Time) (int64, error)
    // ListForMigration 按 id 升序分页读取（迁移复制用）。
    ListForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeAccessLog, error)
}

// ObservabilityStore 可观测 4 表（metric snapshots / edge health / frps / frpc）。
type ObservabilityStore interface {
    InsertMetricSnapshot(ctx context.Context, record *model.OpenFlareMetricSnapshot) error
    ListMetricSnapshots(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareMetricSnapshot, error)
    DeleteAllMetricSnapshots(ctx context.Context) (int64, error)
    DeleteMetricSnapshotsBefore(ctx context.Context, cutoff time.Time) (int64, error)
    BatchInsertNodeMetricSnapshots(ctx context.Context, rows []analyticsmodel.NodeMetricSnapshot) error

    InsertEdgeHealth(ctx context.Context, record *model.OpenFlareEdgeHealth) error
    ListEdgeHealth(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareEdgeHealth, error)
    DeleteAllEdgeHealth(ctx context.Context) (int64, error)
    DeleteEdgeHealthBefore(ctx context.Context, cutoff time.Time) (int64, error)
    BatchInsertNodeEdgeHealth(ctx context.Context, rows []analyticsmodel.NodeEdgeHealth) error

    InsertNodeObservationFrps(ctx context.Context, record *model.OpenFlareNodeObservationFrps) error
    ListNodeObservationFrps(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareNodeObservationFrps, error)
    DeleteAllNodeObservationFrps(ctx context.Context) (int64, error)
    DeleteNodeObservationFrpsBefore(ctx context.Context, cutoff time.Time) (int64, error)
    BatchInsertNodeObsFrps(ctx context.Context, rows []analyticsmodel.NodeObsFrps) error

    InsertNodeObservationFrpc(ctx context.Context, record *model.OpenFlareNodeObservationFrpc) error
    ListNodeObservationFrpc(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareNodeObservationFrpc, error)
    DeleteAllNodeObservationFrpc(ctx context.Context) (int64, error)
    DeleteNodeObservationFrpcBefore(ctx context.Context, cutoff time.Time) (int64, error)
    BatchInsertNodeObsFrpc(ctx context.Context, rows []analyticsmodel.NodeObsFrpc) error

    // 迁移复制用：按 id 升序分页读取。
    ListMetricSnapshotsForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeMetricSnapshot, error)
    ListEdgeHealthForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeEdgeHealth, error)
    ListNodeObsFrpsForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeObsFrps, error)
    ListNodeObsFrpcForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.NodeObsFrpc, error)
}

// UserAccessLogStore 用户访问日志（w_user_access_logs）。
type UserAccessLogStore interface {
    BatchInsert(ctx context.Context, logs []analyticsmodel.UserAccessLog) error
    Count(ctx context.Context, filter analyticsmodel.AccessLogFilter) (uint64, error)
    List(ctx context.Context, filter analyticsmodel.AccessLogFilter, page, pageSize int) ([]analyticsmodel.UserAccessLog, uint64, error)
    GetDailyTrend(ctx context.Context, days int) ([]analyticsmodel.DailyTrend, error)
    GetBrowserDistribution(ctx context.Context, startTime time.Time) ([]analyticsmodel.BrowserShare, error)
    GetTopActiveUsers(ctx context.Context, startTime time.Time, limit int) ([]analyticsmodel.TopUser, error)
}

// StatusStore 日志库状态（供管理端状态端点）。
type StatusStore interface {
    ActiveDatabase(ctx context.Context) (string, error)
    ClickHouseOperationalStats(ctx context.Context) (*analyticsmodel.ClickHouseOperationalStats, error) // 仅 CH 激活时非 nil
}

// Store 聚合当前生效日志库的全部域存储。
type Store struct {
    AccessLogs     AccessLogStore
    Observability  ObservabilityStore
    UserAccessLogs UserAccessLogStore
    Status         StatusStore
}
```

- [ ] **Step 2: 写 provider（provider.go）**

```go
package logstore

import (
    "context"
    "errors"
    "fmt"
    "sync"

    "github.com/Rain-kl/Wavelet/internal/infra/config"
    db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
)

// logDatabaseKey / logMigrationKey 暂用字符串，Task 8 换为 model.ConfigKey*。
const (
    logDatabaseKey  = "log_database"
    logMigrationKey = "log_db_migration"
)

// ConfigReader 读取系统配置字符串值，由 bootstrap 注入（避免 logstore ↔ repository 循环依赖）。
type ConfigReader func(ctx context.Context, key string) (string, error)

var (
    configReader ConfigReader

    storeMu  sync.RWMutex
    active   *Store
    activeDB string
)

// SetConfigReader 注入系统配置读取函数（bootstrap 调用，测试可注入内存实现）。
func SetConfigReader(fn ConfigReader) { configReader = fn }

func getConfig(ctx context.Context, key string) (string, error) {
    if configReader == nil {
        return "", errors.New("logstore: config reader not wired")
    }
    return configReader(ctx, key)
}

// Active 返回当前生效的日志库 Store。按 log_database 系统配置惰性解析并缓存，
// 配置更新（含迁移任务翻转）后自动重建。
func Active(ctx context.Context) (*Store, error) {
    current, err := resolveDatabase(ctx)
    if err != nil {
        return nil, err
    }
    storeMu.RLock()
    if active != nil && activeDB == current {
        s := active
        storeMu.RUnlock()
        return s, nil
    }
    storeMu.RUnlock()

    storeMu.Lock()
    defer storeMu.Unlock()
    if active != nil && activeDB == current {
        return active, nil
    }
    s, err := buildStore(ctx, current)
    if err != nil {
        return nil, err
    }
    active = s
    activeDB = current
    return s, nil
}

// Migrating 返回日志库是否处于迁移冻结状态。
func Migrating(ctx context.Context) bool {
    v, err := getConfig(ctx, logMigrationKey)
    if err != nil {
        return false
    }
    return v == "migrating"
}

// Init 在 bootstrap 阶段预热一次激活 store（幂等，失败不致命——首次使用时再解析）。
func Init(ctx context.Context) {
    _, _ = Active(ctx)
}

// ResetForTest 清空缓存的激活 store 与 reader，便于测试注入。
func ResetForTest() {
    storeMu.Lock()
    active = nil
    activeDB = ""
    storeMu.Unlock()
}

// Build 直接按目标构造 store（迁移任务复制到目标库时使用，不经 Active 缓存）。
func Build(ctx context.Context, database string) (*Store, error) {
    return buildStore(ctx, database)
}

// ActiveDatabase 返回当前日志主库名（postgres|sqlite|clickhouse）。
func ActiveDatabase(ctx context.Context) (string, error) {
    return resolveDatabase(ctx)
}

// resolveDatabase 读取 log_database，缺失时按启动规则 seed 并返回。
func resolveDatabase(ctx context.Context) (string, error) {
    v, err := getConfig(ctx, logDatabaseKey)
    if err == nil && v != "" {
        return v, nil
    }
    // 首次启动 seed：CH 启用 → clickhouse；否则随主库。
    defaultDB := "sqlite"
    if config.Config.Database.Enabled {
        defaultDB = "postgres"
    }
    if config.Config.ClickHouse.Enabled {
        defaultDB = "clickhouse"
    }
    return defaultDB, nil
}

// buildStore 按目标构造实现（Task 3-5 提供构造函数）。
func buildStore(ctx context.Context, database string) (*Store, error) {
    switch database {
    case "clickhouse":
        ch := newClickHouseStore()
        return &Store{AccessLogs: ch, Observability: ch, UserAccessLogs: ch, Status: ch}, nil
    case "postgres", "sqlite":
        g := newGormStore(db.DB(ctx))
        return &Store{AccessLogs: g, Observability: g, UserAccessLogs: g, Status: g}, nil
    default:
        return nil, fmt.Errorf("unsupported log database: %s", database)
    }
}
```

（`db.DB(ctx)` 返回 `*gorm.DB`，见 `internal/infra/persistence/postgres.go`；`newGormStore`/`newClickHouseStore` 在 Task 3-5 实现。）

- [ ] **Step 3: 写 provider 单测（provider_test.go）**——用 `SetStoreForTest` 注入 fake 验证 `Active` 缓存与切换：

```go
package logstore

import (
    "context"
    "testing"
)

func TestMigratingReadsConfig(t *testing.T) {
    ResetForTest()
    SetConfigReader(func(_ context.Context, key string) (string, error) {
        if key == logMigrationKey {
            return "migrating", nil
        }
        return "", nil
    })
    if !Migrating(context.Background()) {
        t.Fatal("Migrating() = false, want true when key=migrating")
    }
    SetConfigReader(func(_ context.Context, key string) (string, error) {
        return "", nil
    })
    if Migrating(context.Background()) {
        t.Fatal("Migrating() = true, want false when key empty")
    }
}

func TestResolveDatabaseDefaults(t *testing.T) {
    ResetForTest()
    // 配置缺失时按主库规则 seed（config.Config 默认值由既有测试基建决定）。
    got, err := resolveDatabase(context.Background())
    if err != nil {
        t.Fatalf("resolveDatabase: %v", err)
    }
    if got != "postgres" && got != "sqlite" && got != "clickhouse" {
        t.Fatalf("unexpected default log database: %s", got)
    }
}
```

- [ ] **Step 4: 运行测试** `go test ./internal/repository/logstore/` 期望 PASS。
- [ ] **Step 5: 提交** `git add internal/repository/logstore/ && git commit -m "feat(logstore): add log store interfaces and provider skeleton"`

### Task 3: GORM 实现——节点访问日志（AccessLogStore）

**Files:**
- Create: `internal/repository/logstore/postgres_store.go`
- Create: `internal/repository/logstore/dialect_postgres.go`
- Create: `internal/repository/logstore/dialect_sqlite.go`
- Create: `internal/repository/logstore/postgres_store_test.go`

**Interfaces:**
- Consumes: `db.DB(ctx)`、`analyticsmodel.*`、`model.OpenFlareAccessLog*`、`hooks` 注册表（Task 5 提供 `QueueNodeAccessLogs`）。
- Produces: `newGormStore(db *gorm.DB) *gormLogStore`（实现 `AccessLogStore`/`ObservabilityStore`/`UserAccessLogStore`）。

- [ ] **Step 1: 写 dialect 小文件**

`dialect_postgres.go`：
```go
package logstore

import "gorm.io/gorm"

// timeBucketSQL 返回 PG 时间分桶表达式（epoch 秒 -> 分桶起点）。
func timeBucketSQL(column string, bucketSeconds int64) string {
    return "to_timestamp(floor(extract(epoch from " + column + ")/" + itoa(bucketSeconds) + ")*" + itoa(bucketSeconds) + ")"
}

// gormDBForWrite 返回写句柄（PG/SQLite 相同）。
func gormDBForWrite(db *gorm.DB) *gorm.DB { return db }
```

`dialect_sqlite.go`：
```go
package logstore

import (
    "strconv"

    "gorm.io/gorm"
)

func timeBucketSQL(column string, bucketSeconds int64) string {
    return "(floor(unixepoch(" + column + ")/" + strconv.FormatInt(bucketSeconds, 10) + ")*" + strconv.FormatInt(bucketSeconds, 10) + ")"
}

func gormDBForWrite(db *gorm.DB) *gorm.DB { return db }
```

> 若需要精确到毫秒的分桶（现有 CH 用秒级分桶即可），以现有 `node_access_log_stats.go` 的 bucket 语义为准，两种方言输出同一语义。

- [ ] **Step 2: 写 `postgres_store.go`（节点访问日志部分）**

```go
package logstore

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/Rain-kl/Wavelet/internal/model"
    analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
    "gorm.io/gorm"
)

// gormLogStore 是 PG/SQLite 共用的 GORM 日志存储实现。
type gormLogStore struct {
    db *gorm.DB
}

func newGormStore(db *gorm.DB) *gormLogStore { return &gormLogStore{db: db} }

// ensureWritable 冻结期拒绝写入。
func (s *gormLogStore) ensureWritable(ctx context.Context) error {
    if Migrating(ctx) {
        return ErrMigrating
    }
    return nil
}

// InsertBatch 节点访问日志写入入口：冻结检查后经 hook 入队（异步），与现状一致。
func (s *gormLogStore) InsertBatch(ctx context.Context, records []*model.OpenFlareAccessLog) error {
    if err := s.ensureWritable(ctx); err != nil {
        return err
    }
    rows := make([]analyticsmodel.NodeAccessLog, 0, len(records))
    for _, r := range records {
        if r == nil {
            continue
        }
        rows = append(rows, toAnalyticsNodeAccessLog(r))
    }
    if h := currentAccessLogHooks().QueueNodeAccessLogs; h != nil {
        h(rows)
    }
    return nil
}

// BatchInsertNodeAccessLogs 是 batchwriter flush 目标：GORM 分批落库。
func (s *gormLogStore) BatchInsertNodeAccessLogs(ctx context.Context, rows []analyticsmodel.NodeAccessLog) error {
    if len(rows) == 0 {
        return nil
    }
    if err := s.ensureWritable(ctx); err != nil {
        return err
    }
    return s.db.WithContext(ctx).CreateInBatches(rows, 500).Error
}

func (s *gormLogStore) List(ctx context.Context, query model.OpenFlareAccessLogQuery) ([]*model.OpenFlareAccessLog, error) {
    f := toNodeAccessLogFilter(query)
    var rows []analyticsmodel.NodeAccessLog
    q := s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{})
    if f.Since.IsZero() == false {
        q = q.Where("logged_at >= ?", f.Since)
    }
    if f.Until.IsZero() == false {
        q = q.Where("logged_at <= ?", f.Until)
    }
    if f.NodeID != "" {
        q = q.Where("node_id = ?", f.NodeID)
    }
    if f.RemoteAddr != "" {
        q = q.Where("remote_addr = ?", f.RemoteAddr)
    }
    if len(f.Hosts) > 0 {
        q = q.Where("host IN ?", f.Hosts)
    }
    if f.Host != "" {
        q = q.Where("host = ?", f.Host)
    }
    if f.Path != "" {
        q = q.Where("path = ?", f.Path)
    }
    order := "logged_at DESC, id DESC"
    if f.SortOrder == "asc" {
        order = "logged_at ASC, id ASC"
    }
    if err := q.Order(order).Limit(limitOr(f.PageSize, 100)).Offset(offsetOf(f.Page, f.PageSize)).Find(&rows).Error; err != nil {
        return nil, err
    }
    return fromAnalyticsNodeAccessLogs(rows), nil
}

func (s *gormLogStore) Count(ctx context.Context, query model.OpenFlareAccessLogQuery) (int64, int64, int64, error) {
    f := toNodeAccessLogFilter(query)
    var total, uniqIP, bytesSent int64
    q := s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{})
    if !f.Since.IsZero() {
        q = q.Where("logged_at >= ?", f.Since)
    }
    if !f.Until.IsZero() {
        q = q.Where("logged_at <= ?", f.Until)
    }
    if f.NodeID != "" {
        q = q.Where("node_id = ?", f.NodeID)
    }
    if f.RemoteAddr != "" {
        q = q.Where("remote_addr = ?", f.RemoteAddr)
    }
    if len(f.Hosts) > 0 {
        q = q.Where("host IN ?", f.Hosts)
    }
    if f.Host != "" {
        q = q.Where("host = ?", f.Host)
    }
    if f.Path != "" {
        q = q.Where("path = ?", f.Path)
    }
    if err := q.Count(&total).Error; err != nil {
        return 0, 0, 0, err
    }
    if err := q.Distinct("remote_addr").Count(&uniqIP).Error; err != nil {
        return 0, 0, 0, err
    }
    if err := q.Select("COALESCE(SUM(bytes_sent),0)").Scan(&bytesSent).Error; err != nil {
        return 0, 0, 0, err
    }
    return total, uniqIP, bytesSent, nil
}

func (s *gormLogStore) TrafficSummary(ctx context.Context, query model.OpenFlareAccessLogQuery) (model.OpenFlareAccessLogTrafficSummary, error) {
    f := toNodeAccessLogFilter(query)
    var out struct {
        RequestCount  int64
        ErrorCount    int64
        UniqueIPCount int64
        BytesSent     int64
        RequestLength int64
        NodeCount     int64
    }
    q := s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{})
    if !f.Since.IsZero() {
        q = q.Where("logged_at >= ?", f.Since)
    }
    if !f.Until.IsZero() {
        q = q.Where("logged_at <= ?", f.Until)
    }
    if f.NodeID != "" {
        q = q.Where("node_id = ?", f.NodeID)
    }
    if f.Host != "" {
        q = q.Where("host = ?", f.Host)
    }
    err := q.Select(`
        COUNT(*) AS request_count,
        COUNT(*) FILTER (WHERE status_code >= 500) AS error_count,
        COUNT(DISTINCT remote_addr) AS unique_ip_count,
        COALESCE(SUM(bytes_sent),0) AS bytes_sent,
        COALESCE(SUM(request_length),0) AS request_length,
        COUNT(DISTINCT node_id) AS node_count`).Scan(&out).Error
    if err != nil {
        return model.OpenFlareAccessLogTrafficSummary{}, err
    }
    return model.OpenFlareAccessLogTrafficSummary{
        RequestCount:  out.RequestCount,
        ErrorCount:    out.ErrorCount,
        UniqueIPCount: out.UniqueIPCount,
        BytesSent:     out.BytesSent,
        RequestLength: out.RequestLength,
        NodeCount:     out.NodeCount,
    }, nil
}

func (s *gormLogStore) ValueCounts(ctx context.Context, query model.OpenFlareAccessLogQuery, column string, limit int) ([]model.OpenFlareAccessLogValueCount, error) {
    col, ok := nodeAccessLogValueColumn(column)
    if !ok {
        return nil, fmt.Errorf("unsupported value count column: %s", column)
    }
    f := toNodeAccessLogFilter(query)
    q := s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}).
        Select(col+" AS value, COUNT(*) AS count")
    if !f.Since.IsZero() {
        q = q.Where("logged_at >= ?", f.Since)
    }
    if !f.Until.IsZero() {
        q = q.Where("logged_at <= ?", f.Until)
    }
    if f.NodeID != "" {
        q = q.Where("node_id = ?", f.NodeID)
    }
    if f.Host != "" {
        q = q.Where("host = ?", f.Host)
    }
    type row struct {
        Value string
        Count int64
    }
    var rows []row
    if err := q.Group(col).Order("count DESC").Limit(limitOr(limit, 10)).Scan(&rows).Error; err != nil {
        return nil, err
    }
    out := make([]model.OpenFlareAccessLogValueCount, len(rows))
    for i, r := range rows {
        out[i] = model.OpenFlareAccessLogValueCount{Value: r.Value, Count: r.Count}
    }
    return out, nil
}

func (s *gormLogStore) NodeAggregates(ctx context.Context, query model.OpenFlareAccessLogQuery) ([]model.OpenFlareAccessLogNodeAggregate, error) {
    f := toNodeAccessLogFilter(query)
    type row struct {
        NodeID        string
        RequestCount  int64
        ErrorCount    int64
        UniqueIPCount int64
    }
    var rows []row
    q := s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}).
        Select("node_id, COUNT(*) AS request_count, COUNT(*) FILTER (WHERE status_code >= 500) AS error_count, COUNT(DISTINCT remote_addr) AS unique_ip_count")
    if !f.Since.IsZero() {
        q = q.Where("logged_at >= ?", f.Since)
    }
    if !f.Until.IsZero() {
        q = q.Where("logged_at <= ?", f.Until)
    }
    if f.NodeID != "" {
        q = q.Where("node_id = ?", f.NodeID)
    }
    if f.Host != "" {
        q = q.Where("host = ?", f.Host)
    }
    if err := q.Group("node_id").Order("request_count DESC").Scan(&rows).Error; err != nil {
        return nil, err
    }
    out := make([]model.OpenFlareAccessLogNodeAggregate, len(rows))
    for i, r := range rows {
        out[i] = model.OpenFlareAccessLogNodeAggregate{NodeID: r.NodeID, RequestCount: r.RequestCount, ErrorCount: r.ErrorCount, UniqueIPCount: r.UniqueIPCount}
    }
    return out, nil
}

func (s *gormLogStore) RegionCounts(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareAccessLogRegionCount, error) {
    type row struct {
        Region string
        Count  int64
    }
    var rows []row
    q := s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}).
        Select("region, COUNT(*) AS count").
        Where("node_id = ? AND region <> '' AND logged_at >= ?", nodeID, since)
    if err := q.Group("region").Order("count DESC").Limit(limitOr(limit, 10)).Scan(&rows).Error; err != nil {
        return nil, err
    }
    out := make([]*model.OpenFlareAccessLogRegionCount, len(rows))
    for i, r := range rows {
        out[i] = &model.OpenFlareAccessLogRegionCount{Region: r.Region, Count: r.Count}
    }
    return out, nil
}

func (s *gormLogStore) BucketAggregates(ctx context.Context, query model.OpenFlareAccessLogQuery, bucketSeconds int64) ([]analyticsmodel.NodeAccessLogBucketAggregate, error) {
    f := toNodeAccessLogFilter(query)
    expr := timeBucketSQL("logged_at", bucketSeconds)
    type row struct {
        Bucket       int64
        RequestCount int64
        ErrorCount   int64
    }
    var rows []row
    q := s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}).
        Select(expr+" AS bucket, COUNT(*) AS request_count, COUNT(*) FILTER (WHERE status_code >= 500) AS error_count")
    if !f.Since.IsZero() {
        q = q.Where("logged_at >= ?", f.Since)
    }
    if !f.Until.IsZero() {
        q = q.Where("logged_at <= ?", f.Until)
    }
    if f.NodeID != "" {
        q = q.Where("node_id = ?", f.NodeID)
    }
    if f.Host != "" {
        q = q.Where("host = ?", f.Host)
    }
    if err := q.Group(expr).Order("bucket ASC").Scan(&rows).Error; err != nil {
        return nil, err
    }
    out := make([]analyticsmodel.NodeAccessLogBucketAggregate, len(rows))
    for i, r := range rows {
        out[i] = analyticsmodel.NodeAccessLogBucketAggregate{Bucket: r.Bucket, RequestCount: r.RequestCount, ErrorCount: r.ErrorCount}
    }
    return out, nil
}

func (s *gormLogStore) DeleteAll(ctx context.Context) (int64, error) {
    if err := s.ensureWritable(ctx); err != nil {
        return 0, err
    }
    res := s.db.WithContext(ctx).Where("1 = 1").Delete(&analyticsmodel.NodeAccessLog{})
    return res.RowsAffected, res.Error
}

func (s *gormLogStore) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
    if err := s.ensureWritable(ctx); err != nil {
        return 0, err
    }
    res := s.db.WithContext(ctx).Where("logged_at < ?", cutoff).Delete(&analyticsmodel.NodeAccessLog{})
    return res.RowsAffected, res.Error
}

func (s *gormLogStore) DeleteByNodeBefore(ctx context.Context, nodeID string, before time.Time) (int64, error) {
    if err := s.ensureWritable(ctx); err != nil {
        return 0, err
    }
    res := s.db.WithContext(ctx).Where("node_id = ? AND logged_at < ?", nodeID, before).Delete(&analyticsmodel.NodeAccessLog{})
    return res.RowsAffected, res.Error
}
```

- [ ] **Step 2b: 补齐 AccessLogStore 剩余聚合方法（必须全部实现 + 编译期断言）**

`gormLogStore` 必须实现 `AccessLogStore` 的**全部 20 个方法**（当前 Step 1 只含 13 个）。补齐：`CountBuckets`、`BucketDimensions`、`IPAggregates`、`IPSummaries`、`CountIPSummaries`、`WAFIPAggregates`、`IPTrend`。语义以 `internal/repository/analytics/node_access_log_stats.go`（及 `node_access_log.go` 中对应函数）为准，用 GORM/方言 SQL 等价实现：

- 时间分桶统一返回 epoch 秒整型：PG `(floor(extract(epoch from <col>)/<N>)*<N>)::bigint`；SQLite `(floor(unixepoch(<col>)/<N>)*<N>)`（修正 `timeBucketSQL`，保证 PG/SQLite 输出同为 int64 epoch，与 `BucketEpoch` 扫描类型一致）。
- `CountBuckets`：`SELECT COUNT(*) FROM (SELECT 1 FROM t WHERE ... GROUP BY bucket) x`。
- `BucketDimensions`：`GROUP BY bucket, <column>` 返回维度计数。
- `IPAggregates`：按 remote_addr（或精确 remote_addr）聚合 request_count / error_count / unique host 等，字段对照 `NodeAccessLogIPAggregate`。
- `IPSummaries` / `CountIPSummaries`：按 IP 汇总近窗口（含最近活跃时间），字段对照 `NodeAccessLogIPSummary`。
- `WAFIPAggregates`：按 IP 聚合状态码分布，字段对照 `NodeAccessLogWAFIPAggregate`。
- `IPTrend`：按 IP × 时间桶聚合，字段对照 `NodeAccessLogIPTrend`。
- **过滤语义对齐 CH**（`node_access_log_filter.go`）：remote_addr/host/path 用 `LIKE trim(value)+'%'` 前缀匹配；hosts 用 `lower(trim(host)) IN (...)`；until 用开区间 `<`；node_id 先 trim。
- 文件底部加编译期断言：`var _ AccessLogStore = (*gormLogStore)(nil)`。
- 测试：`postgres_store_test.go` 至少覆盖 `CountBuckets`/`IPTrend`（sqlite 内存库写入若干行后断言分桶数量与趋势），其余方法以编译期断言 + 既有语义测试兜底。

- [ ] **Step 3: 写 helper（postgres_store.go 同文件底部）**

```go
func limitOr(v, def int) int {
    if v <= 0 {
        return def
    }
    return v
}

func offsetOf(page, pageSize int) int {
    if page < 1 {
        page = 1
    }
    if pageSize < 1 {
        pageSize = 20
    }
    return (page - 1) * pageSize
}

func nodeAccessLogValueColumn(column string) (string, bool) {
    switch column {
    case "remote_addr":
        return "remote_addr", true
    case "host":
        return "host", true
    case "path":
        return "path", true
    case "region":
        return "region", true
    case "status_code":
        return "status_code", true
    case "user_agent":
        return "user_agent", true
    case "cache_status":
        return "cache_status", true
    }
    return "", false
}
```

> `toAnalyticsNodeAccessLog`/`fromAnalyticsNodeAccessLogs`/`toNodeAccessLogFilter` 从 `internal/repository/openflare_access_log_store.go` 复制（含 math 边界保护逻辑）；Task 6 删除旧文件后这些 helper 不再冲突。

- [ ] **Step 4: 写单测（postgres_store_test.go，sqlite 内存库 + AutoMigrate）**

```go
package logstore

import (
    "context"
    "testing"
    "time"

    "github.com/glebarez/sqlite"
    "gorm.io/gorm"
    "gorm.io/gorm/logger"

    "github.com/Rain-kl/Wavelet/internal/model"
    analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

func newTestGormStore(t *testing.T) *gormLogStore {
    t.Helper()
    db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
    if err != nil {
        t.Fatalf("open sqlite: %v", err)
    }
    if err := db.AutoMigrate(&analyticsmodel.NodeAccessLog{}); err != nil {
        t.Fatalf("automigrate: %v", err)
    }
    return newGormStore(db)
}

func TestGormBatchInsertAndCount(t *testing.T) {
    ResetForTest()
    SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
    s := newTestGormStore(t)
    now := time.Now()
    rows := []analyticsmodel.NodeAccessLog{
        {ID: 1, NodeID: "n1", LoggedAt: now, RemoteAddr: "1.1.1.1", StatusCode: 200, BytesSent: 100},
        {ID: 2, NodeID: "n1", LoggedAt: now, RemoteAddr: "2.2.2.2", StatusCode: 500, BytesSent: 200},
    }
    if err := s.BatchInsertNodeAccessLogs(context.Background(), rows); err != nil {
        t.Fatalf("insert: %v", err)
    }
    total, uniqIP, bytesSent, err := s.Count(context.Background(), model.OpenFlareAccessLogQuery{NodeID: "n1"})
    if err != nil {
        t.Fatalf("count: %v", err)
    }
    if total != 2 || uniqIP != 2 || bytesSent != 300 {
        t.Fatalf("count got total=%d uniq=%d bytes=%d", total, uniqIP, bytesSent)
    }
}
```

（`nodeQuery` 返回 `model.OpenFlareAccessLogQuery{NodeID: "n1"}`；`InsertBatch` 冻结与 hook 测试放 Task 6。）

- [ ] **Step 5: 运行测试** `go test ./internal/repository/logstore/` 期望 PASS。
- [ ] **Step 6: 提交** `git add internal/repository/logstore/ && git commit -m "feat(logstore): GORM node access log store"`

### Task 4: GORM 实现——可观测 4 表 + 用户访问日志

**Files:**
- Modify: `internal/repository/logstore/postgres_store.go`（追加方法）
- Modify: `internal/repository/logstore/postgres_store_test.go`

**Interfaces:**
- Consumes: `model.OpenFlareMetricSnapshot`/`OpenFlareEdgeHealth`/`OpenFlareNodeObservationFrps`/`OpenFlareNodeObservationFrpc`、`analyticsmodel.NodeMetricSnapshot` 等、`currentObservabilityHooks()`（Task 5）。
- Produces: `gormLogStore` 完整实现 `ObservabilityStore` 与 `UserAccessLogStore`。

- [ ] **Step 1: 可观测写入入口 + flush + 查询（追加到 postgres_store.go）**

```go
// ---- ObservabilityStore ----

func (s *gormLogStore) InsertMetricSnapshot(ctx context.Context, record *model.OpenFlareMetricSnapshot) error {
    if record == nil {
        return nil
    }
    if err := s.ensureWritable(ctx); err != nil {
        return err
    }
    if h := currentObservabilityHooks().QueueMetricSnapshot; h != nil {
        h(toAnalyticsNodeMetricSnapshot(record))
    }
    return nil
}

func (s *gormLogStore) ListMetricSnapshots(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareMetricSnapshot, error) {
    var rows []analyticsmodel.NodeMetricSnapshot
    q := s.db.WithContext(ctx).Where("node_id = ? AND captured_at >= ?", nodeID, since).Order("captured_at DESC, id DESC")
    if err := q.Limit(limitOr(limit, 100)).Find(&rows).Error; err != nil {
        return nil, err
    }
    return fromAnalyticsNodeMetricSnapshots(rows), nil
}

func (s *gormLogStore) DeleteAllMetricSnapshots(ctx context.Context) (int64, error) {
    if err := s.ensureWritable(ctx); err != nil {
        return 0, err
    }
    res := s.db.WithContext(ctx).Where("1 = 1").Delete(&analyticsmodel.NodeMetricSnapshot{})
    return res.RowsAffected, res.Error
}

func (s *gormLogStore) DeleteMetricSnapshotsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
    if err := s.ensureWritable(ctx); err != nil {
        return 0, err
    }
    res := s.db.WithContext(ctx).Where("captured_at < ?", cutoff).Delete(&analyticsmodel.NodeMetricSnapshot{})
    return res.RowsAffected, res.Error
}

func (s *gormLogStore) BatchInsertNodeMetricSnapshots(ctx context.Context, rows []analyticsmodel.NodeMetricSnapshot) error {
    if len(rows) == 0 {
        return nil
    }
    if err := s.ensureWritable(ctx); err != nil {
        return err
    }
    return s.db.WithContext(ctx).CreateInBatches(rows, 500).Error
}

// InsertEdgeHealth 等 8 个 entry/list/delete + 3 个 flush 全部与 metric snapshots 同构。
// 完整模板（以 edge health 为例）：

func (s *gormLogStore) InsertEdgeHealth(ctx context.Context, record *model.OpenFlareEdgeHealth) error {
    if record == nil {
        return nil
    }
    if err := s.ensureWritable(ctx); err != nil {
        return err
    }
    if h := currentObservabilityHooks().QueueEdgeHealth; h != nil {
        h(toAnalyticsNodeEdgeHealth(record))
    }
    return nil
}

func (s *gormLogStore) ListEdgeHealth(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareEdgeHealth, error) {
    var rows []analyticsmodel.NodeEdgeHealth
    if err := s.db.WithContext(ctx).Where("node_id = ? AND captured_at >= ?", nodeID, since).
        Order("captured_at DESC, id DESC").Limit(limitOr(limit, 100)).Find(&rows).Error; err != nil {
        return nil, err
    }
    return fromAnalyticsNodeEdgeHealths(rows), nil
}

func (s *gormLogStore) DeleteAllEdgeHealth(ctx context.Context) (int64, error) {
    if err := s.ensureWritable(ctx); err != nil {
        return 0, err
    }
    res := s.db.WithContext(ctx).Where("1 = 1").Delete(&analyticsmodel.NodeEdgeHealth{})
    return res.RowsAffected, res.Error
}

func (s *gormLogStore) DeleteEdgeHealthBefore(ctx context.Context, cutoff time.Time) (int64, error) {
    if err := s.ensureWritable(ctx); err != nil {
        return 0, err
    }
    res := s.db.WithContext(ctx).Where("captured_at < ?", cutoff).Delete(&analyticsmodel.NodeEdgeHealth{})
    return res.RowsAffected, res.Error
}

func (s *gormLogStore) BatchInsertNodeEdgeHealth(ctx context.Context, rows []analyticsmodel.NodeEdgeHealth) error {
    if len(rows) == 0 {
        return nil
    }
    if err := s.ensureWritable(ctx); err != nil {
        return err
    }
    return s.db.WithContext(ctx).CreateInBatches(rows, 500).Error
}

// FRPS/FRPC 两组按同一模板，替换映射如下：
//   FRPS:  model.OpenFlareNodeObservationFrps ↔ analyticsmodel.NodeObsFrps；hook=QueueNodeObsFrps；转换 toAnalyticsNodeObsFrps
//   FRPC:  model.OpenFlareNodeObservationFrpc ↔ analyticsmodel.NodeObsFrpc；hook=QueueNodeObsFrpc；转换 toAnalyticsNodeObsFrpc
//   list 列名统一 captured_at；delete 统一 captured_at < cutoff。
//   转换函数（toAnalyticsNodeEdgeHealth/fromAnalyticsNodeEdgeHealths/toAnalyticsNodeObsFrps/toAnalyticsNodeObsFrpc）
//   从旧 openflare_observability_store.go 复制。
```

> 逐方法补齐（8 个 entry/list/delete + 3 个 flush），表名/模型：`analyticsmodel.NodeEdgeHealth`、`analyticsmodel.NodeObsFrps`、`analyticsmodel.NodeObsFrpc`；model 侧 `OpenFlareEdgeHealth`、`OpenFlareNodeObservationFrps`、`OpenFlareNodeObservationFrpc`。`toAnalyticsNodeEdgeHealth` 等转换函数从旧 `openflare_observability_store.go` 复制。

- [ ] **Step 2: 用户访问日志（追加）**

```go
// ---- UserAccessLogStore ----

func (s *gormLogStore) BatchInsert(ctx context.Context, logs []analyticsmodel.UserAccessLog) error {
    if len(logs) == 0 {
        return nil
    }
    if err := s.ensureWritable(ctx); err != nil {
        return err
    }
    return s.db.WithContext(ctx).CreateInBatches(logs, 500).Error
}

func (s *gormLogStore) Count(ctx context.Context, filter analyticsmodel.AccessLogFilter) (uint64, error) {
    var total int64
    q := s.db.WithContext(ctx).Model(&analyticsmodel.UserAccessLog{})
    if filter.UserID != 0 {
        q = q.Where("user_id = ?", filter.UserID)
    }
    if filter.Path != "" {
        q = q.Where("path = ?", filter.Path)
    }
    if filter.Method != "" {
        q = q.Where("method = ?", filter.Method)
    }
    if filter.IP != "" {
        q = q.Where("ip = ?", filter.IP)
    }
    if filter.Status != 0 {
        q = q.Where("status = ?", filter.Status)
    }
    if !filter.Since.IsZero() {
        q = q.Where("created_at >= ?", filter.Since)
    }
    if !filter.Until.IsZero() {
        q = q.Where("created_at <= ?", filter.Until)
    }
    if err := q.Count(&total).Error; err != nil {
        return 0, err
    }
    return uint64(total), nil
}

func (s *gormLogStore) List(ctx context.Context, filter analyticsmodel.AccessLogFilter, page, pageSize int) ([]analyticsmodel.UserAccessLog, uint64, error) {
    total, err := s.Count(ctx, filter)
    if err != nil {
        return nil, 0, err
    }
    if total == 0 {
        return []analyticsmodel.UserAccessLog{}, 0, nil
    }
    var rows []analyticsmodel.UserAccessLog
    q := s.db.WithContext(ctx).Where(buildUserAccessLogWhere(filter)).Order("created_at DESC, id DESC")
    if err := q.Limit(pageSize).Offset(offsetOf(page, pageSize)).Find(&rows).Error; err != nil {
        return nil, 0, err
    }
    return rows, total, nil
}

func (s *gormLogStore) GetDailyTrend(ctx context.Context, days int) ([]analyticsmodel.DailyTrend, error) {
    if days <= 0 {
        days = 7
    }
    // 镜像 CH access_log_stats.go：起点 = (days-1) 天前当日零点；必须返回恰好 days 个日历日并补零。
    start := time.Now().AddDate(0, 0, -(days - 1)).Truncate(24 * time.Hour)
    type row struct {
        Date string
        Cnt  uint64
    }
    var rows []row
    err := s.db.WithContext(ctx).Model(&analyticsmodel.UserAccessLog{}).
        Select(dailyTrendDateSQL()+" AS date, COUNT(*) AS cnt").
        Where("created_at >= ?", start).
        Group("date").Order("date ASC").Scan(&rows).Error
    if err != nil {
        return nil, err
    }
    counts := make(map[string]uint64, len(rows))
    for _, r := range rows {
        counts[r.Date] = r.Cnt
    }
    out := make([]analyticsmodel.DailyTrend, 0, days)
    for i := 0; i < days; i++ {
        d := start.AddDate(0, 0, i).Format("2006-01-02")
        out = append(out, analyticsmodel.DailyTrend{Date: d, Cnt: counts[d]})
    }
    return out, nil
}

func (s *gormLogStore) GetBrowserDistribution(ctx context.Context, startTime time.Time) ([]analyticsmodel.BrowserShare, error) {
    return s.userAgentGroupCount(ctx, startTime, "browser")
}

func (s *gormLogStore) GetTopActiveUsers(ctx context.Context, startTime time.Time, limit int) ([]analyticsmodel.TopUser, error) {
    type row struct {
        UserID uint64
        Cnt    uint64
    }
    var rows []row
    err := s.db.WithContext(ctx).Model(&analyticsmodel.UserAccessLog{}).
        Select("user_id, COUNT(*) AS cnt").
        Where("user_id <> 0 AND created_at >= ?", startTime).
        Group("user_id").Order("cnt DESC").Limit(limitOr(limit, 10)).Scan(&rows).Error
    if err != nil {
        return nil, err
    }
    out := make([]analyticsmodel.TopUser, len(rows))
    for i, r := range rows {
        out[i] = analyticsmodel.TopUser{UserID: r.UserID, Cnt: r.Cnt}
    }
    return out, nil
}
```

> `buildUserAccessLogWhere` 与 `Count` 内联条件一致。**AccessLogFilter 使用单一权威字段集（Task 1 迁入的 CH 原字段）**：`UserIDs []uint64`、`Path`、`StartTime`/`EndTime *time.Time`。GORM 的 Count/List 必须用该字段集并镜像 CH 过滤语义（`user_id IN ?`、`path LIKE '%..%'`、`StartTime >=`、`EndTime <`）——**禁止在 AccessLogFilter 上追加仅 GORM 使用的字段**（会造成双字段集静默分叉）。`GetDailyTrend` 的日期格式化拆到 dialect 文件：`dailyTrendDateSQL()` 返回 PG `to_char(created_at,'YYYY-MM-DD')` / SQLite `strftime('%Y-%m-%d', created_at)`。`userAgentGroupCount` 用现有 `analyticsrepo.ParseBrowserName` 语义改为 SQL 侧 `CASE` 或复用 helper——实现时对照 `access_log_stats.go` 的浏览器判定逻辑，保持统计口径一致。

- [ ] **Step 3: 单测追加**——`TestGormUserAccessLogCountList`、`TestGormObservabilityInsertList`（sqlite AutoMigrate 对应模型，断言写入/查询/删除）。
- [ ] **Step 4: 运行** `go test ./internal/repository/logstore/` PASS。
- [ ] **Step 5: 提交** `git add internal/repository/logstore/ && git commit -m "feat(logstore): GORM observability and user access log store"`

### Task 5: CH 包装实现 + hooks 注册表迁入 logstore

**Files:**
- Create: `internal/repository/logstore/clickhouse_store.go`
- Create: `internal/repository/logstore/hooks.go`
- Modify: `internal/repository/openflare_access_log_store.go`、`internal/repository/openflare_observability_store.go`（删除，被吸收）

**Interfaces:**
- Consumes: `analyticsrepo.*` 全部现成函数、`db.ChConn`/`db.ChDB`。
- Produces: `newClickHouseStore() *clickhouseLogStore`；`SetAccessLogHooks`/`SetObservabilityHooks`/`currentAccessLogHooks`/`currentObservabilityHooks`。

- [ ] **Step 1: hooks.go**

```go
package logstore

import (
    "sync"

    "github.com/Rain-kl/Wavelet/internal/model"
    analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

// AccessLogHooks 节点访问日志异步入队回调（由 chwriter 装配）。
type AccessLogHooks struct {
    QueueNodeAccessLogs func(logs []analyticsmodel.NodeAccessLog)
}

// ObservabilityHooks 可观测异步入队回调（由 chwriter 装配）。
type ObservabilityHooks struct {
    QueueMetricSnapshot func(record analyticsmodel.NodeMetricSnapshot)
    QueueEdgeHealth     func(record analyticsmodel.NodeEdgeHealth)
    QueueNodeObsFrps    func(record analyticsmodel.NodeObsFrps)
    QueueNodeObsFrpc    func(record analyticsmodel.NodeObsFrpc)
}

var (
    hooksMu           sync.RWMutex
    accessLogHooks    AccessLogHooks
    observabilityHooks ObservabilityHooks
)

func SetAccessLogHooks(h AccessLogHooks) {
    hooksMu.Lock()
    accessLogHooks = h
    hooksMu.Unlock()
}

func SetObservabilityHooks(h ObservabilityHooks) {
    hooksMu.Lock()
    observabilityHooks = h
    hooksMu.Unlock()
}

func currentAccessLogHooks() AccessLogHooks {
    hooksMu.RLock()
    defer hooksMu.RUnlock()
    return accessLogHooks
}

func currentObservabilityHooks() ObservabilityHooks {
    hooksMu.RLock()
    defer hooksMu.RUnlock()
    return observabilityHooks
}
```

> 旧 `AccessLogInsertHooks`/`ObservabilityInsertHooks` 及 `SetAccessLogInsertHooks` 等在 repository 包删除，chwriter 改为调用 `logstore.SetAccessLogHooks`（Task 9）。

- [ ] **Step 2: clickhouse_store.go——逐方法委托 analyticsrepo（仅列代表，全部方法照此）**

```go
package logstore

import (
    "context"
    "errors"
    "time"

    db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
    "github.com/Rain-kl/Wavelet/internal/model"
    analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
    analyticsrepo "github.com/Rain-kl/Wavelet/internal/repository/analytics"
)

type clickhouseLogStore struct{}

func newClickHouseStore() *clickhouseLogStore { return &clickhouseLogStore{} }

func chConnErr() error {
    if !db.ChConnReady() {
        return errors.New("clickhouse connection is not initialized")
    }
    return nil
}

// ---- AccessLogStore ----

func (s *clickhouseLogStore) InsertBatch(ctx context.Context, records []*model.OpenFlareAccessLog) error {
    if err := s.ensureWritable(ctx); err != nil {
        return err
    }
    rows := make([]analyticsmodel.NodeAccessLog, 0, len(records))
    for _, r := range records {
        if r == nil {
            continue
        }
        rows = append(rows, toAnalyticsNodeAccessLog(r))
    }
    if h := currentAccessLogHooks().QueueNodeAccessLogs; h != nil {
        h(rows)
    }
    return nil
}

func (s *clickhouseLogStore) ensureWritable(ctx context.Context) error {
    if Migrating(ctx) {
        return ErrMigrating
    }
    return nil
}

func (s *clickhouseLogStore) BatchInsertNodeAccessLogs(ctx context.Context, rows []analyticsmodel.NodeAccessLog) error {
    if err := s.ensureWritable(ctx); err != nil {
        return err
    }
    return analyticsrepo.BatchInsertNodeAccessLogs(ctx, rows)
}

func (s *clickhouseLogStore) List(ctx context.Context, query model.OpenFlareAccessLogQuery) ([]*model.OpenFlareAccessLog, error) {
    rows, err := analyticsrepo.ListNodeAccessLogs(ctx, toNodeAccessLogFilter(query))
    if err != nil {
        return nil, err
    }
    return fromAnalyticsNodeAccessLogs(rows), nil
}

func (s *clickhouseLogStore) Count(ctx context.Context, query model.OpenFlareAccessLogQuery) (int64, int64, int64, error) {
    return analyticsrepo.CountNodeAccessLogs(ctx, toNodeAccessLogFilter(query))
}

func (s *clickhouseLogStore) RegionCounts(ctx context.Context, nodeID string, since time.Time, limit int) ([]*model.OpenFlareAccessLogRegionCount, error) {
    rows, err := analyticsrepo.RegionCountsNodeAccessLogs(ctx, nodeID, since, limit)
    if err != nil {
        return nil, err
    }
    out := make([]*model.OpenFlareAccessLogRegionCount, len(rows))
    for i, r := range rows {
        out[i] = &model.OpenFlareAccessLogRegionCount{Region: r.Region, Count: r.Count}
    }
    return out, nil
}

func (s *clickhouseLogStore) BucketAggregates(ctx context.Context, query model.OpenFlareAccessLogQuery, bucketSeconds int64) ([]analyticsmodel.NodeAccessLogBucketAggregate, error) {
    return analyticsrepo.BucketAggregatesNodeAccessLogs(ctx, toNodeAccessLogFilter(query), bucketSeconds)
}

func (s *clickhouseLogStore) CountBuckets(ctx context.Context, query model.OpenFlareAccessLogQuery, bucketSeconds int64) (int64, error) {
    return analyticsrepo.CountBucketAggregatesNodeAccessLogs(ctx, toNodeAccessLogFilter(query), bucketSeconds)
}

func (s *clickhouseLogStore) BucketDimensions(ctx context.Context, query model.OpenFlareAccessLogQuery, column string, bucketSeconds int64) ([]analyticsmodel.NodeAccessLogBucketDimension, error) {
    return analyticsrepo.BucketDimensionsNodeAccessLogs(ctx, toNodeAccessLogFilter(query), column, bucketSeconds)
}

func (s *clickhouseLogStore) IPAggregates(ctx context.Context, query model.OpenFlareAccessLogQuery, exactRemoteAddr bool) ([]analyticsmodel.NodeAccessLogIPAggregate, error) {
    return analyticsrepo.IPAggregatesNodeAccessLogs(ctx, toNodeAccessLogFilter(query), exactRemoteAddr)
}

func (s *clickhouseLogStore) IPSummaries(ctx context.Context, query model.OpenFlareAccessLogQuery, recentSince time.Time) ([]analyticsmodel.NodeAccessLogIPSummary, error) {
    return analyticsrepo.IPSummariesNodeAccessLogs(ctx, toNodeAccessLogFilter(query), recentSince)
}

func (s *clickhouseLogStore) CountIPSummaries(ctx context.Context, query model.OpenFlareAccessLogQuery) (int64, error) {
    return analyticsrepo.CountIPSummaryNodeAccessLogs(ctx, toNodeAccessLogFilter(query))
}

func (s *clickhouseLogStore) WAFIPAggregates(ctx context.Context, query model.OpenFlareAccessLogQuery) ([]analyticsmodel.NodeAccessLogWAFIPAggregate, error) {
    return analyticsrepo.IPAggregatesForWAFNodeAccessLogs(ctx, toNodeAccessLogFilter(query))
}

func (s *clickhouseLogStore) IPTrend(ctx context.Context, query model.OpenFlareAccessLogQuery, bucketSeconds int64) ([]analyticsmodel.NodeAccessLogIPTrend, error) {
    return analyticsrepo.IPTrendNodeAccessLogs(ctx, toNodeAccessLogFilter(query), bucketSeconds)
}

func (s *clickhouseLogStore) TrafficSummary(ctx context.Context, query model.OpenFlareAccessLogQuery) (model.OpenFlareAccessLogTrafficSummary, error) {
    row, err := analyticsrepo.TrafficSummaryNodeAccessLogs(ctx, toNodeAccessLogFilter(query))
    if err != nil {
        return model.OpenFlareAccessLogTrafficSummary{}, err
    }
    return model.OpenFlareAccessLogTrafficSummary{
        RequestCount:  int64(row.RequestCount),
        ErrorCount:    int64(row.ErrorCount),
        UniqueIPCount: int64(row.UniqueIPCount),
        BytesSent:     int64(row.BytesSent),
        RequestLength: int64(row.RequestLength),
        NodeCount:     int64(row.NodeCount),
    }, nil
}

func (s *clickhouseLogStore) ValueCounts(ctx context.Context, query model.OpenFlareAccessLogQuery, column string, limit int) ([]model.OpenFlareAccessLogValueCount, error) {
    rows, err := analyticsrepo.ValueCountsNodeAccessLogs(ctx, toNodeAccessLogFilter(query), column, limit)
    if err != nil {
        return nil, err
    }
    out := make([]model.OpenFlareAccessLogValueCount, len(rows))
    for i, r := range rows {
        out[i] = model.OpenFlareAccessLogValueCount{Value: r.Value, Count: int64(r.Count)}
    }
    return out, nil
}

func (s *clickhouseLogStore) NodeAggregates(ctx context.Context, query model.OpenFlareAccessLogQuery) ([]model.OpenFlareAccessLogNodeAggregate, error) {
    rows, err := analyticsrepo.NodeAggregatesNodeAccessLogs(ctx, toNodeAccessLogFilter(query))
    if err != nil {
        return nil, err
    }
    out := make([]model.OpenFlareAccessLogNodeAggregate, len(rows))
    for i, r := range rows {
        out[i] = model.OpenFlareAccessLogNodeAggregate{NodeID: r.NodeID, RequestCount: int64(r.RequestCount), ErrorCount: int64(r.ErrorCount), UniqueIPCount: int64(r.UniqueIPCount)}
    }
    return out, nil
}

func (s *clickhouseLogStore) DeleteAll(ctx context.Context) (int64, error) {
    if err := s.ensureWritable(ctx); err != nil {
        return 0, err
    }
    return analyticsrepo.DeleteAllNodeAccessLogs(ctx)
}

func (s *clickhouseLogStore) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
    if err := s.ensureWritable(ctx); err != nil {
        return 0, err
    }
    return analyticsrepo.DeleteNodeAccessLogsBefore(ctx, cutoff)
}

func (s *clickhouseLogStore) DeleteByNodeBefore(ctx context.Context, nodeID string, before time.Time) (int64, error) {
    if err := s.ensureWritable(ctx); err != nil {
        return 0, err
    }
    return analyticsrepo.DeleteNodeAccessLogsByNodeBefore(ctx, nodeID, before)
}

// ---- ObservabilityStore（entry=ensureWritable+hook；flush/query/delete 委托 analyticsrepo）
// InsertMetricSnapshot / ListMetricSnapshots / DeleteAllMetricSnapshots / DeleteMetricSnapshotsBefore / BatchInsertNodeMetricSnapshots
// ...（同构，参照旧 clickhouseObservabilityStore 委托）
// ---- UserAccessLogStore
// BatchInsert -> analyticsrepo.BatchInsert
// Count/List -> analyticsrepo.CountAccessLogs / ListAccessLogs
// GetDailyTrend / GetBrowserDistribution / GetTopActiveUsers -> analyticsrepo.GetDailyTrend / GetBrowserDistribution / GetTopActiveUsers
```

> 转换函数 `toAnalyticsNodeAccessLog`/`fromAnalyticsNodeAccessLogs`/`toNodeAccessLogFilter`/`toAnalyticsNodeMetricSnapshot` 等集中放 `postgres_store.go` 或本文件共享区域（两个实现共用）。

- [ ] **Step 3: 删除旧 store 文件**——删 `internal/repository/openflare_access_log_store.go`、`internal/repository/openflare_observability_store.go`；其中的 memory store 测试替身迁到 `logstore/memory_store_test.go`（保留 `NewMemoryAccessLogStore` 等价物供 repository 测试）。
- [ ] **Step 4: 编译 + 测试** `go build ./internal/...`；`go test ./internal/repository/...` 修复引用。
- [ ] **Step 5: 提交** `git add internal/repository/logstore/ internal/repository/ && git commit -m "refactor(logstore): wrap ClickHouse analytics repo behind interface"`

### Task 6: repository 公开函数改委托 logstore

**Files:**
- Modify: `internal/repository/openflare_access_log.go`（函数体改为 `logstore.Active(ctx)` 委托）
- Modify: `internal/repository/openflare_observability.go`（同上）

**Interfaces:**
- Consumes: `logstore.Active`、`logstore.Store` 字段。
- Produces: 保留原公开函数签名，行为不变（CH 激活时与现状一致）。

- [ ] **Step 1: 改写 `openflare_access_log.go` 各函数**

```go
package repository

import (
    "context"
    "time"

    "github.com/Rain-kl/Wavelet/internal/model"
    "github.com/Rain-kl/Wavelet/internal/model/analytics" // 若类型别名仍需要
    "github.com/Rain-kl/Wavelet/internal/repository/logstore"
)

// ListOpenFlareAccessLogs lists access logs matching the query.
func ListOpenFlareAccessLogs(ctx context.Context, query model.OpenFlareAccessLogQuery) ([]*model.OpenFlareAccessLog, error) {
    s, err := logstore.Active(ctx)
    if err != nil {
        return nil, err
    }
    return s.AccessLogs.List(ctx, query)
}
```

对同文件其余函数（`ListOpenFlareAccessLogWAFIPAggregates`、`InsertOpenFlareAccessLogsBatch`、`CountOpenFlareAccessLogs`、`TrafficSummaryOpenFlareAccessLogs`、`RegionCountsOpenFlareAccessLogs`、`BucketAggregates*`、`CountBuckets*`、`BucketDimensions*`、`IPAggregates*`、`IPSummaries*`、`CountIPSummaries*`、`IPTrend*`、`ValueCounts*`、`NodeAggregates*`、`Delete*`）逐一委托到 `s.AccessLogs` 对应方法；`InsertOpenFlareAccessLogsBatch` → `s.AccessLogs.InsertBatch`。**保留行类型别名**（`openFlareAccessLogBucketAggregateRow` 等）供调用方编译。

- [ ] **Step 2: 改写 `openflare_observability.go`**——`InsertOpenFlareMetricSnapshot` → `s.Observability.InsertMetricSnapshot`；`ListMetricSnapshots*`/`Delete*` 同理；健康事件（`ReconcileOpenFlareHealthEvents` 等**主库表**逻辑）保持原实现不动。
- [ ] **Step 3: 编译 + 测试** `go build ./internal/...`、`go test ./internal/repository/...`（旧测试若引用 memory store 替换为 logstore 测试替身）。
- [ ] **Step 4: 提交** `git add internal/repository/ && git commit -m "refactor(repository): delegate log CRUD to logstore"`

### Task 7: import-lint 测试（代码级约束验收）

**Files:**
- Create: `internal/repository/logstore/imports_test.go`

- [ ] **Step 1: 写测试**

```go
package logstore

import (
    "os/exec"
    "strings"
    "testing"
)

// forbiddenImports 上层应用禁止直接触碰的底层日志实现。
var forbiddenImports = []string{
    "github.com/Rain-kl/Wavelet/internal/repository/analytics",
}

// allowedInfraPersistence 允许 apps 引入的 infra/persistence 子包。
// batchwriter=批量写入框架；idgen=snowflake ID 生成工具（apps 合法使用，非日志后端访问）。
var allowedInfraPersistence = []string{
    "github.com/Rain-kl/Wavelet/internal/infra/persistence/batchwriter",
    "github.com/Rain-kl/Wavelet/internal/infra/persistence/idgen",
}

func TestAppsMustNotImportLogBackendDirectly(t *testing.T) {
    t.Chdir("../../..")
    out, err := exec.Command("go", "list", "-test", "-f", `{{.ImportPath}} {{join .Imports " "}}`, "./internal/apps/...").Output()
    if err != nil {
        t.Fatalf("go list: %v", err)
    }
    for _, line := range strings.Split(string(out), "\n") {
        fields := strings.Fields(line)
        if len(fields) == 0 {
            continue
        }
        pkg := fields[0]
        if !strings.HasPrefix(pkg, "github.com/Rain-kl/Wavelet/internal/apps") {
            continue
        }
        for _, imp := range fields[1:] {
            for _, forbidden := range forbiddenImports {
                if imp == forbidden && !allowedAnalyticsDelegation[pkg] {
                    t.Errorf("%s must not import forbidden log backend %s", pkg, forbidden)
                }
            }
            if strings.HasPrefix(imp, "github.com/Rain-kl/Wavelet/internal/infra/persistence/") {
                allowed := false
                for _, a := range allowedInfraPersistence {
                    if imp == a || strings.HasPrefix(imp, a+"/") {
                        allowed = true
                        break
                    }
                }
                if !allowed {
                    t.Errorf("%s must not import infra/persistence subpackage directly: %s", pkg, imp)
                }
            }
        }
    }
}
```

> 说明：`go list -deps` 在测试工作目录执行，先 `t.Chdir` 到仓库根（`../../..`）再运行，避免依赖 `go test` 的临时目录。若 `internal/apps/admin/logs` 等仍 import analyticsrepo，本测试失败——正好驱动 Task 9。

- [ ] **Step 2: 运行** `go test ./internal/repository/logstore/ -run TestAppsMustNotImportLogBackendDirectly -v`——预期当前**失败**（列出违规包）。
- [ ] **Step 3: 暂不提交**——本测试在 apps 改造完成前保持 RED（预期失败列出违规包）。Task 9 完成 apps 改造、本测试转绿后，随 Task 9 一并提交（提交信息：`test(logstore): enforce apps must not import log backend directly`）。

### Task 8: 系统配置 key + 启动校验 + key 保护

**Files:**
- Modify: `internal/model/system_configs.go`（新增 key 常量）
- Modify: `internal/platform/bootstrap/bootstrap.go`（`Init` 加校验与 seed）
- Modify: `internal/apps/admin/system_config/routers.go`（受保护 key 拒绝修改）
- Modify: `internal/apps/openflare/option/validate.go`（同）
- Create: `internal/platform/bootstrap/bootstrap_test.go`（追加校验测试）

**Interfaces:**
- Consumes: `config.Config.Database.Enabled`、`config.Config.ClickHouse.Enabled`、`repository.GetSystemConfigByKey`、`repository.UpdateSystemConfigFields`。
- Produces: `model.ConfigKeyLogDatabase = "log_database"`、`model.ConfigKeyLogDBMigration = "log_db_migration"`、`model.ConfigKeyLogRetentionDaysPostgres = "log_retention_days_postgres"`、`model.ConfigKeyLogRetentionDaysSQLite = "log_retention_days_sqlite"`、`model.ConfigKeyLogRetentionDaysClickHouse = "log_retention_days_clickhouse"`。

- [ ] **Step 1: 新增 key 常量（system_configs.go）**

```go
// 日志数据库解耦
ConfigKeyLogDatabase                    = "log_database"                       // 当前日志主库：postgres|sqlite|clickhouse（仅迁移任务写入）
ConfigKeyLogDBMigration                 = "log_db_migration"                   // 迁移冻结标记："migrating" 或空
ConfigKeyLogRetentionDaysPostgres       = "log_retention_days_postgres"        // PostgreSQL 日志保留天数
ConfigKeyLogRetentionDaysSQLite         = "log_retention_days_sqlite"          // SQLite 日志保留天数
ConfigKeyLogRetentionDaysClickHouse     = "log_retention_days_clickhouse"      // ClickHouse 日志保留天数
```

- [ ] **Step 2: bootstrap 校验 + seed（bootstrap.go `Init` 内，`initRuntimeOnce.Do` 开头）**

```go
// validateAndSeedLogDatabase 校验日志主库标记与运行配置的一致性，首次启动 seed。
func validateAndSeedLogDatabase(ctx context.Context) error {
    cfg, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyLogDatabase)
    if err != nil {
        return fmt.Errorf("读取日志主库配置失败: %w", err)
    }
    current := cfg.Value
    if current == "" {
        // 首次启动 seed：CH 启用 → clickhouse；否则随主库。
        current = "sqlite"
        if config.Config.Database.Enabled {
            current = "postgres"
        }
        if config.Config.ClickHouse.Enabled {
            current = "clickhouse"
        }
        if err := repository.UpdateSystemConfigFields(ctx, &model.SystemConfig{Key: model.ConfigKeyLogDatabase}, map[string]any{"value": current}); err != nil {
            return fmt.Errorf("初始化日志主库配置失败: %w", err)
        }
        return nil
    }
    switch current {
    case "clickhouse":
        if !config.Config.ClickHouse.Enabled {
            return errors.New("当前日志主库为 ClickHouse 但 ClickHouse 未启用。请先重新启用 ClickHouse 配置并启动，在任务管理运行『切换日志数据库』迁移到 PostgreSQL/SQLite 后再禁用 ClickHouse")
        }
    case "postgres":
        if !config.Config.Database.Enabled {
            return errors.New("当前日志主库为 PostgreSQL 但 PostgreSQL 未启用（当前为 SQLite 主库）。请运行『切换日志数据库』迁回 SQLite 或启用 PostgreSQL")
        }
    case "sqlite":
        if config.Config.Database.Enabled {
            return errors.New("当前日志主库为 SQLite 但当前主库为 PostgreSQL。请运行『切换日志数据库』迁移到 PostgreSQL")
        }
    default:
        return fmt.Errorf("未知的日志主库配置: %s", current)
    }
    return nil
}
```

在 `Init` 的 `initRuntimeOnce.Do` 内最先调用：`if err := validateAndSeedLogDatabase(ctx); err != nil { logger.ErrorF(...); log.Fatalf(...) }`（或按项目既有致命启动错误处理方式）。

- [ ] **Step 3: key 保护（admin system-config 更新路径）**

`internal/apps/admin/system_config/routers.go` 的 `UpdateSystemConfig` 与 `internal/apps/openflare/option/validate.go` 增加：

```go
// protectedConfigKeys 仅允许内部（迁移任务/bootstrap）写入的 key。
var protectedConfigKeys = map[string]bool{
    model.ConfigKeyLogDatabase:    true,
    model.ConfigKeyLogDBMigration: true,
}

func isProtectedConfigKey(key string) bool { return protectedConfigKeys[key] }
```

更新处理：命中保护 key 时返回业务错误（`response.AbortBadRequest(c, "该配置项由系统任务管理，禁止手动修改")`），且不写库。

- [ ] **Step 4: 单测**——`bootstrap_test.go` 三态校验（clickhouse 未启用 / postgres 但 sqlite 主库 / sqlite 但 postgres 主库）各自返回明确错误；seed 缺失时写入正确默认值。
- [ ] **Step 5: 运行** `go test ./internal/platform/bootstrap/ ./internal/model/ ./internal/apps/admin/system_config/` PASS。
- [ ] **Step 6: 提交** `git add internal/model/system_configs.go internal/platform/bootstrap/ internal/apps/admin/system_config/ internal/apps/openflare/option/ && git commit -m "feat(config): log database marker, boot validation, and protected keys"`

### Task 9: apps 层改走 logstore（消除 import-lint 违规）

**Files:**
- Modify: `internal/apps/risk_control/logics.go`、`internal/apps/openflare/chwriter/writer.go`
- Modify: `internal/apps/openflare/tasks/database_cleanup.go`（本任务只改 import；清理合并到 M2）
- Modify: `internal/apps/openflare/observability/access_log_logics.go`（仅解析 helper 保留 analyticsrepo 合法引用则不动；若违规则把 `ParseDeviceType`/`ParseBrowserName`/`ParseOSName` 迁到 `model/analytics` 或 `internal/util`）
- Modify: `internal/apps/admin/logs/routers.go`、`internal/apps/admin/status/clickhouse.go`
- Test: `internal/repository/logstore/imports_test.go`（回归）

**Interfaces:**
- Consumes: `logstore.Active`、`logstore.Migrating`、`logstore.ErrMigrating`、`logstore.SetAccessLogHooks`/`SetObservabilityHooks`。

- [ ] **Step 1: chwriter flush func 改为 logstore**

`writer.go` 中 5 处 `analyticsrepo.BatchInsertNode*` → `logstore.Active(ctx).Observability/AccessLogs` 对应 flush 方法（或包级 helper）：

```go
func flushNodeAccessLogs(ctx context.Context, rows []analyticsmodel.NodeAccessLog) error {
    s, err := logstore.Active(ctx)
    if err != nil {
        return err
    }
    return s.AccessLogs.BatchInsertNodeAccessLogs(ctx, rows)
}
```

`Init` 内 `if !config.Config.ClickHouse.Enabled { return }` 改为 `if logstore.Active(ctx) == nil ...` 或直接始终初始化 writer（writer flush 走 logstore，激活库由 logstore 决定）；`wireModelInsertHooks` 改为调用 `logstore.SetAccessLogHooks`/`logstore.SetObservabilityHooks`。

- [ ] **Step 2: risk_control flush 与冻结**

`logics.go`：flush func 中 `analyticsrepo.BatchInsert` → `logstore.Active(ctx).UserAccessLogs.BatchInsert`；`InitLogWriter` 的 CH 开关条件移除，改为由 logstore 激活库决定（PG/SQLite 也启用该 writer）；middleware 入队前：

```go
if logstore.Migrating(c.Request.Context()) {
    logger.WarnF(c.Request.Context(), "[RiskControl] log DB migrating, skip audit log")
    return // 不阻断业务请求
}
```

- [ ] **Step 3: admin/logs 改走 logstore**

`routers.go` 中 `analyticsrepo.ListAccessLogs/CountAccessLogs/GetDailyTrend/GetBrowserDistribution/GetTopActiveUsers` → `logstore.Active(ctx).UserAccessLogs.*`；`config.Config.ClickHouse.Enabled || !db.ChConnReady()` 的守卫改为按激活库判断（`logstore.Active(ctx)` 成功即可用），错误文案从「ClickHouse 存储服务未启用」改为「日志存储未启用」。

- [ ] **Step 4: admin/status 端点骨架**

`clickhouse.go` 改为读取 `logstore.Active` 与激活库名，返回统一结构（M3 Task 16 完成前端与完整字段）：

```go
type LogDatabaseStatus struct {
    ActiveDatabase   string          `json:"active_database"`
    Migration        string          `json:"migration"` // idle | migrating
    RetentionDays    map[string]int  `json:"retention_days"`
    AvailableTargets []string        `json:"available_targets"`
}
```

CH 激活时保留 `GetClickHouseOperationalStats` 与 `collectBatchWriterStats`。

- [ ] **Step 5: database_cleanup.go 临时保留 import 但标记 TODO（M2 Task 13 迁移）**——若 import-lint 在 Task 7 已注册，本任务先让 `database_cleanup.go` 改为经 repository 公开函数（其逻辑已走 logstore），并同步 `access_log_logics.go` 解析 helper（迁 `ParseBrowserName` 等为 `model/analytics` 纯函数，analyticsrepo 内部复用）。
- [ ] **Step 6: 运行 import-lint 回归** `go test ./internal/repository/logstore/ -run TestAppsMustNotImportLogBackendDirectly -v` 期望 **PASS**。
- [ ] **Step 7: 全量编译** `go build ./internal/...`、`go test ./internal/apps/...` 修复。
- [ ] **Step 8: 提交** `git add internal/apps/ && git commit -m "refactor(apps): route log reads/writes through logstore"`

### Task 10: bootstrap 装配 logstore

**Files:**
- Modify: `internal/platform/bootstrap/bootstrap.go`
- Modify: `internal/cmd/all.go`、`api.go`、`worker.go`、`root.go`（如有必要）

**Interfaces:**
- Consumes: `logstore.SetConfigReader`、`logstore.Init`。
- Produces: 运行期 `logstore` 激活 store 可解析。

- [ ] **Step 1: 装配 config reader + Init**

`bootstrap.Init` 的 `initRuntimeOnce.Do` 内、校验之后：

```go
logstore.SetConfigReader(func(ctx context.Context, key string) (string, error) {
    cfg, err := repository.GetSystemConfigByKey(ctx, key)
    if err != nil {
        return "", err
    }
    return cfg.Value, nil
})
logstore.Init(ctx)
```

- [ ] **Step 2: worker 进程也需要 Init**——确认 `cmd/worker.go` 与 `cmd/all.go` 都调用 `bootstrap.Init`（现 API 分支启动 writer；worker 迁移任务需能读配置与激活 store，`logstore.Init` 必须在两种进程都执行）。
- [ ] **Step 3: 测试** `go test ./internal/platform/bootstrap/`；`go build ./cmd/...`。
- [ ] **Step 4: 提交** `git add internal/platform/bootstrap/ internal/cmd/ && git commit -m "feat(bootstrap): wire logstore config reader and init"`

---

## M2：建表与清理

### Task 10b: 小时级聚合读经 logstore（PG 实时计算 / CH 读 rollup 表）

**Files:**
- Modify: `internal/repository/logstore/logstore.go`（`ObservabilityStore` 增 3 个方法）
- Modify: `internal/repository/logstore/postgres_store.go`（PG 按小时从原始表实时聚合）
- Modify: `internal/repository/logstore/clickhouse_store.go`（委托 analyticsrepo rollup 读 + 现有 raw 兜底逻辑）
- Modify: `internal/repository/openflare_observability.go`（3 个 `ListOpenFlare*HourlySince` 改委托 logstore）
- Modify: `internal/repository/logstore/imports_test.go`（若 `internal/repository` 不再直接 import analyticsrepo，可移除其对 `allowedAnalyticsDelegation` 的豁免）

**Interfaces:**
- Consumes: Task 3/4 GORM store、Task 5 CH store、`analyticsrepo.ListNodeTrafficHourly`/`ListAccessLogHourly`/`ListNodeMetricHourly` 及 `mergeNodeMetricHourlyPreferRollup`/`listNodeMetricHourlyFromRaw` 语义。
- Produces: `ObservabilityStore.ListTrafficHourly(ctx, nodeID, since) ([]analyticsmodel.NodeTrafficHourly, error)`、`ListAccessLogHourly(...)`、`ListMetricHourly(...)`。

- [ ] **Step 1: 接口加方法**（logstore.go）
- [ ] **Step 2: CH 实现委托 analyticsrepo**（rollup 表 + raw 兜底，逐行复制现有逻辑）
- [ ] **Step 3: PG 实现按小时实时聚合**——`date_trunc('hour', logged_at/captured_at)` 分组（方言 `timeBucketSQL(col, 3600)` 复用），请求/错误/字节数与 CH rollup 同字段；`ListMetricHourly` 用 `avg(cpu)/max-min 计数器` 近似同 CH `mergeNodeMetricHourlyPreferRollup` 口径。
- [ ] **Step 4: repository 门面 3 个函数改委托 logstore**；若门面不再 import analyticsrepo，收紧 lint 豁免。
- [ ] **Step 5: 测试**——PG/SQLite 实时聚合与 CH rollup 口径一致性（sqlite 写原始行断言小时桶输出）；CH 委托回归。
- [ ] **Step 6: 提交** `git add internal/repository/ && git commit -m "feat(logstore): hourly rollup reads with PG real-time aggregation"`

---
### Task 11: goose 双方言建表迁移（6 张原始日志表）

**Files:**
- Create: `internal/infra/persistence/migrator/goose/postgres/202608080001_create_log_tables.sql`
- Create: `internal/infra/persistence/migrator/goose/sqlite/202608080001_create_log_tables.sql`

**Interfaces:**
- Consumes: database-migration 技能规则（双方言同版本号、无物理外键、默认值与 Go 零值一致）。
- Produces: PG/SQLite 各 6 张日志表（`w_user_access_logs`、`of_node_access_logs`、`of_node_metric_snapshots`、`of_node_edge_health`、`of_node_obs_frps`、`of_node_obs_frpc`）。

- [ ] **Step 1: PG 建表（含分区）**

```sql
-- +goose Up
-- 节点访问日志：按月 RANGE 分区，复合主键 (id, logged_at) 满足分区键进唯一索引要求。
CREATE TABLE of_node_access_logs (
    id              BIGINT NOT NULL,
    node_id         VARCHAR(64) NOT NULL,
    logged_at       TIMESTAMPTZ NOT NULL,
    remote_addr     VARCHAR(128) NOT NULL DEFAULT '',
    region          VARCHAR(128) NOT NULL DEFAULT '',
    host            VARCHAR(255) NOT NULL DEFAULT '',
    path            VARCHAR(2048) NOT NULL DEFAULT '',
    user_agent      TEXT NOT NULL DEFAULT '',
    cache_status    VARCHAR(64) NOT NULL DEFAULT '',
    status_code     INTEGER NOT NULL DEFAULT 0,
    bytes_sent      BIGINT NOT NULL DEFAULT 0,
    request_length  BIGINT NOT NULL DEFAULT 0,
    request_time_ms INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, logged_at)
) PARTITION BY RANGE (logged_at);

CREATE INDEX idx_of_node_access_logs_node_id ON of_node_access_logs (node_id, logged_at DESC);
CREATE INDEX idx_of_node_access_logs_host ON of_node_access_logs (host, logged_at DESC);
CREATE INDEX idx_of_node_access_logs_remote_addr ON of_node_access_logs (remote_addr, logged_at DESC);
CREATE INDEX idx_of_node_access_logs_status_code ON of_node_access_logs (status_code, logged_at DESC);

-- 用户访问日志：按月分区。
CREATE TABLE w_user_access_logs (
    id          BIGINT NOT NULL,
    user_id     BIGINT NOT NULL DEFAULT 0,
    path        VARCHAR(2048) NOT NULL DEFAULT '',
    method      VARCHAR(16) NOT NULL DEFAULT '',
    ip          VARCHAR(128) NOT NULL DEFAULT '',
    user_agent  TEXT NOT NULL DEFAULT '',
    headers     TEXT NOT NULL DEFAULT '',
    status      INTEGER NOT NULL DEFAULT 0,
    latency     BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE INDEX idx_w_user_access_logs_user_id ON w_user_access_logs (user_id, created_at DESC);

-- 可观测 4 表：普通表 + 索引。
CREATE TABLE of_node_metric_snapshots (
    id                 BIGINT NOT NULL PRIMARY KEY,
    node_id            VARCHAR(64) NOT NULL,
    captured_at        TIMESTAMPTZ NOT NULL,
    cpu_usage_percent  DOUBLE PRECISION NOT NULL DEFAULT 0,
    memory_used_bytes  BIGINT NOT NULL DEFAULT 0,
    memory_total_bytes BIGINT NOT NULL DEFAULT 0,
    storage_used_bytes BIGINT NOT NULL DEFAULT 0,
    storage_total_bytes BIGINT NOT NULL DEFAULT 0,
    disk_read_bytes    BIGINT NOT NULL DEFAULT 0,
    disk_write_bytes   BIGINT NOT NULL DEFAULT 0,
    network_rx_bytes   BIGINT NOT NULL DEFAULT 0,
    network_tx_bytes   BIGINT NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_of_node_metric_snapshots_node ON of_node_metric_snapshots (node_id, captured_at DESC);

CREATE TABLE of_node_edge_health (
    id          BIGINT NOT NULL PRIMARY KEY,
    node_id     VARCHAR(64) NOT NULL,
    captured_at TIMESTAMPTZ NOT NULL,
    status      VARCHAR(64) NOT NULL DEFAULT '',
    connections BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_of_node_edge_health_node ON of_node_edge_health (node_id, captured_at DESC);

CREATE TABLE of_node_obs_frps (
    id                BIGINT NOT NULL PRIMARY KEY,
    node_id           VARCHAR(64) NOT NULL,
    captured_at       TIMESTAMPTZ NOT NULL,
    frps_connections  INTEGER NOT NULL DEFAULT 0,
    frps_proxy_count  INTEGER NOT NULL DEFAULT 0,
    frps_client_count INTEGER NOT NULL DEFAULT 0,
    frps_proxies      TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_of_node_obs_frps_node ON of_node_obs_frps (node_id, captured_at DESC);

CREATE TABLE of_node_obs_frpc (
    id                    BIGINT NOT NULL PRIMARY KEY,
    node_id               VARCHAR(64) NOT NULL,
    captured_at           TIMESTAMPTZ NOT NULL,
    tunnel_status         VARCHAR(16) NOT NULL DEFAULT '',
    connected_relays_count INTEGER NOT NULL DEFAULT 0,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_of_node_obs_frpc_node ON of_node_obs_frpc (node_id, captured_at DESC);

-- 分区预建：创建未来 3 个月与当前月分区（当月及下两个月）。
DO $$
DECLARE
    d date;
BEGIN
    FOR d IN SELECT generate_series(date_trunc('month', now())::date, (date_trunc('month', now()) + interval '2 months')::date, interval '1 month')::date
    LOOP
        EXECUTE format('CREATE TABLE IF NOT EXISTS of_node_access_logs_%s PARTITION OF of_node_access_logs FOR VALUES FROM (%L) TO (%L)',
            to_char(d, 'YYYYMM'), d, d + interval '1 month');
        EXECUTE format('CREATE TABLE IF NOT EXISTS w_user_access_logs_%s PARTITION OF w_user_access_logs FOR VALUES FROM (%L) TO (%L)',
            to_char(d, 'YYYYMM'), d, d + interval '1 month');
    END LOOP;
END $$;

-- +goose Down
DROP TABLE IF EXISTS w_user_access_logs;
DROP TABLE IF EXISTS of_node_access_logs;
DROP TABLE IF EXISTS of_node_metric_snapshots;
DROP TABLE IF EXISTS of_node_edge_health;
DROP TABLE IF EXISTS of_node_obs_frps;
DROP TABLE IF EXISTS of_node_obs_frpc;
```

- [ ] **Step 2: SQLite 建表（普通表，同语义）**

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS of_node_access_logs (
    id              INTEGER PRIMARY KEY,
    node_id         TEXT NOT NULL DEFAULT '',
    logged_at       DATETIME NOT NULL,
    remote_addr     TEXT NOT NULL DEFAULT '',
    region          TEXT NOT NULL DEFAULT '',
    host            TEXT NOT NULL DEFAULT '',
    path            TEXT NOT NULL DEFAULT '',
    user_agent      TEXT NOT NULL DEFAULT '',
    cache_status    TEXT NOT NULL DEFAULT '',
    status_code     INTEGER NOT NULL DEFAULT 0,
    bytes_sent      INTEGER NOT NULL DEFAULT 0,
    request_length  INTEGER NOT NULL DEFAULT 0,
    request_time_ms INTEGER NOT NULL DEFAULT 0,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_node ON of_node_access_logs (node_id, logged_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_host ON of_node_access_logs (host, logged_at DESC);
CREATE INDEX IF NOT EXISTS idx_of_node_access_logs_remote_addr ON of_node_access_logs (remote_addr, logged_at DESC);
-- 其余 5 表同构（w_user_access_logs 主键 id；可观测表 id INTEGER PRIMARY KEY + (node_id, captured_at DESC) 索引）

-- +goose Down
DROP TABLE IF EXISTS of_node_access_logs;
DROP TABLE IF EXISTS w_user_access_logs;
DROP TABLE IF EXISTS of_node_metric_snapshots;
DROP TABLE IF EXISTS of_node_edge_health;
DROP TABLE IF EXISTS of_node_obs_frps;
DROP TABLE IF EXISTS of_node_obs_frpc;
```

- [ ] **Step 3: 验证 goose** `go test ./internal/infra/persistence/migrator`（空库 Up 全量）。
- [ ] **Step 4: 提交** `git add internal/infra/persistence/migrator/goose/ && git commit -m "feat(migrate): create log tables in postgres and sqlite"`

### Task 12: 保留时间配置 + 旧 key 下线

**Files:**
- Create: `internal/infra/persistence/migrator/goose/postgres/202608080002_log_retention_configs.sql`
- Create: `internal/infra/persistence/migrator/goose/sqlite/202608080002_log_retention_configs.sql`
- Modify: `internal/model/system_configs.go`（删除旧 key 常量或标记废弃）
- Modify: `internal/testhelper/test_helper.go`（seed 同步）

**Interfaces:**
- Produces: 3 个 business 配置（默认 90）；旧 `database_auto_cleanup_enabled`/`database_auto_cleanup_retention_days` 从 `system_configs` 删除。

- [ ] **Step 1: PG 迁移**

```sql
-- +goose Up
INSERT INTO system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES
    ('log_retention_days_postgres',  '90', 'business', 0, 'PostgreSQL 日志保留天数（访问日志与可观测统一）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('log_retention_days_sqlite',    '90', 'business', 0, 'SQLite 日志保留天数', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('log_retention_days_clickhouse','90', 'business', 0, 'ClickHouse 日志保留天数', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

DELETE FROM system_configs WHERE key IN ('database_auto_cleanup_enabled', 'database_auto_cleanup_retention_days');

-- +goose Down
INSERT INTO system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES
    ('database_auto_cleanup_enabled', 'true', 'business', 0, '数据库自动清理开关', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('database_auto_cleanup_retention_days', '30', 'business', 0, '数据库保留天数', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;
DELETE FROM system_configs WHERE key IN ('log_retention_days_postgres', 'log_retention_days_sqlite', 'log_retention_days_clickhouse');
```

- [ ] **Step 2: SQLite 同版本号镜像**（`INSERT OR IGNORE` / `DELETE`，语义一致）。
- [ ] **Step 3: model 常量更新**——旧 key 常量删除；`validate.go` 中 `validateDatabaseCleanupOption` 替换为 `validateLogRetentionOption`（3 个新 key，值 ≥1 整数）。
- [ ] **Step 4: testhelper seed 同步**——`seedDefaultConfigs` 增 3 个新 key、删旧 key（含公共 key 列表如有）。
- [ ] **Step 5: 验证** `go test ./internal/infra/persistence/migrator ./internal/apps/config ./internal/apps/admin/system_config ./internal/testhelper`。
- [ ] **Step 6: 提交** `git add internal/ && git commit -m "feat(config): per-store log retention settings, drop legacy cleanup config"`

### Task 13: CleanupStore + system_cleanup 日志清理步骤 + PG 分区预建

> 含 Task 11 审查跟进：PG 分区表仅在建表迁移时预建当前+2 月；`CleanupExpired` 每次运行时必须先确保「当前月 + 未来 2 个月」的分区存在（幂等 `CREATE TABLE IF NOT EXISTS ... PARTITION OF`），否则 3 个月后新写入会报 "no partition of relation found"。在 `CleanupStore`（或 logstore 包内 `EnsurePartitions(ctx)`）实现，PG 方言执行、SQLite/CH 为 no-op；`system_cleanup` 每日调用保证分区持续存在。

**Files:**
- Create: `internal/repository/logstore/cleanup.go`
- Modify: `internal/apps/upload/task/cleanup.go`（追加日志清理步骤）
- Create: `internal/repository/logstore/cleanup_test.go`

**Interfaces:**
- Consumes: `model.ConfigKeyLogRetentionDays*`、`logstore.Active`。
- Produces: `CleanupExpired(ctx) (*CleanupSummary, error)`（repository 层入口，`system_cleanup` 调用）。

- [ ] **Step 1: cleanup.go**

```go
package logstore

import (
    "context"
    "fmt"
    "strconv"
    "time"

    "github.com/Rain-kl/Wavelet/internal/model"
    analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

// CleanupSummary 汇总本次清理结果。
type CleanupSummary struct {
    ActiveDatabase string `json:"active_database"`
    RetentionDays  int    `json:"retention_days"`
    Deleted        int64  `json:"deleted"`
    Tables         []string `json:"tables"`
}

// retentionDaysForActive 按当前激活库读取保留天数（默认 90）。
func retentionDaysForActive(ctx context.Context) int {
    key := model.ConfigKeyLogRetentionDaysPostgres
    if dbName, _ := resolveDatabase(ctx); dbName == "sqlite" {
        key = model.ConfigKeyLogRetentionDaysSQLite
    } else if dbName == "clickhouse" {
        key = model.ConfigKeyLogRetentionDaysClickHouse
    }
    v, err := getConfig(ctx, key)
    if err != nil {
        return 90
    }
    days, perr := strconv.Atoi(v)
    if perr != nil || days <= 0 {
        return 90
    }
    return days
}

// CleanupExpired 按当前激活库保留天数清理过期日志（每日由 system_cleanup 调用）。
func CleanupExpired(ctx context.Context) (*CleanupSummary, error) {
    s, err := Active(ctx)
    if err != nil {
        return nil, err
    }
    days := retentionDaysForActive(ctx)
    cutoff := time.Now().AddDate(0, 0, -days)
    summary := &CleanupSummary{RetentionDays: days, Tables: []string{}}
    summary.ActiveDatabase, _ = resolveDatabase(ctx)

    if err := cleanupTable(ctx, s, "node_access_logs", func() (int64, error) {
        return s.AccessLogs.DeleteBefore(ctx, cutoff)
    }, summary); err != nil {
        return nil, err
    }
    if err := cleanupTable(ctx, s, "metric_snapshots", func() (int64, error) {
        return s.Observability.DeleteMetricSnapshotsBefore(ctx, cutoff)
    }, summary); err != nil {
        return nil, err
    }
    // edge_health / obs_frps / obs_frpc 同构
    return summary, nil
}

func cleanupTable(ctx context.Context, s *Store, name string, fn func() (int64, error), summary *CleanupSummary) error {
    n, err := fn()
    if err != nil {
        return fmt.Errorf("cleanup %s: %w", name, err)
    }
    summary.Deleted += n
    summary.Tables = append(summary.Tables, name)
    return nil
}
```

> PG 实现优化（可选，首版用 DeleteBefore 即可）：`DeleteBefore` 在 PG 分区表上命中 `logged_at` 分区键，按月 DROP 整分区后再 DELETE 不满月——M1 Task 3 的 `DeleteBefore` 已按 `logged_at < cutoff` 实现，满足正确性；后续再优化为 DROP PARTITION。CH 实现：`DeleteNodeAccessLogsBefore` 已做 TTL materialize；保留天数变化时 `clickhouseLogStore.DeleteBefore` 增加 `ALTER TABLE ... MODIFY TTL`（见 M4 优化项，可延后）。

- [ ] **Step 2: system_cleanup 追加步骤（upload/task/cleanup.go）**

在现有清理步骤之后追加：

```go
task.AppendLog(ctx, "开始清理过期日志（按当前日志库保留天数）...")
summary, err := logstore.CleanupExpired(ctx)
if err != nil {
    task.AppendLog(ctx, "清理过期日志失败: %v", err)
} else if summary.Deleted == 0 {
    task.AppendLog(ctx, "没有需要清理的过期日志 (保留 %d 天)", summary.RetentionDays)
} else {
    task.AppendLog(ctx, "日志清理完成：保留 %d 天，删除 %d 条", summary.RetentionDays, summary.Deleted)
}
```

（`internal/apps/upload/task/cleanup.go` import `internal/repository/logstore`——upload/task 属 apps 层，import logstore 合法。）

- [ ] **Step 3: 单测（cleanup_test.go）**——sqlite store 写入 40 天前/昨天各 1 条，`CleanupExpired` 用 `SetConfigReader` 注入 `log_retention_days_sqlite=30`，断言 40 天前的被删、昨天的保留。
- [ ] **Step 4: 运行** `go test ./internal/repository/logstore/ ./internal/apps/upload/task/`。
- [ ] **Step 5: 提交** `git add internal/repository/logstore/ internal/apps/upload/task/ && git commit -m "feat(cleanup): log retention cleanup in system_cleanup task"`

### Task 14: 下线 of_database_auto_cleanup

**Files:**
- Create: `internal/infra/persistence/migrator/goose/postgres/202608080003_drop_database_cleanup_schedule.sql`、`sqlite/202608080003_...`
- Modify: `internal/apps/openflare/async_tasks.go`（删除 `DatabaseAutoCleanupTask`/`DatabaseAutoCleanupMeta`/`DatabaseAutoCleanupHandler`）
- Modify: `internal/infra/task/handlers/register.go`（注销）
- Modify: `internal/apps/openflare/tasks/database_cleanup.go`（删除；清理能力已并入 system_cleanup）

**Interfaces:**
- Consumes: Task 13 完成。
- Produces: `of_database_auto_cleanup` 从 schedule 与任务注册中消失。

- [ ] **Step 1: goose 删 schedule**

```sql
-- +goose Up
DELETE FROM w_schedules WHERE task_type = 'of_database_auto_cleanup';
-- +goose Down
INSERT INTO w_schedules (id, name, task_type, cron, payload, is_active, created_at, updated_at)
VALUES (102, 'OpenFlare 可观测数据自动清理', 'of_database_auto_cleanup', '0 3 * * *', '{}', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;
```

- [ ] **Step 2: 注销任务与删除文件**——`register.go` 移除对应两行；`async_tasks.go` 删除常量/元数据/Handler；删除 `tasks/database_cleanup.go`。
- [ ] **Step 3: 前端清理**——搜索前端对 `of_database_auto_cleanup` / `database_auto_cleanup_*` 引用并删除（任务页硬编码列表如有）。
- [ ] **Step 4: 验证** `go build ./internal/...`、`go test ./internal/infra/persistence/migrator ./internal/infra/task/`。
- [ ] **Step 5: 提交** `git add internal/ frontend/ && git commit -m "chore(cleanup): decommission of_database_auto_cleanup task and schedule"`

---

## M3：迁移任务与展示

### Task 15: 「切换日志数据库」任务 Handler

**Files:**
- Create: `internal/apps/openflare/tasks/log_db_switch.go`
- Create: `internal/apps/openflare/tasks/log_db_switch_test.go`
- Modify: `internal/apps/openflare/async_tasks.go`（注册元数据）
- Modify: `internal/infra/task/handlers/register.go`（注册 Handler）

**Interfaces:**
- Consumes: `logstore.Active`/`logstore.Migrating`、`repository.UpdateSystemConfigFields`、`model.ConfigKeyLogDatabase`/`ConfigKeyLogDBMigration`、`analyticsmodel.*`、`config.Config`。
- Produces: Asynq `openflare:log_db_switch`，管理类型 `of_log_db_switch`，参数 `target`。

- [ ] **Step 1: 元数据（async_tasks.go）**

```go
// LogDBSwitchTask 切换日志数据库任务标识。
const (
    LogDBSwitchTask   = "openflare:log_db_switch"
    TaskTypeLogDBSwitch = "of_log_db_switch"
)

var LogDBSwitchMeta = task.TaskMeta{
    Type:         TaskTypeLogDBSwitch,
    AsynqTask:    LogDBSwitchTask,
    Name:         "切换日志数据库",
    Description:  "复制迁移日志数据并在成功后切换日志主库（期间禁止日志写入）",
    SupportsTime: false,
    MaxRetry:     task.DefaultMaxRetry,
    Queue:        task.QueueDefault,
    Retryable:    true,
    Params: []task.TaskParam{
        {Name: "target", Label: "目标日志库", Type: "string", Required: true,
         Placeholder: "postgres|sqlite|clickhouse", Description: "迁移目标：postgres（主库为 PG 时）、sqlite（主库为 SQLite 时）或 clickhouse"},
    },
}
```

- [ ] **Step 2: Handler（log_db_switch.go）**

```go
package tasks

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "time"

    "github.com/Rain-kl/Wavelet/internal/infra/config"
    "github.com/Rain-kl/Wavelet/internal/infra/task"
    "github.com/Rain-kl/Wavelet/internal/model"
    analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
    "github.com/Rain-kl/Wavelet/internal/repository"
    "github.com/Rain-kl/Wavelet/internal/repository/logstore"
    "github.com/Rain-kl/Wavelet/pkg/logger"
)

const copyBatchSize = 1000

type logDBSwitchPayload struct {
    Target string `json:"target"`
}

// LogDBSwitchHandler 切换日志数据库任务处理器。
type LogDBSwitchHandler struct{}

// ValidatePayload 校验并规范化参数。
func (h *LogDBSwitchHandler) ValidatePayload(payload []byte) ([]byte, error) {
    var p logDBSwitchPayload
    if err := json.Unmarshal(payload, &p); err != nil {
        return nil, fmt.Errorf("参数解析失败: %w", err)
    }
    p.Target = normalizeTarget(p.Target)
    if !validTarget(p.Target) {
        return nil, fmt.Errorf("目标日志库不合法: %s", p.Target)
    }
    out, err := json.Marshal(p)
    if err != nil {
        return nil, err
    }
    return out, nil
}

func normalizeTarget(v string) string {
    switch v {
    case "postgres", "postgresql":
        return "postgres"
    case "sqlite", "sqlite3":
        return "sqlite"
    case "clickhouse", "ch":
        return "clickhouse"
    }
    return v
}

func validTarget(v string) bool {
    return v == "postgres" || v == "sqlite" || v == "clickhouse"
}

// Execute 执行迁移。
func (h *LogDBSwitchHandler) Execute(ctx context.Context, payload []byte) (*task.TaskResult, error) {
    var p logDBSwitchPayload
    if err := json.Unmarshal(payload, &p); err != nil {
        return nil, fmt.Errorf("参数解析失败: %w", err)
    }
    p.Target = normalizeTarget(p.Target)
    if err := validateSwitch(ctx, p.Target); err != nil {
        return nil, err
    }

    source, _ := currentLogDatabase(ctx)
    task.AppendLog(ctx, "开始切换日志数据库：%s -> %s", source, p.Target)
    if err := setMigrationFlag(ctx, "migrating"); err != nil {
        return nil, err
    }
    defer func() { _ = setMigrationFlag(ctx, "") }() // 失败也清除，保持源库可写

    if err := drainLogWriters(ctx); err != nil {
        return nil, fmt.Errorf("排空日志写入队列失败: %w", err)
    }

    src, err := logstore.Active(ctx)
    if err != nil {
        return nil, err
    }
    dst, err := buildTargetStore(ctx, p.Target)
    if err != nil {
        return nil, err
    }

    // 清空目标库日志表（幂等重试前提）。
    if err := clearTargetLogTables(ctx, dst, p.Target); err != nil {
        return nil, err
    }

    // 逐表复制。
    if err := copyAccessLogs(ctx, src, dst); err != nil {
        return nil, err
    }
    if err := copyUserAccessLogs(ctx, src, dst); err != nil {
        return nil, err
    }
    if err := copyObservability(ctx, src, dst); err != nil {
        return nil, err
    }

    // 翻转主库标记。
    if err := flipLogDatabase(ctx, p.Target); err != nil {
        return nil, err
    }
    task.AppendLog(ctx, "日志数据库已切换为 %s，写入恢复", p.Target)
    return &task.TaskResult{Message: fmt.Sprintf("日志数据库已从 %s 切换为 %s", source, p.Target)}, nil
}
```

- [ ] **Step 3: 辅助函数（同文件）**

```go
func validateSwitch(ctx context.Context, target string) error {
    source, err := currentLogDatabase(ctx)
    if err != nil {
        return err
    }
    if source == target {
        return errors.New("目标日志库与当前日志库相同，无需迁移")
    }
    switch target {
    case "clickhouse":
        if !config.Config.ClickHouse.Enabled {
            return errors.New("ClickHouse 未启用，无法迁移到 ClickHouse")
        }
    case "postgres":
        if !config.Config.Database.Enabled {
            return errors.New("PostgreSQL 未启用（当前主库为 SQLite），无法迁移到 PostgreSQL")
        }
    case "sqlite":
        if config.Config.Database.Enabled {
            return errors.New("当前主库为 PostgreSQL，日志库不能设置为 SQLite")
        }
    }
    return nil
}

func currentLogDatabase(ctx context.Context) (string, error) {
    cfg, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyLogDatabase)
    if err != nil {
        return "", fmt.Errorf("读取日志主库失败: %w", err)
    }
    if cfg.Value == "" {
        return "", errors.New("日志主库配置为空")
    }
    return cfg.Value, nil
}

func setMigrationFlag(ctx context.Context, v string) error {
    // 必须用 SaveOrUpdateSystemConfig：UpdateSystemConfigFields 缺行时静默 no-op，
    // 且不失效 RAM 配置缓存（TTL=-1 永不过期），会导致冻结/翻转不生效、进程间脑裂。
    return repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyLogDBMigration, v)
}

func flipLogDatabase(ctx context.Context, target string) error {
    return repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyLogDatabase, target)
}

// buildTargetStore 构造目标库 Store（不经过 Active 缓存，直接 Build）。
func buildTargetStore(ctx context.Context, database string) (*logstore.Store, error) {
    return logstore.Build(ctx, database)
}

func clearTargetLogTables(ctx context.Context, dst *logstore.Store, target string) error {
    // 依次清空 6 张表：AccessLogs.DeleteAll、UserAccessLogs.DeleteAll、Observability.DeleteAll*（SQLite/PG 用 DeleteAll；CH 用 TRUNCATE 语义）。
    if _, err := dst.AccessLogs.DeleteAll(ctx); err != nil {
        return fmt.Errorf("清空目标访问日志失败: %w", err)
    }
    if _, err := dst.UserAccessLogs.DeleteAll(ctx); err != nil {
        return fmt.Errorf("清空目标用户访问日志失败: %w", err)
    }
    for _, fn := range []func(context.Context) (int64, error){
        dst.Observability.DeleteAllMetricSnapshots,
        dst.Observability.DeleteAllEdgeHealth,
        dst.Observability.DeleteAllNodeObservationFrps,
        dst.Observability.DeleteAllNodeObservationFrpc,
    } {
        if _, err := fn(ctx); err != nil {
            return err
        }
    }
    return nil
}

// copyAccessLogs 从 src 复制节点访问日志到 dst。
func copyAccessLogs(ctx context.Context, src, dst *logstore.Store) error {
    // 注意：迁移期间 src 已冻结，但复制读取不受冻结影响；每批按 id 升序扫描。
    var lastID uint64
    for {
        rows, err := listNodeAccessLogsByID(ctx, src, lastID, copyBatchSize)
        if err != nil {
            return err
        }
        if len(rows) == 0 {
            break
        }
        if err := dst.AccessLogs.BatchInsertNodeAccessLogs(ctx, rows); err != nil {
            return fmt.Errorf("写入目标访问日志失败(批 %d): %w", lastID, err)
        }
        task.AppendLog(ctx, "已复制访问日志 %d 条（截至 id=%d）", len(rows), rows[len(rows)-1].ID)
        lastID = rows[len(rows)-1].ID
        if len(rows) < copyBatchSize {
            break
        }
    }
    return nil
}
```

> `ListForMigration` 已在 Task 2 接口定义：GORM 实现 `Where("id > ?", afterID).Order("id ASC").Limit(limit)`；CH 实现原生 SQL `SELECT ... FROM of_node_access_logs WHERE id > ? ORDER BY id LIMIT ?`。可观测 4 表的 `*ForMigration` 同理（按各自表名/模型）。

```go
// copyObservability 复制 4 张可观测表。
func copyObservability(ctx context.Context, src, dst *logstore.Store) error {
    for _, c := range []struct {
        name string
        read func(ctx context.Context, afterID uint64, limit int) (int, error)
    }{
        {"metric_snapshots", func(ctx context.Context, afterID uint64, limit int) (int, error) {
            rows, err := src.Observability.ListMetricSnapshotsForMigration(ctx, afterID, limit)
            if err != nil || len(rows) == 0 {
                return len(rows), err
            }
            return len(rows), dst.Observability.BatchInsertNodeMetricSnapshots(ctx, rows)
        }},
        // edge_health / obs_frps / obs_frpc 同构，调用各自 ForMigration/BatchInsert 对。
    } {
        var lastID uint64
        for {
            n, err := c.read(ctx, lastID, copyBatchSize)
            if err != nil {
                return fmt.Errorf("复制 %s 失败: %w", c.name, err)
            }
            if n == 0 {
                break
            }
            task.AppendLog(ctx, "已复制 %s %d 条", c.name, n)
            if n < copyBatchSize {
                break
            }
            lastID += uint64(n) // 近似游标；实现时改为每批最后一条 id 更精确
        }
    }
    return nil
}
```

- [ ] **Step 4: 注册**——`register.go` 加 `task.RegisterHandler(openflare.LogDBSwitchTask, &openflare.LogDBSwitchHandler{})` + `task.RegisterTaskMeta(openflare.LogDBSwitchMeta)`。
- [ ] **Step 5: 单测（log_db_switch_test.go）**——sqlite↔sqlite 模拟（源 store 写入 3 条，目标 store 空库），执行 `copyAccessLogs` 断言 ID 保留、数量一致；`validateSwitch` 各非法组合报错；`ValidatePayload` 归一化。
- [ ] **Step 6: 运行** `go test ./internal/apps/openflare/tasks/ ./internal/infra/task/`。
- [ ] **Step 7: 提交** `git add internal/apps/openflare/ internal/infra/task/ && git commit -m "feat(task): add switch log database migration task"`

### Task 16: 日志库状态端点

**Files:**
- Modify: `internal/apps/admin/status/clickhouse.go`（改造为 `log-database` 状态端点，保留旧路径兼容或重命名 + 路由更新）
- Modify: `internal/router/v1/admin.go`（路由注册）
- Modify: `internal/apps/admin/status/swagger` 注释

**Interfaces:**
- Consumes: `logstore.Active`、`logstore.Migrating`、`repository.GetIntByKey`（3 个保留配置）、`config.Config`。
- Produces: `GET /api/v1/admin/status/log-database` 返回 `LogDatabaseStatus`。

- [ ] **Step 1: 实现状态结构（改造 clickhouse.go）**

```go
// GetLogDatabaseStatus 返回当前日志库状态。
// @Summary 获取日志数据库状态
// @Description 返回当前日志主库、迁移状态、各库保留天数与合法迁移目标，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=status.LogDatabaseStatus} "获取成功"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/status/log-database [get]
func GetLogDatabaseStatus(c *gin.Context) {
    ctx := c.Request.Context()
    s, err := logstore.Active(ctx)
    if err != nil {
        response.AbortInternal(c, "日志存储初始化失败")
        return
    }
    activeDB, _ := logstore.ActiveDatabase(ctx) // provider 增加 ActiveDatabase(ctx) 返回当前库名
    migration := "idle"
    if logstore.Migrating(ctx) {
        migration = "migrating"
    }
    out := LogDatabaseStatus{
        ActiveDatabase: activeDB,
        Migration:      migration,
        RetentionDays: map[string]int{
            "postgres":   retentionOr(ctx, model.ConfigKeyLogRetentionDaysPostgres),
            "sqlite":     retentionOr(ctx, model.ConfigKeyLogRetentionDaysSQLite),
            "clickhouse": retentionOr(ctx, model.ConfigKeyLogRetentionDaysClickHouse),
        },
        AvailableTargets: availableTargets(ctx),
    }
    if activeDB == "clickhouse" {
        stats, err := analyticsrepo.GetClickHouseOperationalStats(ctx) // 经 logstore StatusStore 暴露
        if err == nil {
            stats.BatchWriters = collectBatchWriterStats()
            out.ClickHouse = stats
        }
    }
    c.JSON(http.StatusOK, response.OK(out))
}
```

> `logstore.ActiveDatabase(ctx)` 与 `logstore.Build(ctx, database)`（Task 15 用到）需在 provider 增加并实现；`analyticsrepo.GetClickHouseOperationalStats` 改为经 `logstore.StatusStore` 暴露，避免 admin/status import analyticsrepo（违反 import-lint）。

- [ ] **Step 2: 路由**——`internal/router/v1/admin.go` 将 `/status/clickhouse` 替换/新增为 `/status/log-database`；旧路径保留 301 或删除（实现时选删除并同步前端）。
- [ ] **Step 3: 单测**——`logstore.ActiveDatabase`/`Build` 分支测试；`availableTargets`（当前=clickhouse → 主库；当前=主库 → clickhouse）。
- [ ] **Step 4: swagger** `make swagger`。
- [ ] **Step 5: 验证** `go test ./internal/apps/admin/status/`、`go build ./internal/...`。
- [ ] **Step 6: 提交** `git add internal/apps/admin/ internal/router/ && git commit -m "feat(status): log database status endpoint"`

### Task 17: 前端——任务参数、业务配置、状态展示

**Files:**
- Modify: `frontend/lib/services/admin/*`（任务/状态类型，若需）
- Modify: `frontend/components/common/settings/operation-tab.tsx` 或业务配置分组（「日志保留时间」）
- Modify: 任务管理页组件（`frontend/.../tasks.tsx` 或等价文件）——展示当前日志主库 + 迁移状态 + 「切换日志数据库」参数下拉
- Modify: 状态页/仪表盘（日志库状态卡片）

**Interfaces:**
- Consumes: 现有 Admin 任务派发 API、`/api/v1/admin/status/log-database`、`AdminService.updateSystemConfig`。

- [ ] **Step 1: 业务配置分组**——在 `/admin/settings` 业务配置 Tab 新增「日志保留时间」：3 个 `Input type="number"`（PG/SQLite/CH），保存调 `AdminService.updateSystemConfig`，成功后 invalidate `["admin","system-configs"]`，Sonner toast。
- [ ] **Step 2: 任务管理页**——「切换日志数据库」出现在任务列表；参数 `target` 下拉按状态端点 `available_targets` 渲染（显示「PostgreSQL（主库）」/「SQLite（主库）」/「ClickHouse」）；任务卡片显示 `active_database` 与迁移状态徽标。
- [ ] **Step 3: 状态卡片**——仪表盘或任务页展示当前日志主库、保留天数、迁移中提示。
- [ ] **Step 4: 验证** `cd frontend && pnpm build`（或 `pnpm lint`）。
- [ ] **Step 5: 提交** `git add frontend/ && git commit -m "feat(frontend): log database status, retention settings, and switch task UI"`

---

## M4：收尾与全量验证

### Task 18: 全量验证、文档与 changelog

**Files:**
- Modify: `docs/changelog/index.md`（`[Unreleased]` 中文条目）
- Modify: `docs/design/`（如需要，日志数据库解耦设计说明）
- 全局验证

- [ ] **Step 1: 全量检查** 运行：
  - `go build ./...`
  - `go test ./...`
  - `make code-check`
  - `make swagger`（若 API 有变）
  - `make format`
  - goose 三套空库 Up 验证（`go test ./internal/infra/persistence/migrator`）
- [ ] **Step 2: changelog**——在 `docs/changelog/index.md` 的 `[Unreleased]` 增加合并条目：

```markdown
- 日志存储解耦：新增日志存储抽象（`internal/repository/logstore`），ClickHouse 变为可选项，不启用时由 PostgreSQL/SQLite 承担全部日志功能；新增「切换日志数据库」任务支持 PostgreSQL/SQLite 与 ClickHouse 间数据迁移（迁移期间冻结日志写入，成功后自动切换主库并保留源数据）；日志保留时间改为按存储库在业务配置中设置（`log_retention_days_*`），过期清理并入系统垃圾清理每日任务。
```

- [ ] **Step 3: 设计文档归档**——确认 `docs/superpowers/specs/2026-08-08-log-database-decoupling-design.md` 与计划一致；实现偏差在 spec 或 changelog 标注。
- [ ] **Step 4: 提交** `git add docs/ && git commit -m "docs: log database decoupling changelog and design notes"`

---

## 自检记录（writing-plans self-review）

- **规格覆盖**：M1 Task 1-10 覆盖规格第 4 节（包结构/接口/约束/标记校验）；M2 Task 11-14 覆盖第 5、6 节（表/优化/清理）；M3 Task 15-17 覆盖第 7、8 节（迁移任务/API/前端）；M4 Task 18 覆盖第 9 节（测试验证）与文档。
- **已知实现决策（由实现者按此执行，避免歧义）**：
  1. `logstore` 不 import `internal/repository`（防循环）；配置读取经 bootstrap 注入 `SetConfigReader`。
  2. 迁移复制按 id 升序扫描：`AccessLogStore.ListForMigration` + 可观测 4 个 `*ForMigration`（Task 2 已定义），CH 与 GORM 各自实现；`copyObservability` 用每批最后一条 id 作为下一批游标（实现时修正计划里 `lastID += n` 的近似写法）。
  3. `logstore.Build(ctx, database)` 导出供迁移任务构造目标 store；`ActiveDatabase(ctx)` 供状态端点。
  4. admin/status 不直接 import analyticsrepo——CH 运行指标经 `logstore.StatusStore` 暴露。
  5. 解析 helper（`ParseBrowserName` 等）迁至 `model/analytics` 纯函数，apps 不再依赖 analyticsrepo。
  6. 迁移期间源库冻结由 logstore 各实现 `ensureWritable` 统一保证；risk_control 审计中间件在冻结期跳过写日志但不阻断请求。
  7. 失败回退：`defer setMigrationFlag("")` 保证失败后源库恢复可写；重试时先清空目标再复制（幂等）。
