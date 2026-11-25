# Commands & Scripts

You will learn: command-line flags, build targets, and quality gate commands for OpenFlare Server, OpenFlare Edge, Relay, OpenFlared, the admin frontend, Swagger, and the documentation site.

> All commands run at the **repository root** unless noted otherwise.

---

## Control Plane Service (OpenFlare Server)

### 1. Subcommands and CLI Flags

Built binary outputs to `bin/openflare-server` (or run directly via Go source: `go run ./cmd/server`):

```bash
# Print version information
./bin/openflare-server --version

# Launch in fused single-process mode (starts API + Worker + Scheduler by default)
./bin/openflare-server all

# Launch individual service processes
./bin/openflare-server api          # HTTP API service only
./bin/openflare-server worker       # Asynq async task worker only
./bin/openflare-server scheduler    # Scheduled cron task scheduler only

# Reset password for a specific account
./bin/openflare-server reset-passwd --user admin --password "NewPassword123!"
```

`reset-passwd` parameters:
- `--user <string>`: Target username; prompts interactively if omitted.
- `--password <string>`: New password; generates a secure 16-character random password if omitted.

Specify a custom configuration file path:

```bash
CONFIG_PATH=/etc/openflare/config.yaml ./bin/openflare-server all
```

### 2. Build and Test Targets

```bash
# Build backend server binary (output: bin/openflare-server)
make build-backend

# Build single binary embedding static frontend assets
make build-embedded

# Run unit and integration tests
GOCACHE=/tmp/openflare-go-cache go test ./...
```

---

## Edge Gateway (OpenFlare Edge)

OpenFlare Edge is implemented in Rust on top of Cloudflare OpenFlare Edge. The binary is named `openflare-edge`.

### 1. Subcommands and Flags

```bash
# Run edge proxy service
openflare-edge run \
  --server-url "https://panel.example.com" \
  --token-type "node" \
  --token-file "/etc/openflare-edge/node.token" \
  --state-dir "/var/lib/openflare-edge" \
  --http-listen "0.0.0.0:80" \
  --tls-listen "0.0.0.0:443"

# Launch in upgrade mode (verifies and restores active local snapshot)
openflare-edge run --upgrade \
  --server-url "https://panel.example.com" \
  --token-type "node" \
  --token-file "/etc/openflare-edge/node.token" \
  --state-dir "/var/lib/openflare-edge" \
  --http-listen "0.0.0.0:80" \
  --tls-listen "0.0.0.0:443"

# Print supported controller configuration sync protocol version
openflare-edge config-sync-version

# Print supported auto-update protocol version
openflare-edge self-update-version

# Output WAF / CRS attack rule catalog (JSON format)
openflare-edge attack-rule-catalog
```

`openflare-edge run` flag details:

| Flag | Type | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `--server-url` | `string` | yes | none | OpenFlare control-plane address (e.g. `https://panel.example.com`) |
| `--token-type` | `string` | no | `discovery` | Auth token type: `discovery` for initial registration; `node` for dedicated node credential |
| `--token-file` | `path` | yes | none | Local path to file containing auth token |
| `--state-dir` | `path` | yes | none | Persistent storage and snapshot state directory (e.g. `/var/lib/openflare-edge`) |
| `--http-listen` | `string` | no | `127.0.0.1:8080` | HTTP listening socket address (production systemd uses `0.0.0.0:80`) |
| `--tls-listen` | `string` | no | empty | HTTPS/TLS listening socket address (production systemd uses `0.0.0.0:443`) |
| `--upgrade` | `bool` | no | `false` | Upgrade mode; requires active configuration restore if snapshot exists |

### 2. Build and Test Targets

```bash
# Compile OpenFlare Edge release binary
make build-edge
# Output located at: edge/target/release/openflare-edge

# Run Rust test suite
cargo test --manifest-path edge/Cargo.toml --locked
```

---

## Traffic Relay (Relay)

Relay handles reverse proxy traffic distribution for tunnels, managing the embedded `frps` daemon.

### 1. Execution and Flags

```bash
# Run from source
go run ./cmd/relay -config /path/to/relay.json

# Run binary
./bin/openflare-relay -config /etc/openflare-relay/relay.json
```

Accepts only `-config <PATH>` (default `./relay.json`). Supports environment variable overrides:
- `OPENFLARE_SERVER_URL`: Control-plane URL
- `OPENFLARE_AGENT_TOKEN`: Relay node auth token
- `OPENFLARE_DISCOVERY_TOKEN`: Global discovery token
- `OPENFLARE_NODE_NAME`: Node name
- `OPENFLARE_NODE_IP`: Public egress IP
- `OPENFLARE_DATA_DIR`: Runtime data directory
- `OPENFLARE_FRPS_PATH`: System `frps` binary path
- `LOG_LEVEL`: Log level (default `info`)

### 2. Build Target

```bash
make build-relay
# Output: bin/openflare-relay
```

---

## Tunnel Client (OpenFlared)

OpenFlared is the tunnel client daemon running on user origin servers, establishing encrypted tunnels to relay nodes via `frpc`.

### 1. Execution and Flags

```bash
# Run from source
go run ./cmd/flared -config /path/to/flared.json

# Run binary
./bin/flared -config /etc/openflared/flared.json
```

Accepts only `-config <PATH>` (default `./flared.json`). Supports environment variable overrides:
- `OPENFLARE_SERVER_URL`: Control-plane URL
- `OPENFLARE_TUNNEL_TOKEN`: Dedicated tunnel auth token
- `OPENFLARE_DATA_DIR`: Runtime data directory
- `OPENFLARE_FRPC_PATH`: System `frpc` binary path
- `LOG_LEVEL`: Log level (default `info`)

### 2. Build Target

```bash
make build-flared
# Output: bin/flared
```

---

## Consolidated Builds and Quality Gates

### 1. One-Click Full Build

```bash
# Build all project binaries (Server, Edge, Relay, Flared)
make build-all
```

### 2. Automated Formatting

```bash
make format
```
Runs `goimports` to clean up and organize Go imports and runs Prettier across all frontend code under `frontend/`.

### 3. Quality Gate (Code Check)

```bash
make code-check
```
Executes verification steps in order:
1. **Architectural Guard**: Validates that `internal/model` does not import `db.DB` or Redis;
2. **Attack Rule IDs**: Checks WAF rule ID consistency;
3. **Go Linter**: Runs `golangci-lint run`;
4. **Frontend Checks**: Verifies PoW asset consistency, validates i18n fragment completeness, runs TypeScript check (`tsc --noEmit`), and executes ESLint with zero-warning tolerance.

---

## Swagger Specification Generation

Whenever API doc annotations are modified, regenerate OpenAPI / Swagger specification files:

```bash
make swagger
# Output files: api/swagger/swagger.json, api/swagger/swagger.yaml
```

Access the interactive UI in non-production environments at: `http://localhost:3000/api/swagger/index.html`.

---

## Frontend & Documentation Development

### 1. Admin Frontend (`frontend/`)

```bash
cd frontend

# Install dependencies
pnpm install

# Start local dev server with HMR
pnpm dev

# Build standalone embedded static assets (outputs to frontend/out)
pnpm build:embed

# Run linters and typechecks
pnpm lint
pnpm tsc --noEmit --jsx preserve
pnpm check:i18n
```

### 2. Documentation Site (`docs/`)

```bash
cd docs

# Install dependencies
pnpm install

# Start local live preview server
pnpm dev

# Build static documentation bundle
pnpm build
```
