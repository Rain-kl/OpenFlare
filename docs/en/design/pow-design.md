# PoW Protection Design

The Proof-of-Work (PoW) module combines per-site policies, challenge page templates, a browser Web Component solver, and IP/path matchers shared with the WAF engine. Control-plane logic lives in `internal/apps/openflare/pow`, persisting policies in `of_pow_policies` keyed by `route_id` (cleaned up transactionally on route deletion) and compiling `pow_enabled` / `pow_config` directly into `sites.v1` for lock-free atomic reloads in `openflare-edge`.

---

## 1. Configuration and Asset Pipeline

* **Control-plane API**: `GET /api/v1/d/pow/policies` lists configured site policies; `PUT /api/v1/d/pow/policies/{id}` validates ranges and route ownership before saving `enabled` and `config`. Global challenge page styling (`template`, `widget`, and custom `html`) is managed through the `pow_page` setting.
* **Policy constraints (`internal/openflare/edgeconfig/pow_policy.go`)**:
  * `algorithm`: `fast` or `slow`; `difficulty`: `1–16`;
  * `session_ttl`: `60–604800` seconds; `challenge_ttl`: `30–3600` seconds;
  * `path_rules`: `mode` is `all` or `blacklist` (requiring at least one blacklist entry when `blacklist` is chosen). Allow/block rule lists accept up to `100` entries using `exact`, `prefix`, `wildcard` (all prefixed with `/`), or `regex`. Omitting `path_rules` applies PoW site-wide.
* **Single source of HTML assets**: Built-in challenge templates and scripts reside in `shared/edgeassets/pow_templates/` and `pow_static/`. Frontend preview assets sync via `node frontend/scripts/generate-pow-assets.mjs` and are enforced in `make code-check`.

---

## 2. Request Flow and Security Guarantees

1. **Takeover semantics**: When a route has a standalone PoW configuration (`route.pow.is_some()`), `openflare-edge` automatically skips any legacy `pow` nodes inside attached WAF rule graphs so visitors are never challenged twice. Routes without standalone policies keep legacy graph-embedded PoW compatibility.
2. **Matcher & cookie evaluation**: The runtime evaluates allowlists (IP/CIDR/path/UA), blocklist modes, and signed session cookies using matchers precompiled in memory during snapshot swaps. `path_rules` `exact`, `prefix`, and `wildcard` rules are case-sensitive, whereas legacy `paths` wildcards remain case-insensitive.
3. **Challenge rendering & solving**: Unauthenticated requests receive the rendered challenge HTML (escaping `<` inside embedded boot JSON and updating status strings with `textContent`), driving the Web Component and `openflare:pow` / `window.OpenFlarePoW` UI events while Web Workers compute SHA-256 solutions.
4. **Verification & anti-replay**: Submissions to `POW_API_PREFIX` bind strictly to request host and route ID, invalidate the single-use challenge, issue a scoped session cookie, and redirect solely to the server-recorded local target path.

If snapshot decoding or template loading fails, `openflare-edge` rejects the candidate snapshot and retains the last healthy active configuration rather than failing open.
