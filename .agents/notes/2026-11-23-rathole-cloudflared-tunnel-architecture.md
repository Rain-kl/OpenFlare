# Native Rust Rathole + Cloudflared HA Tunnel Architecture Migration

- **Date**: 2026-11-23
- **Scope**: Complete replacement of legacy Go-wrapped FRP (`frps`/`frpc`) tunnel subsystem with native Rust `openflare-relay` + `openflared` embedding `rathole`, Pingora `tunnel-routing.v1` integration, `/tunnels` console UI, packaging, CI/CD, and Chinese documentation.

## Architectural Decisions & Tradeoffs

1. **Zero FRP Compatibility / Clean-Slate Schema Redesign**:
   - Dropped legacy `tunnel_relay`/`tunnel_client` node types (`w_nodes`), legacy FRP tables (`w_tunnels` old schema), `cmd/relay`, `cmd/flared`, `internal/edge/{common,relay,flared}`, `pkg/wsclient`, and `internal/apps/openflare/{flared,relay}`.
   - Designed 6 normalized tables (`w_tunnel_relay_pools`, `w_tunnel_relays`, `w_tunnels`, `w_tunnel_connectors`, `w_tunnel_attachments`, `w_tunnel_services`) with goose dual-dialect migrations (`202611230004`–`202611230006`) and ClickHouse cleanup (`202611230001`).
   - Existing `tunnel_relay` / `tunnel_client` node rows in `w_nodes` are cleaned up in migration so existing control-plane instances upgrade without orphaned rows breaking startup or routing.

2. **Native Rust Data Plane (`tunnel/`) Embedding Upstream `rathole`**:
   - Created standalone Cargo workspace `tunnel/` (`common`, `relay`, `flared`) embedding `rathole` (`rev = a292f7ed5402f840415fc6a53827da2f34337856`) in-process via library crate + `hot-reload` watcher.
   - **Gotcha fixed (`write_file_in_place_0600`)**: `rathole::config_watcher` listens for `notify::EventKind::Modify(_)` on the tracked TOML file inode. Atomic temp+rename replaces the inode and breaks inotify watch updates; therefore runtime updates truncate and rewrite the tracked TOML file in place under mode `0600`.
   - **Gotcha fixed (IPv4-mapped IPv6 CIDR bypass)**: `cidr::check_ip` unmaps `::ffff:x.x.x.x` (`IpAddr::to_canonical`) before evaluating `allowed_cidrs`/`blocked_cidrs`, preventing SSRF bypass against `169.254.169.254/32` or loopback via IPv6-mapped syntax.
   - **Gotcha fixed (`source_token_sha256` identity reuse & systemd notify)**: Persisting `source_token_sha256` on `RelayIdentity` / `ConnectorIdentity` prevents systemd `--token-file` restarts or dual-slot execs from calling `/api/v1/relay/register` redundantly unless the token itself rotated. Systemd `Type=notify` is paired with `notify_systemd_ready()` (`READY=1` via `NOTIFY_SOCKET`) plus `SIGTERM` handling.

3. **Edge-Aligned Operational Design**:
   - Independent `tunnel-v*` release line (`CONTRACT_VERSION = 1`), `oftok1_` bootstrap tokens encapsulating `{server_url, kind, token, pool}`, WebSocket wake (`internal/apps/openflare/websocket/tunnel_hub.go`), emergency quarantine with cache invalidation & token rotation, closest-K relay selection across `zone_tag` failure domains with Make-Before-Break 2-round hysteresis, and Pingora `tunnel-routing.v1` snapshot distribution (`colocated_edge_node_id` preference + Gateway auth).
   - Dedicated `/tunnels` UI console cleanly separated from `/nodes`, plus `tunnel_id` + `insecure_skip_verify` L7 upstream binding in `/proxy-routes`.

## Verification Notes & Skipped Pre-Existing Check

- Verified PostgreSQL/SQLite migrations, Go control plane tests, Pingora Edge Rust tests (`42 passed`), Rust `tunnel/` unit + in-process Noise rathole E2E test (`18 passed`) and `clippy`, and Frontend `check:i18n` + `tsc` + `eslint` + `vitest` (`55 files / 311 tests passed`).
- **Known pre-existing failure outside scope**: `TestOAuthAndReadOnlyTransports` in `internal/apps/openflare/automation/automation_test.go:83` fails on `require.Len(t, tools.Tools, 2)` because earlier commit `6d1ecfcc` added a 3rd tool (`openflare_node_health`) to `catalog.json`. Excluded in final Go test invocation per approved Option A (`go test $(go list ./... | grep -v '/internal/apps/openflare/automation$') && go test -run TestOperationsWorkflow ./internal/apps/openflare/automation`).
