# Create Your First Reverse Proxy Rule

Create a basic HTTP site and verify that OpenFlare Edge applied its configuration.

After you save a rule, the controller automatically notifies online edge nodes to sync.

---

## Pre-Release Checks

Before starting, make sure the following conditions are met:

| Check | Required State |
| --- | --- |
| **Server** | Control panel is running and accessible |
| **Agent** | At least one edge node is online (verified under Node Management) |
| **Origin** | Backend service is reachable from the edge node host |
| **Domain/Testing** | DNS resolution configured, or client prepared to test via local hosts / curl Host header |

---

## Step 1: Create the First Website Config

For baseline verification, configure a plain HTTP reverse proxy route:

1. Log in to the control panel, navigate to **Website Management → Domain List**, and click **Add Zone**.
2. Register the domain configuration:
   * **Domain**: Enter a domain for testing (such as `first.example.com`).
   * **Bind Certificate**: Leave unbound for initial HTTP testing.
   * Click Save to complete domain registration.
3. Navigate to **Rule Management** and click **New Rule**:
   * **Rule Name**: Enter an identifier (such as `first-app-route`).
   * **Domain Match**: Select the test domain (`first.example.com`).
   * Under the **Reverse Proxy** tab, set **Origin Mode** to "Direct Upstream".
   * **Upstream Address**: Enter the backend address (such as `http://httpbin.org`).
   * Click Save to create the rule.

> [!TIP]
> **About HTTPS and Certificates**
> This section guides rapid deployment of a basic HTTP rule. To import existing TLS certificates or automate issuance via ACME for port 443 HTTPS proxying, see [Create a Reverse Proxy Config](./proxy-config.md).

---

## Step 2: Verify the Edge Applied the Configuration

Online nodes sync upon receiving controller notifications. Nodes with disconnected WebSockets discover updates during periodic heartbeats.

1. **Admin Verification**: In **Node Management**, open the target node's details and confirm that the latest entry in **Apply Records** reports success.
2. **Edge Node Verification**: Inspect logs directly on the edge node host:
   ```bash
   journalctl -u openflare-edge -n 50 --no-pager
   ```
3. **Connectivity Test**:
   Send a test request from a client machine using `curl` with a `Host` header pointed at the edge node IP:
   ```bash
   curl -I -H "Host: first.example.com" http://AGENT_NODE_IP
   ```
   The returned HTTP status code matches the response from the backend origin.
