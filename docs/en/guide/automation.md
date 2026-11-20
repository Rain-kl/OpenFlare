---
title: CLI and MCP operations
description: Authorize OpenFlare CLI and MCP for routine site operations and troubleshooting.
---

# CLI and MCP operations

In the console, open **Settings → Operation settings → CLI and MCP**. Save the public HTTPS origin, open the MCP connection instructions, and copy them to the AI client. Use a client that supports Streamable HTTP, OAuth S256 PKCE, and dynamic client registration. The first connection requires administrator sign-in and consent in a browser.

- **Read-only access** enables site, domain, certificate, node, log, metric, and performance policy queries.
- **Operations access** also enables site creation, domain registration and binding, direct upstream replacement, and changes to existing performance policy parameters. OpenFlare does not require approval for each authorized call.

After connecting, verify the instance ID and effective scopes using `openflare_connection_info`. Grants apply to the current instance and are limited by the requested scopes. Revoke your grants from the connection instructions page.

Download the matching `openflare_cli_*` archive from [OpenFlare Releases](https://github.com/Rain-kl/OpenFlare/releases) and add the extracted binary to `PATH`. Linux requires an available Secret Service credential store.

```sh
openflare auth login --profile prod --server https://openflare.example.com
openflare connection info --profile prod
openflare operations list --profile prod
openflare site create --help
```

Write commands require an explicit `--profile`. Use `zone list` and `domain list` to resolve IDs before creating or binding sites. An HTTPS site requires existing certificates that cover its domains. Site writes do not change public DNS or issue certificates. Use `node get` to check whether the configuration has been applied to the edge.
