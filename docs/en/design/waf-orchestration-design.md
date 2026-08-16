# WAF Orchestration Rule Design

This document defines the target architecture, data model, execution semantics, release model, and migration boundaries for reworking OpenFlare WAF from a fixed decision chain into a visual directed acyclic graph (DAG). IP group sources and membership computation still follow [WAF Design](./waf-design.md); this document only changes how rules are composed and executed.

## Goals and Boundaries

When a user adds a WAF rule, they only enter a name. The Server immediately creates a legal default graph `start → pass`, and the frontend enters a standalone orchestration page based on React Flow. Users build policies by adding processing units, configuring nodes, and connecting branches — no more filling in fixed-order allow/block lists and PoW forms.

Supported nodes:

| Node | Count Constraint | Inputs | Outputs | Config |
| --- | --- | --- | --- | --- |
| Start | exactly one per graph | none | `next` | none |
| Pass | exactly one per graph | one or more | none | none |
| Block | multiple allowed | one or more | none | HTTP status code, HTML response body |
| IP match | multiple allowed | one or more | `true`, `false` | IP, CIDR, IP group ID |
| Geo match | multiple allowed | one or more | `true`, `false` | country code, region code |
| UA check | multiple allowed | one or more | `true`, `false` | UA required, browser/OS allowlist with and/or, block crawlers/abnormal UAs (excluding crawlers)/custom regex |
| Security | multiple allowed | one or more | `true`, `false` | basic signature detection (path traversal/file inclusion on by default; SQL/XSS/command injection/SSRF/upload/XXE/CRLF toggleable); any enabled rule hit → false |
| PoW | multiple allowed | one or more | `next` | algorithm, difficulty, session TTL, challenge TTL |

IP match, geo match, UA check, and security do not distinguish allowlist vs blocklist. `true` only means the request passed that node's judgment; `false` only means it did not; the business meaning of allow vs block is entirely determined by the wiring. UA check evaluation order: require UA → block crawlers/abnormal UAs → allowlist match. Security performs signature matching on the request Path/Query/Header/Cookie/Body (bounded). After PoW verification succeeds, execution continues along `next`; when incomplete, the challenge page takes over the request and no `false` branch is produced.

Loops, script nodes, arbitrary expression nodes, subgraph calls, and cross-rule jumps are not implemented.

## Control-Plane Architecture

The rule graph uses a dual model separating control-plane edit state and data-plane runtime state:

1. The React Flow editor submits a versioned graph JSON containing node IDs, node types, display names, coordinates, typed configs, and edges.
2. The Server performs authoritative validation of the whole graph; on success it saves the graph in a single transaction and increments the revision number.
3. On config release, the Server validates all enabled rules again, compiles the graph into a compact runtime DAG without UI fields (coordinates, labels, etc.), and collects the referenced IP group IDs.
4. The Agent atomically writes the full release snapshot and reloads OpenResty. New Workers only load and parse the rule JSON once at startup.
5. The request hot path only traverses the immutable in-memory runtime graph in the Worker — no file reads, no checksum computation, no JSON parsing.

The edit-state JSON uses an explicit `schema_version`. Node configs use per-node-type structures; unconstrained key-value objects bypassing Server validation are not allowed. Initial safety limits: 128 nodes, 256 edges, and 256 KiB of edit-state JSON per rule; these limits are enforced by both the API and the release compiler.

## Graph Structure Constraints

Rule save and release must satisfy all constraints:

* The graph is a DAG; self-loops and arbitrary cycles are forbidden.
* Exactly one start node and one pass node exist; multiple block nodes are allowed.
* The start node has no incoming edges and exactly one `next` outlet; pass and block nodes have no outlets.
* IP match, geo match, UA check, and security must each connect their `true` and `false` outlets once; PoW's `next` must connect once.
* No dangling outlets except terminal nodes; every non-start node has at least one incoming edge.
* All nodes must be reachable from start, and every executable node must be able to reach pass or block.
* Edge source ports must belong to the source node type; the same source port must not connect to multiple targets.
* Node IDs are unique within the graph, edge IDs are unique within the graph, and all referenced nodes must exist.
* Node configs must pass per-type field, range, reference-existence, and size validation.

The frontend provides instant validation and connection restrictions to improve UX, but the Server is the only authoritative validator. When deleting a node, the frontend synchronously removes related edges and marks the rule unsaved; saving is forbidden until the graph is legal again.

## Multi-Rule Execution Semantics

A route can bind multiple custom rules. The binding is an ordered list following this order:

1. Enabled global rules always execute first and do not participate in route-side ordering.
2. Enabled rules bound to the route execute in binding order.
3. When the current rule reaches a block node, it immediately outputs that node's configured response and terminates the request.
4. When the current rule reaches a pass node, that only means the current rule finished; if more rules remain, execution continues.
5. Only after all rules reach a pass node is the request truly allowed to proceed into the OpenResty/origin chain.

The runtime graph is fully validated before release. If the Lua executor still encounters an unknown node, unknown port, missing target, or exceeds the node-step limit, it logs a rate-limited error and blocks the request, preventing a broken security config from accidentally allowing traffic.

## IP Group Memory Refresh

Rule topology only takes effect on release + OpenResty reload; IP group membership can still be updated independently by manual, subscription, or auto tasks without release or reload.

IP groups use a two-level cache of coordinating worker, shared snapshot, and worker-local objects:

1. Requests always read the IP group object in the current worker's memory — no file access or shared-dict JSON parsing.
2. Every 5 seconds only one worker holding a shared lock reads the lightweight checksum file.
3. If the checksum is unchanged, it ends immediately without reading the full `waf_ip_groups.json`.
4. On checksum change, the coordinating worker reads and validates the full JSON once, writes the raw snapshot to a dedicated 64 MiB `ngx.shared.openflare_waf_ip_groups` keyed by checksum, then updates the commit pointer.
5. Other workers detecting a shared version change fetch the snapshot from shared memory, parse it, and atomically replace their local object — no repeated disk reads.
6. On refresh failure, keep using the previous valid object, log a rate-limited error, and retry next cycle.

The Agent must atomically replace the IP group JSON first, then atomically update the checksum last, so workers never recognize a half-written file as a new version. Server release/sync and Agent disk writes jointly enforce the 20 MiB aggregate snapshot limit; shared dict uses a safe write that never force-evicts old keys, keeping the current and previous immutable snapshots on failure.

## API and Editor

The create API only accepts a rule name and returns the rule detail with the default graph. Rule metadata, graph save, and route binding use separate operations, so toggling enabled state or binding does not overwrite the canvas.

The graph detail includes `revision`. Save requests submit `revision + graph`; the Server only updates and increments the revision when it matches. On mismatch it returns a conflict, the frontend prompts a reload, and silent overwrites of another page's changes are forbidden. The route binding API accepts an ordered array of rule IDs.

The React Flow editor page uses a full-width canvas and a fixed right property panel:

* Top bar: back, rule name, enabled state, validation state, and save.
* The canvas uses compact height with a smaller initial fit scale; zoom, pan, box-select, delete, auto-layout, MiniMap/Controls and other necessary navigation are supported. Node dragging is handled by React Flow's local controlled state in real time; coordinates are written back to the edit graph only after the drag ends.
* "Add processing unit" offers IP match, geo match, UA check, security, PoW, and block; start and pass are provided by the default graph and cannot be deleted or duplicated.
* Selecting a normal node or edge allows deletion via the canvas delete button or Delete/Backspace; deleting a node synchronously removes associated edges.
* The right property panel is hidden by default, shown only when a node is selected; it collapses when clicking an edge or blank canvas.
* Geo-match properties use the full country and ISO 3166-2 first-level administrative division data; country options show both localized names and codes; administrative divisions support search by country name, division name, or code to avoid rendering thousands of options at once.
* Leaving the page with unsaved changes must prompt; save conflicts and Server validation errors should locate the relevant node or edge.

The WAF list shows rule name, enabled state, node count, bound route count, and update time. The new-rule dialog only has the name field and navigates to the orchestration page immediately on success.

## Persistence and Migration

Rule records add a versioned graph JSON and a revision number; binding records add execution order. The graph is saved as a single aggregate (not split into node/edge tables) to keep edit operations transactional and let new node types avoid frequent DB schema extensions.

When upgrading existing installs:

* Keep rule names, global flags, enabled state, and route bindings.
* All rule graphs reset to `start → pass`; legacy IP/geo lists, PoW, or block-response configs are not migrated.
* Existing bindings are written into the order field in a stable sequence; global rules remain fixed in front.
* Once the new graph and runtime stabilize, remove the legacy rule fields, fixed-order compile logic, and old frontend forms — do not maintain dual executors long-term.

This migration stops the old protection config from taking effect; the upgrade notes must prominently tell admins to re-orchestrate rules before releasing the next version.

## Release, Failure, and Rollback

Rule graphs only take effect on config release. If validation or compilation fails before release, the release is refused and the current active version stays unchanged. If Agent write, OpenResty config check, or reload fails, the apply flow fails and restores the previous valid released version.

New workers only accept complete, parseable runtime rule configs. During an OpenResty graceful reload, old workers keep the old in-memory graph and new workers use the new graph, so requests never observe a half-updated state.

When the geo database is unavailable, geo match returns `false` with a rate-limited warning, preserving current behavior. On IP group refresh failure, the old in-memory snapshot is kept. An incomplete PoW is taken over by the challenge module, not treated as an execution error; PoW node config is first written to OpenResty shared memory with a short-lived key, then passed to the internal challenge handler via explicit `ngx.exec` parameters — it cannot rely on internal redirects preserving `ngx.ctx` or implicitly inherited request params. Empty rule bindings in the release snapshot must be encoded as JSON empty arrays; at runtime, legacy `null` optional arrays in old snapshots are treated as empty arrays, so `cjson`'s `ngx.null` userdata never breaks the request.
