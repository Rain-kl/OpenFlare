# SmartDNS System Architecture Design

OpenFlare SmartDNS pairs a control-plane multi-window network quality matrix, node capacity metrics, and a visual JSON Decision Model (JDM) rule engine with an EDNS0 Client Subnet (ECS)-aware authoritative DNS data plane to deliver low-latency, overload-protected traffic steering.

---

## 1. Topology and Anti-Corruption Boundary

* **Decoupled storage boundary**: Neither the Go SmartDNS control plane (`internal/apps/openflare/smartdns`) nor the standalone Rust DNS server (`dns/`) queries `netquality` or `pkg/netintel` tables directly. Both consume normalized link-quality predictions and `CompactQualitySnapshot` artifacts through `internal/apps/openflare/qualityapi.Provider` (mirrored on Rust by `dns/src/quality.rs::QualityOracle`), enforced in tests by `internal/apps/openflare/smartdns/imports_test.go`.
* **Snapshot compilation and offline resilience**:
  * `SnapshotManager` compiles online, non-quarantined nodes (with tags, capacities, and live loads), enabled routing policies (with JDM JSON), `target_mode = smartdns` domain mappings, and `CompactQualitySnapshot` into an immutable, versioned `DNSSnapshotEnvelope` with a strong `ETag`.
  * Configuration or pointing updates emit `listener.EmitSmartDNSSnapshotDirty` to trigger a 250 ms debounced rebuild and push `dns:snapshot_changed` wakes over `/api/v1/dns-agent/ws` alongside 10-second polling/heartbeat convergence.
  * Standalone `openflare-dns` persists the latest snapshot to `--cache-dir/last-snapshot.bin` so authoritative serving continues cleanly if the control plane becomes temporarily unreachable.

---

## 2. Authoritative Data Plane (`openflare-dns` and Embedded Mode)

OpenFlare supports both a **standalone Rust server (`openflare-dns`)** and an **embedded Go server**, sharing identical decision semantics:

* **Multi-core socket handling (`dns/src/server.rs`)**: Built on `hickory-server`, binding per-worker `SO_REUSEPORT` UDP sockets and TCP listeners on Unix systems. Authoritative responses handle `A`, `AAAA`, `NS`, and `SOA`; non-authoritative zones return `REFUSED`, and empty record types return RFC-compliant `NODATA` with an `SOA` authority record.
* **ECS extraction, sharded RRL, and L1 answer cache (`dns/src/cache.rs`)**: Uses valid non-private ECS client subnets for Geo/ASN resolution and echoes `/24` (IPv4) or `/48` (IPv6) response scope prefixes. Normalized `ClientPrefixKey` keys drive a 64-shard token-bucket Response Rate Limiter (RRL) and a `moka` L1 answer cache isolated by `(version, qname, qtype, prefix, from_ecs)`.
* **Async JDM isolation (`dns/src/engine.rs`)**: Compiles JDM decision graphs via `zen-engine`. Because `Decision::evaluate` holds `!Send` futures, evaluations run on a dedicated blocking-pool `current_thread` runtime only on L1 cache misses.
* **Remote updates**: `openflare-dns` supports heartbeat `UpdateReceipt` reporting and staged self-update requests (`dns/src/update.rs`) under managed hosts.

---

## 3. Shared JDM Evaluation and Four-Stage Node Selection

Control-plane Go (`smartdns/scoring.go`, `embedded_go_dns.go`) and Rust (`dns/src/engine.rs`) share the same JDM output contract and four-stage scoring flow:

1. **Candidate filtering**: Applies policy `filters` (`node_ids`, `countries`, `tags`, `network_groups`) across online, unquarantined nodes in either `smart` or `load_balance` mode.
2. **Multi-factor scoring**:
   * **`LoadIndex`**: Weighted composite of CPU (`45%`), memory (`25%`), and capacity utilization (`30%`); nodes exceeding `0.92` are flagged overloaded and excluded from the primary pool.
   * **Latency quality score (`S_lat`)**: Combines `effective_rtt_ms`, dynamic `p95_weight` tail penalty, and `loss_rate` penalty. Rust resolves `region` → `country` → country any-ASN fallback → node/geo prior in O(1) hash lookups (see [Network Intelligence and Link Quality Architecture](./network-quality-design.md)).
   * **Geographic proximity (`S_geo`)**: Smooth Haversine distance decay (`2000 km` scale).
   * **Tag multipliers & weighted load balancing**: Scales scores by active node tag multipliers (`[0.25, 4.0]`). In `load_balance` mode, deterministic weighted ordering (`Efraimidis-Spirakis`) hashes client prefix, domain, and node ID.
   * **Bounded exploration budget**: Allocates `0.5%–3%` time-bucketed traffic (with TTL capped at `25s`) to verify stale or exploratory candidates without causing route flapping.
3. **5% hysteresis anti-flapping**: Retains the sticky node tracked for `(policy_id, client_prefix, qtype)` over a 10-minute window unless a non-overloaded challenger beats it by more than `1.05×` (`5%`).
4. **Guaranteed fallback chain**: If strict filtering yields zero healthy candidates, falls back sequentially through configured fallback node IDs → overloaded online nodes → unfiltered online nodes → unquarantined nodes so valid answers still return whenever possible.

---

## 4. Telemetry and Multi-Database Storage

Standalone `openflare-dns` instances record latency percentiles (`p50`, `p95`, `p99`) with a 16-shard `hdrhistogram` and report QPS, query outcome counts, cache hit ratio, snapshot version, CPU, and RAM usage on heartbeats. The control plane writes snapshots via `logstore.Observability.InsertDNSMetricSnapshot` into ClickHouse or PostgreSQL/SQLite (`of_dns_metric_snapshots`).
