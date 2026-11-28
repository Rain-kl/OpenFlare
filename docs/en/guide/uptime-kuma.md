# Uptime Kuma Monitoring Sync

Incrementally sync enabled proxy routes to an Uptime Kuma instance as HTTP monitors, and delete managed monitors when sites are disabled or excluded.

## 1. Configure Integration

Open **Admin** > **System settings** > **OpenFlare** > **Uptime Kuma integration**, complete the fields, save, and optionally click **Sync now**:

| Field | Description | Default |
| :--- | :--- | :--- |
| Enable Uptime Kuma | Enables background diff-sync | Disabled |
| Uptime Kuma URL | Base URL including `http://` or `https://` (e.g., `https://kuma.example.com`) | None |
| Username / Password | Uptime Kuma admin credentials (leave password blank when editing to preserve) | None |
| Sync interval (minutes) | Background reconciliation cadence | `5` |
| Monitor scope | **All sites** (all enabled proxy routes) or **Selected sites** | All sites |
| Check interval (seconds) | Uptime Kuma probe cadence | `60` |
| Retries / Retry interval (seconds) | Retry attempts and delay before marking down | `0` / `60` |
| Request timeout (seconds) | Probe request timeout | `48` |

## 2. Sync Behavior & Logs

- Managed monitors carry the `OpenFlare` tag; sync and cleanup touch only tagged items and leave manual monitors intact.
- Domain or probe updates patch existing monitors in place to preserve uptime history; wildcard-only routes require at least one concrete domain to sync.
- The **OpenFlare Uptime Kuma Sync** job under **Admin** > **Tasks** suppresses success logs by default while preserving failure logs.

See [Uptime Kuma Sync Design](../design/kuma-design.md) for Socket.IO control flow and diff internals.
