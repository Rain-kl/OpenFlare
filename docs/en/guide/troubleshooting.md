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
| Node not updated after release | active version, node heartbeat, apply records |
| OpenResty apply failure | apply records, Agent logs, certificates, upstream addresses, port usage |
| Access analytics empty | OpenResty container state, observability port, Agent backfill logs |
| Static assets never hit cache | global/site cache switch, policy extensions, whether config released, access log `cache_status`, origin Set-Cookie / Cache-Control |

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
go run main.go reset-passwd --user admin --password your-new-password
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

## Node Didn't Apply the New Version After Release

Check in order:

1. Is the target version activated in the version page?
2. Is the node online, and did the last heartbeat time update?
3. Do the apply records show success, warning, or failure for the target version?
4. Is the website config enabled? Disabled sites don't participate in release rendering.
5. Do Agent logs show pull, validation, reload, or rollback messages?

View Agent logs:

```bash
journalctl -u openflare-agent -f
```

Note: once a target `version + checksum` fails to apply and rolls back, the Agent blocks retrying that target in local state. After fixing the config, republish to generate a new checksum, or activate an older version to roll back.

If this is the Agent's first config apply with no historical `nginx.conf` to roll back to, the failed target is still blocked, but the Agent enters a safe fallback runtime. The apply records and Agent logs will contain `fallback runtime started`; OpenResty only listens on port `80` and returns `503` with `OpenFlare: No Valid Configuration` for everything, while keeping the local `stub_status` health endpoint. After fixing the config and republishing a new version, the Agent overwrites the fallback config and resumes normal proxying.

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

1. Confirm the node successfully applied config including the observability Lua assets.
2. Confirm OpenResty is running.
3. Check Agent logs for observability collection or backfill failures.
4. Check whether `openresty_observability_port` is occupied (default `18081`).
5. Confirm the Server's DB cleanup policy hasn't deleted the relevant time window.

## Edge Cache Hit-Rate Anomalies

Access log cache three states: **hit** (HIT/STALE/REVALIDATED/UPDATING), **origin** (MISS/EXPIRED), **not cached** (BYPASS or empty — request didn't enter a cacheable path or response wasn't stored). Design: [Edge Cache Strategy Design](../design/edge-cache-design.md).

### Checklist

1. Global OpenResty cache enabled in **Performance Settings**.  
2. Site **Cache** enabled and policy matches the path (「Standard static assets」covers only built-in extensions, **not HTML/JSON**; `.js.map`'s extension is `map`, in the default table).  
3. Config version **published and activated**, node apply records succeeded (changing cache rules without publishing leaves nodes on old bypass logic).  
4. Request method is **GET** (non-GET is never cached).  
5. Origin doesn't return **`Set-Cookie`** for the target URL (if so, not written to the edge).  
6. Origin doesn't declare **`Cache-Control: private` / `no-store`** (shared caches won't store).  
7. Browser DevTools "Disable cache" only affects the browser; whether the edge HITs is judged by access log `cache_status`, not the Network panel.

### Common Misconceptions

| Symptom | Explanation |
| --- | --- |
| Everything「not cached」after login, never republished | old config bypassed session cookies; after upgrade you must republish node configs |
| `/api/foo` or `/index.html` not cached under `static` | expected (extension not in the default cacheable table) |
| HTML cross-user leakage after switching to `all` | origin didn't forbid shared caching; switch back to `static` or add `private`/`no-store` to dynamic responses |
| URLs with `?v=` have low hit rates | default cache key includes the full `$request_uri`; different query = different object |
| First MISS, second still MISS | check whether the origin sets `Set-Cookie`/`private` every time, or node disk/cache `inactive` is too short |

### Expected Behavior (aligned with Cloudflare defaults)

* A logged-in user accessing `/_app/**/*.js` static assets: **can HIT**.  
* Response with `Set-Cookie` or `private`: **not stored**.  
* Cacheable status codes without origin cache headers: use the default Edge TTL (e.g. ~120 min for 200).
