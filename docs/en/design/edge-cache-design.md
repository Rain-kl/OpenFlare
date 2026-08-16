# Edge Cache Strategy Design

You will learn: how OpenFlare's edge `proxy_cache` aligns with the Cloudflare default loop between "should cache" and "should not cache": request eligibility (extension/policy) × response shareability (origin `Cache-Control` / `Expires` / `Set-Cookie`), and the differences from the previous over-strict request bypass.

This design is the productized chapter on "basic caching" in [System Architecture](./architecture.md); cache results in access logs are in [Observability Data Model §3.5.1](./observability-data-model.md).

---

## 1. Goals and Non-Goals

### 1.1 Goals

* **Close to CF default out of the box**: after enabling cache on a route, **only static extensions are cached by default** — HTML is not cached by default; **request session cookies / Authorization / client Cache-Control no longer cause a blanket BYPASS**.
* **Cacheable content hits**: a logged-in user visiting `/_app/**/*.js` and other static assets can show `MISS` → `HIT`.
* **Non-cacheable stays blocked**: policy not eligible (equivalent to CF `DYNAMIC`); origin `private` / `no-store`; responses with **`Set-Cookie` not stored** (aligned with CF OCC default); `all` is an advanced option with documented warnings.
* **Default Edge TTL when no origin freshness**: aligned with CF's per-status default TTL (see §3.5).
* **Consistent observability**: keep relying on `$upstream_cache_status` → three-state `cache_status` detail.
* **Backward compatible**: legacy route `cache_policy=url` maps to `all`; policy enum and migration rules stay in [§5](#5-兼容与迁移).

### 1.2 Non-Goals (later iterations)

* Cache Rules expression engine
* Forced Edge TTL ignoring origin `Cache-Control` (CF Cache Rules "Ignore cache-control")
* Purge (by URL/prefix/site-wide)
* Browser TTL rewriting, client `CF-Cache-Status` response header
* Full RFC conditions: `Authorization` cached only when the response has `public`/`s-maxage`/`must-revalidate` (needs Lua; this iteration deletes the request-side bypass entirely, relying on policy + origin headers)
* HEAD → GET conversion then cache
* Hit-rate dashboard

---

## 2. Cloudflare Decision Loop (Alignment Baseline)

CF default is a **two-stage decision**, **not** "request has Cookie → don't cache".

### 2.1 Stage A — Eligible at Request Time

| Condition | CF Result |
| --- | --- |
| Non-GET | not cached by default |
| Extension not in default cacheable table, no Rules forcing eligible | **`DYNAMIC`** (no cache lookup) |
| Extension in default table, or Rules eligible | continue to Stage B |
| **Request Cookie** | **no effect by default** |
| Cache Rules Bypass | `DYNAMIC` |

CF's default cacheable extensions are keyed by **extension** rather than MIME; **HTML / JSON are not cached by default**.

### 2.2 Stage B — Response Storeable (OCC on, Free/Pro/Biz default)

| Condition | Result |
| --- | --- |
| `Cache-Control: no-store` / `private` | not stored |
| `public` + `max-age>0`, or future `Expires` | cacheable |
| No Cache-Control / Expires | still cacheable with per-status **default Edge TTL** (e.g. 200 → 120m) |
| Response **`Set-Cookie`** (default cache level + OCC) | **not stored**, status tends toward **BYPASS** |
| Request `Authorization` | cacheable only when the response also has `public` / `s-maxage` / `must-revalidate` (full condition simplified with Nginx this iteration, see §3.4) |

### 2.3 Status Semantics (vs. Observability)

| CF | Meaning | OpenFlare `cache_status` |
| --- | --- | --- |
| HIT / STALE / UPDATING / REVALIDATED | hit class | same-name or equivalent |
| MISS / EXPIRED | fetch from origin | same-name |
| BYPASS | eligible at request time, response not cacheable | `BYPASS` → UI "not cached" |
| DYNAMIC | not eligible at request time | policy skip mostly `BYPASS` or empty → UI "not cached" |

---

## 3. Product Semantics

### 3.1 Two-Level Switch (unchanged)

* **Global** `openresty_cache_enabled`: generates `proxy_cache_path` etc.; when off, route-level cache directives are inert.
* **Route** `cache_enabled`: whether to enable `proxy_cache` in that site's `location`.

Cache logic only runs when both are on.

### 3.2 Policy Enum

| `cache_policy` | Meaning | New Default | Legacy Compatibility |
| --- | --- | --- | --- |
| **`static`** | only eligible when URI matches **standard static extensions** | **yes** | — |
| **`all`** | after method bypass, no path/extension restriction (advanced; risk similar to CF Cache Everything) | no | legacy `url` → `all` |
| **`suffix`** | custom extension list (`cache_rules`) | no | kept |
| **`path_prefix`** | custom path prefix | no | kept |
| **`path_exact`** | custom exact path | no | kept |

Render layer: historical `url` is treated as `all`; API/UI only expose the enum above.

### 3.3 Standard Static Extensions (built-in)

Aligned with CF default "no HTML/JSON caching"; keeps modern frontend-friendly enhancements:

```text
css js mjs map
ico cur gif jpg jpeg png webp avif svg svgz
ttf otf woff woff2 eot
mp3 mp4 webm ogg flac
wasm pdf
zip 7z gz tar
```

* **Excludes** `html` / `htm` / **`json`** (aligned with CF not caching JSON by default).  
* **Includes** `map` / `mjs` / `wasm` (deliberate enhancement for sourcemap / ES module / WASM hits).  
* Matching: `$uri` extension, case-insensitive:  
  `if ($uri !~* \.(?:css|js|…)$) { set $openflare_skip_cache 1; }`

### 3.4 Request-Side Bypass (after CF alignment)

Only kept:

1. `$request_method != GET` (HEAD included, consistent with current network; no CF HEAD→GET)

**Removed** (previously over-strict, causing low hit rates):

* Session-cookie regex
* `$http_authorization != ""`
* request `$http_cache_control` matching `no-cache|no-store|private`

**How security still holds:**

| Threat | Gate |
| --- | --- |
| Accidentally caching HTML/API | default `static` extensions (no html/json) |
| Personalized content | origin `private` / `no-store` (respected by Nginx) |
| Response writes session | **`Set-Cookie` → not stored** (§3.6) |
| `all` too broad | UI/doc warning: needs correct origin Cache-Control |
| API with Bearer | rely on policy (don't use `all` for APIs) + origin headers; full Auth conditional caching is later |

### 3.5 Default Edge TTL (no origin freshness)

Aligned with CF's per-status default TTL without `Cache-Control`/`Expires`, emitted in cache-enabled locations:

| Status | TTL |
| --- | --- |
| 200, 206, 301 | 120m |
| 302, 303 | 20m |
| 404, 410 | 3m |

```nginx
proxy_cache_valid 200 206 301 120m;
proxy_cache_valid 302 303 20m;
proxy_cache_valid 404 410 3m;
```

* When the origin provides valid `Cache-Control` / `Expires`, the origin freshness wins (no `proxy_ignore_headers`).  
* **No** forced Edge TTL override ignoring origin headers.

### 3.6 Response Side: Set-Cookie Not Stored

Aligned with CF OCC default: an eligible request whose origin returns **`Set-Cookie`** is **not written** into `proxy_cache` (read path may still have MISS/BYPASS semantics).

```nginx
proxy_no_cache $openflare_skip_cache $upstream_http_set_cookie;
```

(`proxy_no_cache` multi-arg: any non-empty and non-`"0"` arg means no write.)

`proxy_cache_bypass` still only binds `$openflare_skip_cache` (request-side skip); the response side only affects **writes**, consistent with CF "eligible but response not cacheable".

### 3.7 Relationship with Origin Headers

* **Eligibility**: policy + method bypass.  
* **Store / duration**: origin `Cache-Control` / `Expires` + default `proxy_cache_valid` + Set-Cookie gate + global `inactive`.  

---

## 4. Rendering and Data Flow

```text
Global cache_enabled?
    │ no → no proxy_cache_* generated
    ▼ yes
Route cache_enabled?
    │ no → location without proxy_cache
    ▼ yes
set $openflare_skip_cache 0
    → non-GET → set 1
    → policy if (static/all/suffix/…) → may set 1
proxy_cache openflare_cache
proxy_cache_methods GET
proxy_cache_bypass $openflare_skip_cache
proxy_no_cache $openflare_skip_cache $upstream_http_set_cookie
proxy_cache_valid …
    →
access.log cache_status=$upstream_cache_status
```

### 4.1 Policy → Nginx Conditions

| Policy | Extra Condition |
| --- | --- |
| `static` | `$uri` not matching built-in extension table → skip |
| `all` | no extra path condition |
| `suffix` | not matching `cache_rules` extensions → skip |
| `path_prefix` / `path_exact` | same as current implementation |

### 4.2 Code Areas Involved

| Area | Path |
| --- | --- |
| Rendering | `pkg/render/openresty/render.go` (bypass, Set-Cookie, `proxy_cache_valid`, extension constants) |
| Validation | `internal/apps/openflare/proxy_route/helpers.go` |
| Model/defaults | creating a route defaults `cache_policy=static`; `url`→`all` on read/write |
| Snapshot | `config_version` snapshot normalization |
| UI | `proxy-routes/detail/components/cache-section.tsx` |

---

## 5. Compatibility and Migration

| Data | Handling |
| --- | --- |
| `cache_policy=''` or `url` in DB (and cache enabled) | read / snapshot / render → **`all`** |
| API write with enabled and empty policy | normalized to **`all`**; UI new-create with cache on **explicitly submits** `static` |
| New routes | default **`static`** when cache enabled |
| Bypass behavior change | **breaking vs. old implementation**: cookie/auth traffic goes from "not cached" to cacheable HIT; requires **republishing node configs** |
| Default extensions | **remove `json`** from the table; sites relying on caching `*.json` can use custom `suffix` or `all` |

**Release note:** document this alignment with the CF default model; hit rate expected to rise; `all` and wrong origin headers need ops self-check.

---

## 6. UI Copy Points (Cache Tab)

* After enabling cache, default: **standard static assets** (summary extensions, **excluding HTML/JSON**; including map/mjs etc.).  
* Options: standard static / all cacheable GET (advanced) / custom suffix / path prefix / exact path.  
* CF-aligned notes:  
  * login cookies are **not** separately skipped from caching;  
  * origin `private` / `no-store` / response **`Set-Cookie`** are not written to the edge cache;  
  * default Edge TTL used when no origin cache headers.  
* **Advanced `all`**: warn "similar to Cache Everything; personalized pages must declare private/no-store from the origin".  
* Global Performance cache master switch must be on.

---

## 7. Decision Matrix (Avoid Missed Judgments)

| Scenario | CF | OpenFlare (this design) |
| --- | --- | --- |
| GET static + session Cookie + origin public max-age | HIT | HIT |
| GET HTML + static policy | DYNAMIC | policy skip → not cached |
| GET + all + origin private | not stored | not stored |
| GET static + response Set-Cookie | BYPASS (OCC) | not stored |
| GET + Authorization + static public | conditional cache | cacheable (simplified; rely on origin not marking sensitive APIs public) |
| GET + no-CC 200 static | default 120m | `proxy_cache_valid` 120m |
| DevTools Disable cache (request no-cache) | edge may still HIT by default | edge may still HIT by default |
| POST | not cached | non-GET skip |

---

## 8. Decision Record

| Decision | Choice | Reason |
| --- | --- | --- |
| Request Cookie bypass | **removed** | aligned with CF; restore static hit rate for logged-in users |
| Request Authorization / Cache-Control bypass | **removed** | aligned with CF request-eligibility model; response gate as backstop |
| Set-Cookie | **bind to proxy_no_cache** | aligned with CF OCC "response Set-Cookie not stored" |
| Default Edge TTL | **per-status proxy_cache_valid** | aligned with CF default TTL when headerless, avoiding "never stored" |
| Remove json from default table | **yes** | aligned with CF not caching JSON by default |
| Keep map/mjs/wasm | **yes** | useful hits for modern frontend, deliberate enhancement |
| Default cacheable scope | cache-on defaults to `static` | benchmarked to CF, reduces HTML/API mis-caching |
| Legacy `url` | maps to `all` | doesn't narrow existing behavior |
| Full Auth conditions / Purge / Rules | later | close the default loop first, then extend |
