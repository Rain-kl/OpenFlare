# Troubleshooting

You will learn: how to troubleshoot OpenFlare Server, database, login, Pingora Edge, and configuration synchronization issues by symptom.

First determine which layer the issue occurs in: browser, Server, database, Pingora Edge, origin server, or DNS. Nodes fetch new configuration manifests upon receiving WebSocket notifications and re-synchronize periodically through node heartbeats.

## Quick Identification

| Symptom | Look Here First |
| --- | --- |
| Admin panel inaccessible | Server container or process logs, port listening status |
| Login failure | Default credentials, Session Cookie, Server logs |
| Data cannot be saved | Database connection, SQLite file permissions, PostgreSQL health |
| Agent offline | Agent logs, Token, Server address, network connectivity |
| Node not updated after saving | Node heartbeat, apply records, controller snapshot compilation logs |
| Pingora config apply failure | Apply records, Edge logs, certificates, origin addresses, port conflicts |
| Access analytics empty | Node heartbeat, Edge access log queue, and controller logs |
| Static assets never hit cache | Global/site cache toggle, policy file extensions, configuration apply status, access log `cache_status`, origin Set-Cookie / Cache-Control |

## Server Won't Start

1. Check logs:

```bash
docker compose logs -n 200 openflare
```

For source runs, inspect terminal output.

2. Check port usage:

```bash
lsof -i :3000
```

3. If using PostgreSQL, confirm database health:

```bash
docker compose ps postgres
docker compose logs -n 100 postgres
```

4. If using SQLite, confirm the database directory is writable:

```bash
ls -ld "$(dirname /path/to/openflare.db)"
```

Common causes:

| Log or Symptom | Handling |
| --- | --- |
| DB connection failed | Check `DB_HOST`, `DB_PORT`, `DB_USERNAME`, `DB_PASSWORD`, `DB_NAME`, `DB_SSL_MODE` consistency |
| SQLite cannot create file | Check that the directory containing `SQLITE_PATH` exists and is writable |
| Port occupied | Change `PORT` or `--port`, or stop the process holding the port |

## Admin Panel Won't Open or Is Blank

1. Confirm Server is listening:

```bash
curl -I http://127.0.0.1:3000
```

2. Check that the browser URL matches the configured reverse proxy address.

## Default Account Cannot Log In

The default account is `admin` / `12345678`. If the password was modified after first login, use the updated password.

Troubleshooting steps:

1. Confirm you are connected to the expected database, avoiding `SQLITE_PATH` or `DB_HOST` / `DB_NAME` pointing to another environment.
2. Check Server logs to confirm whether `sqlite` or `postgres` is in use.
3. In browser developer tools, verify that admin API requests carry the Session Cookie correctly.
4. Clear browser cache and cookies, then attempt login again.

### Emergency Admin Password Reset

If you forget the `admin` password, reset it using the `reset-passwd` command (supports both SQLite and PostgreSQL):

```bash
go run ./cmd/server reset-passwd --user admin --password your-new-password
```

With SQLite, stop the Server process before running the command to prevent database file lock conflicts. Without `--password`, the command generates a random password and prints it to stdout. After resetting, log in immediately and update your password.

## Agent Cannot Register or Stays Offline

On the Agent host:

```bash
curl -I http://your-server:3000
```

Check Agent logs:

```bash
journalctl -u openflare-agent -n 200 --no-pager
```

Check configuration file:

```bash
sed -n '1,160p' /opt/openflare-agent/agent.json
```

Verify key settings:

| Configuration | Description |
| --- | --- |
| `server_url` | Must be a Server address accessible from the Agent host |
| `agent_token` / `discovery_token` | At least one must be configured |
| `heartbeat_interval` | Supports millisecond integer or Go duration string |
| `request_timeout` | Increase appropriately for high-latency networks |

If logs report an invalid Token, generate a new Token in the admin panel, update `agent.json`, and restart the service:

```bash
systemctl restart openflare-agent
```

## Node Did Not Apply Saved Configuration

1. Confirm the site is enabled and the node's heartbeat timestamp updates continuously.
2. Check **Apply Records** in the node details. Nodes report validation errors for candidate configurations and continue serving the last healthy snapshot.
3. If the controller failed snapshot compilation, search for `refresh live edge snapshot` in Server logs, correct configuration errors, and save again.
4. After resolving node environment issues, click **Force Sync** in the node details. Restarting the node also clears failure flags and retries the latest revision.

## Pingora Config Apply Failure

Run `systemctl status openflare-edge` to inspect service status, and run `journalctl -u openflare-edge` to inspect pull, validation, apply, and listener logs. The node's `ready.json` contains the active site version and content digest.

## HTTPS Not Taking Effect

1. Confirm the certificate has been uploaded or issued via ACME.
2. Confirm the domain in website configuration is bound to the certificate.
3. Confirm site settings and certificate bindings are saved.
4. Verify that Apply Records report successful application.
5. Inspect certificate and HTTP status with `curl`:

```bash
curl -Iv https://your-domain
```

Domains without a bound certificate are not added to HTTPS listeners; this is intended behavior.

## Access Analytics Empty

1. Confirm `openflare-edge` is running and the node reports regular heartbeats.
2. Inspect node logs for queue overflow warnings, disk spool write failures, or heartbeat acknowledgment errors. On write failures, check write permissions and disk space in the state directory.
3. Confirm that the Server retention policy has not purged records for the target time window. For queue and disk limits, see [Access Log Spooling](./performance-policies.md#access-log-spooling).

## Edge Cache Hit-Rate Anomalies

Access logs classify cache status into three categories: **Hit** (HIT/STALE/REVALIDATED/UPDATING), **Origin** (MISS/EXPIRED), and **Not Cached** (BYPASS or empty, meaning the request did not match cacheable paths or the response was uncacheable). See [Edge Cache Strategy Design](../design/edge-cache-design.md).

### Checklist

1. The cache switch in the node's Performance Policy is enabled.
2. Site **Cache** is enabled, and the policy matches the request path (Standard Static Assets only covers built-in extensions, **excluding HTML/JSON**; `.js.map` has extension `map`, included by default).
3. The node's Apply Records show that the latest configuration applied successfully.
4. Request method is **GET** (non-GET requests bypass caching).
5. The origin does not return **`Set-Cookie`** for the requested URL (responses with `Set-Cookie` are not stored on the edge).
6. The origin does not declare **`Cache-Control: private` / `no-store`** (shared caches do not store private responses).
7. Browser DevTools "Disable cache" only affects browser-side caching; verify edge hits using `cache_status` in access logs rather than the browser Network panel.

### Common Misconceptions

| Symptom | Explanation |
| --- | --- |
| `/api/foo` or `/index.html` not cached under `static` | Expected behavior (extension not in default cacheable list) |
| HTML cross-user leakage under `all` policy | Origin did not forbid shared caching; revert to `static` or set `private`/`no-store` on dynamic responses |
| Low hit rate for URLs with `?v=` | Default cache key includes full URI with query; different query parameters create distinct cache entries |
| Continuous MISS across consecutive requests | Verify whether origin returns `Set-Cookie` or `private`, or if disk/memory cache inactive window is too short |

### Expected Behavior (Cloudflare Defaults Alignment)

* Authenticated users with login cookies requesting `/_app/**/*.js`: **Can HIT**.
* Responses with `Set-Cookie` or `private`: **Not stored**.
* Cacheable status codes without origin caching headers: use default Edge TTL (~120 minutes for HTTP 200).
