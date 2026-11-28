---
title: Rotate the session secret
description: Re-encrypt stored database credentials with rotate-credentials before switching app.session_secret.
---

# Rotate the session secret

OpenFlare Server uses `app.session_secret` to sign sessions and encrypt TLS private keys, DNS credentials, ACME keys, and Cloudflare tokens (MCP requires at least 32 bytes). Always re-encrypt stored records with `rotate-credentials` before changing the active secret.

## 1. Generate a Secret & Run a Dry-Run Check

With the current `app.session_secret` or `APP_SESSION_SECRET` active (the environment variable overrides YAML):

```sh
rotation_secret_file="$HOME/.openflare-new-session-secret"
(umask 077; set -C; openssl rand -hex 32 > "$rotation_secret_file")
openflare-server rotate-credentials --new-secret-file "$rotation_secret_file"
```

The dry-run outputs pending record counts; skip to step 3 if all pending counts are `0`. Fix any decryption error by matching the active secret and primary database before continuing.

## 2. Stop Services, Back Up & Apply Migration

1. Stop all OpenFlare Server processes (`api`, `worker`, `scheduler`) while leaving the database and Redis running.
2. Back up the current secret and primary database (SQLite: `(umask 077; sqlite3 /actual/path/openflare.db '.backup openflare-before-rotation.db')`; PostgreSQL: `pg_dump`).
3. Execute transactional re-encryption with round-trip verification:

```sh
openflare-server rotate-credentials --new-secret-file "$rotation_secret_file" --apply
```

For Docker Compose (`openflare` service), run `docker compose stop openflare` first, then run a one-off container (omit `--apply` to dry-run):

```sh
docker compose run --rm --no-deps \
  -v "$rotation_secret_file:/run/secrets/new-session-secret:ro" \
  openflare rotate-credentials --new-secret-file /run/secrets/new-session-secret --apply
```

## 3. Switch Config, Restart & Roll Back

1. Update `app.session_secret` or `APP_SESSION_SECRET` identically across all Server instances and start them (`docker compose up -d openflare` in Compose).
2. Sign in, check the **MCP** tab, re-authorize any OAuth CLI/MCP grants, and delete `$rotation_secret_file`.
3. To roll back, stop Server and restore both the pre-migration database backup and the old secret together; reverting the secret alone renders migrated records unreadable.
