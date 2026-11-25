# Edge Cache Strategy Design

You will learn: how OpenFlare edge gateway caching aligns with the Cloudflare default lifecycle between what to cache and what not to cache: request eligibility (extensions/policy) x shareable response cacheability (origin `Cache-Control` / `Expires` / `Set-Cookie`), as well as differences from previous overly strict request bypass rules.

This design is the productized chapter for basic caching in [System Architecture](./architecture.md); cache status in access logs is documented in [Observability Data Model §3.5.1](./observability-data-model.md).

---

## 1. Goals and Non-Goals

### 1.1 Goals

* **Near-CF default out of the box**: Once caching is enabled on a route, **only standard static extensions are cached by default**, excluding HTML; **requests are not bypassed wholesale simply due to session Cookies, Authorization, or client Cache-Control headers**.
* **Eligible requests hit cache**: Authenticated users with session cookies accessing static assets like `/_app/**/*.js` experience `MISS` -> `HIT`.
* **Ineligible requests blocked**: Ineligible policies (equivalent to CF `DYNAMIC`); origin `private` / `no-store`; responses containing **`Set-Cookie` are not stored** (aligning with CF OCC defaults); `all` remains an advanced option with documentation warnings.
* **Default Edge TTL without origin freshness**: Aligns with CF status-code-based default TTLs (see §3.5).
* **Consistent observability**: Continues relying on edge-reported `cache_status` log attributes.
* **Backward compatibility**: Legacy route `cache_policy=url` maps to `all`; policy enums and migration rules follow [§5](#5-compatibility-and-migration).

### 1.2 Non-Goals (Future Iterations)

* Cache Rules expression engine  
* Forced Edge TTL ignoring origin `Cache-Control` (CF "Ignore cache-control")  
* Purge operations (by URL, prefix, or whole site)  
* Browser TTL rewriting or client-facing `CF-Cache-Status` response headers  
* Full RFC conditionality: `Authorization` cached only when responses contain `public`/`s-maxage`/`must-revalidate`  
* HEAD transformed to GET before caching  
* Dedicated cache hit ratio dashboard  

---

## 2. Cloudflare Decision Cycle (Reference Baseline)

CF default uses a **two-phase evaluation**, not "bypass whenever a Cookie exists".

### 2.1 Phase A — Request-Time Eligibility

| Condition | CF Result |
| --- | --- |
| Non-GET | Default uncacheable |
| Extension not in default cacheable table and no Rules forcing eligibility | **`DYNAMIC`** (cache not queried) |
| Extension in default table, or Rules Eligible | Proceed to Phase B |
| **Request Cookie** | **No effect by default** |
| Cache Rules Bypass | `DYNAMIC` |

CF default cacheable extensions match by **file extension**, not MIME type; **HTML and JSON are not cached by default**.

### 2.2 Phase B — Response Storage Eligibility (OCC On)

| Condition | Result |
| --- | --- |
| `Cache-Control: no-store` / `private` | Not stored |
| `public` + `max-age>0`, or valid `Expires` | Cacheable |
| Missing Cache-Control / Expires | Cacheable via status code **Default Edge TTL** (e.g. 200 -> 120m) |
| Response **`Set-Cookie`** (default level + OCC) | **Not stored**, status tends toward **BYPASS** |
| Request `Authorization` | Cacheable only if response specifies `public`, `s-maxage`, or `must-revalidate` |

### 2.3 Status Semantics (Observability Mapping)

| CF | Meaning | OpenFlare `cache_status` |
| --- | --- | --- |
| HIT / STALE / UPDATING / REVALIDATED | Hit variants | Identical or equivalent |
| MISS / EXPIRED | Fetched from origin | Identical |
| BYPASS | Request eligible, response uncacheable | `BYPASS` -> UI "Uncached" |
| DYNAMIC | Request not eligible | Policy skip -> UI "Uncached" |

---

## 3. Product Semantics

### 3.1 Dual Switches

* **Performance policy master switch**: Node performance policy configures `cache_enabled`; when disabled, the node does not initialize cache storage backends.  
* **Route-level cache switch**: Proxy route configures `cache_enabled`, controlling whether the site executes through the cache pipeline.

Caching logic activates only when both switches are enabled.

### 3.2 Policy Enumeration

| `cache_policy` | Meaning | New Route Default | Legacy Compatibility |
| --- | --- | --- | --- |
| **`static`** | Eligible only when URI matches **standard static extensions** | **Yes** | — |
| **`all`** | After method filter, does not restrict paths or extensions (Advanced) | No | Legacy `url` -> `all` |
| **`suffix`** | Custom extension list (`cache_rules`) | No | Preserved |
| **`path_prefix`** | Custom path prefix | No | Preserved |
| **`path_exact`** | Custom exact path | No | Preserved |

APIs and UI expose the table above; legacy `url` values normalize to `all` during read/write operations.

### 3.3 Standard Static Extensions (Built-in)

Aligns with CF defaults excluding HTML and JSON, while including modern frontend assets:

```text
css js mjs map
ico cur gif jpg jpeg png webp avif svg svgz
ttf otf woff woff2 eot
mp3 mp4 webm ogg flac
wasm pdf
zip 7z gz tar
```

* **Excludes** `html` / `htm` / **`json`** (aligns with CF default uncacheable JSON).  
* **Includes** `map` / `mjs` / `wasm` (improves hit rates for sourcemaps, ES modules, and WASM binaries).  
* Extension matching is case-insensitive.

### 3.4 Request-Side Bypass

Retains only:

1. `$request_method != GET` (including HEAD)

**Removed overly strict conditions:**

* Session Cookie filtering  
* Unconditional request `Authorization` bypass  
* Unconditional client `Cache-Control: no-cache|no-store|private` bypass  

**Safety Safeguards:**

| Threat | Gate |
| --- | --- |
| Inadvertent HTML/API caching | Default `static` extension list (excludes html/json) |
| Personalized content leakage | Origin `private` / `no-store` (strictly honored by gateway) |
| Response session writes | **`Set-Cookie` -> Not stored** (§3.6) |
| `all` policy over-breadth | Documentation warning: origin must declare Cache-Control |
| Bearer-authenticated APIs | Governed by policy (do not assign `all` to APIs) + origin headers |

### 3.5 Default Edge TTL (Without Origin Freshness)

Aligns with CF default TTLs by status code when `Cache-Control` and `Expires` are missing:

| Status Code | TTL |
| --- | --- |
| 200, 206, 301 | 120m (7200s) |
| 302, 303 | 20m (1200s) |
| 404, 410 | 3m (180s) |

* When origin provides valid `Cache-Control` or `Expires`, origin freshness takes precedence.  
* Mandatory Edge TTL overrides ignoring origin headers are not enforced by default.

### 3.6 Response Side: Set-Cookie Prevention

Aligns with CF OCC defaults: If an eligible request returns **`Set-Cookie`**, the response is **not stored** in cache storage.

### 3.7 Relationship with Origin Headers

* **Eligibility**: Policy + Request method.  
* **Storage and duration**: Origin `Cache-Control` / `Expires` + Default Edge TTL + Set-Cookie gate + Performance policy storage limits.  

---

## 4. Runtime and Data Flow

```text
Node performance policy cache_enabled?
    │ no -> Cache storage not initialized
    ▼ yes
Route configuration cache_enabled?
    │ no -> Cache pipeline skipped for this site
    ▼ yes
Request phase (GET / HEAD)
    -> Policy matcher (static / all / suffix / path_prefix / path_exact)
    -> Ineligible -> SKIP (BYPASS)
    -> Eligible -> Generate Cache Key, query TinyUFO in-memory/disk hot tier
    -> Cache Hit & Valid -> Return cached response directly (HIT)
Upstream response phase
    -> Evaluate origin Cache-Control / Expires
    -> Contains Set-Cookie or private / no-store -> Serve to client, do not write to cache
    -> Missing origin cache headers -> Apply status code Default Edge TTL
    -> Write to cache storage and return response to client
    -> Record access log cache_status
```

### 4.1 Code Organization

| Area | Path |
| --- | --- |
| Edge cache runtime | `edge/src/proxy/cache.rs` (cache key, TTL, evaluation, TinyUFO storage) |
| Performance policy & storage | `internal/openflare/performance/`, `edge/src/runtime/performance.rs` |
| Route snapshot compilation | `internal/openflare/edgeconfig/`, `internal/apps/openflare/config_version/snapshot.go` |
| Validation | `internal/apps/openflare/proxy_route/helpers.go` |
| Models & Defaults | New routes default `cache_policy=static`; read/write normalizes `url` -> `all` |
| UI | `frontend/app/(main)/proxy-routes/detail/components/cache-section.tsx` |

---

## 5. Compatibility and Migration

| Data | Handling |
| --- | --- |
| DB `cache_policy=''` or `url` (with cache enabled) | Read / Snapshot / Render -> **`all`** |
| API writes enabled with empty policy | Normalized to **`all`**; UI explicitly submits `static` on creation |
| New routes | Defaults to **`static`** when caching is toggled on |
| Bypass behavior change | Requests carrying Cookie/Auth transition from uncacheable to eligible for static assets; takes effect upon snapshot sync |
| Default extensions | **`json` removed**; sites relying on cached JSON can switch to `suffix` or `all` |

---

## 6. UI Guidelines (Cache Tab)

* Default when enabled: **Standard static assets** (extension matching, **excluding HTML/JSON**; includes map/mjs/wasm).  
* Options: Standard Static / All Cacheable GET (Advanced) / Custom Suffix / Path Prefix / Exact Path.  
* Descriptions aligned with CF:  
  * Login cookies do **not** bypass caching on their own.  
  * Origin `private`, `no-store`, and response `Set-Cookie` headers prevent edge caching.  
  * Missing origin cache headers use Default Edge TTL.  
* **Advanced `all`**: Warnings indicate "Similar to Cache Everything; personalized content must declare private/no-store".  
* The node's bound performance policy must enable the master cache switch.

---

## 7. Decision Matrix

| Scenario | CF | OpenFlare |
| --- | --- | --- |
| GET static + session Cookie + origin public max-age | HIT | HIT |
| GET HTML + static policy | DYNAMIC | Policy skip -> Uncached |
| GET + all + origin private | Not stored | Not stored |
| GET static + response Set-Cookie | BYPASS (OCC) | Not stored |
| GET + Authorization + static public | Conditional cache | Cacheable (simplified) |
| GET + 200 static without CC | Default 120m | Default TTL 120m |
| DevTools Disable Cache (request no-cache) | Default HIT allowed | Default HIT allowed |
| POST | Uncacheable | Non-GET skip |

---

## 8. Decision Records

| Decision | Selection | Rationale |
| --- | --- | --- |
| Request Cookie bypass | **Removed** | Aligns with CF; restores static asset hit rates for authenticated users |
| Request Authorization / Cache-Control bypass | **Removed** | Aligns with CF eligibility model; guarded by response-side gates |
| Set-Cookie | **Prevent storage** | Aligns with CF OCC default preventing session leakages |
| Default Edge TTL | **Status code default TTL** | Aligns with CF default TTLs to prevent uncacheable responses when headers are absent |
| Remove json from default table | **Yes** | Aligns with CF default preventing unintended API caching |
| Retain map/mjs/wasm | **Yes** | Improves modern frontend build artifact hit rates |
| Default cacheable scope | Default `static` | Lowers risk of inadvertent HTML/API caching |
| Legacy `url` | Map to `all` | Preserves existing behavior |
