# Create Your First Reverse Proxy Rule

Create a basic HTTP site and verify that Pingora Edge applied its configuration.

After you save a rule, the controller automatically notifies online edge nodes to sync.

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

## Step 2: Verify the Edge Applied the Configuration

Online nodes sync when notified. Nodes with a disconnected WebSocket discover changes through their periodic heartbeat.

1. **Admin-side verification**: go to「Node Management」-> click the node to open details; check that the latest entry in **Apply Records** shows success.
2. **Edge node verification**: check application via logs on the Agent host:
   ```bash
   journalctl -u openflare-edge -n 50 --no-pager
   ```
3. **Connectivity test**:
   On the client machine, use `curl` with a test Host header against the Agent node's IP for final verification:
   ```bash
   curl -I -H "Host: first.example.com" http://AGENT_NODE_IP
   ```
   If the returned status code matches the backend origin's response, your first reverse proxy rule has successfully landed on the edge node!
