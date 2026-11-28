# Origin Error Page Design

When an upstream origin or edge gateway returns a configured error status code, `openflare-edge` replaces the response body with a globally configured custom or built-in HTML page while preserving the exact HTTP status code and substituting `{{status}}` and `{{host}}` in memory.

---

## 1. Behavior and Scope

* **Global configuration**: Managed under **Websites → Response Pages** (`/responses`, `/responses/error-page/edit`, `/responses/error-page/preview`) and applied across reverse-proxy routes. Excluded from Pages static deployments, stale-cache (`stale`) hits, and WAF/PoW/rate-limit native responses (unless those status codes are explicitly included).
* **Exact HTTP status preservation**: Responses return `Content-Type: text/html; charset=utf-8` with the original upstream or gateway status code intact (such as `500`, `502`, or `504`), never rewritten to `200`.
* **Optional GET-only mode**: When `origin_error_page_get_only` is `true`, only `GET` requests receive HTML replacement; non-`GET` methods pass through unmodified.
* **Built-in template & placeholders**: `internal/openflare/edgeconfig/origin_error_page.go` embeds an inline brand SVG mark with zero external asset dependencies and substitutes `{{status}}` and `{{host}}` dynamically. Custom HTML is trusted administrator content stored without sanitization.

---

## 2. Configuration Model and Sync Pipeline

Settings persist in `w_system_configs` via `/api/v1/d/option` and `/api/v1/d/option/update-batch`, then compile into `sites.v1` snapshots:

| Option Key | Type | Default | Validation & Limits |
| :--- | :--- | :--- | :--- |
| `origin_error_page_enabled` | bool string | `true` | Global enable toggle |
| `origin_error_page_status_codes` | JSON string array | `["500-599"]` | Single codes (`522`) or closed ranges (`500-599`), normalized within `400–599` (non-empty when enabled) |
| `origin_error_page_html` | text | `""` | Empty selects built-in template; maximum `256 KiB` bytes |
| `origin_error_page_get_only` | bool string | `false` | Restricts replacement to `GET` requests |

Shared pure parsing and validation functions live in `internal/openflare/edgeconfig/origin_error_page.go` so API validation and snapshot compilation stay consistent. Changes propagate atomically through `sites.v1` without restarting edge listeners.
