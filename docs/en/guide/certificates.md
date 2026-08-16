# TLS Certificates and Auto-Renewal

This guide explains how to manage TLS certificates in OpenFlare. To secure traffic with HTTPS, you need to configure the corresponding certificate. OpenFlare supports **manually importing existing certificates** and **automatic issuance and managed renewal via ACME**.

---

## Method 1: Manually Import an Existing Certificate

If you have obtained a free or paid certificate from a third-party provider (such as Tencent Cloud, Alibaba Cloud, etc.), or generated a self-signed certificate locally:

1. Log in to the admin panel, go to **「Website Management」->「TLS Certificates」** in the left navigation.
2. Click **「Import Certificate」** in the top-right corner.
3. Fill in the configuration:
   * **Certificate Name**: Enter an easily recognizable alias (e.g. `my-domain-cert`).
   * **Certificate Content (PEM)**: Paste the PEM-format certificate public key (usually starts with `-----BEGIN CERTIFICATE-----`).
   * **Private Key (KEY)**: Paste the certificate private key (usually starts with `-----BEGIN PRIVATE KEY-----` or `-----BEGIN RSA PRIVATE KEY-----`).
4. Click **「Save」**. After a successful import, the certificate can be directly bound when configuring domains.

---

## Method 2: Automatic Issuance and Auto-Renewal (ACME)

OpenFlare has a built-in ACME client integrated with the **Asynq async task queue**. With the DNS API of your cloud DNS provider, the system can automatically complete DNS-01 challenge validation, apply for wildcard/single-domain certificates from a CA (Let's Encrypt by default), and **automatically trigger renewal 7 days before expiry**.

### Step 1: Create a DNS API Token in Cloudflare

To let OpenFlare automatically add TXT records under your domain for DNS validation, you need a Cloudflare API Token with specific permissions.

> [!IMPORTANT]
> For security, **it is strongly recommended to use a permission-scoped API Token** rather than the Global API Key.

1. Log in to the [Cloudflare dashboard](https://dash.cloudflare.com/).
2. Click the user avatar in the top-right corner and select **「My Profile」**.
3. In the left menu select **「API Tokens」**, then click **「Create Token」**.
4. Find the **「Edit Zone DNS」** template and click **「Use template」**.
5. Configure the token permissions and scope (keep defaults or restrict as needed):
   * **Permissions**:
     * `Zone` - `DNS` - `Edit` (required, for ACME to write TXT records)
     * `Zone` - `Zone` - `Read` (required, to list and retrieve zone IDs)
   * **Zone Resources**:
     * Select **「Include」** -> **「All zones」**, or select **「Specific zone」** and point to the specific domain you manage.
6. Click **「Continue to summary」**, confirm, then click **「Create Token」**.
7. Copy the generated **API Token** string. It is only shown once, so save it carefully.

### Step 2: Add a DNS Account in the Control Plane

1. Log in to the OpenFlare admin panel, go to **「Website Management」->「DNS Accounts」**.
2. Click **「Add Account」**.
3. Fill in the configuration:
   * **Account Name**: e.g. `cloudflare-main`.
   * **DNS Provider**: Select `Cloudflare`.
   * **API Token**: Paste the API token copied from Cloudflare (stored encrypted automatically).
4. Click **「Save」**.

### Step 3: Submit a Certificate Application Task

1. Go to **「Website Management」->「TLS Certificates」**, click **「Apply for Certificate」** in the top-right corner.
2. Fill in the application form:
   * **Certificate Name**: Custom name (e.g. `wildcard-example-cert`).
   * **Primary Domain**: The domain to apply for (wildcards supported, e.g. `example.com` or `*.example.com`).
   * **Associated Domains**: Append more domains if any (wildcards supported, comma-separated).
   * **DNS Account**: Select the DNS account just added from the dropdown (e.g. `cloudflare-main`).
3. Click **「Save and Apply」**.

### Step 4: Track Application Progress and Renewal Status

- **Real-time progress**: After saving, the system delivers a certificate renewal/application task (`of_ssl_single_renew`) to the Asynq queue. You can view detailed step-by-step logs (adding TXT records, DNS record global propagation probing, ACME validation, certificate issuance, etc.) in the admin task or node log pages.
- **Automatic renewal**: All certificates issued via ACME are automatically managed by the system. The background Scheduler scans certificate validity daily and automatically triggers renewal via async tasks 7 days before expiry — no manual maintenance needed.
