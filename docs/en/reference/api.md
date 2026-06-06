# API Conventions

You will learn: The response structure, path conventions, authentication methods, and Swagger entrance for the OpenFlare Admin API and Agent API.

Both the OpenFlare Admin API and Agent API communicate using JSON.

## Response Structure

Both successful and failed API responses must return a clear `message`:

```json
{
  "success": true,
  "message": "",
  "data": {}
}
```

## Path Conventions

| Category | Convention |
| --- | --- |
| Admin API | Authenticated via the Admin Session |
| Agent API | Located strictly under `/api/agent/*` |
| Relay API | Located strictly under `/api/relay/*`, authenticated via `X-Agent-Token` (reusing the Agent's token) |
| OpenFlared API | Located strictly under `/api/flared/*`, authenticated via `X-Tunnel-Token` (dedicated tunnel_token) |
| Read-only APIs | Use the `GET` method |
| Mutating APIs | Use the `POST` method |

## WAF IP Group APIs

The Admin WAF IP Group APIs require Admin Session authentication:

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/waf/ip-groups` | Query IP groups list |
| `GET` | `/api/waf/ip-groups/:id` | Query a single IP group |
| `POST` | `/api/waf/ip-groups` | Create a new IP group |
| `POST` | `/api/waf/ip-groups/test` | Test automatic IP group Expr rules; returns matching IPs in the lookback window without persisting the config |
| `POST` | `/api/waf/ip-groups/:id/update` | Update an existing IP group |
| `POST` | `/api/waf/ip-groups/:id/delete` | Delete an IP group; denied if currently referenced by any rule group |
| `POST` | `/api/waf/ip-groups/:id/sync` | Manually sync subscription IP groups or execute automatic IP group aggregation |

The IP group `type` supports `manual`, `automatic`, and `subscription`. The `auto_config` parameter for automatic IP groups is a JSON object:

```json
{
  "lookback_minutes": 60,
  "rules": [
    {
      "name": "Single IP High-Frequency 404 Scanning",
      "expr": "request_count > 100 && status_404_ratio >= 0.8"
    },
    {
      "name": "Single IP Direct IP Access Mismatch",
      "expr": "ip_host_count > 50 && ip_host_ratio > 0.5"
    }
  ]
}
```

Automatic rules evaluate Expr boolean expressions against metrics aggregated on a per-client-IP basis. The available metrics include `ip`, `request_count`, `status_404_count`, `status_404_ratio`, `ip_host_count`, `ip_host_ratio`, `client_error_count`, `server_error_count`, and `last_seen_unix`. The full syntax is detailed in [WAF Auto IP Group Expressions](../guide/waf-ip-group-expr.md). 

Subscription formats support `text` and `json`: plain text parsing resolves one IP or CIDR per line, ignoring empty lines and comments starting with `#`; JSON parsing decodes arrays, reading the root array by default.

## Authentication

The Admin panel continues to reuse the existing login, role, and Session validation.

Agent requests must carry the node-specific `agent_token` (except for first-time registration, which can use the global `discovery_token`). The header is formatted as:

```http
X-Agent-Token: <token>
```

### Agent WAF IP Group Synchronization

The Agent heartbeat payload can carry local WAF IP group checksums:

```json
{
  "waf_ip_group_checksums": {
    "1": "sha256..."
  }
}
```

The Server evaluates the checksums against active configurations, returning mismatched IP groups in the heartbeat response:

```json
{
  "waf_ip_groups": [
    {
      "id": 1,
      "name": "Auto Blacklist",
      "type": "automatic",
      "enabled": true,
      "ip_list": ["203.0.113.10"],
      "checksum": "sha256..."
    }
  ]
}
```

Alternatively, the Agent can proactively request differential updates upon applying a new configuration version:

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/api/agent/waf/ip-groups/sync` | Returns mismatched WAF IP groups based on Agent-supplied `ids` and `checksums` |

When an IP group is updated on the Server, connected Agents receive a WebSocket push containing `type = "waf_ip_groups"` with the changed IP groups array as payload. The Agent updates only the changed groups incrementally.

## OpenFlared API

The OpenFlared client communicates with the Server via a dedicated `tunnel_token`, completely decoupled from the Agent authentication system. All endpoints require `X-Tunnel-Token` authentication; requests are denied with `403` if the token is invalid.

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/api/flared/heartbeat` | Client heartbeat, updates online status and retrieves active tunnel config version summaries |
| `GET` | `/api/flared/config/active` | Pulls the complete tunnel routing configuration (relay list + frpc proxy definitions) |
| `POST` | `/api/flared/apply-log` | Reports configuration application results (success / warning / failed) |
| `GET` | `/api/flared/ws` | Upgrades to a WebSocket connection for real-time `active_config` pushes |

Heartbeat request example:

```http
POST /api/flared/heartbeat
X-Tunnel-Token: <tunnel_token>
Content-Type: application/json

{
  "client_version": "v0.2.0",
  "frp_version": "0.61.0",
  "tunnel_status": "running",
  "connected_relays": [
    { "relay_node_id": "node-relay-1", "status": "healthy", "proxy_count": 3 }
  ],
  "current_version": "v1",
  "current_checksum": "sha256..."
}
```

The heartbeat response returns the `active_config` summary and `tunnel_settings` (containing runtime settings like heartbeat intervals and WebSocket upgrade switches). When a new configuration version is published, the Server broadcasts a message `type = "active_config"` with the version summary as payload to all connected Clients over WebSockets, prompting them to fetch and apply the config immediately.

Full tokens must never be logged.

## Swagger

Once logged into the management console, the Swagger page is accessible at:

```text
/swagger/index.html
```

The Swagger definition file is stored in `openflare-server/docs`, generated by `swag init`.
