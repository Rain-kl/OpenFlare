# Agent 上报协议与观测落库数据模型

你会学到：重构后 Agent 心跳/WS 上报的 **数据结构**、Server **如何解析与写入**、ClickHouse / 关系库 **目标表结构**，以及与旧字段/旧表的兼容关系。

本设计是 [边缘可观测与业务流量统计重构](./observability-design.md) 的 **协议与存储专章**，实现时以本文字段与 DDL 为准。

**先读传输全景与示例：** [观测数据传输模型](./observability-transport-model.md)。

---

## 1. 设计目标

| 目标 | 说明 |
| --- | --- |
| Agent 只报事实 | 明细 + 主机读数 + 边缘健康瞬时态；无业务预聚合 |
| 一张业务明细表 | 访问日志是 L1 唯一写入路径 |
| 聚合在库内/控制面 | 小时汇总由 ClickHouse MV 或查询生成，Agent 不写汇总表 |
| 字段不重叠 | `bytes_sent` = 已提供数据；网卡 `network_*` = 宿主机；不再有业务 `openresty_tx` |
| 可演进 | 新字段可选；旧 Agent 缺字段时 Server 填默认值 |

---

## 2. 分层与写入总览

```text
                    Agent NodePayload (v2)
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
         access_logs     host_metrics    edge_health
         (L1 明细)       (L3 读数)       (L2 瞬时)
              │               │               │
              ▼               ▼               ▼
     of_node_access_logs  of_node_metric_  of_node_edge_health
              │           snapshots              │
              │               │                  │
              ▼               ▼                  │
     of_access_log_hourly  of_node_metric_       │
     (MV, Server 侧)       capacity_hourly (MV)  │
              │               │                  │
              └─────── 管理端聚合 API ───────────┘

关系库 (PostgreSQL/SQLite)：节点最新状态、Profile、健康事件（非明细湖）
```

| 层 | 含义 | Agent 上报块 | ClickHouse 事实表 |
| --- | --- | --- | --- |
| L1 | 业务交付 | `access_logs` | `of_node_access_logs` |
| L2 | 边缘健康 | `edge_health` | `of_node_edge_health` |
| L3 | 宿主机资源 | `host_metrics` | `of_node_metric_snapshots` |

---

## 3. Agent 上报数据结构（协议 v2）

### 3.1 顶层 `NodePayload`

传输：HTTP 心跳 body 与 WebSocket `status` 消息共用同一结构。

```json
{
  "schema_version": 2,
  "node_id": "n_xxx",
  "name": "edge-1",
  "ip": "1.2.3.4",
  "version": "3.3.0",
  "ext_version": "",
  "current_version": "cfg-checksum-or-version",
  "last_error": "",
  "profile": { },
  "host_metrics": { },
  "edge_health": { },
  "access_logs": [ ],
  "buffered": [ ],
  "health_events": [ ],
  "waf_ip_group_checksums": { "1": "md5..." }
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `schema_version` | int | 建议 | `2` = 本设计；缺省或 `0/1` 按旧协议兼容解析 |
| `node_id` | string | ✅ | 节点 ID |
| `name` | string | ✅ | 显示名 |
| `ip` | string | ✅ | 上报 IP |
| `version` / `ext_version` | string | ✅ | Agent 版本 |
| `current_version` | string | | 本地激活配置版本摘要 |
| `last_error` | string | | 最近同步/运行错误，可空 |
| `profile` | object | | 主机概况，变化时上报（可节流） |
| `host_metrics` | object | 建议每拍 | L3 资源快照 |
| `edge_health` | object | 建议每拍 | L2 OpenResty 健康 |
| `access_logs` | array | | 本拍增量访问明细 |
| `buffered` | array | | 离线补传的事实批次（见 §3.6） |
| `health_events` | array | | 边缘健康事件 |
| `waf_ip_group_checksums` | map | | 差分同步用，非观测湖 |

**协议 v2 删除（不再作为权威，兼容期可忽略）：**

| 旧字段 | 处置 |
| --- | --- |
| `traffic_report` | 忽略，不落业务表 |
| `openresty_observation.openresty_rx_bytes` / `tx` | 忽略 |
| `openresty_status` / `openresty_message`（顶层） | 迁入 `edge_health`；兼容期从旧字段回填 |
| `snapshot` | 重命名为 `host_metrics`；兼容期别名读取 |
| `buffered_observability` | 重命名为 `buffered`；结构见 §3.6 |

### 3.2 `profile` — 主机概况（低频）

对应关系库 `of_node_system_profiles`（或现有等价表），**不进 ClickHouse 明细湖**。

```json
{
  "hostname": "edge-1",
  "os_name": "linux",
  "os_version": "...",
  "kernel_version": "...",
  "architecture": "amd64",
  "cpu_model": "...",
  "cpu_cores": 8,
  "total_memory_bytes": 16106127360,
  "total_disk_bytes": 107374182400,
  "uptime_seconds": 864000,
  "reported_at_unix": 1720000000
}
```

| 字段 | 语义 |
| --- | --- |
| 硬件/OS 描述字段 | 事实读数 |
| `reported_at_unix` | Agent 采集时刻（UTC 秒） |

### 3.3 `host_metrics` — 宿主机资源（L3）

**全部为读数，不做 24h 业务总量。**  
网卡/磁盘字节为 **内核累计计数器原值**（单调递增，重启可归零）；CPU 为瞬时百分比；内存/磁盘占用为当前用量。

```json
{
  "captured_at_unix": 1720000000,
  "cpu_usage_percent": 12.5,
  "memory_used_bytes": 4294967296,
  "memory_total_bytes": 16106127360,
  "storage_used_bytes": 50000000000,
  "storage_total_bytes": 107374182400,
  "disk_read_bytes": 9000000000,
  "disk_write_bytes": 12000000000,
  "network_rx_bytes": 500000000000,
  "network_tx_bytes": 800000000000
}
```

| 字段 | 类型 | 语义 | Server 如何用 |
| --- | --- | --- | --- |
| `captured_at_unix` | int64 | 采样时刻 | `captured_at` |
| `cpu_usage_percent` | float | 瞬时 CPU% | 直接存；趋势取平均 |
| `memory_*` / `storage_*` | int64 | 当前用量/总量 | 直接存；算占用率 |
| `disk_read_bytes` / `disk_write_bytes` | int64 | **累计** IO 字节 | 存原值；查询时相邻差分 |
| `network_rx_bytes` / `network_tx_bytes` | int64 | **累计** 网卡字节 | 存原值；查询时相邻差分 →「宿主机网卡入/出站」 |

> Agent **禁止** 在上报前对网卡/磁盘做「本周期增量」替换累计值（否则 Server 差分会错）。

### 3.4 `edge_health` — OpenResty 边缘健康（L2）

**仅瞬时态，不包含业务吞吐。**

```json
{
  "captured_at_unix": 1720000000,
  "status": "healthy",
  "message": "",
  "connections": 42
}
```

| 字段 | 类型 | 语义 |
| --- | --- | --- |
| `status` | string | `healthy` / `unhealthy` / `unknown` |
| `message` | string | 状态说明 |
| `connections` | int64 | stub_status Active connections |

`status`/`message` 同步更新关系库节点最新状态；`connections` 写入 CH `of_node_edge_health` 供节点详情曲线（可选）。

### 3.5 `access_logs[]` — 访问明细（L1，业务唯一事实）

Agent：tail access.log → 解析 JSON 行 → 原样字段上报（可截断 path）。

```json
{
  "logged_at_unix": 1720000001,
  "remote_addr": "203.0.113.10",
  "host": "www.example.com",
  "path": "/api/v1/ping",
  "status_code": 200,
  "bytes_sent": 1024,
  "request_length": 128,
  "request_time_ms": 15
}
```

| 字段 | 类型 | 必填 | 来源（OpenResty） | 业务含义 |
| --- | --- | --- | --- | --- |
| `logged_at_unix` | int64 | ✅ | `$time_iso8601` 解析 | 请求完成时间 |
| `remote_addr` | string | ✅ | `$remote_addr` | 客户端 IP → UV |
| `host` | string | ✅ | `$host` | 域名 → Zone 归属 |
| `path` | string | ✅ | `$request_uri`，Agent 可截断 | 路径 |
| `status_code` | int | ✅ | `$status` | 状态码 |
| `bytes_sent` | int64 | ✅ | **`$body_bytes_sent`** | **已提供数据**（响应体） |
| `request_length` | int64 | 建议 | `$request_length` | **接收数据**；旧 Agent 缺省 0 |
| `request_time_ms` | int64 | 可选 | `$request_time * 1000` | 耗时；缺省 0 |

**明确不由 Agent 上报（由 Server 写入）：**

* `region` / 国家：入库时 GeoIP 解析  
* `id` / `created_at`：Server 生成  
* `node_id`：取自 payload / 鉴权上下文  

**单次心跳条数建议：**

* 软上限例如 2000 条/拍；超出进入 `buffered` 下一批，**禁止** 在 Agent 压成 TrafficReport。

### 3.6 `buffered[]` — 离线补传（只装事实）

```json
{
  "captured_at_unix": 1719999900,
  "host_metrics": { },
  "edge_health": { },
  "access_logs": [ ]
}
```

| 字段 | 说明 |
| --- | --- |
| `captured_at_unix` | 该批次采集/缓冲时刻，用于 ack 与去重窗口 |
| `host_metrics` / `edge_health` / `access_logs` | 与主 payload 同结构；可省略空块 |

**禁止** 在 buffered 中携带 `traffic_report` 或 rx/tx 吞吐。

### 3.7 `health_events[]`

```json
{
  "event_type": "openresty_unhealthy",
  "severity": "critical",
  "message": "...",
  "triggered_at_unix": 1720000000,
  "metadata": { }
}
```

写入关系库健康事件表（现有模型即可），不进访问日志湖。

### 3.8 Go 协议草图（目标）

```go
// pkg/protocol/agent.go（目标形态，实现时替换旧类型）

type NodePayload struct {
    SchemaVersion         int                    `json:"schema_version,omitempty"`
    NodeID                string                 `json:"node_id"`
    Name                  string                 `json:"name"`
    IP                    string                 `json:"ip"`
    Version               string                 `json:"version"`
    ExtVersion            string                 `json:"ext_version"`
    CurrentVersion        string                 `json:"current_version"`
    LastError             string                 `json:"last_error"`
    Profile               *NodeSystemProfile     `json:"profile,omitempty"`
    HostMetrics           *NodeHostMetrics       `json:"host_metrics,omitempty"`
    EdgeHealth            *NodeEdgeHealth        `json:"edge_health,omitempty"`
    AccessLogs            []NodeAccessLog        `json:"access_logs,omitempty"`
    Buffered              []BufferedFacts        `json:"buffered,omitempty"`
    HealthEvents          []NodeHealthEvent      `json:"health_events"`
    WAFIPGroupChecksums   map[string]string      `json:"waf_ip_group_checksums,omitempty"`

    // Deprecated: schema_version < 2 兼容
    Snapshot              *NodeHostMetrics       `json:"snapshot,omitempty"`
    OpenrestyStatus       string                 `json:"openresty_status,omitempty"`
    OpenrestyMessage      string                 `json:"openresty_message,omitempty"`
    OpenrestyObservation  json.RawMessage        `json:"openresty_observation,omitempty"` // 仅解析 connections
    TrafficReport         json.RawMessage        `json:"traffic_report,omitempty"`        // 忽略
    BufferedObservability []BufferedFacts        `json:"buffered_observability,omitempty"`
}

type NodeHostMetrics struct {
    CapturedAtUnix    int64   `json:"captured_at_unix"`
    CPUUsagePercent   float64 `json:"cpu_usage_percent"`
    MemoryUsedBytes   int64   `json:"memory_used_bytes"`
    MemoryTotalBytes  int64   `json:"memory_total_bytes"`
    StorageUsedBytes  int64   `json:"storage_used_bytes"`
    StorageTotalBytes int64   `json:"storage_total_bytes"`
    DiskReadBytes     int64   `json:"disk_read_bytes"`
    DiskWriteBytes    int64   `json:"disk_write_bytes"`
    NetworkRxBytes    int64   `json:"network_rx_bytes"`
    NetworkTxBytes    int64   `json:"network_tx_bytes"`
}

type NodeEdgeHealth struct {
    CapturedAtUnix int64  `json:"captured_at_unix"`
    Status         string `json:"status"`
    Message        string `json:"message"`
    Connections    int64  `json:"connections"`
}

type NodeAccessLog struct {
    LoggedAtUnix  int64  `json:"logged_at_unix"`
    RemoteAddr    string `json:"remote_addr"`
    Host          string `json:"host"`
    Path          string `json:"path"`
    StatusCode    int    `json:"status_code"`
    BytesSent     int64  `json:"bytes_sent"`      // body_bytes_sent，已提供数据
    RequestLength int64  `json:"request_length"`  // 接收数据
    RequestTimeMs int64  `json:"request_time_ms"` // 可选
}

type BufferedFacts struct {
    CapturedAtUnix int64            `json:"captured_at_unix"`
    HostMetrics    *NodeHostMetrics `json:"host_metrics,omitempty"`
    EdgeHealth     *NodeEdgeHealth  `json:"edge_health,omitempty"`
    AccessLogs     []NodeAccessLog  `json:"access_logs,omitempty"`
}
```

---

## 4. Server 解析与落库流程

### 4.1 入口

* HTTP：`POST /api/v1/agent/...` 心跳（现有路径）  
* WebSocket：`type=status` payload = `NodePayload`  
* 鉴权：`X-Agent-Token` → 绑定 `node_id`（payload.node_id 必须与 token 节点一致）

### 4.2 处理流水线（单次 payload）

```text
1. 反序列化 NodePayload
2. 归一化（normalize）
   - schema_version < 2：
       host_metrics ← snapshot
       edge_health.status ← openresty_status
       edge_health.connections ← openresty_observation.connections（若有）
       traffic_report → drop
       openresty_observation.rx/tx → drop
       buffered ← buffered_observability
   - path 再截断、status 范围钳制、负数字节 → 0
3. 关系库事务（节点最新态）
   - 更新 node 在线时间、IP、版本、edge_health.status/message
   - upsert profile（若有）
   - insert health_events（若有）
4. ClickHouse 异步 batch（失败记日志，不阻断心跳响应的配置下发）
   a. access_logs + buffered[].access_logs
        → 补 region（GeoIP）
        → 分配 snowflake id
        → BatchInsert of_node_access_logs
   b. host_metrics + buffered[].host_metrics
        → of_node_metric_snapshots
   c. edge_health + buffered[].edge_health
        → of_node_edge_health（仅 connections + status 快照可选）
5. 返回心跳响应（settings / active_config / waf 差分）
6. 若使用 buffer ack：按 buffered.captured_at_unix 列表确认
```

### 4.3 归一化规则（硬约束）

| 规则 | 行为 |
| --- | --- |
| `logged_at` 超前 now+5m | 钳制为 now 或丢弃该条（实现选定一种并单测） |
| `logged_at` 早于 now−TTL | 仍可写入，依赖表 TTL 清理 |
| 空 `host` | 允许，聚合进「未归属」 |
| `bytes_sent` / `request_length` < 0 | 置 0 |
| 单批 access_logs > N | 截断并打点监控（或只入 buffer 队列），不改为预聚合 |
| 重复补传 | CH 允许少量重复行；查询用 sum 近似（不强制精确去重） |

### 4.4 字段映射表（上报 → 表）

| 上报路径 | 目标存储 | 列 |
| --- | --- | --- |
| `access_logs[]` | CH `of_node_access_logs` | 见 §5.1 |
| `host_metrics` | CH `of_node_metric_snapshots` | 见 §5.2 |
| `edge_health` | CH `of_node_edge_health` + PG node 最新状态 | 见 §5.3 / §5.6 |
| `profile` | PG `of_node_system_profiles` | 现有列 |
| `health_events` | PG 健康事件表 | 现有模型 |
| `waf_ip_group_checksums` | 不落观测表 | 同步逻辑 |
| `traffic_report`（旧） | **不写** | — |
| `openresty_rx/tx`（旧） | **不写** | — |

### 4.5 查询侧（不落新「业务出站」列）

| 产品指标 | SQL 语义（示意） |
| --- | --- |
| 已提供数据 | `sum(bytes_sent)` |
| 接收数据 | `sum(request_length)` |
| 请求数 | `count()` |
| UV | `uniqExact(remote_addr)` |
| 5xx | `countIf(status_code >= 500)` |
| 按域名/状态码/地区 | `GROUP BY host / status_code / region` |
| 宿主机网卡出站 | 对 `network_tx_bytes` 按 node 时间序非负差分后 sum |
| OpenResty 连接 | `of_node_edge_health.connections` 最新或平均 |

---

## 5. 表结构（目标 DDL）

> 引擎与 TTL 与现网一致倾向：访问日志 90 天，指标 30 天。  
> `id` 使用控制面 Snowflake/唯一 UInt64。

### 5.1 L1 事实表：`of_node_access_logs`

```sql
CREATE TABLE IF NOT EXISTS of_node_access_logs
(
    id              UInt64,
    node_id         String,
    logged_at       DateTime64(3, 'UTC'),
    remote_addr     String,
    region          String,              -- Server GeoIP 写入，Agent 不传
    host            String,
    path            String,
    status_code     Int32,
    bytes_sent      UInt64,              -- 已提供数据（body）
    request_length  UInt64 DEFAULT 0,    -- 接收数据；新增
    request_time_ms UInt32 DEFAULT 0,    -- 可选；新增
    created_at      DateTime64(3, 'UTC')
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(logged_at)
ORDER BY (node_id, logged_at, host, status_code, remote_addr)
TTL toDateTime(logged_at) + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;
```

| 列 | 类型 | 来源 |
| --- | --- | --- |
| `id` | UInt64 | Server |
| `node_id` | String | 鉴权/payload |
| `logged_at` | DateTime64(3) | `logged_at_unix` |
| `remote_addr` | String | 上报 |
| `region` | String | Server GeoIP |
| `host` | String | 上报 |
| `path` | String | 上报 |
| `status_code` | Int32 | 上报 |
| `bytes_sent` | UInt64 | 上报 → **已提供数据** |
| `request_length` | UInt64 | 上报 → **接收数据** |
| `request_time_ms` | UInt32 | 上报可选 |
| `created_at` | DateTime64(3) | Server now |

**迁移：** 现表已有 `bytes_sent`；新增：

```sql
ALTER TABLE of_node_access_logs
    ADD COLUMN IF NOT EXISTS request_length UInt64 DEFAULT 0,
    ADD COLUMN IF NOT EXISTS request_time_ms UInt32 DEFAULT 0;
```

### 5.2 L1 小时汇总（Server 侧 MV）

**禁止 Agent 写入。** 供看板/节点 24h 快速查询请求数、错误数、字节量。

**已实现选型：`SummingMergeTree` + 不含 UV 列。**

```sql
CREATE TABLE IF NOT EXISTS of_access_log_hourly
(
    node_id         String,
    hour            DateTime('UTC'),
    host            String,
    request_count   UInt64,
    error_count     UInt64,
    bytes_sent      UInt64,
    request_length  UInt64
)
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(hour)
ORDER BY (node_id, hour, host)
TTL hour + INTERVAL 90 DAY;

CREATE MATERIALIZED VIEW IF NOT EXISTS of_access_log_hourly_mv
TO of_access_log_hourly
AS
SELECT
    node_id,
    toStartOfHour(logged_at) AS hour,
    host,
    toUInt64(count()) AS request_count,
    toUInt64(countIf(status_code >= 500)) AS error_count,
    sum(bytes_sent) AS bytes_sent,
    sum(request_length) AS request_length
FROM of_node_access_logs
GROUP BY node_id, hour, host;
```

历史小时（MV 创建前已入库的明细）需一次性回填，见迁移 `202607180003_backfill_access_log_hourly.sql`（ANTI JOIN 防重）。

#### UV 策略（必须遵守）

| 场景 | 数据源 | 算法 | 说明 |
| --- | --- | --- | --- |
| **窗口总 UV**（看板汇总、节点卡片、Zone 汇总） | `of_node_access_logs` 明细 | `uniqExact(remote_addr)`（`TrafficSummary` / 节点聚合） | **唯一权威**；不可用小时 UV 相加 |
| **24h 趋势折线请求/错误/字节** | `of_access_log_hourly` 优先，缺数据回落明细桶 | `sum(request_count)` 等 | 小时路径 **不填** `unique_visitor_count`（恒为 0） |
| **24h 趋势折线分时 UV** | 仅明细桶路径 | 桶内 `uniqExact` | 走 hourly 时 UI 应展示空/0 或隐藏 UV 序列，**禁止**对小时行做 `sum(UV)` |

**为何 hourly 不存 UV：**

1. `SummingMergeTree` 只能安全合并可加和计数；`uniqExact` 跨 part 合并需要 `AggregatingMergeTree` + state，实现与查询更重。  
2. 即便存每小时 UV，对多小时窗口 **相加会严重高估**（同一 IP 跨小时重复计）。  
3. 产品「24h 独立访客」只认整窗 `uniqExact`；趋势图主序列是请求量/错误/字节，分时 UV 非主指标。

可选未来：若需要分时 UV 曲线，再单独加 `AggregatingMergeTree` 状态表或查询时对明细做 `uniqExact` 按小时 group（成本更高，不阻塞当前看板）。
### 5.3 L3 事实表：`of_node_metric_snapshots`（保留，语义明确）

```sql
CREATE TABLE IF NOT EXISTS of_node_metric_snapshots
(
    id                  UInt64,
    node_id             String,
    captured_at         DateTime64(3, 'UTC'),
    cpu_usage_percent   Float64,
    memory_used_bytes   Int64,
    memory_total_bytes  Int64,
    storage_used_bytes  Int64,
    storage_total_bytes Int64,
    disk_read_bytes     Int64,    -- 累计原值
    disk_write_bytes    Int64,
    network_rx_bytes    Int64,    -- 累计原值 → 宿主机网卡入站
    network_tx_bytes    Int64,    -- 累计原值 → 宿主机网卡出站
    created_at          DateTime64(3, 'UTC')
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(captured_at)
ORDER BY (node_id, captured_at, id)
TTL toDateTime(captured_at) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;
```

列与现网一致；**文档与 API 必须标注 network_* 为宿主机网卡累计值**。

### 5.4 L3 小时汇总：`of_node_metric_capacity_hourly`（保留）

现有 min/max 用于累计计数器小时增量近似 + CPU/内存平均。逻辑不变：

* `network_tx_max - network_tx_min` ≈ 该小时宿主机出站  
* **不得** 用于「已提供数据」

### 5.5 L2 事实表：`of_node_edge_health`（新建，替换吞吐型 openresty 表）

```sql
CREATE TABLE IF NOT EXISTS of_node_edge_health
(
    id           UInt64,
    node_id      String,
    captured_at  DateTime64(3, 'UTC'),
    status       LowCardinality(String),  -- healthy / unhealthy / unknown
    connections  Int64,
    created_at   DateTime64(3, 'UTC')
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(captured_at)
ORDER BY (node_id, captured_at, id)
TTL toDateTime(captured_at) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;
```

| 列 | 说明 |
| --- | --- |
| `status` | 瞬时健康 |
| `connections` | 当前连接数 |

**无** `openresty_rx_bytes` / `openresty_tx_bytes`。

### 5.6 关系库（节点最新态，非分析湖）

与观测湖分离，保持「最新一份」：

| 表（逻辑名） | 用途 | 关键列 |
| --- | --- | --- |
| `of_nodes`（或现节点表） | 在线、版本、IP | `last_seen_at`, `openresty_status`, `openresty_message`, `agent_version` |
| `of_node_system_profiles` | profile upsert | hostname, cpu_cores, total_memory_bytes, ... |
| 健康事件表 | `health_events` | event_type, severity, message, triggered_at |

> 具体物理表名以仓库现有 GORM 模型为准；本设计不强制改名，只强制 **不再把业务吞吐写进节点表**。

### 5.7 废弃表（停止写入 → TTL 后删除）

| 表 | 原因 | 替代 |
| --- | --- | --- |
| `of_node_request_reports` | Agent 预聚合 | `of_node_access_logs` + hourly |
| `of_node_traffic_hourly` + MV | 依赖 request_reports | `of_access_log_hourly` |
| `of_node_obs_openresty` | 含业务 rx/tx | `of_node_edge_health` |
| `of_node_openresty_hourly` + MV | 业务吞吐差分 | `of_access_log_hourly` 的 bytes_* |

Relay 专用 `of_node_obs_frps` / `of_node_obs_frpc` **保留**（非本 Agent 主路径，但同属 CH 观测）。

---

## 6. 表与协议对照总表

| 产品概念 | 协议字段 | 表.列 | 聚合 |
| --- | --- | --- | --- |
| 已提供数据 | `access_logs[].bytes_sent` | `of_node_access_logs.bytes_sent` | `sum` |
| 接收数据 | `access_logs[].request_length` | `...request_length` | `sum` |
| 请求数 | 行数 | — | `count` |
| UV（窗口总） | `remote_addr` | 同左明细 | `uniqExact`（**禁止** sum 小时 UV） |
| Top 域名 | `host` | 同左 | `group by` |
| 状态码分布 | `status_code` | 同左 | `group by` |
| 来源地区 | — | `region`（Server） | `group by` |
| 宿主机网卡出站 | `host_metrics.network_tx_bytes` | `of_node_metric_snapshots.network_tx_bytes` | 时间序差分 |
| 宿主机网卡入站 | `network_rx_bytes` | 同左 | 差分 |
| 磁盘读/写 | `disk_*_bytes` | 同左 | 差分 |
| CPU/内存 | 瞬时字段 | 同左 | avg |
| OpenResty 连接 | `edge_health.connections` | `of_node_edge_health.connections` | 最新/avg |
| OpenResty 健康 | `edge_health.status` | 节点表 + 可选 CH | 最新 |

**不再存在的映射：**

| 旧概念 | 旧字段 | 处置 |
| --- | --- | --- |
| OpenResty 出站 | `openresty_tx_bytes` | 删除；用已提供数据 |
| OpenResty 入站 | `openresty_rx_bytes` | 删除；用接收数据 |
| 窗口请求报告 | `traffic_report` | 删除 |

---

## 7. OpenResty 日志格式（与明细对齐）

目标 `log_format`（与现网一致，保证 `bytes_sent` 键 = body）：

```nginx
log_format openflare_json escape=json
  '{"ts":"$time_iso8601","host":"$host","path":"$request_uri",'
  '"remote_addr":"$remote_addr","status":$status,'
  '"request_time":$request_time,'
  '"bytes_sent":$body_bytes_sent,"request_length":$request_length}';
```

Agent 解析：

* `ts` → `logged_at_unix`  
* `bytes_sent` → 协议 `bytes_sent`（已提供）  
* `request_length` → 协议 `request_length`  
* `request_time` → 可选 `request_time_ms = round(sec * 1000)`

---

## 8. 兼容策略（协议 v1 → v2）

| 客户端 | Server 行为 |
| --- | --- |
| 新 Agent `schema_version=2` | 按本文写入 L1/L2/L3 |
| 旧 Agent 带 `snapshot` + `access_logs` | 映射为 host_metrics；access_logs 无 request_length 则 0 |
| 旧 Agent 带 `traffic_report` | **丢弃** |
| 旧 Agent 带 `openresty_observation` | 只取 `connections` + 顶层 status；rx/tx 丢弃 |
| 读路径 | 业务 API **只读** access_logs（及 hourly）；不再读 request_reports / openresty 吞吐 |

---

## 9. 示例：一次心跳的落库结果

**Agent 上报（节选）：**

```json
{
  "schema_version": 2,
  "node_id": "n1",
  "host_metrics": {
    "captured_at_unix": 1720000000,
    "cpu_usage_percent": 10,
    "memory_used_bytes": 1,
    "memory_total_bytes": 2,
    "storage_used_bytes": 3,
    "storage_total_bytes": 4,
    "disk_read_bytes": 100,
    "disk_write_bytes": 200,
    "network_rx_bytes": 1000,
    "network_tx_bytes": 2000
  },
  "edge_health": {
    "captured_at_unix": 1720000000,
    "status": "healthy",
    "message": "",
    "connections": 5
  },
  "access_logs": [
    {
      "logged_at_unix": 1720000001,
      "remote_addr": "1.1.1.1",
      "host": "a.example.com",
      "path": "/",
      "status_code": 200,
      "bytes_sent": 500,
      "request_length": 80
    }
  ]
}
```

**写入：**

1. `of_node_metric_snapshots` 1 行（network_tx=2000 累计）  
2. `of_node_edge_health` 1 行（connections=5）  
3. `of_node_access_logs` 1 行（bytes_sent=500, request_length=80, region=Server 填充）  
4. MV 异步计入 `of_access_log_hourly`  

**查询 24h 已提供数据：** `sum(bytes_sent)` → 至少 500（加历史）  
**查询宿主机出站：** 对 snapshots 差分，与 500 **无强制相等关系**。

---

## 10. 实现检查清单

- [x] `pkg/protocol`：v2 类型与 deprecated 兼容字段  
- [x] Agent：只组 `host_metrics` / `edge_health` / `access_logs` / `buffered`  
- [x] Server normalize + 停写 request_reports / openresty rx/tx  
- [x] CH migration：`request_length`、`request_time_ms`、`of_node_edge_health`、`of_access_log_hourly`、hourly 回填  
- [x] 看板/Zone API 统一读 access log 聚合  
- [x] 文档与前端文案：已提供数据 ≠ 宿主机网卡出站；UV 策略（整窗 uniqExact / 小时路径 UV=0）  

---

## 11. 修订记录

| 日期 | 说明 |
| --- | --- |
| 2026-07-17 | 初稿：协议 v2、Server 落库流水线、CH/关系库目标表结构与废弃表清单 |
