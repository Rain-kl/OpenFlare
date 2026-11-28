# Commands & Scripts

Command-line flags, subcommands, build targets, and quality gate commands for `openflare-server`, `openflare` CLI, `openflare-edge`, `openflare-dns`, `openflare-relay`, `openflared`, the frontend, Swagger, and the documentation site.

> All commands run at the **repository root** unless noted otherwise.

---

## Control Plane Service (`openflare-server`)

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

# Dry-run and re-encrypt stored credentials (before rotating app.session_secret)
./bin/openflare-server rotate-credentials --new-secret-file ~/.openflare-new-session-secret
./bin/openflare-server rotate-credentials --new-secret-file ~/.openflare-new-session-secret --apply
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

## Operations CLI (`openflare`)

Interactive and scriptable operator CLI (`cmd/openflare`) for sites, zones, certificates, WAF rules, nodes, and probes. See [CLI and MCP](../guide/automation.md) for usage examples.

```bash
# Build the operations CLI binary
go build -o bin/openflare ./cmd/openflare

# Authenticate against a control plane and inspect available commands
openflare auth login --server https://openflare.example.com
openflare connection info
openflare operations list
```

---

## Edge Gateway (`openflare-edge`)

OpenFlare Edge is built in Rust with Cloudflare Pingora and ships as `openflare-edge`.

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

# Protocol versions, rule catalog, and isolated network probe worker
openflare-edge config-sync-version
openflare-edge self-update-version
openflare-edge attack-rule-catalog
openflare-edge probe-worker --idle-timeout-secs 300
```

`openflare-edge run` flag details:

| Flag | Type | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `--server-url` | `string` | yes | none | OpenFlare control-plane URL |
| `--token-type` | `string` | no | `discovery` | Auth token type: `discovery` for initial enrollment; `node` for dedicated node credential |
| `--token-file` | `path` | yes | none | Local path to auth token file |
| `--state-dir` | `path` | yes | none | Persistent storage and snapshot state directory (e.g. `/var/lib/openflare-edge`) |
| `--http-listen` | `string` | no | `127.0.0.1:8080` | HTTP listening socket (`0.0.0.0:80` in systemd services) |
| `--tls-listen` | `string` | no | empty | HTTPS/TLS listening socket (`0.0.0.0:443` in systemd services) |
| `--upgrade` | `bool` | no | `false` | Upgrade mode; requires restoring existing active snapshot |

### 2. Build, Lint, and Test Targets

```bash
# Build OpenFlare Edge release binary (edge/target/release/openflare-edge)
make build-edge

# Format, lint, unit tests, and runtime smoke verification
cargo fmt --manifest-path edge/Cargo.toml --check
cargo clippy --manifest-path edge/Cargo.toml --all-targets -- -D warnings
cargo test --manifest-path edge/Cargo.toml --locked
python3 edge/tests/runtime_smoke.py
```

---

## Authoritative SmartDNS (`openflare-dns`)

`openflare-dns` is implemented in Rust under `dns/` and runs either as a standalone authoritative server or an external process supervised by Server.

### 1. Subcommands and Flags

```bash
# Run standalone SmartDNS authoritative server
openflare-dns serve \
  --controller-url "https://panel.example.com" \
  --discovery-token "<DISCOVERY_TOKEN>" \
  --listen "0.0.0.0:53" \
  --geoip-dir "/var/lib/openflare-dns/geoip" \
  --cache-dir "/var/lib/openflare-dns"

# Run handler throughput benchmark
openflare-dns bench
```

See [Configuration Reference](./configuration.md#openflare-dns-flags--env-vars) for all `openflare-dns serve` flags and environment variable overrides.

### 2. Build and Test Targets

```bash
# Build openflare-dns (output: bin/openflare-dns)
make build-dns

# Run SmartDNS unit tests
make test-dns
```

---

## Traffic Relay (`openflare-relay`)

`openflare-relay` is the Rust public tunnel relay server implemented under `tunnel/relay`.

### 1. Execution and Flags

```bash
openflare-relay \
  --server-url "https://panel.example.com" \
  --bootstrap-token "<RELAY_TOKEN>" \
  --state-dir "/var/lib/openflare-relay" \
  --public-addr "203.0.113.10" \
  --transport-port 7443 \
  --ws-port 7444 \
  --gateway-port 8443
```

See [Configuration Reference](./configuration.md#relay-flags--env-vars) for all CLI flags and `OPENFLARE_RELAY_*` environment variables.

### 2. Build and Test Targets

```bash
# Output: bin/openflare-relay
make build-relay

# Run tunnel workspace tests (shared with openflared)
make test-tunnel
```

---

## Tunnel Client (`openflared`)

`openflared` is the Rust private-network connector daemon implemented under `tunnel/flared`.

### 1. Execution and Flags

```bash
openflared \
  --server-url "https://panel.example.com" \
  --token-file "/var/lib/openflared/bootstrap-token" \
  --state-dir "/var/lib/openflared"
```

See [Configuration Reference](./configuration.md#openflared-flags--env-vars) for CLI flags and environment variables.

### 2. Build Target

```bash
# Output: bin/openflared
make build-flared
```

---

## Consolidated Builds and Quality Gates

### 1. One-Click Full Build

```bash
# Build all binaries (Server, Edge, Relay, OpenFlared, SmartDNS)
make build-all
```

### 2. Automated Formatting

```bash
make format
```

Runs `goimports` to organize Go imports and Prettier across `frontend/`.

### 3. Quality Gate (`make code-check`)

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

Whenever API annotations change, regenerate OpenAPI / Swagger specs:

```bash
make swagger
# Output files: api/swagger/swagger.json, api/swagger/swagger.yaml
```

Interactive console in non-production environments: `http://localhost:3000/api/swagger/index.html`.

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
