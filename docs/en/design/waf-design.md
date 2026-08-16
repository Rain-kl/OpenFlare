# WAF Design

OpenFlare's current WAF rule model is a visual DAG. Node semantics, graph constraints, multi-rule ordering, release compilation, and migration boundaries are all governed by [WAF Orchestration Rule Design](./waf-orchestration-design.md).

## System Boundaries

The Server stores the edit graph with coordinates and a revision number; at release it re-validates and compiles it into a compact runtime graph; the Agent atomically writes the snapshot and reloads OpenResty; the request hot path only traverses the immutable in-memory graph in the Worker.

IP groups update independently of rule topology. Manual, subscription, and auto IP groups are maintained by the control plane; the Agent atomically replaces the JSON first and updates the checksum last. A coordinating worker checks the checksum every 5 seconds, reading and distributing the full snapshot only on change; on failure it keeps the previous valid data. The full runtime snapshot is capped at 20 MiB; Server release/sync and Agent disk writes use the same serialization validation; OpenResty uses a separate 64 MiB shared dict with non-evicting writes, refusing new versions on capacity shortage without breaking committed snapshots.

Geo nodes use Country and City MMDB. Docker images bundle the database files; bare-binary installs have the Agent download missing files at first startup and update them periodically per config; request handling always reads the DB already loaded by OpenResty. When the DB is unavailable, geo match returns `false` with a rate-limited warning; other execution errors must not be accidentally allowed through due to data corruption.

## Security Ordering

Enabled global rules always run first; route rules execute by binding sequence. A block node terminates immediately; a pass node only ends the current rule; only after all rules pass does traffic enter the origin chain. Unknown nodes, missing outlets, or step-limit overruns always block the request.
