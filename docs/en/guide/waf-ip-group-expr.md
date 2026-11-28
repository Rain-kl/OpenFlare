# WAF Auto IP Group Rule Syntax

Aggregate edge request logs per client IP over a lookback window and evaluate Expr boolean expressions to populate WAF **IP Match** groups.

## 1. Configuration Schema & Scope

| Field | Type | Description |
| :--- | :--- | :--- |
| `lookback` | string | Go Duration window (`30m`, `1h`); default `1h`, max `30d` (legacy integer `lookback_minutes` supported) |
| `rules` | array | Rule list; any rule returning `true` adds the client IP |
| `rules[].name` | string | Rule label displayed in the console and error messages |
| `rules[].expr` | string | Expr expression returning a boolean |

Both scheduled runs and **Test Rule** group by normalized `remote_addr` while automatically skipping private IPv4, IPv6 ULA, loopback, link-local, unspecified, multicast, and broadcast addresses (including IPv4-mapped IPv6).

## 2. Variables & Functions

| Identifier | Type | Description |
| :--- | :--- | :--- |
| `ip` | string | Client IP address under evaluation |
| `request_count` | number | Total requests in `lookback` |
| `status_404_count` | number | Requests returning HTTP `404` |
| `status_404_ratio` | number | Ratio of `404` responses (`0` to `1`, e.g. `0.8` = 80%) |
| `ip_host_count` | number | Requests using an IP-literal `Host` header (e.g. `203.0.113.10`) |
| `ip_host_ratio` | number | Ratio of IP-literal `Host` requests (`0` to `1`) |
| `client_error_count` | number | Responses with `4xx` status codes |
| `server_error_count` | number | Responses with `5xx` status codes |
| `last_seen_unix` | number | Unix timestamp (seconds) of the latest request |
| `StatusCount(code)` | number | Response count for an exact code (`403`) or class (`"4xx"`) |
| `StatusRatio(code)` | number | Response ratio (`0` to `1`) for an exact code (`502`) or class (`"5xx"`) |

## 3. Operators & Presets

| Operator | Meaning | Example |
| :--- | :--- | :--- |
| `>`, `>=`, `<`, `<=` | Numeric comparison | `request_count > 100` |
| `==`, `!=` | Equality | `ip != "203.0.113.10"` |
| `&&`, `||`, `!` | Logical AND, OR, NOT | `request_count > 100 && StatusRatio(404) >= 0.8` |
| `in`, `not in` | Membership | `ip not in ["203.0.113.10", "198.51.100.20"]` |
| `()` | Grouping | `(client_error_count > 80 && request_count > 100) || server_error_count > 30` |

Built-in presets available in the editor:

| Preset | Expression |
| :--- | :--- |
| Single-IP 404 scanning | `request_count > 100 && StatusRatio(404) >= 0.8` |
| Single-IP direct-IP access | `ip_host_count > 50 && ip_host_ratio > 0.5` |

Combined example:

```json
{
  "lookback": "2h",
  "rules": [
    {
      "name": "Single-IP high-frequency 404 scanning",
      "expr": "request_count > 100 && StatusRatio(404) >= 0.8"
    },
    {
      "name": "High error ratio excluding trusted IPs",
      "expr": "ip not in [\"203.0.113.10\", \"198.51.100.20\"] && request_count > 100 && (StatusRatio(\"4xx\") >= 0.8 || StatusRatio(\"5xx\") >= 0.3)"
    }
  ]
}
```

## 4. Best Practices

- Validate new thresholds with **Test Rule** over a short `lookback` before saving.
- Store permanent allow or block entries in a separate manual IP group, since scheduled evaluations overwrite auto IP group membership on each run.
