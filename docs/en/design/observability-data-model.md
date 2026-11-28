# OpenFlare Edge Reporting Protocol & Observability Data Model

This document defines the `NodePayload` telemetry contract reported by `openflare-edge`, the server normalization and ingestion pipeline, and the target table schemas across ClickHouse and PostgreSQL/SQLite (`internal/repository/logstore`). For layer definitions and collection cadence, see [Edge Observability & Traffic Analytics Architecture](./observability-design.md).

---

## 1. Ingestion Overview

```text
NodePayload (schema_version=2, engine="pingora")
  ├─ access_logs[] + buffered[].access_logs ─▶ logstore.AccessLogs ─▶ of_node_access_logs ─▶ of_access_log_hourly (CH MV)
  │                                        └─▶ TCP_INFO + NetIntel ─▶ netquality.passive ─▶ of_network_quality_samples
  ├─ host_metrics + buffered[].host_metrics  ─▶ logstore.Observability ─▶ of_node_metric_snapshots ─▶ of_node_metric_capacity_hourly (CH MV)
  ├─ edge_health + openresty_status/message  ─▶ Primary DB of_nodes latest state (of_node_edge_health removed)
  ├─ profile                                 ─▶ Primary DB of_node_system_profiles (upsert)
  └─ health_events[]                         ─▶ Primary DB health events (excluded from analytics lake)
```

Control-plane listeners also ingest active latency probes and SmartDNS metrics through the same `logstore` interface:

* Active latency results (WebSocket `ping_result` / `POST /api/v1/agent/ping/result`) → `of_node_ping_records` (tasks defined in `of_ping_tasks`);
* SmartDNS metrics → `of_dns_metric_snapshots` (via `logstore.Observability.InsertDNSMetricSnapshot`).

---

## 2. Wire Structures (`internal/openflare/protocol/agent.go`)

### 2.1 Top-Level `NodePayload`

Both HTTP heartbeats (`POST /api/v1/agent/nodes/heartbeat`) and WebSocket `status` frames share `NodePayload`:

| Field | Type | Semantics |
| :--- | :--- | :--- |
| `schema_version` | `int` | Payload schema version (`2`) |
| `node_id` / `name` / `ip` | `string` | Node identifier, display name, and reported public IP |
| `version` / `ext_version` | `string` | Running `openflare-edge` version |
| `current_version` / `latest_config_version` | `string` | Serving snapshot revision and latest observed revision |
| `engine` / `contract_version` / `config_sync_version` | `string` / `int` | Data-plane engine (`pingora`), contract version (`4`), config sync version (`1`) |
| `edge_update_supported` / `edge_update_ready` | `bool` | Whether staged self-update is active and trial update passed readiness |
| `last_error` | `string` | Latest configuration apply or runtime error |
| `openresty_status` / `openresty_message` | `string` | Legacy wire keys preserved for compatibility; hold gateway health (`healthy` / `unhealthy` / `unknown`) written only to `of_nodes` |
| `profile` | `*NodeSystemProfile` | Komari-aligned OS/hardware profile (`of_node_system_profiles`) |
| `host_metrics` | `*NodeMetricSnapshot` | L3 Komari-aligned instant readings and cumulative host counters |
| `edge_health` | `*NodeEdgeHealth` | L2 instantaneous gateway status and `/proc/self/fd` connected sockets (`of_nodes.openresty_connections`) |
| `access_logs` | `[]NodeAccessLog` | L1 per-request records (with optional `TCP_INFO` and WAF audit) |
| `buffered` | `[]BufferedObservabilityRecord` | Offline replay batches carrying historical `host_metrics`, `edge_health`, and `access_logs` |
| `health_events` | `[]NodeHealthEvent` | Edge runtime state transitions |

> [!NOTE]
> Legacy `traffic_report`, `openresty_observation`, `snapshot`, `buffered_observability`, and heartbeat `waf_ip_group_checksums` fields have been removed from `NodePayload`. WAF IP groups synchronize via `config_sync v1` under `waf.ip-group.v1/<id>`.

### 2.2 `profile` (Komari-Aligned Hardware Inventory)

Persisted to primary DB `of_node_system_profiles` with `hostname`, `os_name`, `os_version`, `kernel_version`, `architecture`, `cpu_model`, `cpu_cores`, `cpu_physical_cores`, `virtualization`, `gpu_name`, `total_memory_bytes`, `swap_total_bytes`, `total_disk_bytes`, `ipv4`, `ipv6`, `probe_version`, `uptime_seconds`, and `reported_at_unix`.

### 2.3 `host_metrics` (Komari-Aligned Node Snapshot)

Network byte totals and disk byte counters report **raw cumulative kernel counters**. The control plane computes adjacent non-negative deltas at query time (treating negative resets across reboots as `0` for that interval):

| Payload Field | Table Column (`of_node_metric_snapshots`) | Meaning |
| :--- | :--- | :--- |
| `captured_at_unix` | `captured_at` | UTC capture timestamp |
| `cpu_usage_percent` / `gpu_usage_percent` | `cpu` / `gpu` | Instantaneous CPU / GPU utilization (%) |
| `memory_used_bytes` / `memory_total_bytes` | `ram` / `ram_total` | Memory usage and capacity (bytes) |
| `swap_used_bytes` / `swap_total_bytes` | `swap` / `swap_total` | Swap usage and capacity (bytes) |
| `load1` / `load5` / `load15` / `temp` | `load` / `load5` / `load15` / `temp` | 1/5/15m load averages and temperature |
| `storage_used_bytes` / `storage_total_bytes` | `disk` / `disk_total` | Storage usage and capacity (bytes) |
| `disk_read_bytes` / `disk_write_bytes` | `disk_read_bytes` / `disk_write_bytes` | Cumulative disk read/write bytes |
| `net_in` / `net_out` | `net_in` / `net_out` | Instantaneous host NIC ingress/egress rate |
| `net_total_down` / `net_total_up` | `net_total_down` / `net_total_up` | Cumulative host NIC ingress/egress bytes |
| `process_count` / `tcp_connections` / `udp_connections` / `uptime_seconds` | `process` / `connections` / `connections_udp` / `uptime` | Process count, TCP/UDP socket counts, uptime |

### 2.4 `edge_health` (L2 Instantaneous Health)

Contains `captured_at_unix`, `status` (`healthy`, `unhealthy`, `unknown`), `message`, and `connections` (live connected socket count from `/proc/self/fd`). Normalized directly onto `of_nodes` without a time-series table.

### 2.5 `access_logs[]` (L1 Business Fact & Passive RTT)

| Field | Type | Description |
| :--- | :--- | :--- |
| `logged_at_unix` | `int64` | Request completion time (UTC seconds) |
| `remote_addr` | `string` | Client IP (used for UV and NetIntel lookup) |
| `host` | `string` | Request Host (used for Zone attribution) |
| `path` | `string` | Path and query string (truncated to `100` chars normally; expanded for WAF audit logs) |
| `user_agent` | `string` | User-Agent (truncated to `512` chars) |
| `cache_status` | `string` | Edge cache phase: `HIT`, `MISS`, `BYPASS`, `EXPIRED`, `STALE`, `UPDATING`, `REVALIDATED`, `-` |
| `status_code` | `int` | Final HTTP status code |
| `bytes_sent` | `int64` | Response body bytes delivered ("Data provided") |
| `request_length` | `int64` | Estimated incoming request bytes ("Data received") |
| `request_time_ms` | `int64` | Request duration in milliseconds |
| `tcp_rtt_us` / `tcp_min_rtt_us` / `tcp_rtt_var_us` | `uint32` | Optional kernel `TCP_INFO` smoothed RTT, min RTT, and RTT variance (µs) |
| `tcp_retrans` / `tcp_segs_out` | `uint32` | Optional `TCP_INFO` cumulative retransmits and outbound segments |
| `tls_handshake_ms` | `uint32` | Optional TLS handshake duration (ms) |
| `waf` | `*WAFAudit` | Optional blocked/observed audit (`action`, `reason`, `flow_id`, `flow_node_id`, `attack_group_id`, `rules`, truncated request preview); written to `waf_action` & `waf_audit` |

`internal/apps/openflare/agent/observability.go` populates Snowflake `id`, GeoIP `region`, and NetIntel `client_asn`, and emits records with `client_asn > 0 && tcp_rtt_us > 0` into `netquality.RecordPassiveSampleInput`.

---

## 3. Target Schemas (ClickHouse & Primary DB Parity)

> Matching column definitions are maintained across ClickHouse, PostgreSQL, and SQLite via `internal/infra/persistence/migrator/goose/`. PostgreSQL uses monthly `PARTITION BY RANGE` on high-volume tables, whereas ClickHouse uses `MergeTree` family engines with `toYYYYMM` partitioning and native `TTL`.

### 3.1 `of_node_access_logs` (90-Day Default Retention)

* **ClickHouse sort key**: `ORDER BY (node_id, logged_at, host, status_code, remote_addr)`, TTL `90 DAY`.
* **Columns**: `id`, `node_id`, `logged_at`, `remote_addr`, `region`, `host`, `path`, `user_agent`, `cache_status`, `status_code`, `bytes_sent`, `request_length`, `request_time_ms`, `waf_action`, `waf_audit`, `client_asn`, `tcp_rtt_us`, `created_at`.
* **ClickHouse hourly MV**: `of_access_log_hourly` (`SummingMergeTree`, keyed by `(node_id, hour, host)`) aggregates `request_count`, `error_count`, `bytes_sent`, and `request_length` only; UV stays excluded so multi-hour sums cannot overcount unique IPs.

### 3.2 `of_node_metric_snapshots` (30-Day Default Retention)

* **ClickHouse sort key**: `ORDER BY (node_id, captured_at, id)`, TTL `30 DAY`.
* **Columns**: `id`, `node_id`, `captured_at`, `cpu`, `gpu`, `ram`, `ram_total`, `swap`, `swap_total`, `load`, `load5`, `load15`, `temp`, `disk`, `disk_total`, `disk_read_bytes`, `disk_write_bytes`, `net_in`, `net_out`, `net_total_up`, `net_total_down`, `process`, `connections`, `connections_udp`, `uptime`, `created_at`.
* **ClickHouse hourly MV**: `of_node_metric_capacity_hourly` (`AggregatingMergeTree`) tracks CPU/RAM sums/counts plus hourly `min`/`max` of cumulative disk and NIC byte counters.

### 3.3 `of_node_ping_records` and `of_dns_metric_snapshots`

* **`of_node_ping_records`**: `id`, `node_id`, `task_id`, `recorded_at`, `value` (ms, `-1` on failure), `created_at`; ClickHouse `ORDER BY (node_id, task_id, recorded_at, id)`, TTL `30 DAY`.
* **`of_dns_metric_snapshots`**: `id`, `instance_id`, `captured_at`, `queries_total`, `queries_success`, `queries_nxdomain`, `queries_servfail`, `queries_ratelimited`, `qps`, `latency_p50_ms`, `latency_p95_ms`, `latency_p99_ms`, `cache_hit_ratio`, `snapshot_version`, `cpu`, `ram_percent`, `created_at`; ClickHouse `ORDER BY (instance_id, captured_at, id)`, TTL `30 DAY`.

### 3.4 `of_network_quality_samples` (30-Day Default Retention)

* **ClickHouse sort key**: `ORDER BY (node_id, client_asn, captured_at, id)`, TTL `30 DAY`.
* **Columns**: `id`, `node_id`, `node_asn`, `client_asn`, `client_country`, `client_region`, `ip_family`, `source` (`tcp_passive`/`tcp_active`/`icmp_active`/`traceroute`), `sample_count`, `rtt_min_ms`, `rtt_p50_ms`, `rtt_p95_ms`, `retrans_rate`, `loss_rate`, `hop_count`, `stats_version`, `rtt_hist` (`LogHistogram64`), `prefix_sketch` (`PrefixSketch`), `distinct_prefix_count`, `client_lat`, `client_lon`, `geo_accuracy_km`, `geo_source`, `captured_at`, `created_at`.

### 3.5 Removed Legacy Tables

The following obsolete OpenResty, FRP, and pre-aggregation tables have been dropped across Goose migrations:

* `of_node_edge_health` (dropped by `202611270006` in PG/SQLite and `202611270003` in ClickHouse; latest socket count moved to `of_nodes.openresty_connections`)
* `of_node_obs_frps`, `of_node_obs_frpc` (dropped by `202611230001` in ClickHouse)
* `of_node_request_reports`, `of_node_traffic_hourly`, `of_node_obs_openresty`, `of_node_openresty_hourly`
