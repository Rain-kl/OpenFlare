# OpenFlare Edge node Development Notes

OpenFlare Edge nodes are built under `edge/` and communicate with the control plane.

## Package Structure

| Module | Responsibility |
| --- | --- |
| `edge/src/main.rs`, `runtime/` | Startup arguments, component assembly, and listener readiness |
| `protocol/` | Decodes unified snapshot contracts and converts runtime parameters |
| `control/` | Registration, identity, heartbeat, WebSocket notifications, configuration and resource synchronization |
| `config.rs` | Candidate validation, snapshot recovery, persistence, and in-memory publishing |
| `proxy/` | OpenFlare Edge listeners, TLS, request phases, upstream proxying, caching, and response handling |
| `policy/` | Authentication, PoW, WAF, rate limiting, and Service Worker integration |
| `pages/`, `geoip.rs`, `telemetry/` | Pages artifact deployment and static serving, GeoIP data, and node metrics reporting |
| `internal/openflare/edgeconfig/` | Generates Edge transmission payloads on the control plane using shared snapshots and published resources |

## Configuration Interfaces

Heartbeats return a versioned resource manifest. The WebSocket `config_changed` event triggers immediate heartbeats. Small configurations are inlined directly into the manifest, while large configurations are pulled by SHA-256 digest from dedicated endpoints. See [OpenFlare Edge node Design](./pingora-edge-design.md#configuration-synchronization) for resource providers and synchronization contracts.

## Build and Verification

```bash
cargo fmt --manifest-path edge/Cargo.toml --check
cargo clippy --manifest-path edge/Cargo.toml --locked --all-targets -- -D warnings
cargo test --manifest-path edge/Cargo.toml --locked
cargo build --manifest-path edge/Cargo.toml --locked
python3 edge/tests/runtime_smoke.py edge/target/debug/openflare-edge
```

When building directly with Cargo, the binary version defaults to `dev`. To reproduce release builds, set `OPENFLARE_EDGE_VERSION=0.1.3` before compilation, using the release tag value stripped of the `edge-v` prefix.

Smoke tests use a local mock control plane and origin server, covering node registration, configuration acknowledgement, HTTPS redirection, per-domain HTTP/2 negotiation, site authentication, PoW browser protocol, WAF IP groups, Pages switching, cache hits, and offline restarts. Smoke tests also verify WebSocket wakeups, hash-based resource pulls, live performance parameter updates, and automatic worker thread reloads.

```bash
openflare-edge run \
  --server-url https://control.example.com \
  --token-type discovery \
  --token-file /etc/openflare-edge/bootstrap-token \
  --state-dir /var/lib/openflare-edge
```

Pre-allocated nodes use `--token-type node`. Nodes with persisted dedicated credentials restore identity directly from their state directory. Linux distribution packages use the `edge-v*` release tag and publish both `amd64` and `arm64` artifacts. Installation and update scripts atomically replace the single binary and verify startup via systemd readiness notifications. Nodes restore valid snapshots from their isolated state directory upon restart.
