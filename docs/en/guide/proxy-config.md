# Create a Reverse Proxy Config

This guide explains how to prepare certificates and origins, create reverse proxy routes, and verify configuration deployment on edge nodes.

---

## Recommended Workflow

In the gateway control plane, follow these steps to add a reverse proxy rule:

```text
  [ Step 1. Certificates ] ──► [ Step 2. Define Origins (optional) ] ──► [ Step 3. Create Website Route ]
                                                                                  │
  [ Step 4. Verify Access & Sync ] ◄──────────────────────────────────────────────┘
```

---

## Step 1: Prepare Certificates

Before securing traffic with HTTPS, prepare the corresponding TLS certificate (either by importing existing certificates or issuing and managing renewals automatically via ACME).

To keep this guide focused, certificate workflows (including generating dedicated Cloudflare DNS API tokens) are documented in **[TLS Certificates & Auto-Renewal](./certificates.md)**. Complete certificate preparation there before returning to this guide.

---

## Step 2: Prepare Upstream Origins (Optional)

An origin represents the backend server address being proxied. While IP addresses can be entered directly when creating a route, registering them in the origin library allows reuse and centralized maintenance:

1. Navigate to **Website Management → Origin Addresses** in the left menu, then click **Add Origin**.
2. Enter an origin name (such as `production-api`).
3. Enter a valid upstream address (such as `http://10.0.0.10:8080`), then click Save.

---

## Step 3: Create Website Configuration

Once certificates and origins are ready, configure the proxy route:

1. Navigate to **Website Management → Domain List**, click **Add Zone**, and register the root zone (such as `example.com`; wildcard entries like `*.example.com` are not permitted as zones). Navigate to that Zone's domain list to add specific or wildcard domain names:
   * **Domain**: Enter `api`, `@`, `*`, or a full hostname (such as `*.s3.example.com`). Entering `*` expands to `*.example.com`; shorthands like `*.s3` are rejected.
   * **Bind Certificate**: Select the certificate prepared in Step 1.
   * To clean up unused domain records in bulk, click **Clean Unbound Domains** on the Zone's **Domains** tab to remove domain records not attached to proxy routes.
2. Configure routing rules: navigate to **Rule Management**, then click **New Rule** or edit an existing rule:
   * **Rule Name**: Enter a unique identifier (such as `app-portal-route`).
   * **Domain Match**: Select registered domains or wildcard domains, or use **Quick Add Domain** to register and attach domains inline.
   * Under the **Reverse Proxy** tab, set **Origin Mode** to "Direct Upstream".
   * **Origin Selection**: Select an existing origin from the dropdown, or enter an address manually (such as `http://10.0.0.20:9000`).
3. Click Save to create the configuration.

### Using IP Addresses for HTTPS Upstreams

Upstreams can be configured with raw IP addresses, such as `https://203.0.113.10` or `https://[2001:db8::10]:8443`. When **Origin Host Header** is left empty, the edge node connects to the configured IP and port while using the requested hostname as the upstream TLS SNI and certificate validation name, forwarding the client `Host` header unchanged. Wildcard routes pass the client's concrete subdomain. When upstream certificate verification is enabled, the origin certificate must cover the requested hostname.

If the origin requires a fixed domain name, specify that domain in **Origin Host Header** to set both upstream TLS SNI and the HTTP `Host` header. When the upstream address itself uses a domain name and this field is left empty, the upstream TLS SNI uses the upstream domain while the client `Host` header passes through.

### Configuring Access Authentication

1. Bind TLS certificates and enable HTTPS for all domains on the site. Open **Access Authentication** in the rule details, set authentication mode to **Identity Authentication**, and configure a username and password.
2. Set **Session Lifetime**. Authenticated visitors retain access for this duration (defaults to 24 hours; configurable from 1 to 720 hours). Modifying credentials or lifetime requires visitors to log in again.
3. Navigate to **Response Pages → Authentication Page** and click the preview area or **Edit** to open the HTML editor. Load built-in templates to customize layout, text, and styles with real-time preview. Saved templates are capped at 256 KiB. All sites with Identity Authentication share this login page.

Custom login pages use same-page POST forms; omitting the `action` attribute preserves the original redirect URL. The `{{login_error}}` placeholder is replaced with error messages upon failed attempts.

```html
<form method="post">
  <label>Username <input name="username" autocomplete="username" required></label>
  <label>Password <input name="password" type="password" autocomplete="current-password" required></label>
  <p role="alert">{{login_error}}</p>
  <button type="submit">Sign In</button>
</form>
```

Authentication and session validation execute entirely on edge nodes. Authenticated requests validate cookie signatures locally without contacting the control plane. Plain HTTP requests redirect to HTTPS automatically.

### Wildcard Domain Example

To share a single routing rule across subdomains like `bucket.s3.example.com`:

1. Under the `example.com` Zone, add `*.s3.example.com` and bind the route, avoiding individual bucket domain registration.
2. For HTTPS traffic, bind a certificate whose SAN includes `*.s3.example.com`. A certificate for `*.example.com` does not cover second-level wildcards; similarly, `*.s3.example.com` does not cover `s3.example.com` or deeper subdomains like `a.bucket.s3.example.com`.
3. In Cloudflare groups, select this wildcard domain: Node mode writes A records matching node IPv4 addresses, while Custom mode allows A, AAAA, or CNAME records. Verify DNS propagation and node configuration application independently.
4. After saving routes, Pingora synchronizes wildcard matching rules. Concrete domains take precedence over wildcards, and overlapping wildcards resolve by longest suffix match.
5. Verify access using a concrete hostname, such as `https://bucket.s3.example.com`. Zone statistics aggregate requests under the actual requested host. Uptime Kuma requires at least one concrete domain bound to the route; otherwise, monitoring task creation is skipped.

Malformed entries such as `foo*.example.com`, `a.*.example.com`, or multiple `*` wildcards are rejected.

---

## Step 4: Verify Synchronization and Access

After saving a route, the controller notifies online edge nodes to pull and validate the new snapshot. If candidate configuration fails validation, the node continues serving the last healthy snapshot and reports the failure under **Apply Records**.

### 1. Verify Access
Verify configuration using:
* **Browser Access**: Open `https://your-domain.com` in a browser to confirm upstream proxying.
* **CLI Validation**: Probe with `curl`:
  ```bash
  curl -I https://your-domain.com
  ```
* **Bypass DNS**: If DNS resolution is not yet configured, specify the `Host` header against the node's physical IP:
  ```bash
  curl -I -H "Host: your-domain.com" https://AGENT_NODE_IP --insecure
  ```

### 2. Handling Apply Failures

Inspect failure reasons under **Apply Records** in node details. Correct the configuration and save again; nodes automatically fetch the new snapshot. If the underlying node environment was repaired, trigger **Force Sync** to re-attempt application.

---

## Edge Caching (Optional)

Enable caching and configure matching rules under the site's **Cache** tab. Cache capacities and object limits on edge nodes are configured in [Pingora Performance Policies](./performance-policies.md).

### Recommended Settings

| Item | Recommendation |
| --- | --- |
| Strategy | **Standard Static Assets** (recommended default): covers only css/js/map/images/fonts, **excluding HTML/JSON** |
| Session Cookies | Requests with session cookies are **not** automatically bypassed; authenticated users still hit static assets |
| Origin Headers | Static assets should return `Cache-Control: public, max-age=…`; dynamic responses must declare `private` or `no-store` |
| Set-Cookie Response | Responses containing `Set-Cookie` are not stored in edge cache |
| Missing Cache Headers | Applies default Edge TTL based on status code (~120 minutes for HTTP 200) |

### Advanced Strategy: All Cacheable GET

Similar to Cloudflare's Cache Everything: paths are no longer filtered by file extension. If the origin omits `private` or `no-store` headers on HTML responses, **personalized content may be cached and served across users**. Use this strategy only when origin cache-control headers are accurate or responses are strictly public.

### How It Takes Effect

Saving site cache rules causes the controller to synchronize cache policies to edge nodes automatically.

### Quick Verification Checklist

1. Node performance policy enables caching, site cache is enabled, and strategy is set to "Standard Static Assets".
2. Configuration is saved and nodes report successful application.
3. Make two consecutive requests with session cookies to the same `/assets/app.js` asset; access logs should report **HIT** on the second request.
4. If status remains Not Cached, verify file extensions, ensure the method is GET, confirm the origin does not set `Set-Cookie` or `private`, and verify node application status. See [Troubleshooting · Edge Cache](./troubleshooting.md#edge-cache-hit-rate-anomalies).
