# Uptime Kuma Monitoring Sync

You will learn: how to enable and configure the Uptime Kuma auto-sync integration, control the sync scope and heartbeat probe parameters for monitored sites, and the underlying principles of differential synchronization between OpenFlare and Uptime Kuma.

---

## Feature Overview

In edge multi-node operations, knowing the availability of each proxied site in time is critical. To avoid manually re-entering site information into a monitoring system, OpenFlare provides deep integration with the open-source monitoring service **Uptime Kuma**.

Once enabled, OpenFlare starts a background sync scheduler that automatically syncs the proxy sites configured in the admin panel as HTTP monitor tasks in Uptime Kuma. It supports scope filtering, differential attribute updates, and automatic cleanup of decommissioned sites.

---

## Step 1: Configure the Integration in System Settings

1. Log in to the admin panel, go to **「System Settings」** in the left navigation, select the **「OpenFlare」** tab, and configure the **「Uptime Kuma Integration」** section.
2. Configure the following core connection parameters:
   * **Enabled**: Turn on the integration switch.
   * **Instance URL**: Your Uptime Kuma service address, e.g. `http://192.168.1.100:3001` or `https://kuma.example.com` (the protocol prefix `http://` or `https://` is required).
   * **Username** and **Password**: Credentials for a Uptime Kuma account with admin privileges, used for API authentication.

---

## Step 2: Control Monitor Scope and Heartbeat Parameters

In the integration panel you can finely control the monitor scope and probe behavior:

### 1. Monitor Scope
* **All sites**: Default option. OpenFlare automatically syncs all **enabled** proxy route sites. When a new site is created and enabled, or an old site is disabled, the monitor list is updated automatically.
* **Selected sites**: Only monitor specified sites. After selecting this mode, click the **「Select Monitored Sites」** dialog. Inside the dialog you can filter sites by search and check the ones you want. Sites that are unchecked or not checked will not be synced (and will be automatically cleaned up if they already exist).

### 2. Probe Frequency and Heartbeat Settings
You can specify uniform probe parameters for auto-generated monitors:
* **Sync Interval**: Frequency (minutes) of automatic differential sync, default `5` minutes. The control plane compares state with Uptime Kuma every 5 minutes.
* **Heartbeat Interval**: Frequency (seconds) at which Uptime Kuma probes sites, default `60` seconds.
* **Retry**: Maximum number of retries before a failed probe is judged Down, default `0`.
* **Retry Interval**: Seconds to wait between retries, default `60` seconds.
* **Request Timeout**: Seconds after which a probe request is judged timed out, default `48` seconds.

---

## Sync and Cleanup Mechanism

* **Dedicated tag isolation**: All auto-created monitors are bound with the `OpenFlare`-specific tag (purple-blue). The sync and cleanup routines only operate on monitors with this tag, and will not interfere with or damage other monitors you created manually in Uptime Kuma.
* **Differential incremental sync**: The sync routine periodically compares monitor metadata. When a domain or heartbeat configuration change is detected, only a differential update is performed to avoid interrupting historical statistics; when a site is disabled or moved out of scope, it is automatically taken offline and cleaned up.

> [!TIP]
> For details on the Socket.IO control flow, anti-pollution tag model, and differential comparison algorithm of Uptime Kuma monitoring sync, see [Uptime Kuma Sync Design](../design/kuma-design.md).
