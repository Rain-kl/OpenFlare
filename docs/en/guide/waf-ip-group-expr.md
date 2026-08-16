# WAF Auto IP Group Rule Syntax

Auto IP groups aggregate metrics per client IP from request logs, then use Expr expressions to decide whether to add an IP to the group list. Auto IP groups can be referenced by a WAF rule group's IP blocklist or allowlist; on config release, the Server only writes the IP group reference IDs into `waf_config.json` — IP group members are synced independently by the Agent to the local runtime file.

## Config Structure

An auto IP group config is a JSON object:

```json
{
  "lookback": "1h",
  "rules": [
    {
      "name": "Single-IP high-frequency 404 scanning",
      "expr": "request_count > 100 && StatusRatio(404) >= 0.8"
    }
  ]
}
```

Field reference:

| Field | Type | Purpose |
| --- | --- | --- |
| `lookback` | string | lookback window duration in Go Duration syntax, e.g. `30m`, `1h`, `90m`. Defaults to `1h`, max 30 days. Compatible with the legacy `lookback_minutes` (integer minutes). |
| `rules` | array | auto-rule list. If any rule matches, the IP enters the auto IP group list. |
| `rules[].name` | string | rule name, only for UI display and error messages. |
| `rules[].expr` | string | Expr expression, must return a boolean. |

## Execution Semantics

Auto IP groups first aggregate metrics per client IP, then run rule expressions against each IP:

1. The Server reads request logs from the last `lookback` window.
2. Groups by normalized `remote_addr` IP.
3. Computes per-IP metrics: request count, 404 count, direct-IP Host count, etc.
4. Runs `rules[].expr` per IP.
5. If an IP matches any rule, it is written into the auto IP group's `IP / IP segment` list.

Whether a Host is "accessed via IP" is judged by the `Host` field in request logs: if the Host is an IPv4 or IPv6 literal, e.g. `203.0.113.10`, `[2001:db8::10]`, `203.0.113.10:443`, it counts toward `ip_host_count`.

## Available Keywords

The expression can use these fields directly:

| Keyword | Type | Purpose |
| --- | --- | --- |
| `ip` | string | the client IP currently being judged. |
| `request_count` | number | the current IP's total requests in the lookback window. |
| `status_404_count` | number | the current IP's requests returning 404 in the window. |
| `status_404_ratio` | number | 404 ratio, computed as `status_404_count / request_count`. |
| `ip_host_count` | number | requests where the current IP accessed via an IP-literal Host. |
| `ip_host_ratio` | number | ratio of IP-address access, computed as `ip_host_count / request_count`. |
| `client_error_count` | number | the current IP's requests returning 4xx. |
| `server_error_count` | number | the current IP's requests returning 5xx. |
| `last_seen_unix` | number | the current IP's last request Unix timestamp (seconds) in the window. |

Ratio fields are decimals between `0` and `1`. 80% is written `0.8`, 50% is `0.5`.

### Custom Status Code Matching

If the built-in `status_404_count` / `status_404_ratio` don't fit, use these built-in methods to match arbitrary status codes:

* **`StatusCount(code)`**: request count of the current IP returning the given status code (or class) in the window.
  * exact code: `StatusCount(403) > 10`
  * status class (`1xx`–`5xx`, case-insensitive): `StatusCount("4xx") > 50`
* **`StatusRatio(code)`**: the ratio of the above count to the IP's total requests.
  * exact code: `StatusRatio(502) >= 0.5`
  * status class: `StatusRatio("4xx") >= 0.8`, `StatusRatio("5xx") >= 0.3`

A status class aggregates all codes in that hundred range, e.g. `"4xx"` covers 400–499, `"2xx"` covers 200–299.

## Common Expr Patterns

Auto IP groups use Expr syntax; the expression must return a boolean.

Common operators:

| Pattern | Purpose | Example |
| --- | --- | --- |
| `>`、`>=`、`<`、`<=` | numeric comparison | `request_count > 100` |
| `==`、`!=` | equal / not equal | `ip != "127.0.0.1"` |
| `&&` | and | `request_count > 100 && StatusRatio(404) >= 0.8` |
| `||` | or | `StatusRatio(404) >= 0.8 || server_error_count > 20` |
| `!` | negation | `!(ip == "127.0.0.1")` |
| `in` | value in list | `ip in ["203.0.113.10", "198.51.100.20"]` |
| `not in` | value not in list | `ip not in ["127.0.0.1"]` |
| `()` | grouping precedence | `(request_count > 100 && StatusRatio(404) >= 0.8) || server_error_count > 50` |

## Built-in Presets

The admin panel ships two preset rules that can be added and then adjusted:

```json
{
  "name": "Single-IP high-frequency 404 scanning",
  "expr": "request_count > 100 && StatusRatio(404) >= 0.8"
}
```

Meaning: a single IP has over 100 requests in the window and a 404 ratio of at least 80%.

```json
{
  "name": "Single-IP direct-access anomaly",
  "expr": "ip_host_count > 50 && ip_host_ratio > 0.5"
}
```

Meaning: a single IP accessed via IP-literal Host over 50 times, and that access ratio exceeds 50%.

## Examples

High-frequency 404 scanning:

```json
{
  "lookback": "1h",
  "rules": [
    {
      "name": "High-frequency 404 scanning",
      "expr": "request_count > 100 && StatusRatio(404) >= 0.8"
    }
  ]
}
```

IP direct-access anomaly:

```json
{
  "lookback": "30m",
  "rules": [
    {
      "name": "IP direct-access anomaly",
      "expr": "ip_host_count > 50 && ip_host_ratio > 0.5"
    }
  ]
}
```

Catching both high 4xx and high 5xx:

```json
{
  "lookback": "2h",
  "rules": [
    {
      "name": "Abnormal error rate",
      "expr": "(client_error_count > 80 && request_count > 100) || server_error_count > 30"
    }
  ]
}
```

Using status-class syntax (equivalent thinking to `client_error_count` / `server_error_count`):

```json
{
  "lookback": "2h",
  "rules": [
    {
      "name": "High 4xx or 5xx ratio",
      "expr": "request_count > 100 && (StatusRatio(\"4xx\") >= 0.8 || StatusRatio(\"5xx\") >= 0.3)"
    }
  ]
}
```

Excluding trusted IPs:

```json
{
  "lookback": "1h",
  "rules": [
    {
      "name": "404 scanning excluding trusted IPs",
      "expr": "ip not in [\"203.0.113.10\", \"198.51.100.20\"] && request_count > 100 && StatusRatio(404) >= 0.8"
    }
  ]
}
```

## Usage Tips

Start with a shorter lookback and higher thresholds to observe hits, then tune gradually. The admin IP group page supports clicking **Test Rule** before saving to directly view the IPs hit in the current window; when the auto IP group actually runs, it overwrites the group's IP list. To keep certain addresses long-term, put them in a manual IP group and reference both the manual and auto groups in the WAF rule group.

Auto IP group updates don't require republishing a config version. Online Agents receive the changed IP groups via WebSocket and update the local `waf_ip_groups.json`; when the WebSocket is unavailable, the Agent reports the local IP group checksum in the next heartbeat and the Server only returns the groups whose checksums differ.
