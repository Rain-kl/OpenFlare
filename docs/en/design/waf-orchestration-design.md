# WAF Orchestration Rule Design

OpenFlare WAF uses a visual directed acyclic graph (DAG) to compose request filtering policies. The control plane validates and compiles rule graphs into runtime topologies, while `openflare-edge` executes request matching, `zentinel-modsec` OWASP CRS attack inspection, and PoW challenges in memory—independently refreshing IP groups over `config_sync v1`.

---

## 1. Security Boundaries and Evaluation Order

Before proxying upstream or serving static Pages assets, `openflare-edge` evaluates security controls in deterministic order:

1. **Global request whitelist (`waf-request-whitelist-v1`)**: Delivered alongside the site snapshot and precompiled into exact, prefix, suffix, and regex path matchers (keyed by exact Host or single-level `*.suffix`). Matching requests bypass all WAF flows and request-body inspection.
2. **Global enabled WAF rule groups**: Executed first on every site without per-route reordering.
3. **Route-bound WAF rule groups**: Executed in the route's configured binding sequence.
   * Reaching a `block` node terminates the request immediately with its configured HTTP status and HTML body and records a `WafAudit` entry.
   * Reaching an `allow` node completes the current rule and advances to the next bound rule; traffic proceeds upstream only after every active rule reaches `allow`.
   * On any invalid graph state (missing target, unknown port, or step overflow), the executor fails closed and logs an error rather than allowing uninspected traffic through.
4. **Standalone PoW policy**: Evaluated via [PoW Protection Design](./pow-design.md). Legacy inline PoW nodes inside WAF graphs automatically skip when a standalone route PoW policy is configured.

### GeoIP Resilience and Shared IP Matching

* **Weak GeoIP dependency**: `openflare-edge` loads `GeoLite2-City.mmdb` and `GeoLite2-Country.mmdb` from `$STATE_DIR` with 24-hour background refresh (15-minute retry on failure, `128 MiB` file cap), preferring City for country + ISO 3166-2 subdivisions and falling back to Country. If MMDB files are unavailable or an IP has no geo record, `geo_match` evaluates to non-matching without disrupting IP matching or proxying.
* **Shared IP / CIDR matcher**: Both WAF and PoW reuse `edge/src/policy/mod.rs::ip_in_cidr` supporting IPv4, IPv6, and normalized CIDR formats.

---

## 2. DAG Node Types and Runtime Semantics

Creating a WAF rule requires only a name, initializing a valid `start → allow` default graph edited in React Flow. Loops, arbitrary script nodes, subgraphs, and cross-rule jumps are prohibited.

| Node Type | Cardinality | Inputs | Outputs | Configuration & Behavior |
| :--- | :--- | :--- | :--- | :--- |
| **`start`** | Exactly 1 | None | `next` | Graph entry point |
| **`allow`** | Exactly 1 | `>= 1` | None | Pass exit for the current rule graph |
| **`block`** | Multiple | `>= 1` | None | HTTP status code (default `403`) and optional HTML response body |
| **`ip_match`** | Multiple | `>= 1` | `true`, `false` | Inline IPs, CIDRs, and referenced IP group IDs |
| **`geo_match`** | Multiple | `>= 1` | `true`, `false` | Country ISO codes and ISO 3166-2 subdivision codes |
| **`ua_check`** | Multiple | `>= 1` | `true`, `false` | Evaluation order: require non-empty UA → block bots/abnormal UA/custom substrings → browser & OS allowlists (`and`/`or`) |
| **`attack_protection`** | Multiple | `>= 1` | `true`, `false` | Binds an attack protection group (17 OWASP CRS categories, `off`/`observe`/`essential`/`balanced`/`strict` levels + per-rule overrides); `zentinel-modsec` inspects URI, headers, and bodies up to `64 KiB`. Observe hits take `true` with audit logging; blocking hits take `false` |
| **Legacy `pow`** | Retained | `>= 1` | `next` | Algorithm, difficulty, session TTL, challenge TTL; bypassed if standalone route PoW exists |

> [!NOTE]
> `attack_protection` loads request-phase CRS rules mapped to OpenFlare's 17 attack categories (excluding response-phase rules and anomaly scoring) alongside supplemental XXE and ASP injection rules. Within a snapshot, nodes sharing identical preset/override combinations share compiled `EnginePool` instances.

---

## 3. Control-Plane Validation, Compilation, and IP Group Sync

### 3.1 Authoring vs. Runtime Models

* **Authoritative graph validation**: The React Flow editor saves graph JSON with optimistic-lock `revision` checks (returning HTTP 409 on conflict). `CompileRuleGraph` strips canvas coordinates and compiles exits into ID lookup maps while enforcing limits of `128` nodes, `256` edges, and `256 KiB` editor JSON per rule:
  * Strictly acyclic (no self-loops or cycles);
  * Exactly one `start`, one `allow`, and every node reachable from `start` with a valid path to `allow` or `block`;
  * Conditional nodes must wire both `true` and `false` ports cleanly without multi-target fanout per port.

### 3.2 Independent IP Group Refresh (`waf.ip-group.v1/<id>`)

* Supports `manual` lists, `automatic` behavioral blocking (`expr-lang` window rules), and `subscription` feeds (text or JSON extraction).
* While graph topologies deploy inside `sites.v1`, IP group memberships update independently via `waf.ip-group.v1/<id>` under `config_sync v1`, bounded by a **`20 MiB`** runtime snapshot cap (`protocol.MaxWAFIPGroupSnapshotBytes`).
* `openflare-edge` verifies SHA-256 digests and swaps IP groups atomically in memory; if snapshot validation or rule compilation fails, the node keeps serving the previous healthy configuration.

---

## 4. API, Editor, and Audit Integration

* Management APIs under `internal/apps/openflare/waf/` isolate rule metadata, `revision`-guarded graph saves, ordered route bindings, global request whitelists, attack protection groups, and IP group syncs.
* Audit details (`action`, `reason`, `flow_id`, `flow_node_id`, `attack_group_id`, matched CRS `rules`, and truncated request previews) report back with access logs into `of_node_access_logs` (`waf_action` and `waf_audit`).
