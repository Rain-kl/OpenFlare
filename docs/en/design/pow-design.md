# Standalone PoW Module Design

PoW consists of standalone site policies, page templates, a browser runtime, and UI components. Business APIs reside in `internal/apps/openflare/pow`, with policies persisted in `of_pow_policies` and accessed via `internal/repository`. Configuration is accessible via the dedicated "PoW" tab in proxy route details, directly editing the route's policy. Each site maintains one policy keyed by `route_id`; site deletions clean up associated policies within the same transaction.

## Configuration and Publishing

- `GET /api/v1/d/pow/policies`: Lists configured site policies.
- `PUT /api/v1/d/pow/policies/{id}`: Saves site `enabled` status and `config`; administrator authorization and site existence are validated server-side.
- `pow_page`: Internal configuration storing global `template`, `widget`, and custom `html`.

Policies specify `algorithm`, `difficulty`, `session_ttl`, `challenge_ttl`, and optional `path_rules`. `path_rules.mode` can be `all` or `blacklist`, where `whitelist` / `blacklist` entries are arrays of `{ "type": "exact|prefix|wildcard|regex", "value": "..." }`. Omitting `path_rules` applies PoW across the entire site.

Snapshot publishing attaches standalone policies to the corresponding site's `pow_enabled` / `pow_config`; the control plane compiles policies into the `sites.v1` snapshot. Challenge page templates and assets are tracked under configuration checksum and rollback mechanisms. Standalone PoW endpoints and static assets render on sites without requiring dedicated WAF PoW graph nodes.

The single source of truth for built-in HTML templates is `shared/edgeassets/pow_templates/`. Running `node frontend/scripts/generate-pow-assets.mjs` synchronizes frontend preview assets; `make code-check` validates synchronization. Generated files are committed alongside frontend source code and are available for container builds.

Edge node release packages cover `amd64` and `arm64` targets, built and verified via dedicated CI pipelines. New PoW policies take effect dynamically across edge nodes via control-plane snapshot synchronization.

## Request Flow

1. `pow.runtime.prepare()` resolves the current site from the configuration snapshot, allowing legacy WAF PoW nodes to detect takeover state.
2. WAF executes; existing PoW nodes proceed if a standalone policy is already configured for the site.
3. `pow.runtime.check()` executes the standalone PoW policy, evaluating allowlist patterns first, then blocklist patterns, and finally verifying valid clearance cookies.
4. When a challenge is required, the current policy state is preserved, and the request internally transfers to the challenge handler with explicit arguments, preserving the original browser URL.
5. The challenge handler renders HTML, escapes placeholder text, injects Widget and JSON bootstrap data, and loads the browser runtime.
6. The Worker computes the SHA-256 solution; the browser submits proof, the server verifies difficulty, domain, and site binding, consumes the challenge token, and issues a clearance cookie. Upon receiving a success response, the frontend displays the verified state and refreshes the original URL.

Standalone policies, templates, and translations load during runtime initialization, with IP and wildcard matchers pre-compiled; live requests and challenge pages do not perform repetitive configuration I/O. Changes to policies or templates take effect via control plane snapshot sync; the OpenFlare Edge gateway atomically swaps snapshots in memory without process restarts or connection disruption. If a new version omits a standalone policy, site bindings are not loaded; unreadable or corrupt policy payloads reject initialization to prevent invalid configurations from silently disabling protection. Template loading errors fall back to built-in fallback pages while logging the error.

PoW IP matching shares the WAF implementation, supporting IPv4, IPv6, CIDR blocks, and canonical IPv6 representations. Legacy `paths` wildcard matching retains case-insensitive behavior; `path_rules` exact, prefix, and wildcard matching remain case-sensitive, while regular expressions are compiled and cached in memory by the OpenFlare Edge engine.

The verification runtime exposes display state via Web Components and `openflare:pow` / `window.OpenFlarePoW`, avoiding dependencies on specific DOM element IDs. Solvers transition to `verifying` upon completion and enter `success` only after receiving server-side confirmation; errors display a retry action.

## Compatibility and Boundaries

PoW execution paths inside legacy WAF DAGs are preserved to avoid automatically converting legacy conditional branches into site-wide policies. Legacy PoW nodes are skipped only when a site explicitly configures a standalone policy. Legacy graph configurations and standalone site policies are stored separately, restoring their respective versions upon snapshot rollback.

The challenge JSON escapes `<` to prevent premature script element termination; text states use `textContent`. Challenge validation binds the original domain and site, invalidating the challenge token immediately upon successful verification. Server redirects use only local paths stored in the challenge token, rejecting client-supplied redirect targets.
