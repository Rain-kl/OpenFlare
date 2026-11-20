# Pingora Performance Benchmarks

On November 20, 2026, US-A tested two OpenFlare Edge `0.1.8` nodes: JP-B and US-C. Public-IP tests measured request latency and download throughput. Separate tests against isolated Edge instances measured the throughput and CPU cost of WAF inspection.

## Test Environment and Performance Policy

| Label | Role | CPU | Effective Pingora worker threads | Effective idle upstream connection pool (per worker) |
| --- | --- | --- | ---: | ---: |
| US-A | Load generator | 1 vCPU | — | — |
| JP-B | Edge node | 3 vCPUs, ARM64 | 3 | 192 |
| US-C | Edge node | 1 vCPU, x86-64 | 1 | 64 |

Both Edge nodes used the same policy with `threads_auto` enabled. Automatic sizing overrode the manual policy values of 1 worker and 128 idle upstream connections per worker. The effective values above come from the benchmark instances' startup logs.

| Category | Settings shared by both Edge nodes |
| --- | --- |
| Runtime | Work stealing enabled; 1 listener task per file descriptor; alternate timer disabled |
| Downstream HTTP/2 | Negotiated over TLS; h2c disabled; up to 100 concurrent streams per connection; 1 MiB initial stream and connection windows; 16 KiB maximum frame, 32 KiB maximum header list, 4 KiB header table, and 256 KiB maximum send buffer |
| Listeners and reuse | TLS offload pools, `SO_REUSEPORT`, TCP Fast Open, and listener TCP keepalive disabled; OS-default TCP send and receive buffers; no explicit keep-alive request cap or downstream HTTP/2 idle timeout |
| Reverse proxy cache | Node cache enabled; 128 MiB memory, 512 MiB disk, and 16 MiB maximum object; disk read/write concurrency limits of 32 / 16 and a 48-hour inactivity threshold; 5-second cache lock timeout and one lock retry |
| Upstreams | HTTP/1.1; connection, total connection, read, write, and idle timeouts of 3 / 8 / 30 / 30 / 60 seconds, respectively; up to three retries; origin certificate and hostname verification enabled |

Local Pages served the test assets directly, without using the reverse proxy cache or an upstream service. The public-IP latency and download baselines used full WAF and a site limit of 50 requests/s per IP. The isolated WAF comparison disabled that rate limit while retaining the same performance policy and access logging.

## Direct Public-IP Tests

The client connected directly to each node's public IP, bypassing DNS resolution and Cloudflare. The HTTP `Host` header and TLS SNI selected the Pages site and its certificate. Both nodes served the same 923 B static file for request tests and a local 30 MiB Pages file for downloads, with no upstream requests.

| Metric | JP-B | US-C |
| --- | ---: | ---: |
| Mean ICMP RTT (5 probes) | 115.7 ms | 1.0 ms |
| HTTP/1.1 keep-alive P50 / P95 (40 requests/s) | 117.3 / 121.4 ms | 3.5 / 4.9 ms |
| HTTPS / HTTP/1.1 keep-alive P50 / P95 (40 requests/s) | 117.7 / 120.2 ms | 3.8 / 6.9 ms |
| HTTPS / HTTP/1.1 new-connection P50 / P95 (5 requests/s) | 354.3 / 363.9 ms | 10.7 / 14.2 ms |
| HTTPS / HTTP/2 30 MiB download, median end-to-end throughput over three runs | 111 Mbps | 775 Mbps |
| Same download, median throughput after the first byte | 131 Mbps | 826 Mbps |

Request latency is measured through receipt of the full response. The keep-alive tests used 12 connections each, completing all 560 HTTP requests and 640 HTTPS requests per node. The new-connection HTTPS test scheduled 60 requests across five concurrent lanes. New connections include TCP and TLS setup; the initial keep-alive requests also include connection setup. Download tests used a single connection; end-to-end throughput is file size in bits divided by total elapsed time.

## WAF Comparison

The isolated instances used each node's deployed Edge binary. The load generator connected directly to the instance IPs over a peer-to-peer private network. All three variants served the same Pages file, and the test site had no additional site-specific WAF rule groups bound to it.

| Variant | Active policy |
| --- | --- |
| Full WAF | Global IP matching and the Balanced attack detection profile |
| IP matching only | Global IP matching, with the attack detection step removed |
| WAF disabled | All WAF rule groups removed from the test instance |

US-A used `h2load` (nghttp2 `1.68.0`) with a 2-second warm-up and a 6-second measurement window. The main comparison used eight connections with up to 16 in-flight HTTP/2 streams each. The variants ran once in forward order and once in reverse order; throughput is the number of HTTP `200` responses received across both runs divided by 12 seconds. One additional run used 16 connections with 24 streams each to test whether throughput continued to rise. The fixed-load test targeted 20 requests/s per connection, or 160 requests/s total.

| Node and metric | Full WAF | IP matching only | WAF disabled |
| --- | ---: | ---: | ---: |
| JP-B: successful throughput, 8 × 16 | 419.5 requests/s | 486.1 requests/s | 488.0 requests/s |
| JP-B: successful throughput, 16 × 24 | 415.8 requests/s | 445.0 requests/s | 489.8 requests/s |
| JP-B: P95 at 160 requests/s | 126.9 ms | 123.4 ms | 120.5 ms |
| JP-B: Edge CPU at 160 requests/s | 32.2% | 12.1% | 10.2% |
| US-C: successful throughput, 8 × 16 | 273.8 requests/s | 255.7 requests/s | 283.1 requests/s |
| US-C: successful throughput, 16 × 24 | 280.8 requests/s | 261.3 requests/s | 255.8 requests/s |
| US-C: P95 at 160 requests/s | 21.9 ms | 5.0 ms | 5.8 ms |
| US-C: Edge CPU at 160 requests/s | 25.7% | 9.4% | 8.7% |

CPU figures average 2-second samples of the service cgroup during the fixed-load test, with 100% representing one fully utilized vCPU. Every fixed-load variant returned 960 HTTP `200` responses on each node. Latency percentiles include only HTTP `200` responses.

::: details HTTP 200 response counts by round at 8 × 16
| Node and round | Full WAF | IP matching only | WAF disabled |
| --- | ---: | ---: | ---: |
| JP-B, round 1 | 2,546 | 2,895 | 2,871 |
| JP-B, round 2 | 2,488 | 2,938 | 2,985 |
| US-C, round 1 | 1,671 | 1,516 | 1,759 |
| US-C, round 2 | 1,615 | 1,552 | 1,638 |
:::

## Interpretation

- **Attack detection accounted for most of the additional WAF cost.** At 160 requests/s, full WAF used about three times as much Edge CPU as the disabled variant on both nodes. IP matching alone added about 1.9 and 0.7 CPU percentage points, respectively.
- **JP-B showed a repeatable throughput reduction.** Full WAF served about 14% fewer successful requests than the disabled variant at 8 × 16. With WAF disabled, increasing concurrency to 16 × 24 kept throughput around 490 requests/s while P95 rose from 334–373 ms in the two 8 × 16 runs to 1,045 ms.
- **US-C did not show a consistent throughput difference.** Full WAF was about 3% slower at 8 × 16, but the ordering reversed in the single 16 × 24 run. At fixed load, full WAF clearly increased CPU use and latency.
- **Access log persistence is the next variable to isolate.** All three variants retained the per-request log lock and `sync_data()` call. The observed throughput plateau includes that cost; a separate logging comparison is needed to quantify it.
