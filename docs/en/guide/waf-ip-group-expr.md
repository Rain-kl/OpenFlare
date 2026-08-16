# WAF Auto IP Group Expressions

Automatic IP groups are used to aggregate metrics from request logs on a per-client-IP basis, using Expr expressions to determine if an IP should be added to the group. Automatic IP groups can be referenced by IP blacklists or whitelists in WAF rule groups; during publication, the Server only writes the referenced IP group ID to `waf_config.json`, while IP group members are synchronized independently by the Agent into the local runtime files.

## Configuration Structure

The configuration of an automatic IP group is a JSON object:

```json
{
  "lookback_minutes": 60,
  "rules": [
    {
      "name": "Single IP High-Frequency 404 Scanning",
      "expr": "request_count > 100 && status_404_ratio >= 0.8"
    }
  ]
}
```

Field Descriptions:

| Field | Type | Role |
| --- | --- | --- |
| `lookback_minutes` | number | How many minutes of request logs to look back during execution. Defaults to 60 minutes if blank, minimum 5 minutes, maximum 43200 minutes. |
| `rules` | array | List of automatic rules. If any rule matches, the IP is added to the automatic IP group list. |
| `rules[].name` | string | Rule name, used only for UI display and error messages. |
| `rules[].expr` | string | Expr expression, must return a boolean value. |

## Evaluation Mechanics

Automatic rules do not evaluate logs request-by-request, but instead aggregate them by client IP first:

1. The Server reads request logs from the past `lookback_minutes` minutes.
2. Groups them by normalized IP (`remote_addr`).
3. Computes metrics like request count, 404 count, and direct IP host count for each IP.
4. Evaluates `rules[].expr` for each IP.
5. If an IP matches any rule, it is written to the automatic IP group's IP member list.

Whether a request is "accessing via IP directly" is determined by the `Host` field in the request logs. If the Host header is an IPv4 or IPv6 literal (e.g., `203.0.113.10`, `[2001:db8::10]`, `203.0.113.10:443`), it is counted in `ip_host_count`.

## Available Metrics

The following metrics are directly available in Expr expressions:

| Keyword | Type | Role |
| --- | --- | --- |
| `ip` | string | The client IP currently being evaluated. |
| `request_count` | number | Total request count of the IP in the lookback window. |
| `status_404_count` | number | Number of 404 responses returned to the IP in the lookback window. |
| `status_404_ratio` | number | 404 request ratio, calculated as `status_404_count / request_count`. |
| `ip_host_count` | number | Number of requests from the IP using an IP address directly as the Host header. |
| `ip_host_ratio` | number | Ratio of direct IP address accesses, calculated as `ip_host_count / request_count`. |
| `client_error_count` | number | Number of requests returning 4xx status codes. |
| `server_error_count` | number | Number of requests returning 5xx status codes. |
| `last_seen_unix` | number | Unix timestamp (in seconds) of the last request from the IP in the lookback window. |

All ratio fields are decimals between `0` and `1`. An 80% ratio should be written as `0.8`, and 50% as `0.5`.

## Common Expr Syntax

Automatic IP groups use the Expr syntax. The expression must return a boolean value.

Common Operators:

| Operator | Role | Example |
| --- | --- | --- |
| `>`, `>=`, `<`, `<=` | Numeric comparison | `request_count > 100` |
| `==`, `!=` | Equality / Inequality | `ip != "127.0.0.1"` |
| `&&` | Logical AND | `request_count > 100 && status_404_ratio >= 0.8` |
| `||` | Logical OR | `status_404_ratio >= 0.8 || server_error_count > 20` |
| `!` | Logical NOT | `!(ip == "127.0.0.1")` |
| `in` | Value is in list | `ip in ["203.0.113.10", "198.51.100.20"]` |
| `not in` | Value is not in list | `ip not in ["127.0.0.1"]` |
| `()` | Grouping controls operator priority | `(request_count > 100 && status_404_ratio >= 0.8) || server_error_count > 50` |

## Built-in Presets

The management console provides two built-in preset rules that can be added directly and adjusted as needed:

```json
{
  "name": "Single IP High-Frequency 404 Scanning",
  "expr": "request_count > 100 && status_404_ratio >= 0.8"
}
```

Meaning: A single IP requests more than 100 times in the lookback window, and the 404 status code ratio is at least 80%.

```json
{
  "name": "Single IP Direct IP Access Mismatch",
  "expr": "ip_host_count > 50 && ip_host_ratio > 0.5"
}
```

Meaning: A single IP accesses the server directly using an IP address as the Host header more than 50 times, and this type of access represents more than 50% of its total requests.

## Examples

High-frequency 404 scanning:

```json
{
  "lookback_minutes": 60,
  "rules": [
    {
      "name": "High-Frequency 404 Scanning",
      "expr": "request_count > 100 && status_404_ratio >= 0.8"
    }
  ]
}
```

Direct IP access mismatch:

```json
{
  "lookback_minutes": 30,
  "rules": [
    {
      "name": "Direct IP Access Mismatch",
      "expr": "ip_host_count > 50 && ip_host_ratio > 0.5"
    }
  ]
}
```

Capture both high 4xx and 5xx errors:

```json
{
  "lookback_minutes": 120,
  "rules": [
    {
      "name": "Abnormal Error Rates",
      "expr": "(client_error_count > 80 && request_count > 100) || server_error_count > 30"
    }
  ]
}
```

Exclude trusted IPs:

```json
{
  "lookback_minutes": 60,
  "rules": [
    {
      "name": "404 Scanning Excluding Trusted IPs",
      "expr": "ip not in [\"203.0.113.10\", \"198.51.100.20\"] && request_count > 100 && status_404_ratio >= 0.8"
    }
  ]
}
```

## Usage Recommendations

Start with a shorter lookback window and higher thresholds to monitor matches, then adjust thresholds gradually. The IP Groups page in the management console allows you to click **"Test Rule"** before saving to view matching IPs in the current window immediately. Once an automatic IP group runs, it overwrites the list of IPs. If you want to permanently whitelist or blacklist certain IPs, add them to a manual IP group instead, and reference both manual and automatic groups in your WAF rule groups.

Updating automatic IP groups does not require publishing configuration versions. Online Agents receive changes via WebSocket and update the local `waf_ip_groups.json` instantly. If WebSocket is unavailable, the Agent reports its local checksum in heartbeats, and the Server syncs only the mismatched IP groups.
