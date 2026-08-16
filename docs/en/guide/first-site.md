# Publishing Your First Site

You will learn: How to create your first website configuration, bind origins and certificates, publish the configuration version, and verify that the Agent applied it successfully.

The publishing pipeline of OpenFlare centers on a complete configuration version snapshot. After modifying website configurations in the management console, you need to publish and activate the new version to let the Agent pull and apply it in the next heartbeat.

## Pre-publish Checks

Verify that the following conditions are met:

| Item | Expectation |
| --- | --- |
| Server | Management console is accessible and log-in succeeds |
| Agent | At least one node is online |
| Origin | The Agent node can reach the origin server address |
| Domain | Domain is resolved to the OpenResty node, or prepared to verify via local `hosts` / `curl` Host header |
| HTTPS | If HTTPS is required, the certificate is uploaded or hosted |

## Create Website Configuration

A new website configuration requires at least:

| Field | Description |
| --- | --- |
| Website Name | Business unique identifier; the primary domain is used if left blank |
| Domain | At least one domain, where the first is treated as the primary domain |
| Origin Address | A valid `http://` or `https://` upstream address |
| Enabled Status | Only enabled website configurations will participate in publishing and rendering |

Example:

| Field | Example |
| --- | --- |
| Website Name | `app` |
| Domain | `app.example.com` |
| Origin Address | `http://10.0.0.20:8080` |

A single domain can belong to only one website configuration. Rate limiting, reverse proxy, and caching parameters are shared site-wide.

## Bind Certificate

HTTPS certificates are bound by domain. Domains without a bound certificate will not be placed into `443 ssl` server blocks automatically.

If a website contains multiple domains, the rendering pipeline groups the HTTPS configurations by certificate while ensuring all domains belong to the same site snapshot.

## Publish & Activate

Standard Pipeline:

```text
Modify rules -> Preview / Diff -> Publish -> Generate complete version -> Activate version -> Agent pulls -> Local application -> Report result
```

During publication, the Server reads all enabled website configurations, the main OpenResty config templates, performance and cache parameters, rendering the complete OpenResty configuration and calculating its `checksum`, saving to `config_versions`, and switching the active version.

## Verify Results

Verify in the management console after publishing:

| Position | Expected Result |
| --- | --- |
| Node List | Node status is online |
| Node Details | Current version matches active version |
| Apply Logs | Most recent application succeeded |
| Version Page | The new version is currently active |

Verify Agent logs on the node:

```bash
journalctl -u openflare-agent -n 100 --no-pager
```

Access via domain:

```bash
curl -I http://app.example.com
```

If the domain has not been officially resolved, you can verify by specifying the Host header against the node IP:

```bash
curl -I -H 'Host: app.example.com' http://NODE_IP
```

HTTPS Validation:

```bash
curl -I https://app.example.com
```

## Rollback

If a target version application fails and triggers a rollback, the Agent blocks repeated synchronization of the same failing `version + checksum` until the active version or checksum changes on the control plane.

Roll back to an older version:

1. Open the Configuration Versions page.
2. Locate the last known good historic version.
3. Re-activate that version.
4. Check the node application logs to verify that the Agent successfully applied the rollback.
