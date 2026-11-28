# Upgrades & maintenance

Back up the primary relational database and verify site configuration sync before upgrading components.

## Migrating from v3.6.x to v4.0.0

v4.0.0 replaces the legacy OpenResty runtime with the Pingora-based `openflare-edge` gateway. Record each node's dedicated token beforehand and reconfigure legacy performance settings under **Performance policies**.

1. Upgrade OpenFlare Server to v4.0.0+ and wait for automatic migrations and snapshot compilation.
2. Stop legacy services on each edge host and install `openflare-edge` following [Deploy OpenFlare Edge](./openflare-edge.md) using the original node token (preserving node ID, metrics, and domain bindings):

   ```bash
   sudo systemctl stop openflare-agent openresty 2>/dev/null || true
   sudo systemctl disable openflare-agent openresty 2>/dev/null || true
   ```

3. Verify HTTP/HTTPS traffic on each migrated node before moving to the next host.

## Upgrading to configuration contract v4 (in-memory TLS keys)

Contract v4 (`tls-memory-keys-v1`) removes TLS private keys from edge disk snapshots:

1. Upgrade Server and all `openflare-edge` nodes together to versions supporting `POST /api/v1/agent/tls/keys`.
2. Migration `202611230002_scrub_config_version_private_keys.sql` automatically strips legacy private keys from stored snapshots and triggers recompilation.
3. During control-plane outages, cold-restarted edge nodes recover HTTP routes immediately from `snapshot.json` and hydrate HTTPS keys into memory automatically once connectivity returns, with no restart needed.

## OpenFlare Server upgrades

### Back up the database

```bash
# PostgreSQL
docker compose exec postgres pg_dump -U openflare openflare > openflare_backup_$(date +%Y%m%d).sql

# SQLite
cp ./openflare-data/openflare.db ./openflare-data/openflare_backup_$(date +%Y%m%d).db
```

### Update the service

```bash
# Docker Compose
docker compose pull && docker compose up -d

# Source build
git fetch --tags && git checkout <TARGET_VERSION_TAG>
make build-embedded && ./bin/openflare-server all
```

On startup, the Server applies pending goose migrations and recompiles site snapshots.

## OpenFlare Edge updates

- **Console remote update**: Open **Nodes** > node details and click **Update Edge Node** for a stable or Canary tag, or enable **Auto-update Node** (checks stable tags every 6 hours).
- **Host script update**: Preserves tokens, snapshots, and GeoIP databases, and adds `CAP_NET_RAW` if missing:

  ```bash
  curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-edge.sh | sudo bash
  # Append canary for prereleases, or --version edge-vX.Y.Z to pin
  ```

- **Docker update**: Pull `:latest` (or `:canary`) and recreate the container while keeping the `openflare-edge-data` volume.

## OpenFlare DNS updates

- **Console remote update**: Linux systemd hosts with `openflare-dns-update.path` enabled support single or batch updates (with automatic rollback on failed readiness) under **DNS** > **SmartDNS** > **Authoritative Instances**.
- **Host script update**: Preserves `identity.json`, `last-snapshot.bin`, and GeoIP files:

  ```bash
  curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-dns.sh | sudo bash
  # Append canary for prereleases, or --version dns-vX.Y.Z to pin
  ```

- **Docker update**: Pull `:latest` (or `:canary`) and recreate the container keeping the `openflare_dns_data` volume.

## OpenFlare Relay and OpenFlared updates

- **Console remote update**: On non-root Linux systemd hosts, trigger staged updates or enable 6-hour stable checks (`next -> trial -> active`) from Relay or Tunnel details under **Tunnels**.
- **Host script update**:

  ```bash
  curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-relay.sh | sudo bash
  curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflared.sh | sudo bash
  # Append canary for prereleases, or --version tunnel-vX.Y.Z to pin
  ```

- **Docker update**: Pull the latest image and recreate the container with the existing volume and environment variables.

## Rollbacks and troubleshooting

If a component fails to start after an update, inspect service logs:

```bash
sudo journalctl -u openflare-edge -n 100 --no-pager
sudo journalctl -u openflare-dns -n 100 --no-pager
sudo journalctl -u openflare-relay -n 100 --no-pager
sudo journalctl -u openflared -n 100 --no-pager
```

Each updater preserves existing healthy state or restores the previous binary automatically on startup failure. To pin or roll back manually, rerun the component updater with `--version <TAG>`; existing tokens and state directories stay intact.
