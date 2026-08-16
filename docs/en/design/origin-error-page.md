# Origin Error Page Design

You will learn: when an origin or the gateway returns a specified error status code, how OpenFlare replaces the pass-through response with a globally configurable page; how the config enters the immutable config version; and how the edge OpenResty keeps the real HTTP status code while displaying it in the page.

This design is the productized complement of the reverse proxy traffic path in [System Architecture](./architecture.md); the config release model is in [Agent & Publish Model](./agent-design.md).

---

## 1. Goals and Non-Goals

### 1.1 Goals

* **Interceptable**: for a user-configured status code set, replace the previously pass-through origin/Nginx default error response with a unified HTML.
* **Disableable**: when the global switch is off, behavior matches today (pass-through / Nginx default page).
* **Visible by default**: enabled by default, default status code tag `500-599`, default minimal OpenFlare error page.
* **Customizable**: admins can edit the full HTML online; empty HTML means the built-in default template.
* **Status passthrough**: the HTTP response `status` keeps the original error code (e.g. 502, 522); the page body shows the same value via `{{status}}`.
* **Globally unified**: a single config under sidebar「Website Management → Response Pages」shared by all reverse proxy routes.
* **Consistent with release**: the config persists via Option, enters the config version snapshot, and is distributed with release/rollback.

### 1.2 Non-Goals

* Per-route / per-Zone error page overrides
* Hosting error pages via file upload (online HTML only)
* Modifying WAF / PoW / rate-limit's own response pages (unless the user adds those status codes to the list)
* Pages static route error pages
* Multi-language error pages, brand asset CDN

---

## 2. Product Behavior

### 2.1 When to Replace

| Condition | Behavior |
| --- | --- |
| Switch on and the response status falls in the expanded set | return custom/default HTML, **status unchanged** |
| Switch on with GET-only enabled, non-GET request returns a matching status | pass through the origin's raw response, no replacement |
| Switch off | no `error_page` directives generated, pass through |
| Status not in the set | no replacement |
| Pages upstream routes | this feature is not applied |
| Origin returns 2xx/3xx/4xx successfully (not configured) | no replacement |

In all-methods mode, `proxy_intercept_errors on` is enabled on the reverse proxy `location`, so **origin-returned** matching 5xx etc. are also intercepted, not just gateway-local 502s; GET-only mode switches to Lua header/body filters that only replace GET response bodies.

### 2.2 Status Code Tag Syntax

Each Tags Input entry:

| Form | Example | Meaning |
| --- | --- | --- |
| Single code | `522` | only that code |
| Closed range | `500-599` | expand including endpoints |

* Valid range: single codes and range endpoints must be in **400–599**; `lo ≤ hi`.  
* Default tag list: `["500-599"]`.  
* Persist the **raw tags** (JSON array string); expand, dedupe, and sort at render time.  
* If the expanded result is empty while enabled → save rejected.  
* Invalid tags → save rejected with a readable error.

### 2.3 Page Placeholders

| Placeholder | Meaning |
| --- | --- |
| `{{status}}` | the current response status code (consistent with the HTTP status) |
| `{{host}}` | request Host |

Both custom HTML and the default template support these placeholders; replaced at the edge at runtime. Unused placeholders may be omitted from the template.

### 2.4 Default Page

Built-in minimal white-background OpenFlare default page: large pass-through status code, short English description, Host, and a brand footer. Supports `{{status}}` / `{{host}}`; the frontend can load prebuilt styles from the built-in template catalog on the edit page.

---

## 3. Config Model

### 3.1 Option Keys (`w_system_configs` / OpenFlare Option API)

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `origin_error_page_enabled` | bool string | `true` | master switch |
| `origin_error_page_status_codes` | JSON string array | `["500-599"]` | raw tags |
| `origin_error_page_html` | text | `""` | empty = built-in default; max **256 KiB** |
| `origin_error_page_get_only` | bool string | `false` | replace error pages only for GET; other methods pass through |

Reuses APIs:

* `GET /api/v1/d/option`  
* `POST /api/v1/d/option/update-batch`  

No new resource routes. goose migration writes the seed; constants defined in the `internal/model` config key area.

### 3.2 Validation (update-batch)

1. `enabled`: parseable as bool.  
2. `status_codes`: valid JSON array; each entry `^\d{3}$` or `^\d{3}-\d{3}$`; expanded values all in 400–599; non-empty when enabled.  
3. `html`: length ≤ 256 KiB (bytes); empty allowed.  
4. Parse/expand logic is a **pure function** shared by the API and `pkg/render/openresty` to avoid semantic forks.

No XSS sanitization on HTML: it's an admin global ops config consistent with public edge display; docs warn not to embed untrusted third-party scripts.

### 3.3 Config Version Snapshot

`ConfigSnapshot` adds fields:

```text
OriginErrorPageEnabled     bool
OriginErrorPageStatusCodes []string  // raw tags
OriginErrorPageHTML        string    // empty => renderer uses built-in default
OriginErrorPageGetOnly     bool
```

Read from Option when building the snapshot; the Agent only consumes the snapshot, never reading the control-plane DB directly.

---

## 4. Edge Rendering

### 4.1 Content Generated When Enabled

1. **SupportFile**: error page template (e.g. `error_pages/origin_error.html.tmpl`), content is the custom HTML or built-in default, keeping `{{status}}` / `{{host}}`.  
2. **Each reverse proxy server** (HTTP/HTTPS proxy; excluding Pages):

```nginx
proxy_intercept_errors on;
error_page <expanded codes...> @__openflare_origin_error;

location @__openflare_origin_error {
    default_type text/html;
    charset utf-8;
    content_by_lua_block {
        # read template, replace {{status}} / {{host}}, output body
        # ngx.status keeps the original error code
    }
}
```

### 4.2 Runtime Replacement

Use a **named location with `content_by_lua_block`** to read the template and replace placeholders — the status is **not** baked into a static file (status differs per request). GET-only mode uses `header_filter_by_lua_block` + `body_filter_by_lua_block` inside the reverse proxy location to replace only GET response bodies; non-GET requests pass through.

Never rewrite the error page to HTTP 200.

### 4.3 When Disabled

Do not output `proxy_intercept_errors`, `error_page`, the internal location, or the corresponding SupportFile (or the file may be written but unreferenced). GET-only mode also omits the Lua filters.

### 4.4 Interaction with Cache / Stale

If global `proxy_cache_use_stale` returns stale cache for some error codes, **successful stale responses never enter `error_page`**. The error page is only shown when the client actually receives an error status in the configured list. Behavior depends on existing cache directives; this feature does not change stale policy.

---

## 5. Frontend

### 5.1 Entry

* Sidebar「Website Management → Response Pages」: Error Page tab (`/responses`), edit page `/responses/error-page/edit`, preview page `/responses/error-page/preview`.  

### 5.2 Page Structure

* Header note: takes effect after releasing via「Version Release」.  
* **Switch + Tags Input** (shadcn-extension Tags Input: `@/components/ui/tags-input`): status code tags.  
* **HTML editor area** +「Load default template」「Restore default (clear)」+ placeholder docs.  
* **Client-side preview**: replace with sample `status=502`, `host=example.com` and preview in sandbox/iframe.  
* Save: `OptionService.updateBatch`; permissions same as the performance tuning page (admin).  

### 5.3 Component Dependencies

Tags Input and the HTML editor reuse existing shadcn/ui components, consistent with the existing UI style.

---

## 6. Data Flow

```text
Admin /responses (Error Page tab)
    → Option update-batch (validate tags & HTML)
    → w_system_configs

Release config version
    → snapshot writes OriginErrorPage*
    → render OpenResty conf + SupportFile
    → Agent pulls and reloads

Visitor requests a proxied domain
    → origin/gateway produces a matching status code
    → error_page → named location
    → replace placeholders, keep original status, return HTML
```

---

## 7. Decision Record

| Decision | Choice | Reason |
| --- | --- | --- |
| Config scope | global | product requirement; simple implementation and ops |
| Storage | Option + config version | consistent with performance tuning, rollbackable |
| Status input | tags: single code and range | default whole 5xx, but can name 522 |
| Response status | keep original | correct for monitoring/SEO/client semantics |
| Runtime replacement | internal + lightweight template replacement | status differs per request |
| Customization | online HTML | flexible without a file-upload chain |
