# Agent Reporting Protocol and Observability Data Model

You will learn: the **data structures** of the refactored Agent heartbeat/WS reports, how the Server **parses and writes** them, and the **target table structures** in ClickHouse / relational DBs.  
**No protocol compatibility layer**: Agents upgrade via destroy-and-recreate or binary replacement; old fields are not parsed, old buffers are discarded wholesale.

This design is the **protocol & storage chapter** of [Edge Observability & Business Traffic Stats Refactor](./observability-design.md); implement against the fields and DDL in this document.

**First read the transport overview and examples:** [Observability Transport Model](./observability-transport-model.md).

---

## 1. Design Goals

| Goal | Description |
| --- | --- |
| Agent reports only facts | details + host readings + edge health instant state; no business pre-aggregation |
| One business detail table | access logs are the only L1 write path |
| Aggregation in DB/control plane | hourly summaries come from ClickHouse MV or queries; the Agent never writes summary tables |
| No field overlap | `bytes_sent` = data provided; NIC `network_*` = host; no business `openresty_tx` anymore |
| Evolvable | new fields optional; missing numeric values default to 0; removed legacy protocol fields are not parsed |

---

## 2. Layering and Write Overview

```text
                    Agent NodePayload (v2)
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
         access_logs     host_metrics    edge_health
         (L1 details)    (L3 readings)   (L2 instant)
              │               │               │
              ▼               ▼               ▼
     of_node_access_logs  of_node_metric_  of_node_edge_health
              │           snapshots              │
              │               │                  │
              ▼               ▼                  │
     of_access_log_hourly  of_node_metric_       │
     (MV, Server side)     capacity_hourly (MV)  │
              │               │                  │
              └─────── admin aggregation API ────┘

Relational DB (PostgreSQL/SQLite): node latest state, Profile, health events (not a detail lake)
```

| Layer | Meaning | Agent Report Block | ClickHouse Fact Table |
| --- | --- | --- | --- |
| L1 | business delivery | `access_logs` | `of_node_access_logs` |
| L2 | edge health | `edge_health` | `of_node_edge_health` |
| L3 | host capacity | `host_metrics` | `of_node_metric_snapshots` |

---

## 3. Agent Report Data Structures (protocol v2)

### 3.1 Top-Level `NodePayload`

Transport: HTTP heartbeat body and WebSocket `status` messages share the same structure.

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

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `schema_version` | int | suggested | fixed to `2` (this design) |
| `node_id` | string | ✅ | node ID |
| `name` | string | ✅ | display name |
| `ip` | string | ✅ | reporting IP |
| `version` / `ext_version` | string | ✅ | Agent version |
| `current_version` | string | | locally active config version summary |
| `last_error` | string | | latest sync/runtime error, nullable |
| `openresty_status` | string | ✅ (when OpenResty present) | **latest health-state authoritative field** → written to PG node table |
| `openresty_message` | string | | **latest health-description authoritative field** → written to PG node table (**not into CH**) |
| `profile` | object | | host overview, report on change (may throttle) |
| `host_metrics` | object | suggested each beat | L3 resource snapshot |
| `edge_health` | object | suggested each beat | L2 connection time series + status aligned with top level |
| `access_logs` | array | | this beat's incremental access details |
| `buffered` | array | | offline backfill fact batches (see §3.6) |
| `health_events` | array | | edge health events |
| `waf_ip_group_checksums` | map | | for differential sync, not an observability lake |

**Removed, Server no longer parses (no compatibility layer):**

| Old Field | Disposition |
| --- | --- |
| `traffic_report` | not in the protocol; not stored |
| `openresty_observation` | not present; connections/status go through `edge_health` |
| `snapshot` | not present; only `host_metrics` |
| `buffered_observability` | not present; only `buffered` |

### 3.2 `profile` — Host Overview (low frequency)

Maps to relational `of_node_system_profiles` (or an existing equivalent), **not into the ClickHouse detail lake**.

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

| Field | Semantics |
| --- | --- |
| hardware/OS description fields | factual readings |
| `reported_at_unix` | Agent collection time (UTC seconds) |

### 3.3 `host_metrics` — Host Capacity (L3)

**All readings, no 24h business totals.**  
NIC/disk bytes are **kernel cumulative counter raw values** (monotonically increasing, may reset on restart); CPU is an instant percentage; memory/disk usage is current usage.

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

| Field | Type | Semantics | How Server Uses It |
| --- | --- | --- | --- |
| `captured_at_unix` | int64 | sampling time | `captured_at` |
| `cpu_usage_percent` | float | instant CPU% | store directly; average for trends |
| `memory_*` / `storage_*` | int64 | current used/total | store directly; compute usage rate |
| `disk_read_bytes` / `disk_write_bytes` | int64 | **cumulative** IO bytes | store raw; adjacent deltas at query time |
| `network_rx_bytes` / `network_tx_bytes` | int64 | **cumulative** NIC bytes | store raw; adjacent deltas at query time → "host NIC in/outbound" |

> The Agent is **forbidden** from replacing cumulative values with "this period's delta" before reporting (otherwise Server deltas would be wrong).

### 3.4 `edge_health` — OpenResty Edge Health (L2)

**Instant state only, no business throughput.**

```json
{
  "captured_at_unix": 1720000000,
  "status": "healthy",
  "message": "",
  "connections": 42
}
```

| Field | Type | Semantics |
| --- | --- | --- |
| `status` | string | `healthy` / `unhealthy` / `unknown` (must match top-level `openresty_status`) |
| `message` | string | status description (may be reported; **only backfills PG latest state, not into CH**) |
| `connections` | int64 | stub_status Active connections |

#### Health-State Authoritative Sources (converged)

| Data | Authoritative Storage | Description |
| --- | --- | --- |
| **Current** OpenResty health + description | **PG node table** `openresty_status` / `openresty_message` | UI badges, lists, alerts use this |
| **Time series** health status + connections | **CH** `of_node_edge_health` (`status`, `connections`) | connection curves / health history; **no message column** |
| Agent report | top-level status/message + `edge_health` | Server normalizes both statuses aligned; message **only written to PG** |

So: "is it unhealthy now" → read PG; "connections over the past 24h" → read CH.

### 3.5 `access_logs[]` — Access Details (L1, single business truth)

Agent: tail access.log → parse JSON lines → report fields as-is (path may be truncated).

```json
{
  "logged_at_unix": 1720000001,
  "remote_addr": "203.0.113.10",
  "host": "www.example.com",
  "path": "/api/v1/ping",
  "status_code": 200,
  "bytes_sent": 1024,
  "request_length": 128,
  "request_time_ms": 15,
  "user_agent": "Mozilla/5.0 ...",
  "cache_status": "HIT"
}
```

| Field | Type | Required | Source (OpenResty) | Business Meaning |
| --- | --- | --- | --- | --- |
| `logged_at_unix` | int64 | ✅ | parse `$time_iso8601` | request completion time |
| `remote_addr` | string | ✅ | `$remote_addr` | client IP → UV |
| `host` | string | ✅ | `$host` | domain → Zone ownership |
| `path` | string | ✅ | `$request_uri`, Agent may truncate | path |
| `status_code` | int | ✅ | `$status` | status code |
| `bytes_sent` | int64 | ✅ | **`$body_bytes_sent`** | **data provided** (response body) |
| `request_length` | int64 | suggested | `$request_length` | **data received** |
| `request_time_ms` | int64 | optional | `$request_time * 1000` | latency; default 0 |
| `user_agent` | string | suggested | `$http_user_agent` | UA; may truncate on store |
| `cache_status` | string | suggested | **`$upstream_cache_status`** | edge cache result (see §3.5.1) |

**Explicitly not reported by Agent (written by Server):**

* `region` / country: GeoIP resolved at insert time  
* `id` / `created_at`: Server-generated  
* `node_id`: from payload / auth context  

**Explicitly not reported:**

* `upstream_addr` / origin address / `origin_fetched`: no origin-endpoint tracking; "did it fetch from origin" is only derived from `cache_status` at the control plane (§3.5.1)

### 3.5.1 `cache_status` — Cache Hit and Origin Fetch (detail-first)

**Goal (phase 1):** access log details/list can show "cache hit / origin fetch / no cache used".  
**Caliber:** only store the raw OpenResty `$upstream_cache_status`; **no upstream address reported**.

#### Raw Values (stored)

| Value | Meaning (OpenResty) |
| --- | --- |
| `HIT` | cache hit |
| `MISS` | miss, fetched from upstream |
| `BYPASS` | cache skipped (e.g. method/cookie/policy caused `$openflare_skip_cache`) |
| `EXPIRED` | expired then origin fetch |
| `STALE` | served stale |
| `UPDATING` | background updating, may return old cache |
| `REVALIDATED` | revalidated, still used cache |
| `-` or empty | didn't pass through `proxy_cache` (e.g. Pages local static, non-proxy location) |

#### UI Three-State Derivation (not stored)

Control-plane display uses derived enum `cache_outcome`, **not written to CH**:

| Three-State | Condition (`cache_status`) | Suggested List Label |
| --- | --- | --- |
| **Cache hit** | `HIT` / `STALE` / `REVALIDATED` / `UPDATING` | hit |
| **Origin fetch** | `MISS` / `EXPIRED` | origin |
| **No cache used** | `BYPASS` / `-` / `""` | not cached |

Details can show both the three-state and the raw `cache_status`.

#### Boundaries

* Pages static / locations without `proxy_cache`: mostly empty or `-` → **no cache used**, must not be labeled "hit".  
* Detail pages show cache state; hit-rate dashboards and hourly dimensions can extend from the same column.

**Per-heartbeat count suggestion:**

* Soft cap e.g. 2000 lines/beat; overflow goes into `buffered` next batch, **forbidden** to compress into a TrafficReport in the Agent.

### 3.6 `buffered[]` — Offline Backfill (facts only)

```json
{
  "captured_at_unix": 1719999900,
  "host_metrics": { },
  "edge_health": { },
  "access_logs": [ ]
}
```

| Field | Description |
| --- | --- |
| `captured_at_unix` | batch collection/buffer time, used for ack and dedup window |
| `host_metrics` / `edge_health` / `access_logs` | same structures as the main payload; empty blocks may be omitted |

**Forbidden** to carry `traffic_report` or rx/tx throughput in buffered.

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

Written to the relational health-event table (existing model suffices), not into the access log lake.

### 3.8 Go Protocol Structures

```go
// pkg/protocol/agent.go (current implementation)

type NodePayload struct {
    SchemaVersion       int                    `json:"schema_version,omitempty"`
    NodeID              string                 `json:"node_id"`
    Name                string                 `json:"name"`
    IP                  string                 `json:"ip"`
    Version             string                 `json:"version"`
    ExtVersion          string                 `json:"ext_version"`
    CurrentVersion      string                 `json:"current_version"`
    LastError           string                 `json:"last_error"`
    OpenrestyStatus     string                 `json:"openresty_status"`  // PG latest-state authority
    OpenrestyMessage    string                 `json:"openresty_message"` // PG latest-state authority; not into CH
    Profile             *NodeSystemProfile     `json:"profile,omitempty"`
    HostMetrics         *NodeHostMetrics       `json:"host_metrics,omitempty"`
    EdgeHealth          *NodeEdgeHealth        `json:"edge_health,omitempty"`
    AccessLogs          []NodeAccessLog        `json:"access_logs,omitempty"`
    Buffered            []BufferedFacts        `json:"buffered,omitempty"`
    HealthEvents        []NodeHealthEvent      `json:"health_events"`
    WAFIPGroupChecksums map[string]string      `json:"waf_ip_group_checksums,omitempty"`
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
    UserAgent     string `json:"user_agent,omitempty"`
    CacheStatus   string `json:"cache_status,omitempty"` // $upstream_cache_status
    StatusCode    int    `json:"status_code"`
    BytesSent     int64  `json:"bytes_sent"`      // body_bytes_sent, data provided
    RequestLength int64  `json:"request_length"`  // data received
    RequestTimeMs int64  `json:"request_time_ms"` // optional
}

type BufferedFacts struct {
    CapturedAtUnix int64            `json:"captured_at_unix"`
    HostMetrics    *NodeHostMetrics `json:"host_metrics,omitempty"`
    EdgeHealth     *NodeEdgeHealth  `json:"edge_health,omitempty"`
    AccessLogs     []NodeAccessLog  `json:"access_logs,omitempty"`
}
```

---

## 4. Server Parsing and Storage Flow

### 4.1 Entry Points

* HTTP: `POST /api/v1/agent/...` heartbeat (existing path)  
* WebSocket: `type=status` payload = `NodePayload`  
* Auth: `X-Agent-Token` → binds `node_id` (payload.node_id must match the token node)

### 4.2 Processing Pipeline (single payload)

```text
1. Deserialize NodePayload
2. Normalize
   - schema_version < 2:
       host_metrics ← snapshot
       edge_health.status ← openresty_status
       edge_health.connections ← openresty_observation.connections (if present)
       traffic_report → drop
       openresty_observation.rx/tx → drop
       buffered ← buffered_observability
   - path truncated again, status clamped, negative bytes → 0
3. Relational transaction (node latest state)
   - update node online time, IP, version, edge_health.status/message
   - upsert profile (if present)
   - insert health_events (if present)
4. ClickHouse async batch (log on failure; doesn't block heartbeat response config delivery)
   a. access_logs + buffered[].access_logs
        → fill region (GeoIP)
        → assign snowflake id
        → BatchInsert of_node_access_logs
   b. host_metrics + buffered[].host_metrics
        → of_node_metric_snapshots
   c. edge_health + buffered[].edge_health
        → of_node_edge_health (connections + status snapshot only, optional)
5. Return heartbeat response (settings / active_config / waf diff)
6. If using buffer ack: confirm by the list of buffered.captured_at_unix
```

### 4.3 Normalization Rules (hard constraints)

| Rule | Behavior |
| --- | --- |
| `logged_at` ahead of now+5m | clamp to now or drop the line (pick one in implementation and unit-test it) |
| `logged_at` older than now−TTL | still writable; relies on table TTL cleanup |
| empty `host` | allowed, aggregated into "unassigned" |
| `bytes_sent` / `request_length` < 0 | set to 0 |
| single batch access_logs > N | truncate and alert-metric it (or only queue into buffer), never switch to pre-aggregation |
| duplicate backfill | CH tolerates a few duplicate rows; queries approximate with sum (no forced exact dedup) |

### 4.4 Field Mapping Table (report → table)

| Report Path | Target Storage | Columns |
| --- | --- | --- |
| `access_logs[]` | CH `of_node_access_logs` | see §5.1 |
| `host_metrics` | CH `of_node_metric_snapshots` | see §5.2 |
| `edge_health` | CH `of_node_edge_health` + PG node latest state | see §5.3 / §5.6 |
| `profile` | PG `of_node_system_profiles` | existing columns |
| `health_events` | PG health-event table | existing model |
| `waf_ip_group_checksums` | not into observability tables | sync logic |
| `traffic_report` (legacy) | **not written** | — |
| `openresty_rx/tx` (legacy) | **not written** | — |

### 4.5 Query Side (no new "business outbound" column)

| Product Metric | SQL Semantics (sketch) |
| --- | --- |
| Data provided | `sum(bytes_sent)` |
| Data received | `sum(request_length)` |
| Request count | `count()` |
| UV | `uniqExact(remote_addr)` |
| 5xx | `countIf(status_code >= 500)` |
| By domain/status/region | `GROUP BY host / status_code / region` |
| Host NIC outbound | non-negative delta over `network_tx_bytes` per node in time order, then sum |
| OpenResty connections | `of_node_edge_health.connections` latest or average |

---

## 5. Table Structures (DDL)

> Engine and TTL tend to match production: access logs 90 days, metrics 30 days.  
> `id` uses control-plane Snowflake/unique UInt64.

### 5.1 L1 Fact Table: `of_node_access_logs`

```sql
CREATE TABLE IF NOT EXISTS of_node_access_logs
(
    id              UInt64,
    node_id         String,
    logged_at       DateTime64(3, 'UTC'),
    remote_addr     String,
    region          String,              -- Server GeoIP writes, Agent doesn't send
    host            String,
    path            String,
    user_agent      String DEFAULT '',   -- $http_user_agent
    cache_status    String DEFAULT '',   -- $upstream_cache_status
    status_code     Int32,
    bytes_sent      UInt64,              -- data provided (body)
    request_length  UInt64 DEFAULT 0,    -- data received
    request_time_ms UInt32 DEFAULT 0,    -- optional
    created_at      DateTime64(3, 'UTC')
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(logged_at)
ORDER BY (node_id, logged_at, host, status_code, remote_addr)
TTL toDateTime(logged_at) + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;
```

| Column | Type | Source |
| --- | --- | --- |
| `id` | UInt64 | Server |
| `node_id` | String | auth/payload |
| `logged_at` | DateTime64(3) | `logged_at_unix` |
| `remote_addr` | String | report |
| `region` | String | Server GeoIP |
| `host` | String | report |
| `path` | String | report |
| `user_agent` | String | report (nullable) |
| `cache_status` | String | report (nullable) → **cache status** |
| `status_code` | Int32 | report |
| `bytes_sent` | UInt64 | report → **data provided** |
| `request_length` | UInt64 | report → **data received** |
| `request_time_ms` | UInt32 | report optional |
| `created_at` | DateTime64(3) | Server now |

**Migration:** the current table already has `bytes_sent` / `request_length` / `request_time_ms` / `user_agent`; cache status adds:

```sql
ALTER TABLE of_node_access_logs
    ADD COLUMN IF NOT EXISTS cache_status String DEFAULT '';
```

### 5.2 L1 Hourly Rollup (Server-side MV)

**Agent forbidden to write.** Serves dashboard/node 24h fast queries of request count, error count, bytes.

**Implemented choice: `SummingMergeTree` + no UV column.**

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

Historical hours (details stored before the MV existed) need a one-time backfill, see migration `202607180003_backfill_access_log_hourly.sql` (ANTI JOIN to prevent duplicates).

#### UV Policy (must follow)

| Scenario | Data Source | Algorithm | Notes |
| --- | --- | --- | --- |
| **Window total UV** (dashboard totals, node cards, Zone totals) | `of_node_access_logs` details | `uniqExact(remote_addr)` (`TrafficSummary` / node aggregation) | **single authority**; never sum hourly UV |
| **24h trend line request/error/bytes** | `of_access_log_hourly` preferred, fall back to detail buckets | `sum(request_count)` etc. | hourly path **doesn't fill** `unique_visitor_count` (always 0) |
| **24h trend per-hour UV** | detail bucket path only | in-bucket `uniqExact` | when using hourly, UI should show empty/0 or hide the UV series; **forbidden** to `sum(UV)` over hourly rows |

**Why hourly doesn't store UV:**

1. `SummingMergeTree` can only safely merge addable counts; `uniqExact` across parts needs `AggregatingMergeTree` + state, heavier to implement and query.  
2. Even if hourly UV were stored, **summing over multi-hour windows severely overestimates** (the same IP is counted once per hour).  
3. Product "24h unique visitors" only recognizes whole-window `uniqExact`; the trend chart's main series are requests/errors/bytes — per-hour UV is not a primary metric.

### 5.3 L3 Fact Table: `of_node_metric_snapshots` (kept, semantics clarified)

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
    disk_read_bytes     Int64,    -- cumulative raw
    disk_write_bytes    Int64,
    network_rx_bytes    Int64,    -- cumulative raw → host NIC inbound
    network_tx_bytes    Int64,    -- cumulative raw → host NIC outbound
    created_at          DateTime64(3, 'UTC')
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(captured_at)
ORDER BY (node_id, captured_at, id)
TTL toDateTime(captured_at) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;
```

Columns match production; **docs and API must label `network_*` as host NIC cumulative values**.

### 5.4 L3 Hourly Rollup: `of_node_metric_capacity_hourly` (kept)

Existing min/max used for cumulative-counter hourly increment approximation + CPU/memory averages. Logic unchanged:

* `network_tx_max - network_tx_min` ≈ that hour's host outbound  
* **must not** be used for "data provided"

### 5.5 L2 Fact Table: `of_node_edge_health` (new, replaces throughput-style openresty table)

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

| Column | Description |
| --- | --- |
| `status` | instant health (same source as PG current state; for time series, not the sole UI authority) |
| `connections` | current connection count |

**No** `message` column (description only in PG latest state).  
**No** `openresty_rx_bytes` / `openresty_tx_bytes`.

### 5.6 Relational DB (node latest state, not an analytics lake)

Separate from the observability lake, keeping "latest one":

| Table (logical name) | Purpose | Key Columns |
| --- | --- | --- |
| `of_nodes` (or current node table) | online, version, IP | `last_seen_at`, `openresty_status`, `openresty_message`, `agent_version` |
| `of_node_system_profiles` | profile upsert | hostname, cpu_cores, total_memory_bytes, ... |
| health-event table | `health_events` | event_type, severity, message, triggered_at |

> Actual physical table names follow the repo's existing GORM models; this design doesn't force renames, only forces **business throughput no longer written into node tables**.

### 5.7 Deprecated Tables (stop writing → delete after TTL)

| Table | Reason | Replacement |
| --- | --- | --- |
| `of_node_request_reports` | Agent pre-aggregation | `of_node_access_logs` + hourly |
| `of_node_traffic_hourly` + MV | depends on request_reports | `of_access_log_hourly` |
| `of_node_obs_openresty` | contains business rx/tx | `of_node_edge_health` |
| `of_node_openresty_hourly` + MV | business throughput deltas | `of_access_log_hourly` bytes_* |

Relay-specific `of_node_obs_frps` / `of_node_obs_frpc` **kept** (not this Agent's main path, but same CH observability).

---

## 6. Table-Protocol Cross-Reference

| Product Concept | Protocol Field | Table.Column | Aggregation |
| --- | --- | --- | --- |
| Data provided | `access_logs[].bytes_sent` | `of_node_access_logs.bytes_sent` | `sum` |
| Data received | `access_logs[].request_length` | `...request_length` | `sum` |
| Request count | row count | — | `count` |
| UV (window total) | `remote_addr` | same details | `uniqExact` (**forbidden** to sum hourly UV) |
| Top domains | `host` | same | `group by` |
| Status distribution | `status_code` | same | `group by` |
| Source region | — | `region` (Server) | `group by` |
| Host NIC outbound | `host_metrics.network_tx_bytes` | `of_node_metric_snapshots.network_tx_bytes` | time-series delta |
| Host NIC inbound | `network_rx_bytes` | same | delta |
| Disk read/write | `disk_*_bytes` | same | delta |
| CPU/memory | instant fields | same | avg |
| OpenResty connections | `edge_health.connections` | `of_node_edge_health.connections` | latest/avg |
| OpenResty health | `edge_health.status` | node table + optional CH | latest |

**Mappings that no longer exist:**

| Old Concept | Old Field | Disposition |
| --- | --- | --- |
| OpenResty outbound | `openresty_tx_bytes` | removed; use data provided |
| OpenResty inbound | `openresty_rx_bytes` | removed; use data received |
| Window request report | `traffic_report` | removed |

---

## 7. OpenResty Log Format (aligned with details)

Target `log_format` (ensures the `bytes_sent` key = body; includes UA and cache status):

```nginx
log_format openflare_json escape=json
  '{"ts":"$time_iso8601","host":"$host","path":"$request_uri",'
  '"remote_addr":"$remote_addr","status":$status,'
  '"request_time":$request_time,'
  '"bytes_sent":$body_bytes_sent,"request_length":$request_length,'
  '"user_agent":"$http_user_agent",'
  '"cache_status":"$upstream_cache_status"}';
```

Agent parsing:

* `ts` → `logged_at_unix`  
* `bytes_sent` → protocol `bytes_sent` (provided)  
* `request_length` → protocol `request_length`  
* `request_time` → optional `request_time_ms = round(sec * 1000)`  
* `user_agent` → protocol `user_agent`  
* `cache_status` → protocol `cache_status` (passed through as-is, no three-state compression)

---

## 8. Upgrade Strategy (no compatibility layer)

| Item | Strategy |
| --- | --- |
| Agent upgrade | **destroy-and-recreate** preferred; **binary replacement** allowed |
| Protocol | only schema v2 fields; legacy JSON fields not parsed |
| Local observability buffer | if still in old format (containing `snapshot` / `openresty_observation` / `traffic_report`) or corrupt → **delete the file wholesale**, rebuild at runtime |
| Read path | business APIs **only read** access_logs (and hourly); current health reads PG; connection series reads CH edge_health |
| Old Agents | must upgrade; the control plane provides no v1 dual-read path |

---

## 9. Example: Storage Result of One Heartbeat

**Agent report (excerpt):**

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

**Written:**

1. PG node latest state: `openresty_status` / `openresty_message` (if reported)  
2. `of_node_metric_snapshots` 1 row (network_tx=2000 cumulative)  
3. `of_node_edge_health` 1 row (status + connections=5; **no message**)  
4. `of_node_access_logs` 1 row (bytes_sent=500, request_length=80, region filled by Server)  
5. MV asynchronously counts into `of_access_log_hourly`  

**Query 24h data provided:** `sum(bytes_sent)` → at least 500 (plus history)  
**Query host outbound:** delta over snapshots; **no forced equality** with 500.

---

## 10. Revision History

| Date | Notes |
| --- | --- |
| 2026-07-17 | initial draft: protocol v2, Server storage pipeline, CH/relational target table structures and deprecated table list |
