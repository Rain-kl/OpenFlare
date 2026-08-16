# Zone Domain Migration and Release Acceptance

When migrating from the legacy `managed_domains` / inline domain columns of reverse proxy routes to the Zone + Zone Domain model, data import and table structure upgrades are both completed by the **automatic goose migration at Server startup** — no separate import command is needed.

## What Happens During Upgrade

When starting (or rolling-upgrading) a Server version that includes the Zone rework, **no manual command is required**; `migrator.Migrate()` automatically:

1. Applies goose SQL: creates `of_zones` / `of_zone_domains` (if they do not yet exist).
2. **Automatically imports** the legacy route domain columns (and `of_managed_domains` when routes have no domains) as Zone / Zone Domains, binding `proxy_route_id` / `cert_id` (registering root domains via public suffix list parsing).
3. Continues goose SQL: drops the redundant domain/certificate columns from `of_managed_domains` and `of_proxy_routes`.

The import is idempotent: existing domains are skipped or have their route binding back-filled.

**If historical data cannot be parsed (conflicting domains, invalid root domains, missing certificates, etc.), startup fails.** Fix the data or restore a backup and start again to retry.

## Recommended Actions

### 1. Back Up Before Upgrading

```bash
# PostgreSQL example
pg_dump "$DATABASE_URL" > openflare-pre-zone-$(date +%Y%m%d).sql

# Or copy the backup volume / snapshot; for SQLite, copy the database file in the data directory
```

Optional: note down the current **active config version number** and checksum in the admin panel for config rollback comparison.

### 2. Upgrade and Start the Server

Deploy the new version and start it. Watch the goose success messages in the startup log; if "Zone migration failed (N conflicts)" appears, fix the source data according to the conflicts listed in the log and restart.

### 3. Post-Upgrade Checks

1. Admin panel **Websites** `/websites`: check whether Zone root domains and domain counts are reasonable.
2. Zone details: domains, certificates, associated route IDs.
3. **Reverse proxy routes**: domain bindings come from Zone Domains, not legacy hand-written fields.

### 4. Config Preview and Release

1. Review the config diff / preview in the admin panel.
2. Verify **per route**: `server_name` set, certificate paths, WAF Route ID, Pages references.
3. **Allow** the redundant `domain` / `domains` / `cert_ids` on routes in old snapshot JSON to disappear.
4. **Do not allow** data-plane semantic changes.
5. After the preview passes, release it; if needed, activate the pre-upgrade version in config versions for config rollback. For database rollback, use the pre-upgrade backup (down migrations do not backfill business domain data).

## Related Docs

* [Zone & Domain Resource Design](../design/zone-design.md)
* [Create a Reverse Proxy Config](./proxy-config.md)
* [Publish First Configuration](./first-site.md)
