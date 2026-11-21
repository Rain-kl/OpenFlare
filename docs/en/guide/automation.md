---
title: CLI and MCP operations
description: Authorize OpenFlare CLI and MCP for routine site operations and troubleshooting.
---

# CLI and MCP operations

Open **System settings → MCP** in the console. Save the public HTTPS origin to see the MCP URL, instance ID, and connection options.

- **OAuth:** Copy the connection instructions, or add a Streamable HTTP connection manually using OAuth. The client must support dynamic client registration and S256 PKCE. On the first connection, sign in as an administrator, select the permissions to grant, and confirm.
- **Access token:** Select permissions and generate a token for a client that supports custom headers. The token is shown once and expires after 30 days. Set the `Authorization: Bearer TOKEN` request header in the client.

Call `openflare_connection_info` to verify the instance ID and granted permissions. OAuth access tokens last 15 minutes and can be refreshed for up to 30 days. **Revoke all access** immediately invalidates the account's CLI and MCP grants and access tokens.

Download the matching `openflare_cli_*` archive from [OpenFlare Releases](https://github.com/Rain-kl/OpenFlare/releases) and add the extracted binary to `PATH`. Linux requires an available Secret Service credential store.

```sh
openflare auth login --server https://openflare.example.com
openflare connection info
openflare operations list
openflare site create --help
```

Read and write commands use the saved default connection. To manage another instance, sign in with a name using `openflare auth login --profile staging --server https://staging.example.com`, then pass `--profile staging` on commands for that instance.

Use `zone list` and `domain list` to resolve IDs before creating or binding sites. An HTTPS site requires existing certificates that cover its domains. Site writes do not change public DNS or issue certificates. Use `node get` to check whether the configuration has been applied to the edge.
