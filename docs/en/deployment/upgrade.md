# Upgrade & Maintenance

You will learn: how to upgrade the Server and Agent, how to clean up observability data, and which verification commands to run before and after maintenance.

Before upgrading, confirm the current active version, the most recent Agent apply result, and the DB backup policy. In production, don't upgrade while a config release, a large-scale Agent reconnect, or a DB migration is in progress.

## Server Upgrade

Pull the latest image and upgrade:

```bash
docker compose pull
docker compose up
```

For source deployments, restart the Server and confirm the logs show no DB migration or startup errors.

## Agent Upgrade

The Agent only caches runtime config and state files locally — no business data. To upgrade, directly pull the latest image and recreate the container. For specific deployment commands and install methods, see **[Access Agent](./agent.md)**.
