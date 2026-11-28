# Troubleshooting

Diagnose common control-plane, `openflare-edge`, TLS, caching, analytics, and tunnel connectivity issues.

## Quick Symptom Lookup

| Symptom | First Check |
| :--- | :--- |
| Node remains offline | Control-plane reachability, active token, outbound firewall rules, and `/var/lib/openflare-edge/identity.json` |
| Route edits fail to reach edge | Server logs, reverse-proxy WebSocket pass-through, and polling sync interval |
| HTTP `502` / `504` | Connectivity from edge to origin, forwarded `Host` header, origin scheme, and timeouts |
| TLS handshake error or browser warning | Route certificate binding, ACME validation path reachability, and origin TLS verify mode |
| Edge cache never hits | Origin `Cache-Control: no-store` or `Set-Cookie` headers, plus route cache rules |
| Access analytics empty | Clock sync (`NTP`) between Server and Edge, edge spool shipping, and logstore DB health |
| Redirect loop (`ERR_TOO_MANY_REDIRECTS`) | Conflicting HTTPS redirects between Edge and origin |

## Node Registration Failures & Offline Nodes

1. Verify control-plane reachability from the edge host: `curl -I https://your-openflare-server.com/api/v1/health`.
2. Inspect `/var/lib/openflare-edge/bootstrap-token`, `identity.json`, and `ready.json`, then check node logs via [Deploy OpenFlare Edge](../deployment/openflare-edge.md).
3. For discovery-token enrollment, confirm outbound network access and clean heartbeats in systemd/container logs.

## Config Changes Do Not Propagate

- Control-plane pushes run over `/api/v1/agent/ws`; any reverse proxy in front of Server must forward WebSocket `Upgrade` and `Connection` headers.
- Compare `current_version` against target version in **Nodes** or check `config_sync: synced` via `openflare node health`.

## Origin `502` / `504` or Redirect Loops

- **TLS & redirects**: Disable strict origin verification (or supply a CA) for self-signed origins, and ensure only one layer (Edge or origin) forces HTTPS redirects.
- **Host & timeouts**: Forward the expected `Host` header when origins enforce virtual hosting; tune read timeouts and connection reuse in [Performance Policies](./performance-policies.md).

## Edge cache hit rate anomalies

- **Low or zero hits**: Confirm route **Edge Cache** is enabled; origins sending `Cache-Control: private`, `no-store`, or `Set-Cookie` bypass caching by default.
- **Premature eviction or cache poisoning**: Review cache TTLs, query-string normalization toggles, and path exclusions on dynamic endpoints.

## Access analytics empty

- Ensure NTP clock synchronization across Server and Edge; `openflare-edge` buffers logs in a local disk spool during outages and replays them on recovery (see [OpenFlare Edge Architecture Design](../design/pingora-edge-design.md#53-access-log-buffering-and-spooling)).
- Confirm the active logstore database (ClickHouse, PostgreSQL, or SQLite) is healthy and accepting writes.

## Admin Password Reset & Login Issues

- **Reset admin password** interactively on the Server host or container:

```sh
./bin/openflare-server reset-passwd
# Docker Compose:
docker compose exec openflare openflare-server reset-passwd
```

- **Auth / Captcha failures**: Verify Captcha keys and domain allowlists under **System settings** > **Security**, and confirm identical `session_secret` values across all Server replicas.
- **Upload failures**: Check storage directory permissions, disk space, and reverse-proxy `client_max_body_size` limits.
