# WAF Security Protection

OpenFlare WAF orchestrates rules with a visual directed acyclic graph. When creating a rule you only enter a name; the system creates the default "start → pass" graph and enters the editor.

## Nodes and Connections

- **Start**: unique per rule; enters the graph along `next`.
- **Pass**: ends the current rule; if the route has more rules, execution continues.
- **Block**: immediately terminates the request with the configured status code and HTML response.
- **IP match**: configure an IP, CIDR, or IP group; connect `true` / `false` respectively.
- **Geo match**: branch by country or ISO 3166-2 first-level division code; the country list shows both localized names and codes; divisions are searchable by country name, division name, or code. Country and City MMDB are provided by disk files (Docker images COPY them to the default path; bare binaries download from the configured URL on first startup) and update on the configured cycle. When City MMDB is unavailable, it is treated as no match.
- **PoW**: takes over the request when the challenge is incomplete; after verification passes, continues along `next`.

The server rejects cycles, dangling outlets, unreachable nodes, duplicate port connections, and invalid configs. Save carries the `revision` obtained at page load; on a 409 conflict, reload to avoid overwriting others' changes.

Select a normal node or edge, then click the delete button at the canvas top-right, or press Delete/Backspace. Deleting a node also deletes associated edges; the single "start" and "pass" nodes cannot be deleted. Dragging a node only records the final coordinates on release; the canvas state is not rebuilt repeatedly during movement.

The right node property panel is hidden by default and shows after clicking a node; it auto-collapses when clicking an edge or blank canvas.

The orchestration area defaults to a compact height and smaller initial zoom; you can still zoom freely with the wheel or canvas Controls.

## Binding and Effect

Enabled global rules always execute first; route-bound custom rules execute strictly in list order. After adjusting the order, you must publish a config version for the rule topology to take effect with the OpenResty reload.

IP group members are dynamic resources. The Agent checks the checksum every 5 seconds and updates in-memory snapshots across workers on change — no rule republish or reload needed. Manual, subscription, and auto IP groups can all be referenced by IP-match nodes. A single full IP group runtime snapshot is capped at 20 MiB; exceeding the cap makes publish or sync return an error and keeps using the previous valid snapshot.

> [!IMPORTANT]
> When upgrading from the legacy fixed allow/block-list, geo, or PoW forms, rule graphs reset to "start → pass" and old policy fields are not migrated. Re-orchestrate and verify each rule before publishing a new version.

Architecture, graph validation, and failure rollback details: [WAF Orchestration Rule Design](../design/waf-orchestration-design.md).
