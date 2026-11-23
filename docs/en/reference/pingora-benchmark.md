# OpenFlare Edge Performance Benchmarks

On November 20, 2026, US-A connected directly to isolated OpenFlare Edge `0.1.8` instances on JP-B and US-C to measure WAF overhead and worker scaling with in-memory access logs.

## Test Environment and Performance Policy

| Label | Role | CPU | Effective OpenFlare Edge worker threads | Effective idle upstream connection pool (per worker) |
| --- | --- | --- | ---: | ---: |
| US-A | Load generator | 1 vCPU | — | — |
| JP-B | Edge node | 3 vCPUs, ARM64 | 3 | 192 |
| US-C | Edge node | 1 vCPU, x86-64 | 1 | 64 |

The WAF comparison enabled `threads_auto`, which overrode the manual settings of 1 worker and 128 idle upstream connections per worker. The effective values above come from the benchmark instances' startup logs. The scaling tests disabled automatic sizing and set the worker counts manually.

| Category | Settings shared by both Edge nodes |
| --- | --- |
| Runtime | Work stealing enabled; 1 listener task per file descriptor; alternate timer disabled |
| Downstream HTTP/2 | Negotiated over TLS; h2c disabled; up to 100 concurrent streams per connection; 1 MiB initial stream and connection windows; 16 KiB maximum frame, 32 KiB maximum header list, 4 KiB header table, and 256 KiB maximum send buffer |
| Listeners and reuse | TLS offload pools, `SO_REUSEPORT`, TCP Fast Open, and listener TCP keepalive disabled; OS-default TCP send and receive buffers; no explicit keep-alive request cap or downstream HTTP/2 idle timeout |
| Reverse proxy cache | Node cache enabled; 128 MiB memory, 512 MiB disk, and 16 MiB maximum object; disk read/write concurrency limits of 32 / 16 and a 48-hour inactivity threshold; 5-second cache lock timeout and one lock retry |
| Upstreams | HTTP/1.1; connection, total connection, read, write, and idle timeouts of 3 / 8 / 30 / 30 / 60 seconds, respectively; up to three retries; origin certificate and hostname verification enabled |

The client used `h2load` (nghttp2 `1.68.0`) to connect directly to each instance IP over a peer-to-peer private network, bypassing DNS resolution and Cloudflare. The HTTP `Host` header and TLS SNI selected the Pages site and certificate. With site rate limiting disabled, local Pages served the same 923 B file on both nodes, without using the reverse proxy cache or an upstream service.

The isolated instances kept their state directories on the in-memory filesystem `tmpfs`. The tested version still wrote and called `sync_data()` for each request. The [new asynchronous log writer](../guide/troubleshooting.md#access-analytics-empty) has not been benchmarked on these nodes.

## WAF Comparison

The full-WAF variant enabled global IP matching and the Balanced attack detection profile, with no additional site-specific rule groups. The WAF-off variant removed all WAF rule groups. Each run used 16 connections with up to 24 in-flight HTTP/2 streams per connection, a 2-second warm-up, and a 6-second measurement window. Throughput is the total number of HTTP `200` responses received across two runs divided by 12 seconds.

| Node | Workers | Full WAF (requests/s) | WAF off (requests/s) | Throughput reduction with full WAF |
| --- | ---: | ---: | ---: | ---: |
| JP-B | 3 | 1,158.0 | 2,990.3 | 61.3% |
| US-C | 1 | 717.8 | 2,169.2 | 66.9% |

## Worker Scaling and Peak Retest

With WAF disabled and `tmpfs` logs, each run warmed up for 3 seconds and measured for 22 seconds. Throughput and latency include only received HTTP `200` responses.

| Node | Workers | Connections × streams | HTTP `200` responses | Throughput (requests/s) | P95 |
| --- | ---: | ---: | ---: | ---: | ---: |
| JP-B | 6 | 32 × 24 | 86,851 | 3,947.8 | 304.0 ms |
| US-C | 2 | 16 × 24 | 50,009 | 2,273.1 | 306.3 ms |

In matched 6-second runs at 32 × 24, increasing JP-B from 3 to 6 workers raised throughput from 3,421.2 to 3,709.3 requests/s (+8.4%). Increasing US-C from 1 to 2 workers raised it from 1,967.8 to 2,176.2 requests/s (+10.6%). The 22-second retest confirmed the observed peak range; it does not estimate sustained capacity.
