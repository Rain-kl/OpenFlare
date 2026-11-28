# OpenFlare Edge Architecture Design

OpenFlare Edge (`openflare-edge`) is an edge proxy and security gateway built on Pingora. It terminates TLS, proxies HTTP/HTTPS and WebSocket traffic, evaluates WAF DAG flows and OWASP CRS rules, serves proof-of-work challenges, enforces site authentication and rate limits, caches HTTP responses via TinyUFO, serves local Pages deployments, and collects runtime telemetry. The control plane distributes configuration through a versioned manifest protocol (`config_sync v1`), allowing edge nodes to validate and atomically swap configurations in memory without process reloads or dropped connections.

---

## 1. Dual-Process Model and Supervision

A single `openflare-edge` binary coordinates a control-plane supervisor and a Pingora data-plane worker (`edge/src/main.rs`, `edge/src/control/`, `edge/src/proxy/`):

```text
systemd (Type=notify, Restart=always, RestartSec=5)
  └─ openflare-edge supervisor (control plane)
       ├─ Heartbeat / WebSocket control channel, config & asset sync, TLS memory-key fetch
       ├─ Public IP resolution, Pages package download & atomic extract, access-log spool flush
       ├─ 1s listener liveness polling & ready.json readiness marker
       ├─ Isolated probe subprocess: openflare-edge probe-worker (acquires CAP_NET_RAW on demand)
       └─ Pingora data-plane worker
            ├─ HTTP / HTTPS (SNI + ALPN HTTP/2) listeners (2s graceful drain period)
            ├─ WAF / PoW / Auth / RateLimit / EdgeCache / Origin Error Page
            └─ Direct reverse proxy, tunnel-routing.v1 L7 gateway routing, local Pages serving
```

* **Single control-loop lock and readiness signaling**: An exclusive `flock` on `$STATE_DIR/control.lock` ensures only one control loop owns a given state directory at any time. Once all configured HTTP and TLS listeners are bound, the node writes `$STATE_DIR/ready.json` (`pid` and `binary_version`), satisfying systemd `Type=notify` and container `HEALTHCHECK` readiness gates.
* **Watchdog recovery and clean listener rebinds**: The control plane polls listener readiness at 1-second intervals and restarts failed data-plane workers, while systemd restarts an exited supervisor after 5 seconds. On startup-scope configuration changes (such as worker thread counts or socket tuning), the Pingora worker drains connections over a 2-second grace period (`grace_period_seconds = 2`) and re-executes itself via `exec()` (stripping `--upgrade`) when no binary self-update is pending so updated socket options take effect cleanly.
* **Cold-start local snapshot recovery**: Verified configuration snapshots persist to `$STATE_DIR/snapshot.json` (with fallback to `snapshot.previous.json`). Running nodes continue serving both HTTP and HTTPS during control-plane outages. If a node cold-starts while the control plane is unreachable, HTTP routes and static policies recover immediately from `snapshot.json`, and HTTPS private keys hydrate automatically over `POST /api/v1/agent/tls/keys` once connectivity returns—no restart required.

---

## 2. Process Hardening and Least-Privilege Model

* **Kernel dump and trace hardening**: On Linux, `edge/src/main.rs` runs `harden_process()` at startup, calling `prctl(PR_SET_DUMPABLE, 0)` and `setrlimit(RLIMIT_CORE, 0)` to disable core dumps and block unauthorized `ptrace` memory inspection.
* **systemd sandboxing (`scripts/install-openflare-edge.sh`)**: The host installer creates an unprivileged `openflare-edge` service user/group and grants only `CAP_NET_BIND_SERVICE` and `CAP_NET_RAW` (consumed strictly by the isolated `probe-worker` subprocess). The unit enforces `NoNewPrivileges=true`, `PrivateTmp=true`, `ProtectSystem=strict`, `ProtectHome=true`, `ProtectKernelTunables=true`, `ProtectKernelModules=true`, `ProtectControlGroups=true`, `RestrictSUIDSGID=true`, `LockPersonality=true`, `MemoryDenyWriteExecute=true`, `RestrictNamespaces=true`, `SystemCallFilter=@system-service`, `LimitCORE=0`, and `UMask=0077`.
* **Restricted writable footprint**: Only `/var/lib/openflare-edge` (`0750`, owned by `openflare-edge:openflare-edge`) is writable, storing `identity.json` (`0600`), `snapshot.json` (`0600`), `performance-policy.json` (`0600`), `tunnel-routing.json` (`0600`), GeoIP MMDB files, Pages releases, and `access-logs.jsonl`.
* **Static musl container runtime (`docker/Dockerfile.edge`)**: The multi-arch Alpine image bundles a static musl binary verified free of dynamic `INTERP` or `NEEDED` headers, runs as non-root `10001:10001`, and applies `setcap cap_net_raw+p /usr/local/bin/openflare-edge`. Combined with `--cap-add=NET_RAW`, only active ICMP/traceroute probing threads inside `probe-worker` raise `CAP_NET_RAW`.

---

## 3. In-Memory TLS Private Keys and Emergency Quarantine

Under configuration contract v4 (`CONTRACT_VERSION = 4`, advertising `shared-snapshot-v1`, `tls-sni-h2-v1`, `policy-runtime-v1`, and `tls-memory-keys-v1`), TLS private keys never touch edge node storage:

1. **Sanitized on-disk snapshots**: Control-plane site snapshots and persisted `snapshot.json` files store public certificates (`cert_pem`) and SHA-256 digests (`cert_sha256`), omitting `key_pem`.
2. **Memory key ring and `zeroize`**: Nodes fetch private keys within their active certificate scope over authenticated, cache-disabled `POST /api/v1/agent/tls/keys` requests and load them into `CertificateKeyRing` indexed by `(cert_id, cert_sha256)`. Temporary PEM buffers use `zeroize::Zeroizing<String>`, and snapshot swaps invoke `retain_for()` to purge deleted or rotated keys immediately.
3. **Delivery audit trail and quarantine (`of_node_cert_deliveries`)**: The control plane records every certificate SHA-256 ever delivered to a node's memory alongside delivery count and release sequence. Triggering emergency node quarantine:
   * Rotates and revokes the node's `X-Agent-Token` and invalidates authentication caches;
   * Sends a `quarantined` WebSocket event instructing the edge node to wipe `CertificateKeyRing` (`clear()`);
   * Automatically fails over affected DNS pointing groups (`FailoverQuarantinedNode`);
   * Optionally issues fresh ACME certificates and revokes exposed keys with `CRLReasonKeyCompromise`.

---

## 4. Configuration Synchronization and Fault Isolation

Nodes receive manifests on periodic heartbeats (`POST /api/v1/agent/nodes/heartbeat`) and wake immediately on WebSocket `config_changed` events (forwarded across controller instances via Redis when enabled). Missed WebSocket notifications converge automatically on subsequent heartbeats.

| Resource Key | Payload | Sync Mode |
| :--- | :--- | :--- |
| `performance.v1` | Node-specific performance policy or built-in defaults | Inline JSON |
| `sites.v1` | Routes, public certs, compiled WAF graphs, error/PoW pages, Pages refs | Revision + SHA-256 pull |
| `tunnel-routing.v1` | L7 tunnel gateway relay endpoints sorted by zone/health | Digest-based pull |
| `waf.ip-group.v1/<id>` | IP groups referenced by active snapshot flows | Inline when small, pull otherwise |

Providers are registered in `internal/apps/openflare/agent/config_sync.go` with inline limits of `8 KiB` per resource and `32 KiB` per manifest. Nodes pull large or changed payloads via `GET /api/v1/agent/config/pull?key=<key>&sha256=<digest>` (receiving HTTP `409` if superseded mid-flight) and ignore unknown keys.

* **Validation and fallback**: Heartbeats report both `latest_config_version` and `current_version`. A candidate snapshot only becomes active after digest checks, schema decoding, certificate fingerprint matching, and local Pages asset verification succeed. If any step fails, the node keeps serving its last healthy snapshot, records the error, and skips re-pulling the same failed revision until a newer revision arrives, the node restarts, or an operator triggers force sync.

---

## 5. Performance Tuning, HTTP Cache, and Self-Update

### 5.1 Performance Policies

Each node can attach to an explicit performance policy (falling back to built-in defaults) stored in `$STATE_DIR/performance-policy.json`.

* **Hot reloadable tuning**: Upstream timeouts, TLS verification and origin ALPN (`h1`, `h2h1`, `h2`), cache enablement, and pool parameters apply in memory while draining superseded connections.
* **Startup reload tuning**: Changes affecting worker thread counts (`threads` / `threads_auto`), `listener_tasks_per_fd`, `SO_REUSEPORT`, `TCP_FASTOPEN`, DSCP, or cache capacity trigger a controlled restart/re-exec.

### 5.2 TinyUFO HTTP Response Caching

`edge/src/proxy/cache.rs` integrates TinyUFO storage through [pingap-cache](https://github.com/vicanso/pingap):

* **Memory and optional disk storage**: Defaults to compact in-memory TinyUFO (`memory://openflare?max_size={capacity}MiB&mode=compact`, `128 MiB` default capacity, `16 MiB` default max object size). When disk cache is enabled, objects store in `$STATE_DIR/http-cache` (`0700`, `levels=2:2`) with TinyUFO backing hot pages and a periodic `DiskCleanup` task sweeping objects older than `cache_disk_inactive_seconds` (default `172800`s). Disk initialization failures automatically fall back to memory caching. See [Performance Policies](../guide/performance-policies.md#cache-storage) for tuning fields.
* **Opaque key hashing**: Cache keys hash `{digest}|{max_object_bytes}|{scheme}|{host}|{request_uri}` before passing keys to `pingap-cache`, preventing raw URLs or query parameters from appearing in cache logs or filenames and isolating entries across snapshot revisions and object size limits.

### 5.3 Access-Log Buffering and Spooling

Request logs queue into a bounded `AccessLogBuffer` (`QUEUE_CAPACITY = 2048`, `16 MiB` semaphore budget) and flush every `256` entries or `100 ms` under `access-logs.lock` (`flock`) to `$STATE_DIR/access-logs.jsonl` (`0600`). Disk spooling enforces hard caps of `2048` entries and `4 MiB` to protect disk space when disconnected. Heartbeats peek up to `1024` records (`MAX_HEARTBEAT_ACCESS_LOGS`), sample kernel `TCP_INFO` only while buffer capacity permits, and truncate acknowledged records via `spool.commit()` strictly after control-plane HTTP 200 acknowledgement.

### 5.4 Public IP Detection and Three-Slot Self-Update

* **Public IP resolution (`edge/src/control/node_ip.rs`)**: Scans local network interfaces at startup and every 60 seconds, excluding private/reserved CIDRs (`0.0.0.0/8`, `10.0.0.0/8`, `100.64.0.0/10`, `127.0.0.0/8`, `169.254.0.0/16`, `172.16.0.0/12`, `192.0.0.0/24`, `192.0.2.0/24`, `192.88.99.0/24`, `192.168.0.0/16`, `198.18.0.0/15`, `198.51.100.0/24`, `203.0.113.0/24`, `224.0.0.0/4`, `240.0.0.0/4`, and reserved IPv6 prefixes). When no routable public interface address is present, it queries `https://api.ip.sb/ip`, `https://ip.me`, `https://api.ip2location.io/`, and controller `GET /api/geoip`, falling back to `0.0.0.0` if all probes fail.
* **Three-slot self-update (`edge/src/control/update/mod.rs`)**: Non-root Linux (`x86_64` / `aarch64`) services managed by systemd (`NOTIFY_SOCKET` and `INVOCATION_ID` set) check GitHub `edge-v*` releases every 6 hours (or execute operator-dispatched commands). Downloads enforce HTTPS host allowlists, package bounds (128 MiB compressed / 256 MiB expanded), SHA-256 verification, and compatibility subcommands (`config-sync-version`, `self-update-version`), progressing through `edge-update-next → edge-update-trial → edge-update-active` with automatic rollback if readiness fails within 15 minutes.

---

## 6. Independent Releases and Version Injection

OpenFlare Edge releases independently under `edge-v*` tags, decoupled from Server, Relay, and DNS release schedules:

* **Compile-time version injection**: CI workflows (`.github/workflows/build-release-openflare-edge.yml`, `publish-release-openflare-edge.yml`, `build-image-openflare-edge.yml`) inject `OPENFLARE_EDGE_VERSION` from the release tag without the `edge-v` prefix (for example, `edge-v0.1.6` reports `0.1.6`; direct local builds default to `dev`) and compile with `cargo build --locked --release`.
* **Multi-arch archives and container images**: Releases publish `openflare-edge_${VERSION}_linux_{amd64,arm64}.tar.gz` (binary, `VERSION`, `SHA256SUMS`, licenses, and lifecycle scripts) alongside static musl Alpine images at `ghcr.io/<owner>/openflare-edge`. Pushes to `canary` or manual dispatches generate `edge-v<next-patch>-canary.<run_number>` prereleases, while stable installers and automatic updaters filter strictly for stable `edge-vX.Y.Z` tags.

---

## 7. Code Module Structure

| Module | Responsibility |
| :--- | :--- |
| `edge/src/main.rs`, `runtime/` | CLI subcommands (`run`, `probe-worker`, `config-sync-version`, `self-update-version`, `attack-rule-catalog`), Linux hardening, listener readiness, and policy/tunnel runtime state |
| `edge/src/protocol/` | Control-plane contract decoding (`CONTRACT_VERSION = 4`) and runtime schemas |
| `edge/src/control/` | Enrollment, `identity.json` persistence, heartbeats, WebSocket wakes, `config_sync`, `node_ip` probing, and three-slot self-update |
| `edge/src/config.rs` | Snapshot validation, cold-start recovery, in-memory `CertificateKeyRing`, and atomic swap |
| `edge/src/proxy/` | Pingora listeners (2s graceful period), TLS SNI/ALPN, request pipeline, direct and `tunnel-routing.v1` proxying, TinyUFO cache, and origin error page replacement |
| `edge/src/policy/` | Site auth, Basic auth, PoW, WAF DAG + `zentinel-modsec` CRS execution, rate limiting, and Service Worker |
| `edge/src/pages/`, `geoip.rs`, `telemetry/` | Pages package sync & Range/ETag serving, GeoIP MMDB auto-refresh, `AccessLogBuffer`/`AccessLogSpool`, `HostCollector`, and live log streaming |
| `shared/probe/` | `ProbeSupervisor`, isolated `probe-worker` subprocess execution, bidirectional SSRF checks, and dispatch packet budgets |
| `internal/openflare/edgeconfig/`, `internal/apps/openflare/agent/` | Control-plane snapshot compilation, `/tls/keys` endpoint, and `config_sync v1` resource providers |
