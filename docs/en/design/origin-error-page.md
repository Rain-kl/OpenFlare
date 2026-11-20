# Origin Error Page Design

When an origin server or the gateway returns configured error status codes, OpenFlare substitutes the pass-through response with a globally configurable error page. The configuration is stored within immutable configuration snapshots, and the edge Pingora gateway renders the error page in memory while strictly preserving the authentic HTTP status code.

This design complements the reverse proxy traffic path in [System Architecture](./architecture.md); for the configuration synchronization model, see [Pingora Edge Node Design](./pingora-edge-design.md).

---

## 1. Goals and Non-Goals

### 1.1 Goals

* **Interceptable**: For user-configured status code sets, replace default origin or gateway error responses with unified HTML.
* **Disableable**: When disabled globally, behavior remains standard (passing through origin responses or returning built-in defaults).
* **Visible by default**: Enabled by default with the `500-599` status code range, displaying a built-in clean error page.
* **Embedded branding**: Built-in error and offline pages directly embed an inline SVG OpenFlare logo, avoiding external image requests.
* **Customizable**: Administrators can edit complete HTML online; an empty value defaults to the built-in template.
* **Status passthrough**: The HTTP response status code preserves the original error code (e.g. 502, 522); page content reflects the identical value via `{{status}}`.
* **Globally unified**: A single configuration under sidebar "Website Management → Response Pages" applies across all reverse proxy routes.
* **Synchronized with releases**: Configurations persist via system options, enter configuration snapshots, and synchronize downstream or roll back atomically.

### 1.2 Non-Goals

* Per-route or per-Zone error page overrides  
* Hosting error pages via uploaded static archives (online HTML editing only)  
* Modifying WAF, PoW, or rate-limiting dedicated challenge pages (unless the user explicitly adds their status codes to the set)  
* Pages static hosting route error pages  
* Multi-language error pages or CDN-hosted external assets  

---

## 2. Product Behavior

### 2.1 When to Replace

| Condition | Behavior |
| --- | --- |
| Switch enabled, and response status matches the expanded code set | Return custom or default HTML, **HTTP status unchanged** |
| Switch enabled with GET-only active, non-GET request returns matching status | Pass through origin response untouched |
| Switch disabled | Do not intercept error codes; pass through original response |
| Status code not in configured set | No replacement |
| Pages static hosting routes | Feature not applied |
| Origin returns 2xx/3xx/4xx (unconfigured) | No replacement |

In all-methods mode, the gateway intercepts both origin-returned matching 5xx errors and gateway-local errors; in GET-only mode, only GET response bodies are replaced, and non-GET requests pass through without alteration.

### 2.2 Status Code Tag Syntax

Status code input syntax:

| Format | Example | Meaning |
| --- | --- | --- |
| Single code | `522` | Matches that specific code only |
| Closed range | `500-599` | Expands to all codes within the range inclusively |

* Valid range: Single codes and range boundaries must fall within **400–599**; `lower <= upper`.  
* Default tag list: `["500-599"]`.  
* Storage persists **raw tags** as a JSON array string; expansion, deduplication, and sorting occur during compilation.  
* If the expanded set is empty while enabled, saving is rejected.  
* Invalid tag formats reject saving with descriptive validation errors.

### 2.3 Page Placeholders

| Placeholder | Meaning |
| --- | --- |
| `{{status}}` | Current HTTP response status code |
| `{{host}}` | Request Host header |

Both custom HTML and default templates support these placeholders, replaced in memory at the edge at runtime.

### 2.4 Default Page

A built-in minimal white-background OpenFlare default page displays the large pass-through status code, concise English description, request host, and brand footer.

---

## 3. Configuration Model

### 3.1 Option Keys (`w_system_configs` / OpenFlare Option API)

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `origin_error_page_enabled` | bool string | `true` | Master toggle |
| `origin_error_page_status_codes` | JSON string array | `["500-599"]` | Raw tag array |
| `origin_error_page_html` | text | `""` | Empty = built-in default; max **256 KiB** |
| `origin_error_page_get_only` | bool string | `false` | Replace error pages only for GET; pass through others |

API Endpoints:

* `GET /api/v1/d/option`  
* `POST /api/v1/d/option/update-batch`  

No dedicated resource routes are introduced. Goose migrations seed initial values; constants are defined in `internal/model`.

### 3.2 Validation (update-batch)

1. `enabled`: Parseable as a boolean.  
2. `status_codes`: Valid JSON array; each entry matches `^\d{3}$` or `^\d{3}-\d{3}$`; all expanded codes in 400–599; non-empty when enabled.  
3. `html`: Byte length <= 256 KiB; empty strings permitted.  
4. Parsing and expansion logic is implemented as a **pure function** in `internal/openflare/edgeconfig/origin_error_page.go`, shared between API validation and snapshot compilation.

HTML content is not subjected to XSS sanitation, as it constitutes an administrative operational template.

### 3.3 Configuration Snapshot

Snapshot structures incorporate the following fields:

```go
OriginErrorPageEnabled     bool
OriginErrorPageStatusCodes []string  // raw tags
OriginErrorPageHTML        string    // empty falls back to built-in default
OriginErrorPageGetOnly     bool
```

The snapshot compiler loads values from system options. Edge nodes consume snapshots and never query the control-plane database directly.

---

## 4. Edge Rendering and Processing

### 4.1 Edge Gateway Processing Mechanism

Pingora edge nodes intercept errors during the response handling phase:

1. When a response status code matches the configured set and the feature is active, error page evaluation triggers.
2. If `get_only` is enabled and the request method is not `GET`, replacement is bypassed and the raw response passes through.
3. The engine loads template HTML from the active snapshot (or built-in default) and dynamically substitutes `{{status}}` and `{{host}}` placeholders in memory.
4. The HTTP `Content-Type` header is set to `text/html; charset=utf-8`, and the original error status code (e.g. 500, 502, 504) is strictly preserved. Responses are never rewritten to HTTP 200.

### 4.2 Disabling and Bypassing

When disabled, the gateway bypasses error interception, returning upstream responses or network errors directly to clients.

### 4.3 Stale Cache Interactions

If cache policies specify stale content delivery for specific upstream error codes, successfully served stale responses do not trigger error page substitution. Error pages render only when an actual error status is emitted to the client.

---

## 5. Frontend

### 5.1 Route Entrypoints

* Sidebar "Website Management → Response Pages": Error Page Tab (`/responses`), editor at `/responses/error-page/edit`, preview at `/responses/error-page/preview`.

### 5.2 Page Components

* Page Header: States that changes compile and synchronize automatically upon saving.  
* **Toggle + Tags Input** (`@/components/ui/tags-input`): Status code tags.  
* **HTML Editor** + "Load Default Template" / "Restore Default" + placeholder reference.  
* **Client Preview**: Sandbox/iframe rendering with mock `status=502` and `host=example.com`.  
* Persistence: `OptionService.updateBatch`, governed by administrative permissions.  

---

## 6. Data Flow

```text
Admin /responses (Error Page Tab)
    → Option update-batch (validates tags and HTML)
    → w_system_configs

Save and Auto-Sync
    → Snapshot compiled into sites.v1
    → Edge nodes pull and atomically swap
    → Pingora applies changes in memory

Client requests reverse proxy domain
    → Origin or gateway produces matching error code
    → Pingora response interception phase
    → Replace placeholders, retain original HTTP status, return HTML
```

---

## 7. Decision Records

| Decision | Selection | Rationale |
| --- | --- | --- |
| Configuration Scope | Global | Consistent error presentation; reduces per-site complexity |
| Storage | System Options + Snapshot | Follows standard configuration model, supporting versioned rollbacks |
| Status Code Input | Tags (single codes and ranges) | Efficiently covers broad 5xx ranges while supporting specific codes like 522 |
| Response Status | Retain original code | Preserves correct semantics for clients, monitoring, and search indexing |
| Runtime Substitution | In-memory placeholder replacement | Accommodates per-request dynamic host and status binding |
| Customization Method | Direct HTML text | Avoids unnecessary file upload pipelines |
