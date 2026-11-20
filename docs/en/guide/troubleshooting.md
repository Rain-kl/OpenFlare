# Troubleshooting

You will learn: how to troubleshoot OpenFlare Server, database, login, Agent, OpenResty, and config release issues by symptom.

First determine which layer the problem is in: browser, Server, database, Agent, OpenResty, origin, or DNS. OpenFlare configs are not written to all nodes online directly — only after the active version changes do Agents detect and apply it in their heartbeat.

## Quick Locate

| Symptom | Look Here First |
| --- | --- |
| Admin panel won't open | Server container/process logs, port listening |
| Login abnormal | default account, Session Cookie, Server logs |
| Data won't save | DB connection, SQLite file permissions, PostgreSQL health |
| Agent offline | Agent logs, Token, Server address, network connectivity |
| Node not updated after saving | node heartbeat, apply records, controller compilation logs |
| OpenResty apply failure | apply records, Agent logs, certificates, upstream addresses, port usage |
| Access analytics empty | OpenResty container state, observability port, Agent backfill logs |
| Static assets never hit cache | global/site cache switch, policy extensions, whether the node applied the latest configuration, access log `cache_status`, origin Set-Cookie / Cache-Control |

## Server Won't Start

1. Check the logs:

```bash
docker compose logs -n 200 openflare
```

For source runs, check terminal output.

2. Check port usage:

```bash
lsof -i :3000
```

3. If using PostgreSQL, confirm DB health:

```bash
docker compose ps postgres
docker compose logs -n 100 postgres
```

4. If using SQLite, confirm the DB file directory is writable:

```bash
ls -ld "$(dirname /path/to/openflare.db)"
```

Common causes:

| Log or Symptom | Handling |
| --- | --- |
| DB connection failed | check `DB_HOST`, `DB_PORT`, `DB_USERNAME`, `DB_PASSWORD`, `DB_NAME`, `DB_SSL_MODE` consistency |
| SQLite can't create file | check the `SQLITE_PATH` directory exists and is writable |
| Port occupied | change `PORT` or `--port`, or stop the process holding the port |

## Admin Panel Won't Open or Is Blank

1. Confirm the Server is listening:

```bash
curl -I http://127.0.0.1:3000
```

2. Check that the browser access address matches the reverse proxy config.

## Default Account Can't Log In

The default account is `admin` / `12345678`. If the password was changed after first login, use the changed one.

Steps:

1. Confirm you're connected to the intended database — avoid `SQLITE_PATH` or `DB_HOST` / `DB_NAME` pointing at another environment.
2. Check whether the Server log uses `sqlite` or `postgres`.
3. In browser dev tools, confirm admin API requests carry the Session Cookie correctly.
4. Clear browser cache and cookies, then log in again.

### Emergency Admin Password Reset

If you forget the `admin` password, reset it with the `reset-passwd` command (supports SQLite and PostgreSQL):

```bash
go run ./cmd/server reset-passwd --user admin --password your-new-password
```

With SQLite, stop the Server process first to avoid DB file lock conflicts. Without `--password`, the command generates a random password and prints it. After resetting, log in and change the password immediately.

## Agent Can't Register or Stays Offline

On the Agent node:

```bash
curl -I http://your-server:3000
```

Check Agent logs:

```bash
journalctl -u openflare-agent -n 200 --no-pager
```

Check the config file:

```bash
sed -n '1,160p' /opt/openflare-agent/agent.json
```

Confirm:

| Config | Description |
| --- | --- |
| `server_url` | must be a Server address reachable by the Agent node |
| `agent_token` / `discovery_token` | at least one filled in |
| `heartbeat_interval` | supports millisecond integer or Go duration string |
| `request_timeout` | increase for slow networks |

If logs say the Token is invalid, prepare a new Token in the admin panel, update `agent.json`, then restart:

```bash
systemctl restart openflare-agent
```

## Node Did Not Apply the Saved Configuration

1. Confirm that the site is enabled and the node's heartbeat is current.
2. Check **Apply Records** on the node details page for a validation error. The node keeps serving the last healthy snapshot.
3. If the controller could not compile the snapshot, check its logs for `refresh live edge snapshot`, then correct and save the configuration.
4. After fixing the node environment, select **Force Sync** or restart the node to retry the same revision.

## OpenResty Apply Failure

Common causes:

| Cause | Troubleshooting |
| --- | --- |
| Domain or server block conflict | check whether the same domain is used by multiple site configs |
| Invalid upstream address | confirm all upstreams are `http://` or `https://` |
| Multi-upstream format violates constraints | multi-upstream must be plain `scheme://host[:port]` |
| Certificate missing or wrong path | check whether the domain is bound to a cert and the Agent cert dir is writable |
| Port occupied | check local `80`, `443` ports |

OpenResty config validation:

```bash
openresty -t -c /path/to/openflare/data/etc/nginx/nginx.conf
```

OpenResty running state:

```bash
ps aux | grep openresty
```

The Agent's periodic health check probes the local `http://127.0.0.1:<openresty_observability_port>/openflare/stub_status` to judge OpenResty liveness — it does not repeatedly run `openresty -t`. If a node is marked unhealthy, first confirm that local observability port is listening; if `host not found in upstream` appears only during config apply, the failure comes from config validation or reload, not the periodic health probe.

Actual binary and main config paths follow `openresty_path` and `main_config_path` in `agent.json`.

## HTTPS Not Taking Effect

1. Confirm the certificate is uploaded or managed.
2. Confirm the site config's domain is bound to a certificate.
3. Confirm a new version was published and activated.
4. Check whether the apply records succeeded.
5. Inspect the certificate and status code with `curl`:

```bash
curl -Iv https://your-domain
```

Domains without a bound certificate are not auto-added to the HTTPS config — that's expected.

## Access Analytics Empty

1. Confirm `openflare-edge` is running and the control plane is receiving its heartbeats.
2. Check Edge logs for warnings about dropped entries, disk write failures, or heartbeat acknowledgements. For write failures, check free space and write permissions in the node state directory.
3. Confirm that the Server's retention policy has not removed the relevant time window.

Pingora queues access logs, including WAF audit records, in memory and writes them to `access-logs.jsonl` in batches of 256 entries or every 100 ms. Requests do not wait for disk writes. Heartbeats submit up to 1,024 persisted entries and remove them after the control plane acknowledges the batch.

The queue holds up to 2,048 pending entries. Queued and in-flight entries share a 16 MiB heap budget. At either limit, new entries are dropped and reported together in a warning. When the spool exceeds 4 MiB, the node keeps the newest entries that fit, up to 2,048. On normal shutdown or reload, the writer drains the queue; a forced stop or power loss can discard entries still in memory.

## Edge Cache Hit-Rate Anomalies

Access log cache three states: **hit** (HIT/STALE/REVALIDATED/UPDATING), **origin** (MISS/EXPIRED), **not cached** (BYPASS or empty — request didn't enter a cacheable path or response wasn't stored). Design: [Edge Cache Strategy Design](../design/edge-cache-design.md).

### Checklist

1. Global OpenResty cache enabled in **Performance Settings**.  
2. Site **Cache** enabled and policy matches the path (「Standard static assets」covers only built-in extensions, **not HTML/JSON**; `.js.map`'s extension is `map`, in the default table).  
3. The node applied the latest saved configuration successfully.
4. Request method is **GET** (non-GET is never cached).  
5. Origin doesn't return **`Set-Cookie`** for the target URL (if so, not written to the edge).  
6. Origin doesn't declare **`Cache-Control: private` / `no-store`** (shared caches won't store).  
7. Browser DevTools "Disable cache" only affects the browser; whether the edge HITs is judged by access log `cache_status`, not the Network panel.

### Common Misconceptions

| Symptom | Explanation |
| --- | --- |
| `/api/foo` or `/index.html` not cached under `static` | expected (extension not in the default cacheable table) |
| HTML cross-user leakage after switching to `all` | origin didn't forbid shared caching; switch back to `static` or add `private`/`no-store` to dynamic responses |
| URLs with `?v=` have low hit rates | default cache key includes the full `$request_uri`; different query = different object |
| First MISS, second still MISS | check whether the origin sets `Set-Cookie`/`private` every time, or node disk/cache `inactive` is too short |

### Expected Behavior (aligned with Cloudflare defaults)

* A logged-in user accessing `/_app/**/*.js` static assets: **can HIT**.  
* Response with `Set-Cookie` or `private`: **not stored**.  
* Cacheable status codes without origin cache headers: use the default Edge TTL (e.g. ~120 min for 200).
