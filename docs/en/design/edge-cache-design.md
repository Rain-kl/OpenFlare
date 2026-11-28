# Edge Cache Strategy Design

`openflare-edge` (`edge/src/proxy/cache.rs`) integrates TinyUFO memory and optional disk storage via `pingap-cache`, gating HTTP response caching with both node-level performance policies and route-level rules. For telemetry reporting fields (`cache_status`), see [OpenFlare Edge Reporting Protocol & Observability Data Model](./observability-data-model.md#25-access_logs-l1-business-fact--passive-rtt).

---

## 1. Enablement Gates and Scope

Caching activates only when all of the following conditions hold:

1. **Node performance policy (`cache_enabled`)**: Controls whether `EdgeCache` storage is enabled on the edge node.
2. **Route toggle (`route.cache_enabled`)**: Enables caching per proxy route.
3. **Feature exclusions**: `edge/src/proxy/mod.rs` bypasses HTTP caching on routes with site authentication (`site_auth_ttl_hours != 0`) or HTTP Basic auth enabled. Pages static deployments are served directly by `edge/src/pages/` with `ETag` and byte-range handling outside `EdgeCache`.

---

## 2. Two-Stage Caching Decisions (`edge/src/proxy/cache.rs`)

### 2.1 Request Eligibility (Stage A)

`EdgeCache::enable` skips cache lookup and storage unless every check passes:

* **Method check**: Only `GET` requests are eligible; `HEAD`, `POST`, and other methods bypass cache.
* **Sensitive/special header bypass**: Requests carrying `Authorization`, `Upgrade` (WebSocket), or `Range` headers skip caching directly.
* **Route policy (`cache_policy`) matching**:

| `cache_policy` | Behavior | Default | Legacy Compatibility |
| :--- | :--- | :--- | :--- |
| **`static`** | Matches built-in static extensions case-insensitively (excluding `html`, `htm`, `json`) | **Yes** | — |
| **`all`** | Caches any path/extension passing method & header checks (requires accurate origin `Cache-Control`) | No | Legacy `url` normalizes to `all` |
| **`suffix`** | Matches custom extensions configured in `cache_rules` | No | Requires at least one extension |
| **`path_prefix`** | Matches URI path prefixes in `cache_rules` | No | Prefixes start with `/` |
| **`path_exact`** | Matches exact URI paths in `cache_rules` | No | Paths start with `/` |

Built-in `static` extensions:

```text
css js mjs map
ico cur gif jpg jpeg png webp avif svg svgz
ttf otf woff woff2 eot
mp3 mp4 webm ogg flac
wasm pdf
zip 7z gz tar
```

> [!NOTE]
> Regular visitor cookies alone do not bypass `static` asset caching; session safety is enforced on the response side by blocking `Set-Cookie` and `private`/`no-store` responses.

### 2.2 Response Cacheability and Default Edge TTL (Stage B)

For eligible requests, `EdgeCache::response` validates origin headers before admitting objects into TinyUFO:

* **Non-cacheable responses**: Any response with `Set-Cookie`, `Vary: *`, or non-shareable `Cache-Control` (`private`, `no-store`, `no-cache`) is forwarded to the client without storing in cache.
* **Key isolation & `Vary`**: Primary keys hash `{snapshot_digest}|{max_object_bytes}|{scheme}|{host}|{request_uri}` so raw URLs and query strings never leak into logs or filenames, while `Vary` response headers build secondary variance keys.
* **Request coalescing & object limits**: Global `CacheLock` (60s base timeout tuned via `cache_lock_wait_timeout` / `cache_max_lock_retries`) coalesces concurrent fills. Maximum cached object size respects `cache_max_object_mib` bounded by available store capacity (and 25% of node memory budget in disk mode).
* **Default status TTLs**: When the origin omits explicit freshness headers (`Cache-Control` / `Expires`), `EdgeCache` applies status-based defaults alongside optional `stale_updating` / `stale_on_error` (60s stale window):

| Status Code | Default Edge TTL |
| :--- | :--- |
| `200`, `206`, `301` | `7200s` (120m) |
| `302`, `303` | `1200s` (20m) |
| `404`, `410` | `180s` (3m) |

---

## 3. Code Map

| Area | Path |
| :--- | :--- |
| Key generation, eligibility, TinyUFO backend & disk sweep | `edge/src/proxy/cache.rs`, `edge/src/proxy/mod.rs` |
| Performance policy settings & startup validation | `edge/src/runtime/performance.rs` |
| Route cache policy validation & `url → all` normalization | `internal/apps/openflare/proxy_route/helpers.go` |
| Snapshot compiler defaults | `internal/openflare/edgeconfig/`, `internal/apps/openflare/config_version/snapshot.go` |
| Console cache configuration UI | `frontend/app/(main)/proxy-routes/detail/components/cache-section.tsx` |
