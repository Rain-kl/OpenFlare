---
title: CLI and MCP
description: Authorize OpenFlare CLI and MCP via OAuth or personal access tokens to manage sites and edge nodes.
---

# CLI and MCP

Connect `openflare` CLI and MCP clients over OAuth or personal access tokens to automate sites, WAF policies, edge nodes, and probes.

## 1. Enable MCP Connections

Open **Admin** > **System settings** > **MCP**, save a client-reachable HTTP or HTTPS Server URL (HTTPS recommended in production; clear and save to disable), and select an auth mode:

| Method | Setup & Scope Rules |
| :--- | :--- |
| OAuth (Recommended) | Requires RFC 7591 dynamic registration and S256 PKCE; authorize as an admin and toggle Read/Write scopes (`15m` access / `30d` refresh tokens; click **Revoke all OAuth grants** to invalidate immediately) |
| Access Token | Create an admin token under **Settings** > **Personal access tokens** and send `Authorization: Bearer TOKEN` for full MCP access |

Call `openflare_connection_info` to verify the instance ID and granted scopes. Reverse proxies must forward `/mcp`, `/.well-known/`, `/oauth/automation/`, `/automation/`, and API prefixes while preserving `Host`; multi-replica deployments must share the database and a 32+ byte `session_secret` (see [Rotate the Session Secret](./session-secret-rotation.md)).

## 2. Install & Authenticate CLI

Download `openflare_cli_*` from [OpenFlare Releases](https://github.com/Rain-kl/OpenFlare/releases) into `PATH` (or build with `go build -o bin/openflare ./cmd/openflare`; Linux requires an active Secret Service keyring):

```sh
openflare auth login --server https://openflare.example.com
openflare connection info
openflare operations list
openflare site create --help
```

Isolate multiple environments with `--profile <name>`; pass JSON values for object/array flags and strings for numeric IDs.

## 3. Common Operations

| Task | CLI Command (MCP tools prefix `openflare_`) |
| :--- | :--- |
| Create website | Resolve `zone_id` (`zone list`/`zone create`) and `domain create`, then run `site create` (HTTPS requires matching `certificate list` entry) |
| Add domain alias | Inspect `site list` and `domain list`/`domain create`, then run `site bind-domain` |
| Replace direct upstreams | Run `site set-upstreams` with the complete list (omit `origin_host` to preserve existing value) |
| Patch route settings | Inspect via `site get-settings`, then patch toggles, TLS, redirects, cache, headers, or limits with `site patch-settings` |
| Bind WAF flows | List via `waf list --site-id SITE_ID` and attach/detach with `site set-waf` |
| Configure PoW or auth | Manage PoW with `site get-pow`/`site set-pow`; switch `off` / `basic` / `site` (`site` requires HTTPS) via `site set-auth` |
| Manage attack rule sets | Run `attack-group list` / `create` / `update` / `delete` (groups used by a WAF flow cannot be deleted) |
| Inspect IP groups | View counts and sync state with `ip-group list`; inspect members with `ip-group get` |
| Site traffic overview | Run `site overview` (`openflare_site_overview`) for top hosts, totals, and status codes (`--hours 168` for 7 days) |
| Query traffic & nodes | Run `logs list`, `metrics traffic` (`--with-trends=false` for totals only), `node health`, `node metrics`, and `node ping` |
| Manage ping targets | Run `ping list` / `create` / `update` / `delete` / `reorder` (`ping:read` / `ping:write`) |
| Edge remote exec | After enabling webshell in [Server deployment](../deployment/server.md), run `exec run` / `exec get` / `exec list` (`exec:write` / `exec:read`) |
| Tune performance policies | Inspect policies with `performance list` and update parameters via `performance set` |

## 4. Key Flags & Scope Rules

```sh
openflare site list --search www.example.com
openflare waf list --site-id "$SITE_ID"
openflare site set-waf --site-id "$SITE_ID" --flow-id "$FLOW_ID" --bound=true
openflare site set-pow --site-id "$SITE_ID" --enabled=true
openflare site patch-settings --site-id "$SITE_ID" --limits '{"limit_req_per_ip":"20r/s","limit_conn_per_ip":10}'
openflare node health --node-type edge_node --status online
```

- Site writes do not modify public DNS or issue certificates; check `config_sync: synced` in `node health` or compare `current_version` with `desired_site_version` in `node get`.
- `site set-waf` preserves existing flow bindings and order; `attack-group update` keeps `rule_overrides` when omitted and clears them on `{}`.
- Inside `site patch-settings --limits`, `0` or `""` inherits instance defaults while `-1` / `"-1"` disables the limit; `site set-auth` supports `--password-stdin` and wipes stored credentials when switched to `off`.
- WAF, attack groups, IP groups, and PoW require `security:read` / `security:write` scopes; re-authorize existing OAuth grants after upgrades.
- `site overview` returns the top 50 sites over 24 hours by default (paginate via `until` + `next_offset`) and reports unbound traffic in `unmatched_requests`.
