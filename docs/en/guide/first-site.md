# Publish First Configuration

You will learn: how to create the first reverse proxy rule in the simplest way, publish a config version, and confirm the Agent has pulled and applied the config.

OpenFlare's release chain centers on "immutable config versions". After you modify rules in the admin panel, you must publish and activate a new version for online Agents to auto-sync and apply.

---

## Pre-Release Checks

Before starting, make sure the following conditions are met:

| Check | Required State |
| --- | --- |
| **Server** | control panel started normally and you can log in to the admin panel |
| **Agent** | at least one Agent node online (confirmable in「Node Management」) |
| **Origin** | your backend origin service is reachable from the Agent host |
| **Domain/testing** | the domain's DNS resolves, or you're ready to test with local hosts / curl Host header on the client |

---

## Step 1: Create the First Website Config

For a quick verification, deploy a basic HTTP reverse proxy site first:

1. Log in to the control panel, go to **「Website Management」->「Domain List」** in the left navigation, click **「Add Zone」**.
2. Fill in the domain config:
   * **Domain**: enter the test domain (e.g. `first.example.com`).
   * **Bind Certificate**: choose not to bind a certificate (for HTTP quick verification).
   * Click save to complete domain registration.
3. Go to **「Rule Management」**, click **「New Rule」**:
   * **Rule Name**: enter a simple identifier (e.g. `first-app-route`).
   * **Domain Match**: fill in your test domain (e.g. `first.example.com`).
   * In the **「Reverse Proxy」** tab below, set the origin mode to「Direct Upstream」.
   * **Upstream Address**: fill in the backend service address (e.g. the test-only `http://httpbin.org`).
   * Click save to create the rule.

> [!TIP]
> **About HTTPS and certificate preparation**
> This section only guides the quick deployment of a basic HTTP rule. To import an existing SSL certificate or auto-issue one from Let's Encrypt via ACME and enable HTTPS proxying on port 443, go to [Create a Reverse Proxy Config](./proxy-config.md) for detailed steps.

---

## Step 2: Preview and Publish a Config Version

The new website config is still a draft in the Server database and needs a released version to be distributed to the data plane:

1. Click the **「Preview and Publish」** button in the top-right of the control panel; the system shows the physical config file diff for the newly added route.
2. After confirming the rendered config is correct, click **「Confirm Publish」**.
3. The control plane generates a unique config version number (format `YYYYMMDD-NNN`).

---

## Step 3: Verify the Agent Applied It

After publishing, the control plane immediately notifies online Agents via WebSocket (if the WebSocket is offline, the Agent detects it as a diff in its heartbeat):

1. **Admin-side verification**: go to「Node Management」-> click the node to open details; check that the**current version number** has changed to the just-published latest active version and the「Apply Records」show success.
2. **Edge node verification**: check application via logs on the Agent host:
   ```bash
   # If the Agent is Docker-deployed
   docker logs openflare-agent
   
   # If the Agent is deployed with local systemd
   journalctl -u openflare-agent -n 50 --no-pager
   ```
3. **Connectivity test**:
   On the client machine, use `curl` with a test Host header against the Agent node's IP for final verification:
   ```bash
   curl -I -H "Host: first.example.com" http://AGENT_NODE_IP
   ```
   If the returned status code matches the backend origin's response, your first reverse proxy rule has successfully landed on the edge node!
