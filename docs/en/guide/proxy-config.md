# Create a Reverse Proxy Config

You will learn: how to create and publish a reverse proxy website configuration from scratch, step by step, in OpenFlare. This guide walks you through certificate import and application, origin definition, route rule configuration, version release, and connectivity verification.

---

## Recommended Workflow

In the gateway control plane, follow these steps to add a new reverse proxy rule:

```text
  [ Step 1. Certificate Management ] ──► [ Step 2. Origin Definition (optional) ] ──► [ Step 3. Add Website Config ]
                                                                   │
  [ Step 5. Verify Access ] ◄── [ Step 4. Publish & Activate Version ] ◄───────────────┘
```

---

## Step 1: Prepare Certificates

Before using HTTPS-secured traffic, you need to prepare the corresponding TLS certificate (supports manually importing an existing certificate, or automatically applying from a CA via DNS validation with managed renewal).

To keep this guide concise, the certificate details (including creating a dedicated DNS API Token in Cloudflare) have been split into a dedicated guide. First go to **[TLS Certificates & Auto-Renewal](./certificates.md)** to prepare the certificate, then come back to continue.

---

## Step 2: Prepare the Upstream Origin (optional)

An Origin represents the backend real service address being proxied. Although you can fill in an IP directly when creating a website, it is recommended to register origins in the origin library first for reuse and maintenance:

1. Go to **「Website Management」->「Origin Addresses」** in the left navigation, click **「Add Origin」**.
2. Fill in the origin name (e.g. `production-api`).
3. Enter a valid upstream address (e.g. `http://10.0.0.10:8080`) and save.

---

## Step 3: Create the Website Config

Once the certificate and origin are ready, create the core website proxy route:

1. Go to **「Website Management」->「Domain List」**, click **「Add Zone」**:
   * **Domain**: Enter the domain bound to this site.
   * **Bind Certificate**: Select the certificate prepared or applied for in Step 1.
2. Configure the request route rule: go to the **「Rule Management」** page, click **「New Rule」** or edit an existing rule:
   * **Rule Name**: Enter a unique simple identifier (e.g. `app-portal-route`).
   * **Domain Match**: Enter the corresponding domain (wildcards or exact domains supported; must match the registered domain above).
   * In the **「Reverse Proxy」** tab below, select the origin mode as「Direct Upstream」.
   * **Origin Selection**: Select the origin created in Step 2 from the dropdown; or choose manual input and fill in `http://10.0.0.20:9000`.
3. Click save to create the config.

---

## Step 4: Publish and Apply the Config

Website configs added in the admin panel are only saved in the Server database — **they do not take effect immediately**. You must generate a config version snapshot and distribute it to the Agent edge nodes:

1. Click the **「Preview and Publish」** button in the top-right corner of the control panel.
2. Review the config file diff, confirming the newly added `server` block and certificate binding rules are correct.
3. Click **「Confirm Publish」**.
4. **Agent application mechanism**:
   * The Agent node on the data plane detects the active version Checksum change in its heartbeat, and automatically pulls the full OpenResty config files and certificate bundle locally.
   * It automatically runs a local config validation (similar to `openresty -t`); after confirming no syntax errors, it performs a smooth reload.
   * If reload or validation fails, the Agent safely blocks and rolls back to the previous stable version to keep the node highly available.

---

## Step 5: Connectivity and Rollback Verification

### 1. Verify Access
You can verify the new config takes effect as follows:
* **Browser access**: Open `https://your-domain.com` directly in a browser and check whether it proxies the backend successfully.
* **CLI verification** (recommended): probe with `curl`:
  ```bash
  curl -I https://your-domain.com
  ```
* **Bypass DNS validation**: if your domain is not yet resolvable, temporarily send a `Host` header request to the Agent node's physical IP:
  ```bash
  curl -I -H "Host: your-domain.com" https://AGENT_NODE_IP --insecure
  ```

### 2. One-Click Second-Level Rollback
If the released config causes an online business issue:
1. Navigate to the **「Version Release」** menu on the left.
2. Find the previous stable version before the release in the history list.
3. Click **「Activate」**.
4. All online Agent nodes will automatically reload the historical config within seconds for second-level risk avoidance.

---

## Edge Cache (optional)

The **「Cache」** page in the site details can enable edge `proxy_cache` (requires **Performance Settings → Global OpenResty Cache** to be enabled at the same time). Behavior mirrors the Cloudflare default model; see [Edge Cache Strategy Design](../design/edge-cache-design.md).

### Recommended Settings

| Item | Suggestion |
| --- | --- |
| Strategy | **Standard static assets** (recommended default): only css/js/map/images/fonts, **not HTML/JSON** |
| Login Cookie | Is **not** separately skipped from caching; users with sessions can still hit static assets |
| Origin | Static assets: `Cache-Control: public, max-age=…`; dynamic/personalized must be `private` or `no-store` |
| Response Set-Cookie | Is not written to the edge cache |
| No origin cache headers | Uses default Edge TTL by status code (e.g. ~120 min for 200) |

### Advanced Strategy「All Cacheable GET」

Similar to Cloudflare Cache Everything: the path is no longer limited by extension. If the origin does not declare `private`/`no-store` for HTML, **personalized pages may be cached and served across users**. Use only when origin cache headers are correct or content is globally consistent.

### How It Takes Effect

The cache switch and strategy are written into the config snapshot. After saving the site, you must **publish and activate the config version** for the Agent to apply it. Changing the UI only without publishing leaves nodes on the old rules.

### Quick Self-Check

1. Global cache on, site cache on, strategy「Standard static assets」.  
2. Publish the config and confirm nodes applied successfully.  
3. Request the same `/assets/app.js` (or a hashed immutable path) twice with a login cookie; the `cache_status` in access logs should be **HIT** on the second request.  
4. If still「not cached」: check whether the strategy matches the path extension, whether it is a non-GET request, whether the origin returns `Set-Cookie` / `private`, and whether the node applied the new version. More in [Troubleshooting · Edge Cache](./troubleshooting.md#edge-cache-hit-rate-anomalies).
