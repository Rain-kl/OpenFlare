# Upgrade & maintenance guide

Before performing software upgrades or maintenance operations, back up your primary database and verify that the active site configuration has successfully been applied to all edge nodes.

---

## Upgrading from v3.6.x to v4.0.0

OpenFlare v4.0.0 replaces the legacy OpenResty architecture with the OpenFlare Edge-based OpenFlare Edge gateway. Record each node's dedicated token before upgrading. Legacy OpenResty performance settings must be reconfigured in the admin console under "Performance policies".

### Migration steps

1. Upgrade control plane server: Upgrade the Server to v4.0.0+. Ensure database migrations complete automatically and that the Server generates a valid OpenFlare Edge site configuration snapshot.
2. Migrate edge nodes progressively:
   - Stop and disable legacy Agent and OpenResty services to release TCP ports `80` and `443`:
     ```bash
     sudo systemctl stop openflare-agent openresty 2>/dev/null || true
     sudo systemctl disable openflare-agent openresty 2>/dev/null || true
     ```
   - Deploy the new gateway following the [OpenFlare Edge deployment guide](./openflare-edge.md). Reusing the legacy node's dedicated token preserves the node ID, metric history, and domain bindings.
3. Verify and canary rollout:
   - Confirm the new node transitions to online status and successfully loads the active snapshot.
   - Test inbound HTTP and HTTPS request forwarding through the node before migrating subsequent nodes.

---

## Server control plane upgrades

### 1. Database backup

Always back up the primary database before pulling new release images:

```bash
# PostgreSQL backup
docker compose exec postgres pg_dump -U openflare openflare > openflare_backup_$(date +%Y%m%d).sql

# SQLite backup
cp ./openflare-data/openflare.db ./openflare-data/openflare_backup_$(date +%Y%m%d).db
```

### 2. Docker Compose container upgrades

Pull updated release images and restart services:

```bash
docker compose pull
docker compose up -d
docker compose ps
docker compose logs -f openflare
```

Upon boot, the Server automatically runs pending database migrations (supporting both PostgreSQL and SQLite) and recompiles active site snapshots based on current database state.

### 3. Source deployment upgrades

```bash
git fetch --tags
git checkout <target-version-tag>

# Recompile frontend static bundle
cd frontend && pnpm install && pnpm build:embed && cd ..

# Restart Server
go run ./cmd/server all
```

---

## OpenFlare Edge node updates

### Option 1: Remote console update

1. Navigate to Node Management in the admin console and open the target edge node details.
2. Click "Update Edge Node", select the target stable or preview release tag, and submit the request.
3. The control plane dispatches the instruction on the next heartbeat interval (typically within 10 seconds). The node downloads the package, verifies the SHA-256 checksum, drains in-flight connections, and performs a graceful restart.
4. If the node is currently offline, the task remains queued until the node reconnects.

> [!TIP]
> **Automatic updates**: Enable "Auto-update Node" within the node edit dialog. Once healthy, the node checks for newer stable releases on GitHub every 6 hours. Manual update requests take precedence over automated checks.

### Option 2: In-place host script update

On a Linux host, run the updater script. It detects the existing installation paths, replaces `/opt/openflare-edge/current`, and restarts the systemd service:

```bash
# Download and execute updater
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-edge.sh | sudo bash

# Update to a specific release tag
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-edge.sh | sudo bash -s -- --version edge-v0.1.3
```

### Option 3: Docker container node update

Pull the latest edge container image and recreate the container (the volume preserves node identity and cached state):

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

---

## Relay and tunnel client updates

Update tasks can be scheduled or auto-updates toggled directly within the respective node details pages in the admin console. For Docker-managed Relay and OpenFlared instances:

```bash
# Update Relay
docker pull ghcr.io/rain-kl/openflare-relay:latest
docker stop openflare-relay && docker rm openflare-relay
# Rerun original docker run command...

# Update OpenFlared
docker pull ghcr.io/rain-kl/openflared:latest
docker stop openflared && docker rm openflared
# Rerun original docker run command...
```

---

## Rollback and troubleshooting

If an upgraded component fails to start:

1. Inspect edge service logs:
   ```bash
   sudo journalctl -u openflare-edge -n 100 --no-pager
   ```
2. Snapshot preservation: OpenFlare Edge preserves `/var/lib/openflare-edge/snapshot.json` across failed startup attempts, allowing continued service delivery using the last valid snapshot while troubleshooting.
3. Roll back edge version: Run the installer script with a previous stable version tag:
   ```bash
   sudo bash scripts/install-openflare-edge.sh --version edge-v0.1.2 ...
   ```
