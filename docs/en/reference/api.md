# API & Protocol Reference

You will learn: OpenFlare's OpenAPI / Swagger specification files, interactive UI console, authentication mechanisms, uniform response envelope structure, core API route groups, and developer integration examples.

---

## Specification Files and Interactive Console

The OpenFlare backend automatically maintains OpenAPI 2.0 / Swagger specification documents based on Go source code annotations via [Swaggo](https://github.com/swaggo/swag).

### 1. Static Specification Files

The specification definition files are stored under the `docs/` directory of the repository, ready to be imported into tools like Postman, Apifox, or Insomnia, or used for automated client SDK generation:

- **JSON Format**: `docs/swagger.json`
- **YAML Format**: `docs/swagger.yaml`

Whenever you update Handler annotations, structs, or route bindings, regenerate the specification from the repository root:

```bash
make swagger
# Equivalent to executing scripts/swagger.sh
```

### 2. Interactive Swagger UI

When the server runs in a **non-production environment** (`app.env` set to `development` or `testing`, i.e., `APP_ENV != production`), the backend mounts the interactive Swagger UI console:

- **Access URL**: `http://<SERVER_ADDR>/api/swagger/index.html` (if `app.api_prefix` has been customized, replace `/api` with your actual prefix)
- **Features**: Live API debugging, inspecting request/response schemas, testing header authentication.

> **Note**: In production (`app.env: production`), the `/swagger/*` route is disabled by default for security hardening.

---

## Authentication

OpenFlare provides two standard client authentication methods along with dedicated protocol credentials for edge daemons.

### 1. Access Token (Personal API Key)

Intended for automation scripts, CI/CD pipelines, and third-party system integrations.

- **How to obtain**: After logging into the admin console, navigate to "Profile" or "System Management → API Tokens" to create a token.
- **Header delivery**: Pass via the standard `Authorization: Bearer <TOKEN>` header (the `X-Access-Token: <TOKEN>` header is also supported).

```http
GET /api/v1/d/dashboard/overview HTTP/1.1
Host: panel.example.com
Authorization: Bearer of_pat_xxxxxxxxxxxxxxxxxxxxxxxx
Accept: application/json
```

### 2. Session Cookie (Browser Interactive Session)

Intended for web dashboard operations and interactive user interfaces.

- **Authentication flow**: Submit credentials to `POST /api/v1/oauth/login`, or complete authentication through OIDC (SSO) providers.
- **Cookie structure**: Upon successful login, the server sets a session cookie named according to `app.session_cookie_name` (default: `openflare_session_id`).
- **TTL**: Governed by `app.session_age` (default: 86400 seconds) or the system setting `login_session_ttl_hours`.

### 3. Edge Node Protocol Credentials

Pingora Edge nodes, Relay nodes, and OpenFlared tunnel clients authenticate with the control plane using dedicated credentials:

- **Discovery Token**: A global token used for initial node registration, governed by the system setting `agent_discovery_token`.
- **Node Agent Token**: A node-specific credential issued by the control plane upon registration, used for heartbeat reporting and configuration pull.
- **Tunnel Token**: A dedicated token used by OpenFlared to authenticate tunnel sessions.

---

## Response Envelope Structure

All OpenFlare HTTP API endpoints return a standardized JSON envelope conforming to the following structure.

### 1. Successful Response (HTTP 200 OK)

Successful API invocations always return HTTP status `200`. The `error_msg` field is an empty string, and the payload is contained in `data`:

```json
{
  "error_msg": "",
  "data": {
    "site_name": "OpenFlare",
    "version": "1.0.0"
  }
}
```

When an operation produces no content (such as delete or update operations), `data` is returned as `null`:

```json
{
  "error_msg": "",
  "data": null
}
```

### 2. Paginated List Response

For collection queries, `data` contains `total` (total record count) and `results` (array of items):

```json
{
  "error_msg": "",
  "data": {
    "total": 42,
    "results": [
      {
        "id": "194729103847291024",
        "name": "example-zone",
        "status": "active"
      }
    ]
  }
}
```

### 3. Error Response (HTTP 4xx / 5xx)

When parameter validation fails, credentials are missing, or a server error occurs, the API returns a corresponding HTTP error status code (never HTTP 200). `error_msg` contains a camelCase error identifier, and `data` is `null`:

```json
{
  "error_msg": "unauthorized",
  "data": null
}
```

Common status codes and error identifiers:

| HTTP Status | Scenario | Example `error_msg` |
| --- | --- | --- |
| `400 Bad Request` | Request binding or validation failed | `errBindParamsFailed`, `errInvalidInput` |
| `401 Unauthorized` | Missing, invalid, or expired session/token | `unauthorized`, `errTokenExpired` |
| `403 Forbidden` | Insufficient permissions (e.g. non-admin) | `forbidden`, `errAdminRequired` |
| `404 Not Found` | Requested resource does not exist | `recordNotFound`, `userNotFound` |
| `500 Internal Server Error` | Unhandled database or system exception | `errInternalServer` |
| `503 Service Unavailable` | Log database migration in progress | `日志数据库迁移中，暂不可写` |

---

## Core API Route Groups

OpenFlare control plane endpoints are organized into clean functional groups:

### 1. User and Authentication Routes

| Route | Method | Access Level | Description |
| --- | --- | --- | --- |
| `/api/v1/oauth/login` | `POST` | Public | Password-based login |
| `/api/v1/oauth/logout` | `POST` | Authenticated | Logout and terminate current session |
| `/api/v1/oauth/authorize` | `GET` | Public | Initiate OIDC third-party authorization redirect |
| `/api/v1/oauth/callback` | `GET` | Public | OIDC callback handler |
| `/api/v1/user/profile` | `GET` | Authenticated | Retrieve profile of the current user |
| `/api/v1/user/password` | `POST` | Authenticated | Change user password |
| `/api/v1/user/access-tokens` | `GET`/`POST` | Authenticated | Manage personal access tokens |

### 2. Platform Administration Routes (`/api/v1/admin/*`)

| Route | Method | Access Level | Description |
| --- | --- | --- | --- |
| `/api/v1/admin/users` | `GET`/`POST` | Administrator | List and create platform users |
| `/api/v1/admin/users/:id` | `PUT`/`DELETE` | Administrator | Update or deactivate user accounts |
| `/api/v1/admin/configs` | `GET`/`POST` | Administrator | Read and hot-reload runtime system configs (`w_system_configs`) |
| `/api/v1/admin/database/export`| `GET` | Administrator | Export database snapshot |
| `/api/v1/admin/tasks` | `GET` | Administrator | Inspect Asynq background task queues |

### 3. OpenFlare Console Routes (`/api/v1/d/*`)

| Domain | Base Path | Description |
| --- | --- | --- |
| Dashboard | `/api/v1/d/dashboard/overview` | Platform bandwidth, QPS, request count, and node health summary |
| Zones | `/api/v1/d/zones` | Zone management, DNS records, and Cloudflare DNS sync |
| Origins | `/api/v1/d/origins` | Upstream origin server pools and health check probe settings |
| Proxy Routes | `/api/v1/d/proxy-routes` | Reverse proxy routing rules and load-balancing policies |
| Edge Nodes | `/api/v1/d/nodes` | Pingora Edge node inventory, health metrics, and restart triggers |
| Performance Policies | `/api/v1/d/performance/policies` | Edge runtime, upstream pools, HTTP/2, and caching templates |
| WAF Protection | `/api/v1/d/waf` | WAF rulesets, attack detection modes (CRS), and IP group policies |
| PoW Captcha | `/api/v1/d/pow` | Proof-of-Work challenge configurations and protection triggers |
| Pages Static Sites | `/api/v1/d/pages` | Static site projects, package uploads (`.zip` / `.tar.gz`), and history |
| TLS Certificates | `/api/v1/d/certificates` | SSL/TLS certificate management and automated ACME renewal |

### 4. Edge Communication Routes (`/api/v1/agent/*`, `/api/v1/relay/*`, `/api/v1/tunnel/*`)

| Route | Protocol | Auth Type | Description |
| --- | --- | --- | --- |
| `/api/v1/agent/nodes/register` | HTTP POST | Discovery Token | Automated node registration |
| `/api/v1/agent/nodes/heartbeat` | HTTP POST | Node Agent Token | Periodic health metrics reporting |
| `/api/v1/agent/ws` | WebSocket | Node Agent Token | Persistent control plane channel for snapshot push |
| `/api/v1/relay/ws` | WebSocket | Relay Agent Token | Relay node stream control |
| `/api/v1/tunnel/ws` | WebSocket | Tunnel Token | Tunnel client daemon synchronization |

---

## Developer Integration Examples

### 1. Fetch Dashboard Overview via Access Token

```bash
curl -fsSL -X GET "https://panel.example.com/api/v1/d/dashboard/overview" \
  -H "Authorization: Bearer of_pat_9a8b7c6d5e4f3a2b1c" \
  -H "Accept: application/json"
```

Response example:

```json
{
  "error_msg": "",
  "data": {
    "total_nodes": 4,
    "online_nodes": 4,
    "total_zones": 12,
    "today_requests": 1428570,
    "today_bytes": 10737418240
  }
}
```

### 2. Query Edge Nodes with Pagination

```bash
curl -fsSL -X GET "https://panel.example.com/api/v1/d/nodes?page=1&page_size=10" \
  -H "Authorization: Bearer of_pat_9a8b7c6d5e4f3a2b1c" \
  -H "Accept: application/json"
```

### 3. Create a Reverse Proxy Route

```bash
curl -fsSL -X POST "https://panel.example.com/api/v1/d/proxy-routes" \
  -H "Authorization: Bearer of_pat_9a8b7c6d5e4f3a2b1c" \
  -H "Content-Type: application/json" \
  -d '{
    "zone_id": "194729103847291024",
    "name": "api-service",
    "pattern": "/api/*",
    "origin_id": "194729103847291055",
    "websocket_enabled": true
  }'
```

---

## Related Documentation

- [Configuration Reference](./configuration.md): Server, Agent, and Edge node parameters
- [Commands & Scripts Reference](./cli.md): Makefile targets, CLI flags, and build scripts
- [Architecture Design](../design/architecture.md): Control-plane and data-plane communication models
