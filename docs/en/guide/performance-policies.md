# OpenFlare Edge performance policies

Configure worker threads, upstream connection pools, HTTP/2, TLS, and cache storage for edge nodes under **Performance policies**.

## Create and assign a policy

1. Open **Performance policies**, click **Create Policy** (or start from `1C1G`, `2C2G`, or `4C8G` presets), tune settings, and save.
2. Click **Assign Nodes**, select target nodes, and save; unassigned nodes inherit the top **Default policy**.

With **Auto-configure** enabled, nodes size worker threads (`768 MiB` budget per thread, minimum `1`) and idle upstream pools from CPU/memory limits (including cgroups) on startup; disable it to set manual values.

Upstream timeouts, TLS verification, upstream HTTP/2, and cache toggles hot-reload for new connections; changing thread counts, pool capacities, listeners, or cache caps drains active connections and restarts workers.

## Cache storage

Enable **Node Cache** on the policy's **Cache** tab (site routes must also enable caching individually):

| Setting | Default | Description |
| --- | ---: | --- |
| Memory cache capacity | `128 MiB` | TinyUFO memory limit; acts as hot tier when disk cache is enabled |
| Disk cache capacity | `0 MiB` | Positive values enable disk cache under `$STATE_DIR/http-cache` |
| Disk cleanup interval | `172800` s | Removes disk objects inactive beyond this window |
| Disk read / write concurrency | `32` / `16` | Misses bypass to origin on read saturation; responses skip disk write on write saturation |

Disk cache initialization failures automatically fall back to memory caching; see [OpenFlare Edge Architecture Design](../design/pingora-edge-design.md).
