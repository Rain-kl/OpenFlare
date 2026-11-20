# Deploy OpenFlare Edge

OpenFlare Edge is the Pingora-based edge gateway introduced in OpenFlare v4.0.0. Upon startup, the edge node connects to the OpenFlare Server control plane to perform node registration, heartbeat reporting, configuration synchronization, Pages asset distribution, and traffic proxying. For legacy OpenResty nodes, refer to the [Upgrade & maintenance guide](./upgrade.md) for migration steps.

## Releases and version management

OpenFlare Edge uses dedicated `edge-v*` release tags, decoupled from the release cycles of the Server, Relay, and OpenFlared. The Edge build workflow generates Linux release archives for `amd64` and `arm64`:

- `openflare-edge_edge-v0.1.3_linux_amd64.tar.gz`
- `openflare-edge_edge-v0.1.3_linux_arm64.tar.gz`

Each release archive contains the `openflare-edge` binary, `VERSION`, `SHA256SUMS`, license files, and installer, updater, uninstaller, and GeoIP download scripts.

- Stable releases: Pushing a tag formatted as `edge-v<major>.<minor>.<patch>` triggers the release build. For manual builds, run the **Build OpenFlare Edge** GitHub Actions workflow and specify the target Edge tag.
- Version injection: The build workflow injects the version string from the Git release tag via the `OPENFLARE_EDGE_VERSION` environment variable at compile time. Both `openflare-edge --version` and the version reported to the control plane reflect this value (e.g. tag `edge-v0.1.3` produces binary version `0.1.3`). Releasing does not require manually modifying the `version` field in `Cargo.toml`; builds use `--locked` to ensure dependencies strictly match `Cargo.lock`.
- Canary previews: Pushing to the `canary` branch triggers an Edge preview build. The version uses the next patch version combined with the workflow run number (e.g., `edge-v0.1.4-canary.42` based on `edge-v0.1.3`). The admin console discovers this version under "Check Previews"; default installations and automatic updates only consume stable releases.
- Release indexing: The GitHub Releases `Latest` marker points to the Server release. Edge installation scripts, admin console update checks, and automatic node updates query `edge-v*` tags independently.

Docker container images are published to `ghcr.io/rain-kl/openflare-edge`, based on Alpine Linux with a statically linked musl binary and system CA certificates, supporting Linux `amd64` and `arm64`. Stable releases update the `:latest` tag, and Canary builds update the `:canary` tag.

## Node installation

Configuration synchronization protocol v1 requires OpenFlare Edge `edge-v0.1.2` or later. The installer validates protocol compatibility against the target release before modifying local services.

In the OpenFlare admin console under Node Management, open node details and click "Edge Node Deployment" to view pre-filled installation commands containing your server URL and node token.

### Linux host installation

Run the installation script:

```bash
# Download and run remotely
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-edge.sh -o install-openflare-edge.sh
sudo bash install-openflare-edge.sh \
  --server-url https://control.example.com \
  --token-type node \
  --token <node-token> \
  --http-listen 0.0.0.0:80 \
  --tls-listen 0.0.0.0:443
```

When working from a cloned repository or an extracted release archive, execute the local script directly:

```bash
sudo bash scripts/install-openflare-edge.sh \
  --server-url https://control.example.com \
  --token-type node \
  --token <node-token> \
  --http-listen 0.0.0.0:80 \
  --tls-listen 0.0.0.0:443
```

Installation workflow:
1. Detects system architecture (`amd64` or `arm64`) and queries GitHub Releases for the latest compatible stable release (or a specific version via `--version <tag>`).
2. Downloads the archive and verifies its SHA-256 checksum, then unpacks files into the target directories.
3. Initializes standard file paths:
   - Executable binary: `/opt/openflare-edge/current` (points to the active release binary)
   - Node identity credentials: `/var/lib/openflare-edge/identity.json`
   - Runtime configuration snapshot: `/var/lib/openflare-edge/snapshot.json`
   - Pages static assets: `/var/lib/openflare-edge/pages/`
   - GeoIP databases: `/var/lib/openflare-edge/GeoLite2-Country.mmdb` and `GeoLite2-City.mmdb`
   - Access log buffer queue: `/var/lib/openflare-edge/access-logs.jsonl`
4. Downloads and validates the GeoIP databases (installation aborts if download fails).
5. Creates and starts the systemd service `openflare-edge.service`.

Verify version and service status:

```bash
/opt/openflare-edge/current --version
sudo systemctl status openflare-edge
sudo journalctl -u openflare-edge -f
```

### Docker container deployment

Copy the Docker command from the node details page in the admin console, or run:

```bash
docker run -d --pull always --name openflare-edge --restart unless-stopped \
  -p 80:8080 -p 443:8443 \
  -v openflare-edge-data:/var/lib/openflare-edge \
  -e OPENFLARE_EDGE_SERVER_URL=https://control.example.com \
  -e OPENFLARE_EDGE_TOKEN_TYPE=node \
  -e OPENFLARE_EDGE_TOKEN='<node-token>' \
  ghcr.io/rain-kl/openflare-edge:latest
```

Container operational details:
- Runs as non-root user (UID 10001 / GID 10001). The container listens internally on port `8080` (HTTP) and `8443` (HTTPS), mapped to host ports `80` and `443`.
- Node identity, configuration snapshots, and Pages assets persist in the `openflare-edge-data` named volume (mounted at `/var/lib/openflare-edge`).
- Use `OPENFLARE_EDGE_TOKEN_FILE` to supply a read-only mounted token file readable by UID 10001, avoiding plaintext token exposure in environment variables.
- Container health checks pass once valid configuration is loaded and listeners are active. View logs with `docker logs -f openflare-edge`.

## Security and privilege hardening

The Linux installer creates a dedicated system user `openflare-edge` and configures the systemd service to run under this user. The service requests only `CAP_NET_BIND_SERVICE` (allowing non-root binding to ports 80 and 443) and enforces systemd hardening directives:

- `NoNewPrivileges=yes`
- `PrivateTmp=yes`
- `ProtectSystem=strict`
- `ProtectHome=yes`
- `ProtectKernelTunables=yes`
- `ProtectKernelModules=yes`
- `ProtectControlGroups=yes`
- `RestrictSUIDSGID=yes`
- `LockPersonality=yes`
- `MemoryDenyWriteExecute=yes`
- `RestrictNamespaces=yes`
- `SystemCallFilter=@system-service`

The state directory `/var/lib/openflare-edge` is the only writable path granted to the service, isolating tokens, identity files, configuration snapshots, GeoIP databases, Pages static bundles, and log queues.

## Process architecture and fault tolerance

The `openflare-edge` binary implements a dual-process architecture comprising a control plane supervisor and a Pingora data plane worker:

1. Control plane supervisor: Manages long-lived connections, heartbeats, configuration snapshot downloads, Pages static asset synchronization, metric telemetry, and worker process lifecycle supervision.
2. Pingora data plane worker: Handles listener ports, TLS handshakes, WAF rule evaluations, cache lookups, and reverse proxy forwarding to origin servers.

Fault tolerance mechanisms:
- Process-level supervision: The supervisor checks data plane worker liveness every second. If the worker exits unexpectedly, the supervisor immediately spawns a new worker. If the supervisor process crashes, systemd restarts it (`Restart=always`, 5-second interval) to re-attach supervision.
- Local snapshot recovery: On reboot, the node loads the latest valid configuration from `/var/lib/openflare-edge/snapshot.json` before querying the control plane. If the control plane is unreachable or experiencing a network partition, the data plane continues serving traffic using local snapshots.
- Reliable access log reporting: The data plane appends access log entries to `/var/lib/openflare-edge/access-logs.jsonl` using file locks. The supervisor reads batches during heartbeat intervals and reports them to the Server, truncating acknowledged records. The queue is capped at 2048 entries to prevent unbounded disk or memory consumption during extended network outages.

## Upgrades and maintenance

### In-place binary upgrade

For standard maintenance, use the updater script. It detects existing installation paths and state directories, replaces the binary, and restarts the service without modifying systemd unit configurations, node tokens, snapshot state, or GeoIP databases:

```bash
# Download and execute updater
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-edge.sh | sudo bash

# Or execute locally within the repository
sudo bash scripts/update-openflare-edge.sh
```

Upgrade characteristics:
- Validates the downloaded archive's SHA-256 checksum and protocol compatibility.
- Atomically replaces the `/opt/openflare-edge/current` binary.
- Starting with `edge-v0.1.2`, Pingora graceful drain timeout is tuned to 2 seconds for clean connection draining before restart.
- Validates startup success via systemd. If the new binary fails validation or fails to start, the script logs an error while preserving the previous operational state.

### Docker container upgrade

Pull the latest image and recreate the container:

```bash
docker pull ghcr.io/rain-kl/openflare-edge:latest
docker stop openflare-edge && docker rm openflare-edge
docker run -d --pull always --name openflare-edge --restart unless-stopped \
  -p 80:8080 -p 443:8443 \
  -v openflare-edge-data:/var/lib/openflare-edge \
  -e OPENFLARE_EDGE_SERVER_URL=https://control.example.com \
  -e OPENFLARE_EDGE_TOKEN_TYPE=node \
  -e OPENFLARE_EDGE_TOKEN='<node-token>' \
  ghcr.io/rain-kl/openflare-edge:latest
```

Reusing the `openflare-edge-data` named volume preserves node identity and cached snapshots across container recreation.

### Uninstallation

Stop and remove the service and binaries:

```bash
# Download and execute uninstaller
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-openflare-edge.sh | sudo bash -s --

# Or execute locally within the repository
sudo bash scripts/uninstall-openflare-edge.sh
```

To permanently purge node identity, configuration snapshots, Pages assets, and GeoIP databases, append `--remove-state`:

```bash
sudo bash scripts/uninstall-openflare-edge.sh --remove-state
```
