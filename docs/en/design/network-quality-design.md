# Network Intelligence and Link Quality Architecture

OpenFlare builds link-quality visibility for diagnostic tools and SmartDNS by combining multi-source network intelligence (`NetIntel`), passive `TCP_INFO` telemetry from `openflare-edge`, bounded active probing (`openflare-edge probe-worker`), an adaptive multi-window matrix, and a five-tier latency predictor.

---

## 1. Multi-Source Network Intelligence (`internal/apps/openflare/netintel`)

The control plane maintains a lock-free in-memory lookup engine (`pkg/netintel.Manager`) refreshed by the scheduled `of_netintel_sync` task. When Redis is enabled, reload events broadcast over `netintel:reloaded` to synchronize all controller replicas.

| Dataset | Local Files | Default Upstream & Sync Policy | License / Purpose |
| :--- | :--- | :--- | :--- |
| **GeoIP / ASN** | `GeoLite2-{Country,City,ASN}.mmdb` or `sapics-{country,asn}.mmdb` | FyraLabs GeoLite2 mirror by default (switchable to `sapics`); skips files fresher than 1 hour unless `force=true`; falls back to embedded Country MMDB | Country, ISO 3166-2 region, coordinates, accuracy radius, and origin ASN |
| **BGP Routing & Tags** | `bgptools-asns.csv`, `bgptools-tags.txt`, `bgptools-asn-tags.txt`, `bgptools-table.jsonl.gz` | Synced from `bgp.tools` with `512 MiB` file cap (`32 MiB` for text lists) | Longest-prefix lookup, ASN names/tags, and prefix inventory |
| **AS Relationships (optional)** | `caida-as-rel2.txt.bz2` | Opt-in via `netintel_caida_enabled = true`; downloads latest CAIDA serial-2 archive with bzip2 framing check (12h freshness window) | AS neighbor topology, Tier-1 reachability, and upstream path inference |

Each sync pass also backfills missing `asn` and `asn_org` fields on edge nodes (up to 500 per batch) and updates `manifest.json`.

---

## 2. Passive Telemetry Collection (`openflare-edge` → `netquality`)

`openflare-edge` captures transport-layer RTT and retransmission metrics from real client connections without sending synthetic probes:

```text
Client request completes on openflare-edge
  └─ Reads kernel TCP_INFO when AccessLogBuffer has capacity
       └─ Heartbeat carries tcp_rtt_us / tcp_min_rtt_us / tcp_rtt_var_us / tcp_retrans / tcp_segs_out / tls_handshake_ms
            └─ Server passive_collector (flushes every 60s or at 80% high watermark)
                 ├─ Lock-free NetIntel enrichment: client_asn, client_country, client_region, ip_family, coordinates, geo_source
                 └─ Aggregates into buckets keyed by (node_id, client_asn, client_country, client_region, ip_family)
                      ├─ LogHistogram64 (64 log-uniform bins over [0.5 ms, 10000 ms] with harmonic-geometric midpoints)
                      ├─ PrefixSketch (FNV-64 bottom-64 deduplication over IPv4 /24 and IPv6 /48 prefixes)
                      └─ SphericalCentroid & min RTT
                           └─ Batch insert via logstore (of_network_quality_samples)
```

* **Memory bounds**: `passive_collector` caps in-memory buckets at `maxPassiveBuckets = 8192` (triggering early flush at 80% / `6553` buckets) and discards outliers exceeding `60s` (`60,000,000 µs`).
* **Strict address-family isolation**: `ipv4`, `ipv6`, and `unknown` paths remain separated across samples, matrix rows, and predictions rather than relying on cross-family scaling ratios.

---

## 3. Adaptive Quality Matrix and Five-Tier Predictor

The `of_netquality_rollup` task scans up to `100,000` recent `of_network_quality_samples` over a 24-hour lookback window together with up to `5,000` `of_traceroute_records`, producing two-level rows (`country`, plus `region` drilldowns for whitelisted countries) in `of_network_quality_matrix`.

### 3.1 Multi-Window State Machine (`adaptive_state.go`)

Each `(node_id, client_asn, client_country, client_region, ip_family, aggregation_level)` cell tracks multiple windows to compute `effective_rtt_ms` and `quality_state`:

1. **Baseline (24h, 6h half-life)** and **EWMA (2h half-life)** blending `tcp_passive`, `tcp_active`, `icmp_active`, and `traceroute` measurements.
2. **Instant (`10m`)** and **short (`45m`, 20-minute half-life)** windows:
   * When sample count meets threshold and P50 jumps by `>= 1.35x` and `>= 15 ms` over baseline on the 10-minute window (`1.30x` / `>= 12 ms` on 45-minute), or loss/retrans climbs to `>= 5%` (`>= baseline + 3%`), state immediately transitions to `degraded` and switches `effective_rtt_ms` to the impaired window P50;
   * Recovery requires `2` consecutive healthy confirmation windows (`p50 <= ref * 1.25 + 15 ms` without loss spikes): window 1 enters `recovering` (recording `observed_rtt_ms` while holding conservative `effective_rtt_ms`), and window 2 restores `healthy`;
   * Cells unobserved for more than `60` minutes transition to `stale`, retaining any prior impaired `effective_rtt_ms` so quiet broken links do not appear falsely fast.

### 3.2 Dynamic P95 Tail Weight and Exploration

* **Dynamic P95 penalty**: Tail weight stays `0` below `10` samples and scales smoothly up to `0.35` as both sample volume and `PrefixSketch` `/24` or `/48` prefix diversity increase.
* **Exploration priority (`explore_priority`)**: Combines observation staleness, prediction uncertainty, and latency gap from the current optimal node to prioritize active verification on stale or under-sampled routes.

### 3.3 Five-Tier Latency Predictor (`predictor.go`)

`Predictor` caches results for `5` minutes (up to `4096` entries, invalidated across the cluster slice whenever `CurrentRollupGeneration()` advances), quantizing `lat` / `lon` into `0.2°` buckets and bypassing L3 transfer caching for `confirmed` Anycast targets:

| Tier | Method (`PredictionMethod`) | Matching Rule & Behavior |
| :--- | :--- | :--- |
| **L1** | `exact_region` | Direct match on `(node_id, client_asn, client_country, client_region, ip_family)` |
| **L2** | `country_rollup` | Country-level matrix fallback with a `0.90` confidence multiplier |
| **L3** | `inferred_upstream` | Same-node similarity-weighted residual transfer over shared country and BGP/CAIDA upstream profiles (`minTotalWeight = 0.12`, shrinkage `1.20`, confidence capped at `0.72`; emits weighted IQR as `CandidateDispersionMs`) |
| **L4** | `similar_node` | Cold-start fallback for sparse nodes (`<= 2` measured rows) using sibling nodes in the same `network_group`, within `50 km`, or sharing an ASN (`0.65` confidence multiplier) |
| **L5** | `geo_prior` | Geographic great-circle propagation prior (`1.35 * (2d / 200) + 8 ms`), or `120 ms` (`0.05` confidence) when coordinates are unavailable |

`ComputeQualityScore(res)` folds `effective_rtt_ms + 0.25 * CandidateDispersionMs` and loss penalties into `[1.0, 100.0]` without altering the raw `RTTMs` value used by rule thresholds.

### 3.4 SmartDNS Integration Contract & Zero-SQL Hot Path

* **Anti-corruption boundary (`internal/apps/openflare/qualityapi`)**: SmartDNS never queries `netquality` or `pkg/netintel` storage directly and consumes normalized `PredictionResult` records via `qualityapi.Provider`:
  * `RTTMs`, `ObservedRTTMs`, `PredictedRTTMs`: effective RTT and baseline/observed values;
  * `RTTP95Ms` / `P95Weight`: tail latency estimate and dynamic penalty weight (`[0, 0.35]`);
  * `CandidateDispersionMs` / `ErrorMs`: weighted IQR spread across L3/L4 transfer candidates and holdout P80 calibration error (`nil` when uncalibrated);
  * `QualityState`, `ExplorePriority`, `MeasurementSource`, `SourceTier` (`measured_region`, `measured_country`, `inferred`, `geo_prior`), `GeoBaseRTTMs` / `GeoBaseKnown` (`120 ms` L5 fallback when coordinates are absent), and `AnycastIsolated` (`true` for confirmed Anycast targets to block cross-region transfer).
* **Zero SQL on DNS hot paths**: The embedded Go DNS engine uses `QualityProvider` (in-memory 60 s cache keyed by `CurrentRollupGeneration()`), while standalone `openflare-dns` indexes `CompactQualitySnapshot` and `CompactNodePrior` directly in memory for O(1) lookups.

---

## 4. Controlled Active Probing and Isolated Worker (`shared/probe`)

Interactive console diagnostics (Ping, Traceroute, DNS lookup) and scheduled `of_netquality_probe` runs share the same safety and privilege boundary.

### 4.1 Scheduled Auto-Probing (`of_netquality_probe`)

Controlled by `netintel_auto_probe_enabled` (default `true`), dispatched over WebSocket to online, non-quarantined `edge_node` agents:

* **Targets and backoff**: Managed targets reside in `of_probe_targets`, with per-`(node_id, target_id)` health and exponential failure backoff (`15m` base to `24h` max; `20m` minimum cooldown) tracked in `of_probe_target_states`.
* **Dispatch budgets**: Each scheduled pass caps at `24` global dispatches, `4` targets and `120` packet cost per node (`4` packets per Ping, `48` estimated packets for `24`-hop × `2`-probe Traceroute), and `4` nodes per target.

### 4.2 Bidirectional SSRF Enforcement (`safety.go` & `shared/probe/src/safety.rs`)

Both the control plane at submission time and `probe-worker` right after edge-side DNS resolution reject loopback, private (RFC 1918 / ULA), link-local, CGNAT (`100.64.0.0/10`), benchmarking (`198.18.0.0/15`), multicast, unspecified, IPv4-mapped IPv6 addresses, and internal domain suffixes (`localhost`, `.localhost`, `.local`, `.internal`, `.localdomain`, `.home.arpa`).

### 4.3 Isolated `openflare-edge probe-worker` (`shared/probe`)

Rather than keeping raw-socket loops inside the proxy process, `ProbeSupervisor` (`queue_capacity = 64`, `50s` task timeout, `1s..30s` restart backoff) manages a dedicated `openflare-edge probe-worker` subprocess (default idle timeout `300s`):

* **Process & privilege boundary**: On Linux, `probe-worker` sets `PR_SET_PDEATHSIG(SIGTERM)` so the kernel terminates it if the supervisor dies, sets `PR_SET_DUMPABLE=0`, `RLIMIT_CORE=0`, and `nice +10`, and acquires `CAP_NET_RAW` via `trippy_privilege` solely inside the isolated worker.
* **Local packet budget guard**: Even under control-plane anomalies, `probe-worker` enforces a rolling 60-second cap of `180` global dispatches / `2,400` packets and `16` dispatches / `600` packets per target (`2048` tracked target slots).
