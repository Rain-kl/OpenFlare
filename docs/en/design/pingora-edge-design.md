# Pingora Edge Node Design

OpenFlare Edge uses Pingora to deliver HTTP, HTTPS, WAF, PoW, site authentication, rate limiting, and caching. The control plane automatically compiles site snapshots upon saving configuration, and edge nodes retrieve sites, WAF IP groups, and performance policies through a unified synchronization protocol.

## Configuration Synchronization

Edge nodes report runtime status and fetch configuration manifests via `POST /api/v1/agent/nodes/heartbeat`. When configuration changes are committed on the control plane, a WebSocket `config_changed` event wakes up connected nodes. When Redis is enabled, this event is also forwarded across control plane server processes. Network disconnections, reconnections, and dropped notifications are converged through periodic heartbeats.

| Resource Key | Data | Synchronization Method |
| --- | --- | --- |
| `performance.v1` | Performance parameters or default status of the current node | Inline JSON |
| `sites.v1` | Sites, certificates, WAF bindings, and response assets | Revision number and SHA-256, pulled on demand |
| `waf.ip-group.v1/<id>` | IP groups referenced by the active snapshot | Small objects inline, large objects pulled on demand |

Resource providers are registered in `internal/apps/openflare/agent/config_sync.go`. Each resource uses a stable, versioned name. Descriptors contain the exact SHA-256 digest of the JSON payload, optional inline content, and provider-specific metadata. Inline content is capped at 8 KiB per item and 32 KiB per manifest. Nodes ignore unrecognized resource keys.

On-demand pulls uniformly use `GET /api/v1/agent/config/pull?key=<key>&sha256=<digest>`. The control plane resolves resources based on node identity, returning HTTP 409 if the resource has been superseded. Nodes verify the content digest before applying. New resource types register a provider on the control plane and implement corresponding validation and application logic on the node. Registration, application status reporting, and Pages deployment package downloads maintain separate dedicated endpoints.

## Automatic Synchronization and Local Fallback

When sites, certificates, WAF rules, or global response configurations are saved, the control plane automatically compiles the latest committed data. When snapshot content changes, the control plane assigns a monotonically increasing internal revision number and notifies online nodes to synchronize immediately; background changes are also checked periodically by the control plane. When the last site rule is disabled or deleted, nodes apply an empty site snapshot and cease serving the previous site.

Nodes report both `latest_config_version` and `current_version` in heartbeats, representing the latest observed revision and the currently serving healthy snapshot, respectively. The control plane omits metadata already known to the node based on the latest revision number and continues returning site digests. Only after candidate snapshots pass digest, configuration, certificate, and Pages asset validation and are persisted to disk does the node atomically switch traffic in memory. If validation or application fails, the node logs the failure and continues serving the last known healthy snapshot.

Subsequent heartbeats skip pulling and applying the same failed revision. New revisions are attempted automatically. Node restarts or receiving a "Force Sync" instruction clear failure flags and trigger a retry. Transmission failures continue retrying in subsequent heartbeats. When disconnected from the control plane, nodes can start up directly using the persisted healthy snapshot.

## Performance Policies

Each Pingora node can be explicitly bound to a performance policy; remaining nodes inherit the built-in default policy. Automatic configuration tunes worker threads and idle upstream connection pool capacity based on available CPU cores and memory limits at node startup. Change notifications are published once database transactions for saving or batch assignments commit. Nodes persist received configurations; upstream connection pool settings and select cache parameters are applied via in-memory replacement. Upstream parameter updates isolate connections in the previous connection pool. Changes affecting listeners, worker threads, and runtime initialization trigger automatic reloads by the node.

## HTTP Response Caching

The edge proxy integrates TinyUFO in-memory storage and optional disk storage via [pingap-cache](https://github.com/vicanso/pingap/tree/a2c7975b1573afc8480efae58167bd0a362ed53c/pingap-cache). Disk mode reuses the TinyUFO hot layer and evicts/cleans files based on access timestamps. Cache keys are generated from the site snapshot digest, per-object valid cache limit, protocol, host, and full request URI; storage logs record only hash digests. Whether a request checks the cache, whether a response is shareable, and upstream request collapsing are determined by OpenFlare site rules and the Pingora cache pipeline.

Disk capacity and I/O limits belong to node startup configuration and are synchronized downstream via performance policies. See [Edge Cache Strategy Design](./edge-cache-design.md).

## Edge Node Public IP Reporting

Edge nodes prioritize scanning local network interfaces at startup, excluding all private and reserved CIDR ranges (including `0.0.0.0/8`, `10.0.0.0/8`, `100.64.0.0/10`, `127.0.0.0/8`, `169.254.0.0/16`, `172.16.0.0/12`, `192.0.0.0/24`, `192.0.2.0/24`, `192.88.99.0/24`, `192.168.0.0/16`, `198.18.0.0/15`, `198.51.100.0/24`, `203.0.113.0/24`, `224.0.0.0/4`, `240.0.0.0/4`, and corresponding IPv6 reserved blocks). If no public IP is found on local interfaces, the node queries `https://api.ip.sb/ip`, `https://ip.me`, `https://api.ip2location.io/`, and control plane `GET /api/geoip` sequentially at startup and once every minute. All probe results are validated for public routability. If all probes fail or return private/reserved addresses, the node falls back to reporting `0.0.0.0`. The control plane rejects persisting unlocked node IPs that fall within private or reserved address spaces.
