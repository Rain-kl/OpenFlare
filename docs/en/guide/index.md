# Guide Overview

You will learn: How the OpenFlare documentation is organized, which pages to read when running it for the first time, and where to start for deployment, usage, troubleshooting, and development.

OpenFlare is a self-hosted OpenResty control plane. It integrates reverse proxy website configurations, configuration version publishing, Agent node synchronization, TLS certificates, and basic observability into a single management console, making it ideal for a single team or organization managing multiple proxy nodes.

## Recommended Reading Path

If you are new to OpenFlare, read the documents in the following order:

1. [Quick Start](./quick-start.md): Start the Server using Docker Compose, log into the management console, and connect your first Agent.
2. [Basic Usage](./usage.md): Learn common operations for website configs, origins, certificates, publishing, rollbacks, and observability.
3. [Tunnel & Intranet Penetration](./tunnel-usage.md): Learn to deploy Relay and Client to achieve secure, public IP-free reverse penetration.
4. [WAF Security Protection](./waf-usage.md): Master IP whitelisting/blacklisting, WAF auto IP group aggregation Expr rules, geographical restrictions, and PoW CC protection.
5. [WAF Auto IP Group Expressions](./waf-ip-group-expr.md): Write auto IP group Expr rules and learn keyword definitions and presets.
6. [Deployment Guide](../deployment/deployment.md): Deploy Server and Agent in closer-to-production environments.
7. [Configurations Reference](../reference/configuration.md): Check Server environment variables, runtime Options, and Agent configurations.
8. [Troubleshooting](./troubleshooting.md): Troubleshoot login, database, node sync, OpenResty application, and frontend build issues.

## Role-Based Entrypoints

| What do you want to do? | Recommended Entrance |
| --- | --- |
| Run the console in under 5 minutes | [Quick Start](./quick-start.md) |
| Publish your first reverse proxy configuration | [Publish First Configuration](./first-site.md) |
| Configure intranet penetration mapping | [Tunnel & Intranet Penetration](./tunnel-usage.md) |
| Configure CC protection & IP group blocking | [WAF Security Protection](./waf-usage.md) |
| Write auto IP group aggregation rules | [WAF Auto IP Group Expressions](./waf-ip-group-expr.md) |
| Connect or reinstall a node Agent | [Access Agent](../deployment/agent.md) |
| Start Server from source code | [Launch Server](../deployment/server.md) |
| Configure GitHub or OIDC SSO | [SSO Login Configuration](./sso.md) |
| Upgrade Server or Agent | [Upgrade & Maintenance](../deployment/upgrade.md) |
| Participate in development or bug fixing | [Local Development](../design/development.md) and [Development Constraints](../../guideline/Constraints.md) |
| Understand architecture and publishing | [System Architecture](../design/architecture.md) and [Agent & Publish Model](../design/agent-design.md) |
| View open-source references and credits | [Credits](./credits.md) |

## Documentation Partitions

`guide/` is oriented toward users and deployers, providing actionable steps from installation to daily operations.

`reference/` collects stable facts such as configuration fields, commands, API response structures, and repository layout.

`design/` is oriented toward maintainers and contributors, describing product boundaries, system architecture, Agent & publishing models, and engineering constraints. Before adding capabilities or changing boundaries, update the corresponding design document first.
