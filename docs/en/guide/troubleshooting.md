# Troubleshooting

You will learn: How to troubleshoot OpenFlare Server, database, login, Agent, OpenResty, configuration publishing, and frontend build issues by symptoms.

During troubleshooting, first identify which layer the issue occurs in: browser, Server, database, Agent, OpenResty, origin server, or DNS. OpenFlare configurations are not written directly to nodes online; only after the active version changes will the Agent detect and apply it in heartbeats.

## Quick Diagnostic

| Symptom | Where to check first |
| --- | --- |
| Admin panel fails to open | Server container or process logs, port listening |
| Login anomalies | Default credentials, Session Secret, browser request payloads, Server logs |
| Data fails to save | Database connection, SQLite file permissions, PostgreSQL health |
| Agent offline | Agent logs, Token, Server URL, network connectivity |
| Node not updated after publishing | Active version, node heartbeat, application logs |
| OpenResty application failed | Application logs, Agent logs, certificates, upstream addresses, port conflicts |
| Observability analytics has no data | OpenResty container status, observability port, Agent retry logs |

## Server Fails to Start

1. View logs:

```bash
docker compose logs -n 200 openflare
```

For source-code execution, inspect terminal outputs.

2. Check port conflicts:

```bash
lsof -i :3000
```

3. If using PostgreSQL, verify that the database is healthy:

```bash
docker compose ps postgres
docker compose logs -n 100 postgres
```

4. If using SQLite, verify that the database directory is writable:

```bash
ls -ld "$(dirname /path/to/openflare.db)"
```

Common causes:

| Log or Symptom | Action |
| --- | --- |
| Database connection failed | Check `DSN` username, password, host, port, dbname, and `sslmode` |
| SQLite fails to create files | Check if the parent directory of `SQLITE_PATH` exists and is writable |
| Port is already in use | Change `PORT` or `--port`, or stop the process binding to the port |

## Admin Console Fails to Load or Shows Blank Page

1. Verify that the Server is listening:

```bash
curl -I http://127.0.0.1:3000
```

2. If running from source, verify that the frontend static assets have been built:

```bash
cd openflare-server/web
pnpm build
```

3. Verify if the browser URL matches your reverse proxy domain.

4. If accessing via the frontend dev server, verify the backend proxy configuration:

```bash
cd openflare-server/web
NEXT_DEV_BACKEND_URL=http://127.0.0.1:3000 pnpm dev
```

## Default Credentials Fail to Log In

The default credentials are `root` / `123456`. If you have modified the password after your first login, use your new password.

Troubleshooting Steps:

1. Confirm that you are connecting to the expected database, avoiding `SQLITE_PATH` or `DSN` pointing to a different environment.
2. Check the Server log to see if it is running on `sqlite` or `postgres`.
3. If deployed in multi-replicas or behind a reverse proxy, verify that `SESSION_SECRET` is static and uniform across all instances.
4. Clear browser Cookies and try logging in again.

### Emergency Reset of Admin Password

If you forget the password for the `root` account, you can reset it back to `123456` by directly updating the password hash in the database (please change it immediately after logging in):

#### 1. If using SQLite Database
Stop the Server and open the database file using the `sqlite3` client:
```bash
sqlite3 /path/to/openflare.db
```
Execute the following SQL statement:
```sql
UPDATE users SET password_hash = '$2a$10$wN9aE3zTz83rO7R1uKlhuehJtA3c604pX4Z12B/9.5c0X337t1L4m' WHERE username = 'root';
```
Type `.exit` to exit and restart the Server.

#### 2. If using PostgreSQL Database
Connect to your PostgreSQL instance using a database tool (e.g., `psql`, `pgAdmin`, or `DBeaver`), select the corresponding `openflare` database, and execute the following SQL:
```sql
UPDATE users SET password_hash = '$2a$10$wN9aE3zTz83rO7R1uKlhuehJtA3c604pX4Z12B/9.5c0X337t1L4m' WHERE username = 'root';
```
Once executed successfully, you can log in using the default password `123456`.

## Agent Fails to Register or Stays Offline

Execute on the Agent node:

```bash
curl -I http://your-server:3000
```

Inspect Agent logs:

```bash
journalctl -u openflare-agent -n 200 --no-pager
```

Verify configuration parameters:

```bash
sed -n '1,160p' /opt/openflare-agent/agent.json
```

Key Settings:

| Configuration | Description |
| --- | --- |
| `server_url` | Must be the Server address reachable by the Agent node |
| `agent_token` / `discovery_token` | At least one must be provided |
| `heartbeat_interval` | Supports integer milliseconds or Go duration strings |
| `request_timeout` | Can be increased for slower network links |

If the log warns that the Token is invalid, retrieve a new Token in the management console, update `agent.json`, and restart the Agent:

```bash
systemctl restart openflare-agent
```

## Node Fails to Apply New Version after Publishing

Verify in sequence:

1. Confirm that the target version is activated on the Versions page.
2. Verify if the node is online and if its last heartbeat time has updated.
3. Check the Application Logs for successful, warned, or failed logs for the target version.
4. Verify if the website configuration is enabled; disabled websites do not participate in rendering.
5. Inspect Agent logs for pulls, validations, reloads, or rollback events.

Inspect Agent logs:

```bash
journalctl -u openflare-agent -f
```

Note: If a target `version + checksum` fails to apply and triggers a rollback, the Agent blocks repeated synchronization of that failing target in its local state. You must fix the configuration issues and republish to generate a new checksum, or activate an older version to trigger a rollback.

If this is the Agent's first time applying configurations and no historic `nginx.conf` exists locally to roll back to, the failed version remains blocked but the Agent will attempt to enter the safe fallback runtime. At this point, the application logs and Agent logs will contain `fallback runtime started`. OpenResty will only listen to port `80`, returning a `503` with the body `OpenFlare: No Valid Configuration`, while retaining the local `/openflare/stub_status` health probe. After correcting the configurations and republishing, the Agent overrides the fallback config and restores normal reverse proxies.

## OpenResty Application Fails

Common Causes:

| Cause | Diagnostic |
| --- | --- |
| Domain or server block conflict | Verify if the same domain is used by multiple website configurations |
| Invalid upstream address | Confirm that all upstreams are valid `http://` or `https://` URLs |
| Mismatched multi-upstream format | Multi-upstreams must be pure `scheme://host[:port]` |
| Missing cert or invalid paths | Verify if domains are bound to certs and check if the Agent cert directory is writable |
| Port already in use | Verify ports `80` and `443` on the host |

OpenResty Configuration Validation:

```bash
openresty -t -c /path/to/openflare/data/etc/nginx/nginx.conf
```

OpenResty Runtime Status:

```bash
ps aux | grep openresty
```

The Agent determines OpenResty survival periodically using the local endpoint `http://127.0.0.1:<openresty_observability_port>/openflare/stub_status`, completely bypassing repeated `openresty -t` calls. If a node is marked as unhealthy, confirm if this local observability port is listening. If failures only occur when applying configurations (e.g., `host not found in upstream`), the failure lies in config validation or reload, not the periodic health checks.

Actual binary paths and main configuration paths are governed by `openresty_path` and `main_config_path` in `agent.json`.

## HTTPS Fails to Work

1. Verify that the certificate has been uploaded or hosted.
2. Verify that the website configuration binds the certificate to the domain.
3. Confirm that the configuration version has been published and activated.
4. Check if the Application Logs indicate a success.
5. Check the certificate chain and status code using `curl`:

```bash
curl -Iv https://your-domain
```

Domains without a bound certificate will not be added to the HTTPS configuration automatically; this is expected behavior.

## Traffic Analytics Has No Data

1. Confirm that the node has successfully applied configurations carrying observability Lua scripts.
2. Verify that OpenResty is running.
3. Check Agent logs for observability extraction or upload errors.
4. Check if `openresty_observability_port` (default is `18081`) is bound by other processes.
5. Verify if the Server database has purged data inside the time window.

## Frontend Build Fails

Execute:

```bash
cd openflare-server/web
corepack enable
pnpm install
pnpm lint
pnpm typecheck
pnpm test
pnpm build
```

Common causes:

| Symptom | Action |
| --- | --- |
| pnpm version mismatch | Reinstall packages after executing `corepack enable` |
| TypeScript errors | Locate detailed file bugs by running `pnpm typecheck` |
| API type mismatch | Check responses structures in `lib/api/` and `types/` |
| E2E test failures | Confirm that both the Server and frontend dev server are running |

## Documentation Build Fails

```bash
cd docs
pnpm install
pnpm build
```

If it fails on broken links, check if new pages are added to the `docs/config.ts` sidebar, or if relative markdown links point to existing markdown files.
