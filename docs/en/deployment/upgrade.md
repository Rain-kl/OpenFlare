# Upgrade & Maintenance

You will learn: How to upgrade the Server and the Agent, how to clean up observability data, and which validation commands to execute before and after maintenance.

Before upgrading, verify the currently active version, the most recent Agent application results, and your database backup strategy. In production environments, never trigger upgrades while a configuration is being published, during large-scale Agent reconnections, or while database migrations are in progress.

## Server Upgrade

Root users can check and trigger stable Server upgrades in the top header of the management console. You can also trigger upgrades by uploading the compiled Server binary in the console.

To deploy preview releases, manually check the GitHub Releases page. We highly recommend prioritizing stable releases in production environments.

Verify after upgrading:

```bash
docker compose ps
docker compose logs -n 100 openflare
```

If deployed from source, restart the Server and verify that no database migration or startup errors appear in the logs.

## Agent Upgrade

Node Agents automatically update following stable releases by default. Upgrading to preview releases requires a manual trigger.

You can re-execute the installation script to redeploy or force-update the Agent:

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --agent-token YOUR_AGENT_TOKEN
```

Note: Re-executing the current installation script wipes the entire installation directory, including the existing `agent.json`, local states, cached databases, and downloaded binaries. Ensure you have the node Token handy before executing the script.

Verify after upgrading:

```bash
systemctl status openflare-agent
journalctl -u openflare-agent -n 100 --no-pager
```

## Data Maintenance

The management console's Settings page maintains options for automatic cleanup of observability data:

| Parameter | Description |
| --- | --- |
| `DatabaseAutoCleanupEnabled` | Toggles daily automatic cleanup |
| `DatabaseAutoCleanupRetentionDays` | Data retention duration in days, minimum 1 day |

When enabled, the Server cleans up access logs, metrics snapshots, and request reports at 3:00 AM daily.
