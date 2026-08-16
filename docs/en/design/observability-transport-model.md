# Edge Observability Transport Model (current target version)

> **This document is the latest authoritative description of "how Agent ↔ Server observability data is transmitted".**  
> After reading you should be able to answer: what is sent, where it is collected from, how often, how the Server stores it, and where product metrics are queried from.  
> Protocol fields and DDL details: [Observability Reporting Protocol & Data Model](./observability-data-model.md); background: [Edge Observability & Business Traffic Stats](./observability-design.md).

---

## 0. Remember the Three Layers First

| Layer | Question Answered | Single Data Source | Product Examples |
| --- | --- | --- | --- |
| **L1 Business delivery** | How much data provided? How many requests? | **access.log details** | data provided, request count, UV, status codes, Top domains |
| **L2 Edge health** | Is OpenResty alive? Current connections? | **local `/openflare/observability`** | node health, current connections |
| **L3 Host capacity** | How are CPU/memory/disk/NIC? | **OS readings** | capacity trends, host NIC |

**The three layers are never reconciled against each other.**  
"Data provided" ≠ "current connections" ≠ "host NIC outbound".

---

## 1. Overview: Who Collects, Who Reports, Who Aggregates

```text
┌─────────────────────────────────────────────────────────────┐
│ Edge Node                                                    │
│                                                               │
│  Visitor request ──► OpenResty                                 │
│                 │                                              │
│                 ├─ access.log (one line per request)  ←── L1 collection point │
│                 │                                              │
│                 └─ connection state (maintained in-process)    │
│                        │                                       │
│                        ▼                                       │
│              GET /openflare/observability  ←── L2 reads snapshot │
│              (no log scanning, no business recomputation)      │
│                                                               │
│  OS /proc etc.  ──────────────────────────── L3 reads snapshot │
│                                                               │
│              ┌────────── Agent ──────────┐                     │
│              │ default: one NodePayload per 3s  │              │
│              │  · tail access.log incremental │               │
│              │  · GET local observability     │               │
│              │  · read host_metrics           │               │
│              └────────────┬──────────────┘                     │
└─────────────────────────────│──────────────────────────────────┘
                              │ HTTP heartbeat or WebSocket status
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ Server (control plane)                                       │
│  · details → ClickHouse of_node_access_logs                  │
│  · health → node latest state + of_node_edge_health          │
│  · host → of_node_metric_snapshots                           │
│  · business trends / Zone stats = sum/count/uniq over access_logs only │
└─────────────────────────────────────────────────────────────┘
```

| Role | Does | Doesn't |
| --- | --- | --- |
| OpenResty | writes access.log; maintains connection counts | no direct reporting to the control plane |
| Agent | **collects facts and reports them** | **does not compute** UV/TopN/24h data provided |
| Server | stores + **aggregates/interpreters** | does not trust edge business pre-summaries |

---

## 2. Collection Frequency (defaults)

| Action | Default Frequency | Config |
| --- | --- | --- |
| Agent → Server report | **every 3 seconds** a full payload | `heartbeat_interval` / control-plane `agent_heartbeat_interval` (ms, default `3000`) |
| Tail access.log when packing | **with report** (new lines since last report) | same |
| GET `/openflare/observability` when packing | **with report** (reads **current** connection snapshot) | same |
| Read host metrics when packing | **with report** | same |
| OpenResty writes access.log | **1 line at each request end** | unrelated to heartbeat |
| Connection counts update in-process | **on connection change** (kernel-maintained) | unrelated to heartbeat |
| Offline replay window | keep ~**60 minutes** by default | `observability_replay_minutes` |
| Node offline detection | ~**60s** without a successful heartbeat | `node_offline_threshold` (default `60000` ms) |

**Notes:**

- The Agent has **no separate "sampling clock"**; **sampling points = report points** (default 3s).  
- access.log is "per-request continuous writes"; the Agent only **moves incremental lines** periodically.  
- `/openflare/observability` is **not** "business stats start being counted when called"; for connections it **reads Nginx's existing instantaneous values**.

Transport channels:

- **HTTP heartbeat**: POST the full payload at the interval.  
- **WebSocket**: after connecting, sends `status` messages at the same interval (same content shape); HTTP heartbeat is not double-sent then.

---

## 3. Agent → Server Packet (NodePayload v2)

### 3.1 Structure Skeleton

```json
{
  "schema_version": 2,
  "node_id": "n_01hxyz",
  "name": "edge-shanghai-1",
  "ip": "203.0.113.10",
  "version": "3.4.0",
  "ext_version": "",
  "current_version": "20260718-abc",
  "last_error": "",
  "profile": { },
  "host_metrics": { },
  "edge_health": { },
  "access_logs": [ ],
  "buffered": [ ],
  "health_events": [ ],
  "waf_ip_group_checksums": { }
}
```

| Field | Layer | Meaning |
| --- | --- | --- |
| identity/version/last_error | control | who the node is, what version it runs |
| `profile` | low-frequency overview | hostname, core count, etc. (report on change) |
| `access_logs` | **L1** | access detail increments |
| `edge_health` | **L2** | OpenResty health + current connections |
| `host_metrics` | **L3** | CPU/memory/disk/NIC readings |
| `buffered` | backfill | batches of facts accumulated while offline |
| `health_events` | events | e.g. openresty_unhealthy |
| `waf_ip_group_checksums` | sync | not an observability lake |

**Removed from the protocol (no compatibility layer; old Agents must upgrade):**

- `traffic_report`  
- `openresty_observation` (incl. rx/tx)  
- `snapshot` / `buffered_observability`  
- business-meaning openresty throughput fields  

---

## 4. L1 Business: access_logs

### 4.1 Where Collection Comes From

| Step | Location | Description |
| --- | --- | --- |
| 1 | OpenResty `log_format openflare_json` | writes one JSON line per request to `access_log_path` |
| 2 | Agent **tails increments** by file offset | new lines between two heartbeats |
| 3 | parse and put into `access_logs[]` | overlong paths may be truncated; **no sum/count** |

Log format (OpenResty variables):

```text
ts            ← $time_iso8601
host          ← $host
path          ← $request_uri
remote_addr   ← $remote_addr
status        ← $status
request_time  ← $request_time
bytes_sent    ← $body_bytes_sent     【data provided = response body bytes】
request_length← $request_length      【data received】
user_agent    ← $http_user_agent
cache_status  ← $upstream_cache_status  【cache status; UI can derive hit/origin/un-cached】
```

Observability-port requests **don't write** business access.log (separate server with `access_log off`).

### 4.2 Report Example

```json
"access_logs": [
  {
    "logged_at_unix": 1721289601,
    "remote_addr": "198.51.100.20",
    "host": "www.example.com",
    "path": "/api/v1/ping",
    "status_code": 200,
    "bytes_sent": 1024,
    "request_length": 128,
    "request_time_ms": 15,
    "user_agent": "curl/8.0",
    "cache_status": "MISS"
  },
  {
    "logged_at_unix": 1721289602,
    "remote_addr": "198.51.100.21",
    "host": "www.example.com",
    "path": "/index.html",
    "status_code": 200,
    "bytes_sent": 8192,
    "request_length": 300,
    "request_time_ms": 8,
    "user_agent": "Mozilla/5.0",
    "cache_status": "HIT"
  }
]
```

| Field | Explanation |
| --- | --- |
| `bytes_sent` | **data provided** (single request); global/Zone totals = Server `sum` |
| `request_length` | **data received** (single request) |
| `logged_at_unix` | request completion time (business timeline) |
| `host` | used for Zone domain filtering |
| `cache_status` | `$upstream_cache_status` as-is; detail/list can derive three states (hit/origin/un-cached); **no upstream address reported** |
| no `region` | **written by Server at insert time** via GeoIP |

### 4.3 How the Server Uses It (product metrics)

| Product Metric | Algorithm (L1 only) |
| --- | --- |
| Data provided | `sum(bytes_sent)` |
| Data received | `sum(request_length)` |
| Request count | `count()` |
| UV | `uniqExact(remote_addr)` |
| Status distribution | `group by status_code` |
| Top domains | `group by host` |
| Zone page | same + `host IN (that Zone's domains)` |
| Dashboard business area | same, global or Top-filtered |

Stored in: `of_node_access_logs` (optional Server-side `of_access_log_hourly` acceleration, **Agent never writes it**).

### 4.4 Report Frequency

```text
Request happens ──immediately──► write access.log
Agent every 3s ──moves──► new lines in those 3s (possibly 0, possibly many)
Server ──immediately/batched──► CH
```

Business volume correctness does **not** depend on 3s alignment; 3s only affects "detail arrival latency at the control plane" and per-packet line count.

---

## 5. L2 Health: edge_health and `/openflare/observability`

### 5.1 Local Monitoring Endpoint

**Data collection endpoint:**

```http
GET http://127.0.0.1:{openresty_observability_port}/openflare/observability
```

Default port: **18081** (`openresty_observability_port`).

**Responsibility:** answers "how is OpenResty right now", **not** "how much business data was provided".

#### Response Example

```json
{
  "ok": true,
  "captured_at_unix": 1721289600,
  "connections": {
    "active": 42,
    "reading": 0,
    "writing": 1,
    "waiting": 41
  }
}
```

| Field | Instant? | Source | Description |
| --- | --- | --- | --- |
| `ok` | this probe | returns 200 → true | liveness |
| `captured_at_unix` | sampling time | `ngx.time()` | aligned with report |
| `connections.active` | **instant** | Nginx connection state (original stub_status Active) | current active connections |
| `reading` / `writing` / `waiting` | **instant** | same, subdivided | optional but recommended |

**Not returned (removed):**

| Old Field | Reason |
| --- | --- |
| `request_count` / `error_count` / UV / status_codes / top_domains | business window summaries, now from access log |
| `openresty_rx_bytes` / `openresty_tx_bytes` | duplicates data provided/received and error-prone |
| `source_countries` | never implemented; countries go through Server GeoIP |
| `server.accepts/handled/requests` | process cumulative counters, easily confused with business requests; not on the main path |

**`/openflare/stub_status`:** kept; `/openflare/observability` internally reads that endpoint to assemble the connection-count JSON, and the Agent health check also probes it directly.

### 5.2 Collection Mechanism (read snapshot)

```text
Nginx maintains Active connections etc. on connect/disconnect
        │
Agent GET /openflare/observability
        │
only reads "current values" and returns JSON
```

- No access.log scanning, no 60-second business averages.  
- Returns an **instant gauge snapshot**.

### 5.3 Report Example (packed into NodePayload)

```json
"edge_health": {
  "captured_at_unix": 1721289600,
  "status": "healthy",
  "message": "",
  "connections": 42
}
```

| Field | Source |
| --- | --- |
| `status` / `message` | Agent health probe (config validation/process etc., may work with the observability endpoint's `ok`); must align with top-level `openresty_status` / `openresty_message` |
| `connections` | observability endpoint `connections.active` |

**Storage split (authoritative sources):**

| Content | Written To |
| --- | --- |
| latest `status` + `message` | **PG node table** (UI / list / alerts) |
| time-series `status` + `connections` | **CH `of_node_edge_health`** (**no message**) |

---

## 6. L3 Host: host_metrics

### 6.1 Where Collection Comes From

The Agent reads the local machine (e.g. `/proc`, disk stats), **once per packet**.

| Field | Semantics | Description |
| --- | --- | --- |
| `cpu_usage_percent` | instant | current CPU% |
| `memory_*` / `storage_*` | instant used/total | usage rates computed at Server or display layer |
| `disk_read_bytes` / `disk_write_bytes` | **cumulative counter** | kernel cumulative IO |
| `network_rx_bytes` / `network_tx_bytes` | **cumulative counter** | **host NIC**, not data provided |

### 6.2 Report Example

```json
"host_metrics": {
  "captured_at_unix": 1721289600,
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

### 6.3 How the Server Handles Cumulative Fields

```text
store raw-value time series
when displaying "NIC outbound over this period":
  delta = current - previous
  if delta < 0 → treat as restart/counter reset, record this segment's increment as 0, continue from new baseline
  if delta >= 0 → record into that period's increment
```

- The Agent **reports raw values**, never computes 24h totals at the edge.  
- **Forbidden** to `sum` cumulative raw values as business volume.  
- Copy must be **"host NIC"**, never "data provided / OpenResty outbound".

Stored in: `of_node_metric_snapshots` (optional capacity hourly MV).

---

## 7. One Complete Report Example

```json
{
  "schema_version": 2,
  "node_id": "n_01hxyz",
  "name": "edge-shanghai-1",
  "ip": "203.0.113.10",
  "version": "3.4.0",
  "ext_version": "",
  "current_version": "20260718-abc",
  "last_error": "",
  "host_metrics": {
    "captured_at_unix": 1721289600,
    "cpu_usage_percent": 12.5,
    "memory_used_bytes": 4294967296,
    "memory_total_bytes": 16106127360,
    "storage_used_bytes": 50000000000,
    "storage_total_bytes": 107374182400,
    "disk_read_bytes": 9000000000,
    "disk_write_bytes": 12000000000,
    "network_rx_bytes": 500000000000,
    "network_tx_bytes": 800000000000
  },
  "edge_health": {
    "captured_at_unix": 1721289600,
    "status": "healthy",
    "message": "",
    "connections": 42
  },
  "access_logs": [
    {
      "logged_at_unix": 1721289595,
      "remote_addr": "198.51.100.20",
      "host": "www.example.com",
      "path": "/",
      "status_code": 200,
      "bytes_sent": 4096,
      "request_length": 200,
      "request_time_ms": 12
    }
  ],
  "buffered": [],
  "health_events": [],
  "waf_ip_group_checksums": {
    "1": "d41d8cd98f00b204e9800998ecf8427e"
  }
}
```

**Server storage sketch:**

| payload block | written to |
| --- | --- |
| `access_logs[0]` | one CH row, `bytes_sent=4096`, `region` filled by GeoIP |
| `edge_health` | node `openresty_status=healthy`, connections=42 |
| `host_metrics` | one CH metric row with cumulative/instant fields |

**Product query sketch (24h):**

- data provided = `sum(bytes_sent)` over that node's (or global) logs  
- current connections = latest `edge_health.connections`  
- host NIC outbound = sum of non-negative `network_tx` deltas over metrics  

The three numbers **need not be equal**.

---

## 8. Offline Backfill `buffered`

When reporting fails, the Agent caches **the same kind of facts** locally by window (default ~60 minutes), then packs them into `buffered[]` after recovery:

```json
"buffered": [
  {
    "captured_at_unix": 1721289500,
    "host_metrics": { },
    "edge_health": { },
    "access_logs": [ ]
  }
]
```

- Only facts, no legacy TrafficReport.  
- Server processing logic is identical to the main fields.

---

## 9. End-to-End Timeline (default 3s)

```text
t=0.0s   visitor request completes → writes one access.log line; connection count may change
t=0.1s   another request → another log line
…
t=3s     Agent heartbeat:
           · reads 2 access_logs lines
           · GET observability → connections=42
           · reads host_metrics
           · sends to Server
t=3s+    Server stores; dashboard/Zone queries aggregate logs
t=6s     next round…
```

---

## 10. Old Model Comparison

| Old Approach | New Model |
| --- | --- |
| Lua dict 60s window request_count + Agent 10s pull + Server sum | **removed**; request count = log count |
| openresty_tx as "outbound" | **removed**; data provided = `sum(bytes_sent)` |
| Two endpoints observability + stub_status | data collection unified through observability; stub_status kept as liveness and internal read endpoint |
| TrafficReport pre-aggregation | **removed**; no such path in protocol or API |
| Business and NIC both called "traffic" | **separate copy, separate APIs, separate tables** |
| health status/message | **PG latest-state authority**; CH only status+connection time series |

---

## 11. Config and Implementation Index

| Item | Location/Key |
| --- | --- |
| Heartbeat interval | Agent `heartbeat_interval`; control plane `agent_heartbeat_interval` (default 3000ms) |
| Offline threshold | control plane `node_offline_threshold` (default 60000ms) |
| Observability port | `openresty_observability_port` (default 18081) |
| access.log path | `access_log_path` |
| Replay minutes | `observability_replay_minutes` (default 60) |
| Protocol types | `pkg/protocol/agent.go` (evolves to v2 when landing) |
| Table DDL | [observability-data-model.md](./observability-data-model.md) |

---

## 12. Revision History

| Date | Notes |
| --- | --- |
| 2026-07-18 | initial draft: single-page "latest transport model" — three layers, frequency, sample JSON, collection sources, old-model comparison |
| 2026-07-18 | default report interval 3s; offline threshold 60s; replay window 60 minutes |
| 2026-07-18 | M5: edge_health table, access_log_hourly, deprecate request_reports/obs_openresty throughput tables |
| 2026-07-18 | no compatibility layer: removed "may ignore during compat period" wording; health message only in PG, no message in CH |
