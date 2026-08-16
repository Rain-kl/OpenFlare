# Commands & Scripts

You will learn: common start, build, test, install, and uninstall commands for the OpenFlare Server, admin frontend, Agent, Relay, OpenFlared, Swagger, and the docs site.

> All commands run at the **repo root** unless noted otherwise.

## Server

Source startup:

```bash
cp config.example.yaml config.yaml
go run main.go all
```

Split processes:

```bash
go run main.go api          # HTTP API only
go run main.go worker       # Asynq Worker only
go run main.go scheduler    # scheduled tasks only
```

Build the binary:

```bash
make build-backend
# output: bin/openflare-server
```

Tests:

```bash
GOCACHE=/tmp/openflare-go-cache go test ./...
```

Quality gate:

```bash
make code-check
```

Auto-format backend Go source (organize imports) and frontend source:

```bash
make format
```

This command uses `goimports` to organize backend Go imports and the repo-pinned Prettier version to format `frontend/` source; build artifacts, dependencies, public static assets, and lock files are ignored.

## Frontend

Dev:

```bash
cd frontend
pnpm install
pnpm dev
```

Build the embedded artifact (hosted by the Go Server):

```bash
cd frontend
pnpm build:embed
# or at repo root: make build-embedded
```

Checks:

```bash
cd frontend
pnpm lint
pnpm tsc --noEmit --jsx preserve
pnpm check:i18n
```

## Agent

Source run:

```bash
go run ./cmd/agent -config /path/to/agent.json
```

Build:

```bash
make build-agent
# or: go build -o bin/openflare-agent ./cmd/agent
```

Tests:

```bash
GOCACHE=/tmp/openflare-go-cache go test ./internal/apps/agent/...
```

## Relay

Source run:

```bash
go run ./cmd/relay -config /path/to/relay.json
```

Build:

```bash
make build-relay
# or: go build -o bin/openflare-relay ./cmd/relay
```

## OpenFlared (Tunnel client)

Source run:

```bash
go run ./cmd/flared -config /path/to/flared.json
```

Build:

```bash
make build-flared
# or: go build -o bin/flared ./cmd/flared
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

Regenerate the Swagger docs:

```bash
make swagger
```

Access: `http://localhost:3000/api/swagger/index.html` (default `api_prefix` `/api`; only mounted in non-production)

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
