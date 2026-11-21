---
title: Rotate the session secret
description: Check and re-encrypt stored credentials with the OpenFlare Server command before switching secrets.
---

# Rotate the session secret

The OpenFlare Server session secret signs browser sessions and encrypts TLS private keys, DNS credentials, ACME account private keys, and standalone Cloudflare tokens. MCP requires a secret of at least 32 bytes. Use `rotate-credentials` to check stored credentials and re-encrypt them in one transaction.

## 1. Generate a secret and check the database

Use the same configuration, environment, working directory, and primary database as the running server. Keep the old value in `app.session_secret` or `APP_SESSION_SECRET` while running the command. The environment variable takes precedence.

```sh
rotation_secret_file="$HOME/.openflare-new-session-secret"
(umask 077; set -C; openssl rand -hex 32 > "$rotation_secret_file")
openflare-server rotate-credentials --new-secret-file "$rotation_secret_file"
```

The check reports how many records need migration and how many already use the new secret. If no records need migration, continue to step 3. A decryption error identifies the affected record. Check the old secret and database before proceeding.

## 2. Stop the service, back up, and migrate

Stop every OpenFlare Server process connected to this database, including API, worker, and scheduler processes. Keep database and Redis services running. Back up the primary database and the configuration containing the old secret. For SQLite, use `(umask 077; sqlite3 /actual/path/openflare.db '.backup openflare-before-rotation.db')`. For PostgreSQL, use `pg_dump` for the corresponding instance.

```sh
openflare-server rotate-credentials --new-secret-file "$rotation_secret_file" --apply
```

The command verifies that the new secret can decrypt the replacements before committing. If a run fails, resolve the error and use the same secret file to retry. Run the check again if you need to confirm that no credentials remain to be migrated.

### Docker Compose

For a deployment where the service is named `openflare`, run `docker compose stop openflare`, then use a temporary container with the same database configuration and volumes:

```sh
docker compose run --rm --no-deps \
  -v "$rotation_secret_file:/run/secrets/new-session-secret:ro" \
  openflare rotate-credentials --new-secret-file /run/secrets/new-session-secret --apply
```

Ensure the container user can read the file. Omit `--apply` for a check. Stop other OpenFlare Server containers as well if you use a split deployment.

## 3. Switch the configuration and start the service

1. Set the effective `app.session_secret` or `APP_SESSION_SECRET` to the new value. Use the same value on all Server instances.
2. Start OpenFlare Server. With Docker Compose, run `docker compose up -d openflare` to apply updated environment variables.
3. Sign in again and check the MCP tab. Authorize any CLI or MCP connections that used OAuth again.
4. Keep the protected backup and old secret, and remove the temporary migration secret file.

To recover, stop the server and restore **both the database backup and the old secret** before starting it again. Restoring only the old secret leaves migrated credentials unreadable.
