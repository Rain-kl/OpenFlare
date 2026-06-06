# CLI Commands

You will learn: Common commands for starting, building, testing, installing, and uninstalling the OpenFlare Server, Admin Frontend, Agent, Swagger, and Documentation site.

## Server

Start from source:

```bash
cd openflare-server
export SESSION_SECRET='replace-with-random-string'
export SQLITE_PATH='./openflare.db'
export LOG_LEVEL='info'
go run .
```

Specify listening port and logging directory:

```bash
go run . --port 3000 --log-dir ./logs
```

Run tests:

```bash
cd openflare-server
GOCACHE=/tmp/openflare-go-cache go test ./...
```

## Frontend

Development:

```bash
cd openflare-server/web
pnpm install
pnpm dev
```

Build static assets:

```bash
cd openflare-server/web
pnpm build
```

Linting and testing checks:

```bash
cd openflare-server/web
pnpm lint
pnpm typecheck
pnpm test
```

## Agent

Run from source:

```bash
cd openflare-agent
go run ./cmd/agent -config /path/to/agent.json
```

Compile:

```bash
cd openflare-agent
go build -o openflare-agent ./cmd/agent
```

Run tests:

```bash
cd openflare-agent
GOCACHE=/tmp/openflare-go-cache go test ./...
```

## Relay (Server-side)

Run from source:

```bash
cd openflare-relay
go run ./cmd -config /path/to/relay.json
```

Compile:

```bash
cd openflare-relay
go build -o openflare-relay ./cmd
```

## OpenFlared (Client-side)

Run from source:

```bash
cd openflared
go run ./cmd -config /path/to/flared.json
```

Compile:

```bash
cd openflared
go build -o openflared ./cmd
```

## Install Agent

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --agent-token YOUR_AGENT_TOKEN
```

## Uninstall Agent

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-agent.sh | bash
```

## Swagger

Regenerate Swagger documentation:

```bash
go install github.com/swaggo/swag/cmd/swag@v1.16.4
cd openflare-server
swag init -g main.go -o docs
```

## Docs

Local preview:

```bash
cd docs
pnpm dev
```

Build:

```bash
cd docs
pnpm build
```
