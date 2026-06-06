# Local Development

You will learn: How to build OpenFlare's local development environment, start the Server, the Agent, and the Admin Frontend, run test and build commands, and understand the boundaries to respect before contributing code.

This page is aimed at contributors. Product boundaries, data model constraints, API conventions, and frontend layering specifications are governed by [Development Constraints](../../guideline/Constraints.md); this page only provides actionable workflows for local development.

## Repository Structure

For details on the physical directory structure and responsibilities of each module (Server, Agent, Frontend, etc.), see [Repository Structure](./repository.md).

## Environment Requirements

| Item | Requirement |
| --- | --- |
| Go | `1.25+` |
| Node.js | `18+` |
| pnpm | Recommended enabling via `corepack enable` |
| Docker | Required for Server containers, local integration testing, and Agent Docker images |
| OpenResty | Required to execute `openresty` locally when running the Agent |
| PostgreSQL | Optional; if not configured, the Server defaults to SQLite |

## Initializing Frontend Dependencies

```bash
cd openflare-server/web
corepack enable
pnpm install
```

Build the static assets hosted by the Go Server:

```bash
pnpm build
```

## Starting the Server

SQLite Mode:

```bash
cd openflare-server
export SESSION_SECRET='dev-session-secret'
export SQLITE_PATH='./openflare-dev.db'
export LOG_LEVEL='debug'
go run .
```

PostgreSQL Mode:

```bash
cd openflare-server
export SESSION_SECRET='dev-session-secret'
export DSN='postgres://openflare:secret@127.0.0.1:5432/openflare?sslmode=disable'
export LOG_LEVEL='debug'
go run .
```

Default access URL:

```text
http://localhost:3000
```

The default credentials are `root` / `123456`.

## Starting the Frontend Dev Server

The frontend dev server listens to port `3001` by default and proxies requests to the backend via `NEXT_DEV_BACKEND_URL`:

```bash
cd openflare-server/web
export NEXT_DEV_BACKEND_URL='http://127.0.0.1:3000'
pnpm dev
```

Access:

```text
http://localhost:3001
```

## Starting the Agent

Create a local `agent.json`:

```json
{
  "server_url": "http://127.0.0.1:3000",
  "agent_token": "replace-with-node-auth-token",
  "data_dir": "./data",
  "heartbeat_interval": 10000,
  "request_timeout": 10000
}
```

Run:

```bash
cd openflare-agent
export LOG_LEVEL='debug'
go run ./cmd/agent -config ./agent.json
```

If `openresty_path` is not configured, the Agent calls `openresty` by default. For debugging, you can explicitly configure `openresty_path`, `main_config_path`, `route_config_path`, `access_log_path`, `cert_dir`, `lua_dir`, and `runtime_config_dir`.

## Running Tests

Server:

```bash
cd openflare-server
GOCACHE=/tmp/openflare-go-cache go test ./...
```

Agent:

```bash
cd openflare-agent
GOCACHE=/tmp/openflare-go-cache go test ./...
```

Frontend:

```bash
cd openflare-server/web
pnpm lint
pnpm typecheck
pnpm test
pnpm test:e2e
```

Docs:

```bash
cd docs
pnpm build
```

## Building

Admin static assets:

```bash
cd openflare-server/web
pnpm build
```

Server binary:

```bash
cd openflare-server
go build -o openflare-server .
```

Agent binary:

```bash
cd openflare-agent
go build -o openflare-agent ./cmd/agent
```

## Debugging Entrypoints

| Context | Command or Path |
| --- | --- |
| Server Logs | `LOG_LEVEL=debug go run .` |
| Agent Logs | `LOG_LEVEL=debug go run ./cmd/agent -config ./agent.json` |
| Swagger Docs | `http://localhost:3000/swagger/index.html` |
| Frontend API Proxy | `NEXT_DEV_BACKEND_URL=http://127.0.0.1:3000 pnpm dev` |
| OpenResty Validation | `openresty -t -c ./data/etc/nginx/nginx.conf` |

## Code Style & Change Admission

Before contributing, verify:

1. The requirement matches [Product Boundaries](./index.md).
2. The implementation conforms to [Development Constraints](../guideline/development-constraints.md).
3. The change does not disrupt publishing, sync, rollback, or upgrading lifecycles.
4. Update corresponding documentation if configurations, deployments, APIs, or boundaries change.
5. High-risk edits must be accompanied by unit tests or equivalent integration testing.

Database schema alterations must elevate the database version number and supply explicit migration and validation methods from the previous version.
