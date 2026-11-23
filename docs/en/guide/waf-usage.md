# WAF Security Protection

OpenFlare WAF orchestrates security rules using a visual directed acyclic graph (DAG). When creating a rule, you specify a name, and the system generates a default "Start → Pass" graph inside the visual editor.

## Nodes and Connections

- **Start**: Unique per rule; request execution enters the graph along `next`.
- **Pass**: Concludes the current rule; if subsequent rules are configured on the route, execution continues.
- **Block**: Terminates the request immediately with the configured HTTP status code and response HTML.
- **IP Match**: Configures IP addresses, CIDR blocks, or IP groups, branching into `true` and `false` paths.
- **Geo Match**: Branches by ISO country code or ISO 3166-2 division code. The country selector displays names and codes; divisions are searchable by country, region name, or code. GeoIP data is loaded from MaxMind MMDB files on disk and updated periodically. If City MMDB is unavailable, requests evaluate as unmatched.
- **Attack Protection**: Selects an attack protection rule set. Matches from Essential, Balanced, High protection, or individually enabled rules branch to `false`. Observe-only matches are logged and branch to `true`.
- **Legacy PoW Node**: Retained for backward compatibility. New configurations should use dedicated [Proof-of-Work (PoW) Protection](./pow-usage.md). When a site saves a standalone PoW policy, legacy PoW nodes bypass execution along `next`.

The server rejects cyclic graphs, dangling outlets, unreachable nodes, duplicate connections, and malformed parameters. Rule saving includes an optimistic concurrency `revision` token; on HTTP 409 conflict, reload the editor to avoid overwriting concurrent modifications.

Selecting a node or edge displays deletion controls or allows removal via Delete/Backspace. Deleting a node also removes its connected edges; the root "Start" and "Pass" nodes cannot be removed. Moving nodes records coordinates only upon release, preventing redundant state updates.

The right-hand property inspector is hidden by default and opens when selecting a node; clicking an edge or empty canvas area collapses it.

## Attack Protection Rule Sets

1. In **Security → Attack Protection**, create or edit a rule set. Use the back button at the top left to return to the list and the save button above the form to save changes. Set each category to Disabled, Observe Only, Essential Protection, Balanced, or High Protection.
2. In **Security → WAF**, edit the workflow, add an **Attack Protection** node, and select the rule set. Connect `false` to a **Block** node to reject matching traffic, and connect `true` to **Pass** or downstream inspection nodes.
3. Enable the workflow and attach it to target proxy routes. Changes to rule sets synchronize automatically with site configuration snapshots. Edge nodes must run a version supporting `zentinel-modsec` to apply rule sets.

Open the **Detection Rule Catalog** to filter rules by attack category or search by rule ID and description. The catalog can be opened from the rule set toolbar or by clicking active rule counts for a specific category.

Individual rule actions can be overridden in the catalog: **Disable** skips detection, **Block** routes matches to the `false` branch, and **Log** writes an entry to attack logs while routing requests along the `true` branch. Per-rule actions take precedence over category-level paranoia settings; selecting **Inherit Category Setting** restores default category behavior. Category-level adjustments preserve custom individual overrides, and all modifications persist with the rule set.

Balanced protection skips SQL injection rule `942100` by default. Set it to **Log only** in the catalog to record its matches. Existing per-rule settings continue to apply.

Edge nodes evaluate built-in OWASP CRS 4.30.0 request inspection rules using `zentinel-modsec`. Essential protection includes 22 rules for specific attack signatures and protocol violations. Balanced protection includes 64 rules. High protection and observe-only mode run all rules in the selected category, including CRS PL1 through PL4 and OpenFlare supplemental rules.

Rule evaluation inspects request URIs, query parameters, headers, and cookies. Request body inspection applies to payloads with declared lengths up to 64 KiB, including JSON, URL-encoded forms, XML, and multipart data.

- **SSRF**: Utilizes CRS Server-Side Request Forgery rules to detect suspicious internal addresses and protocols.
- **CSRF**: Uses OpenFlare supplementary rules to inspect non-safe HTTP methods carrying cookies. Essential protection leaves CSRF checks disabled by default. Balanced protection detects cross-origin requests. High protection also flags requests without a verifiable origin.
- **Malformed HTTP Protocol**: Uses CRS protocol rules to inspect abnormal request lines, headers, ranges, and encodings. Requests rejected during initial protocol parsing do not generate WAF log records.

## Global Protection Allowlist

Select **Add allowlist entry** under **Security → Attack Protection**. A request matching both conditions of any entry bypasses all global and site-bound WAF flows. Site authentication, standalone PoW, and rate limits still apply.

- **Domain** accepts an exact hostname such as `api.example.com` or a one-level wildcard such as `*.example.com`. This wildcard matches `api.example.com` and excludes the apex and deeper subdomains.
- **Path** supports exact, prefix, suffix, and RE2 regular expression matching. Matches are case-sensitive against the raw path, with percent-encoding preserved and the query string excluded. Examples include `/api/public/` as a prefix, `.webp` as a suffix, and `^/hooks/[0-9]+$` as a regular expression.

Allowlist changes synchronize through the site snapshot. Upgrade the relevant Edge nodes before using Essential protection or the allowlist. Existing Balanced protection groups use the revised presets after the Edge upgrade.

## Attack Logs

- **Attack Events**: Aggregates block records by source IP, hostname, and fixed 5-minute time windows, allowing inspection of individual log events.
- Both tabs provide filters for source IP, hostname, and action. Node filters, time ranges, and auto-refresh controls are grouped under **More Filters**.
- **Log Records**: Displays block and observation events with filters for client IP, domain, edge node, action, and timestamp. Details show trigger cause, WAF workflow, node ID, and rule set. Matched rules display CRS rule IDs and descriptions; OpenFlare supplementary rules display custom identifiers.
- **Request Packets**: Loads upon opening details, displaying parsed request lines, parameters, and headers. When edge nodes buffer request bodies, uncompressed UTF-8 JSON, URL-encoded form data, and plain text can be previewed up to 8 KiB with sensitive credentials masked.
- Packet storage follows access log spooling, storage, and retention configurations.

## Binding and Scope

Enabled global rules execute first; custom rules attached to routes execute strictly in ordered sequence. Saving rule order synchronizes the configuration to OpenFlare Edge nodes with the next site snapshot.

In the WAF rule set list under **Security → WAF**, click the site count under **Scope** to view attached websites. You can filter by name or domain, navigate to route WAF settings, or select multiple sites and click **Unbind** to remove rule set associations in bulk.

IP group memberships are dynamic resources. Nodes compare digest checksums in the configuration manifest and update in-memory tables without requiring route reloads. Manual, subscription, and automatic IP groups can be referenced by IP Match nodes. Single runtime IP group snapshots are capped at 20 MiB; exceeding this limit causes sync errors and retains the previous valid snapshot.

> [!IMPORTANT]
> When upgrading from legacy allow/block lists, geo-blocking, or old PoW forms, the workflow graph resets to "Start → Pass". Legacy fields are not migrated automatically; reconfigure and verify workflow rules individually.

For architecture, graph validation, and failure rollback specifications, see [WAF Orchestration Rule Design](../design/waf-orchestration-design.md).
