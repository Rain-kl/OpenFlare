# ClickHouse CPU 性能优化计划

> PLAN_ID: `63ba981b`  
> 状态: 执行中  
> 目标: 完成 P0–P2 优化，降低 ClickHouse CPU 占用

## 背景

ClickHouse CPU 偏高由写入侧（小 part 频繁 flush、心跳同步 DELETE mutation）与查询侧（无 LIMIT 全表扫、高频轮询、WAF 全量拉日志）叠加导致。

## PR Plan

### PR 1: 写入路径 P0 优化

- **Description:** 移除心跳路径同步 `ALTER DELETE`；为 `batchwriter` 增加 `MinBatchSize`；调大可观测 writer 批次与 flush 间隔；为 openresty/frps/frpc 补全去重。
- **Files/components affected:** `internal/apps/openflare/agent/observability.go`, `internal/db/batchwriter/`, `internal/apps/openflare/chwriter/`, `internal/db/batchwriter/*_test.go`
- **Dependencies:** None

### PR 2: ClickHouse 客户端与配置 P1

- **Description:** 启用 `async_insert` 等写入优化 settings；提高 `block_buffer_size` 默认值；更新 `config.example.yaml` 与配置模型注释。
- **Files/components affected:** `internal/db/clickhouse.go`, `internal/config/model.go`, `internal/config/config.go`, `config.example.yaml`
- **Dependencies:** None

### PR 3: Dashboard 与可观测查询 P0

- **Description:** 消除 `limit=0` 无界查询；复用已有限制数据构建趋势；增加服务端短 TTL 缓存；降低前端轮询频率。
- **Files/components affected:** `internal/apps/openflare/dashboard/logics.go`, `internal/apps/openflare/observability/node_logics.go`, `frontend/app/(main)/page.tsx`, `frontend/app/(main)/nodes/components/node-observability.tsx`
- **Dependencies:** None

### PR 4: 访问日志与 WAF 查询 P0/P1

- **Description:** WAF IP 同步改为 ClickHouse 侧聚合；IP 汇总与折叠日志 SQL 分页；消除 count 重复全量扫描；列表 API 强制默认时间窗口。
- **Files/components affected:** `internal/apps/openflare/waf/ip_group_sync.go`, `internal/repository/analytics/node_access_log_stats.go`, `internal/model/openflare_access_log.go`, `internal/apps/openflare/observability/access_log_logics.go`, `internal/repository/analytics/access_log_stats.go`
- **Dependencies:** None

### PR 5: ClickHouse DDL 与数据规范化 P1

- **Description:** 为 7 张分析表添加 TTL；收窄 `of_node_access_logs` ORDER BY；插入时规范化 `remote_addr`（去 trim 查询）；将可观测 obs 三表纳入自动清理。
- **Files/components affected:** `internal/db/migrator/goose/clickhouse/`, `internal/repository/analytics/node_access_log_writer.go`, `internal/apps/openflare/tasks/database_cleanup.go`, `internal/model/analytics/`
- **Dependencies:** PR 1

### PR 6: 基础设施与审计减负 P2

- **Description:** Docker ClickHouse 服务端基础调优；审计日志 headers 截断/精简；更新 changelog。
- **Files/components affected:** `docker-compose.yaml`, `docker/clickhouse/` (if needed), `internal/apps/risk_control/middleware.go`, `docs/changelog/index.md`
- **Dependencies:** None