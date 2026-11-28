# TLS certificates and auto-renewal

Import existing PEM certificates or automate single-domain and wildcard issuance via ACME DNS-01.

## Option 1: Import a PEM certificate

1. Open **Websites** > **TLS Certificates** and click **Import Certificate**.
2. Enter a name, PEM certificate chain (`-----BEGIN CERTIFICATE-----`), and private key (`-----BEGIN PRIVATE KEY-----` or `RSA PRIVATE KEY`), then save.

## Option 2: Automated ACME DNS-01 issuance

OpenFlare renews managed certificates automatically 7 days before expiration.

1. **Add a DNS account**: Open **DNS** > **DNS Accounts**, click **Add Account**, select your provider (Cloudflare Token with `Zone:Zone:Read` + `Zone:DNS:Edit`, Alibaba Cloud DNS, or Tencent Cloud DNSPod), and save.
2. **Verify scopes**: Confirm **Verification Status** shows **API Ready** and inspect visible zones (click **Refresh Scopes** when needed).
3. **Request certificate**: Open **Websites** > **TLS Certificates**, click **Request Certificate**, enter the name, primary domain (supports `*.example.com`), optional SANs, and the **DNS Account**, then click **Save & Request**.

Monitor issuance progress under **Admin** > **Task Management**.
