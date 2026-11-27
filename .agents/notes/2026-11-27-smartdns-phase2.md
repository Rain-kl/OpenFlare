# Agent Note: SmartDNS Phase 2 Implementation & Verification

Date: 2026-11-27

## Architecture & Key Decisions
1. **Strict Anti-Corruption Boundary (`qualityapi`)**:
   - Control-plane SmartDNS (`internal/apps/openflare/smartdns`) and Rust `dns/` (`dns/src/quality.rs`) never query `netquality` or `netintel` internal tables directly. Go enforces this via `internal/apps/openflare/qualityapi` adapter (`netquality/quality_provider.go`) and `smartdns/imports_test.go`.
2. **Rust `zen-engine 2.1.8` Multi-Thread Async Isolation**:
   - `zen_engine::Decision::evaluate` holds `rquickjs` `!Send` internals (`Rc<RefCell<...>>`, `OnceCell<ValidatorCache>`), which violates `hickory_server::RequestHandler`'s `Send` future requirement if awaited directly inside `handle_request`.
   - Solved in `dns/src/engine.rs` by converting request/response payloads as pure `Send` `serde_json::Value` across `tokio::task::spawn_blocking`, running `decision.evaluate(...)` inside a thread-local single-threaded `tokio::runtime::Builder::new_current_thread()` runtime. This only runs on L1 cache misses.
3. **Go <-> Rust JDM Expression Parity**:
   - Visual ReactFlow graphs compile in `internal/apps/openflare/smartdns/jdm.go` to standard `zen-engine` `switchNode` + `expressionNode` documents (`mode`, `weights.*`, `filters.*`, `max_answers`, `ttl`, `use_tag_weights`).
   - Both Go `EvaluateJDM` and Rust `zen_engine` consume the identical compiled JSON and execute matching 4-stage scoring (filters, `>0.92` overload exclusion, 5% hysteresis, and mandatory fallback ladder).
4. **Cloudflare `smartdns` Target Mode**:
   - Forced non-proxied CNAME pointing so recursive resolvers pass client ECS/source IP directly to OpenFlare SmartDNS authoritative NS endpoints without Cloudflare proxy IP masking.

## Performance Optimization & Benchmark (`openflare-dns bench`)
Initial benchmark showed `ratio = 0.349` (4575 vs 13102 QPS) due to:
- Default `hickory_server=info` tracing logging 2 lines per DNS packet (~161k stdout lines),
- 5s L1 cache cap expiring mid-benchmark,
- Per-packet heap allocations (`format!`, `.to_string()`, `is_authoritative_query` string allocation) and single-mutex `Histogram` contention.

Resolved by:
- Defaulting tracing filter in `dns/src/main.rs` to `info,hickory_server=warn`.
- Introducing zero-allocation `ClientPrefixKey` (`/24` v4, `/48` v6 bitmask enum), structured `AnswerCacheKey`, early RRL check, `ArcSwap::load()` guards, immediate L1 cache lookup prior to string normalization, and 16-shard `try_lock` histograms in `dns/src/metrics.rs`.
- Verified release benchmark (`queries=40000, concurrency=32`, 64 distinct `/24` ECS prefixes):
  - `bare_hickory_qps = 92904.3 (430.6 ms)`
  - `smartdns_qps     = 80931.8 (494.2 ms)`
  - `ratio            = 0.871 (87.1%)` (>= 70% target)
  - `cache_hits       = 40161 / 40256`

## Frontend Notes
- Moved Cloudflare connection settings to `/dns/settings` with a static-export-safe client redirect from `/cloudflare/settings`.
- Used ReactFlow (`@xyflow/react`) and ECharts (`TrendChart`) matching existing WAF/observability patterns, preserving dashed borderless-shadow `<Card>` aesthetics (`AGENTS.md`).
- Lazy-loaded SmartDNS policy lists in Cloudflare group dialogs (`enabled: createOpen / editOpen`) with a second effect for default selection so lazy fetch completion never resets user-edited fields.
