# Architecture

OpenFlare consists of Server, Agent, and local OpenResty on each node.

```text
OpenFlare Server (Gin + SQLite/PostgreSQL + Web UI)
        |
        | HTTP API / Config Pull
        v
OpenFlare Agent (register / heartbeat / sync / apply / update)
        |
        v
OpenResty binary
        |
        v
Origin
```

## Server

`openflare_server` is a monolithic control plane based on Gin, GORM, SQLite/PostgreSQL, the existing login/session system, and the static frontend build.

It owns the admin UI and API, Agent API, configuration rendering, version publishing, storage, and aggregate queries.

## Agent

`openflare_agent` is a single Go binary that runs on each node. It controls OpenResty through `openresty_path`, or `openresty` by default. Docker deployments use an Agent image that already includes OpenResty and follows the same binary-control flow.

It handles registration, heartbeat, sync, file writes, `openresty -t`, reload, rollback, self-update, and lightweight collection.

## Frontend

`openflare_server/web` is the production frontend baseline: Next.js App Router, React 19, TypeScript, and Tailwind CSS.
