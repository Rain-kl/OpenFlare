---
title: CLI and MCP
description: Authorize OpenFlare CLI and MCP for routine site operations and troubleshooting.
---

# CLI and MCP

Open **System settings → MCP** in the console. Save an HTTP or HTTPS origin reachable by clients to see the MCP URL, instance ID, and connection options. Use HTTPS for public deployments. Some MCP clients require HTTPS.

- **OAuth:** Copy the connection instructions, or add a Streamable HTTP connection manually using OAuth. The client must support dynamic client registration and S256 PKCE. On the first connection, sign in as an administrator. On the consent page, select a group title to expand its permission list. Both permission switches are off by default. Enable a group and confirm to grant all the permissions listed in that group. For queries only, enable read permissions only.
- **Access token:** Create and manage an existing personal access token on the Personal Access Tokens settings page. Set `Authorization: Bearer TOKEN` in the client's custom request headers. Built-in MCP accepts only personal tokens with administrator privileges enabled; that token can call all MCP tools.

Call `openflare_connection_info` to verify the instance ID and granted permissions. OAuth access tokens last 15 minutes and can be refreshed for up to 30 days. **Revoke all OAuth grants** immediately invalidates the account's CLI and MCP OAuth grants. Manage personal access tokens through the existing rotation and revocation controls.

If the settings tab reports a short session secret, check `app.session_secret` or `APP_SESSION_SECRET`. Follow the [session secret rotation guide](./session-secret-rotation.md) to check and migrate stored credentials.

Download the matching `openflare_cli_*` archive from [OpenFlare Releases](https://github.com/Rain-kl/OpenFlare/releases) and add the extracted binary to `PATH`. Linux requires an available Secret Service credential store.

```sh
openflare auth login --server https://openflare.example.com
openflare connection info
openflare operations list
openflare site create --help
```

Read and write commands use the saved default connection. To manage another instance, sign in with a name using `openflare auth login --profile staging --server https://staging.example.com`, then pass `--profile staging` on commands for that instance.

Use `zone list` and `domain list` to resolve IDs before creating or binding sites. An HTTPS site requires existing certificates that cover its domains. Site writes do not change public DNS or issue certificates. Use `node get` to check whether the configuration has been applied to the edge.
