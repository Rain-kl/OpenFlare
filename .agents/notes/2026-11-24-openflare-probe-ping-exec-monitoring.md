# 2026-11-24: `openflare-probe` Library, Komari Monitoring Schema, Latency Probing & Remote Exec

## Context & Decisions

1. **Reusable Rust Probe Library (`probe/` — crate `openflare-probe`)**:
   - Extracted and modernized `ref/komari-monitor-rs` into a pure-Rust, musl/Alpine-safe library crate (`openflare-probe`) with zero C dependencies and zero global mutable statics (`ProbeCollector` owns its own state).
   - Fixed upstream `netlink.rs` off-by-one (`+2` instead of `+1` skipping `NLMSG_DONE` header) and unaligned pointer cast bugs; added `/proc/net` fallback for restricted containers; added container overlayfs disk fallback (`fallback_disk_path`); used `SOCK_DGRAM` ICMP first (works rootless in Docker/Alpine) with `SOCK_RAW` fallback; fixed bare IPv6 host:port handling in TCP ping; used `/bin/bash` with `/bin/sh` fallback and 256 KiB bounded output + `kill_on_drop(true)` in remote exec.
   - `edge/pingora` depends on `openflare-probe` via path dependency (`../../probe`) and executes inbound WebSocket `ping_task` and `remote_exec` dispatches under bounded concurrency semaphores (16 pings, 4 execs).

2. **Database & Logstore Schema Alignment (Migration `202611240001`)**:
   - Adopted Komari's flat time-series `Record` / `PingRecord` schema inside OpenFlare's `logstore` (`of_node_metric_snapshots` recreated cleanly without historical metric migration + new `of_node_ping_records`) across PostgreSQL, SQLite, and ClickHouse (`of_node_metric_capacity_hourly` and MV rebuilt to match).
   - Kept business tables (`of_ping_tasks`, `of_exec_tasks`, `of_exec_task_results`) and enriched `of_node_system_profiles` in the primary DB without physical foreign keys.
   - Wired `ping_records` through `logwriter` batchwriter + `logstore.CleanupExpired` + `of_log_db_switch` so database switching and retention cleanup remain seamless.

3. **Multi-Replica Scheduler & Dispatch Safety**:
   - `ping.StartScheduler` runs interval-grouped tickers and dispatches only to locally connected WebSocket agents (`websocket.IsAgentConnected(nodeID)`), preventing duplicate pings across multi-replica API instances while propagating schedule reloads via Redis pub/sub (`edge:ping_schedule_reload`).
   - `exec.RunCommand` persists `of_exec_tasks` and initial `of_exec_task_results` rows *before* dispatching WebSocket messages so fast command completions cannot race database creation.

4. **MCP & CLI Single-Source Automation Sync**:
   - Added 9 automation operations (`ping list/create/update/delete/reorder`, `node ping`, `exec run/list/get`) and scopes (`ping:read`, `ping:write`, `exec:read`, `exec:write`), plus enriched `node metrics` (`profile`, `realtime`, 720h lookback), regenerated via `make swagger`.
   - Note: `TestOAuthAndReadOnlyTransports` in `internal/apps/openflare/automation/automation_test.go:83` had a pre-existing failure on `HEAD` (`nodes:read` already had 3 tools including `openflare_node_health` while the test asserted 2); left untouched per scope rules and added `internal/apps/openflare/automation/probe_test.go` to verify all 9 new operations.

5. **UI/UX Overhaul**:
   - Added reusable flat-dashed theme-native `<ResourceGauge />` (`frontend/components/data/resource-gauge.tsx`) and shared ECharts CSS-variable resolver (`frontend/components/data/chart-theme.ts`).
   - Converted sidebar「节点管理」into a collapsible menu group (`节点列表` `/nodes` with exact match + `/nodes/detail`, `延迟检测` `/nodes/ping`, `远程执行` `/nodes/exec`).
   - Enhanced Node Detail Overview with 2×2 hardware/system/storage/network cards, 4 resource gauges + 6-panel load trends (`realtime`..`30d`), and dual-channel latency/packet-loss chart with EWMA peak smoothing.
