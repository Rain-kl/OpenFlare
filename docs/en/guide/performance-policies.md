# OpenFlare Edge Performance Policies

In the admin console, navigate to "Performance Policies" to create and manage policies for edge nodes with different hardware specifications or roles. New policies are pre-filled with sensible defaults, and the form provides parameter descriptions, units, and allowed value ranges.

1. Click "New Policy", enter a name, and tune runtime, origin connection, HTTP/2, TLS thread pool, listener, and caching parameters.
2. After saving, click "Assign Nodes" to search and batch-select nodes by name or node ID.
3. Save assignments. Each node uses exactly one policy. Reassigning updates the node to the new policy. Deleting or unassigning a policy reverts the node to the default policy.

Nodes without an explicit assignment automatically use the "Default Policy" at the top of the list, including newly added nodes. The default policy can be edited and starts with "Auto Config" enabled.

Three preconfigured profiles serve as starting points for common server tiers:

| Specification | Workers | Idle Origin Pool | Downstream HTTP/2 Streams | In-Memory Cache |
| --- | ---: | ---: | ---: | ---: |
| 1C1G | 1 | 64 | 64 | 32 MiB |
| 2C2G | 2 | 128 | 128 | 64 MiB |
| 4C8G | 4 | 256 | 256 | 256 MiB |

When "Auto Config" is enabled, edge nodes calculate worker threads and idle origin pool capacity at startup based on available CPU cores and memory limits. Linux nodes inspect standard cgroup mount memory caps. When disabled, these parameters can be configured manually.

Auto mode reserves an estimated budget of 768 MiB per worker thread, keeping at least one worker. The origin connection pool is constrained by both thread count and memory. For instance, detecting 1 core/1 GiB, 2 cores/2 GiB, or 4 cores/8 GiB configures 1, 2, or 4 workers with idle origin pools of 64, 128, and 256 per worker, respectively. Node startup logs report the exact applied values.

Upon saving a policy, the control plane immediately notifies connected nodes. Origin timeouts, TLS validation, upstream HTTP/2, and specific cache parameters take effect dynamically for new requests. When startup parameters change (thread counts, pool capacity, downstream HTTP/2 limits, listeners, or total cache capacity), the node drains active connections and restarts workers gracefully. Offline nodes receive current assignments upon reconnection.

New policies disable certificate and hostname verification for HTTPS origins by default; enable them in the Upstream group if strict origin TLS verification is required. When configuring caching, corresponding rules must also be enabled under site details; the node cache toggle controls whether these rules take effect on the node.

---

## Cache Storage

Configure storage capacity and disk concurrency on the "Cache" tab of a performance policy. After enabling "Node Cache", ensure caching rules are enabled under site details.

| Parameter | Default | Description |
| --- | ---: | --- |
| Memory Cache Capacity | 128 MiB | TinyUFO in-memory cache capacity. When disk cache is enabled, holds frequently accessed small objects |
| Disk Cache Capacity | 0 MiB | Set a positive value to enable disk caching with LRU eviction |
| Disk Cache Inactive TTL | 172800 s | Periodically cleans unaccessed files older than this threshold (48h) |
| Disk Concurrent Read Cap | 32 | Cache misses fetch from origin when reaching this concurrency cap |
| Disk Concurrent Write Cap | 16 | Responses skip disk writing when reaching this write concurrency cap |

Disk cache is stored under `http-cache` within the node's state directory, typically `/var/lib/openflare-edge/http-cache`. Valid disk cache survives node restarts. When site snapshots change, new requests use updated cache keys. Modifying storage parameters triggers an automatic node reload.

Disk cache and in-memory tiers are powered by [pingap-cache](https://github.com/vicanso/pingap/tree/a2c7975b1573afc8480efae58167bd0a362ed53c/pingap-cache).

---

## Access Log Ingestion

OpenFlare Edge queues access logs (including WAF audit records) in an in-memory ring buffer, flushed by a background thread every 256 entries or 100 milliseconds to `access-logs.jsonl` in the node state directory. Client requests never block on disk I/O. Node heartbeats read up to 1,024 records per interval, pruning acknowledged records after the control plane confirms ingestion.
