# Network Tools

Inspect IP intelligence, dispatch active HTTP and ICMP ping probes from edge nodes, and group ASN/ISP attributes for SmartDNS routing and WAF rules.

## Console Features

| Page | Purpose |
| :--- | :--- |
| **GeoIP Lookup** | Query country, city, ASN, and BGP prefix attribution for public IPs |
| **HTTP Ping / Ping Probe** | Run ad-hoc edge probes against domains or IPs to compare status codes and latency across PoPs |
| **Node Latency Matrix** | Inspect scheduled ping histories per node to identify high-latency or packet-loss paths |
| **Network Groups** | Bundle ASN, ISP, or regional matchers referenced directly by WAF condition nodes |
| **System Settings > Network Quality** | Configure GeoIP feeds, BGP/CAIDA enrichment toggles, and auto-probe intervals (raw keys in [Configuration Reference](../reference/configuration.md)) |

## Typical Workflow

1. Verify GeoIP enrichment is enabled and synced under **System Settings > Network Quality**.
2. Create recurring targets under **Ping Probe** and monitor node trends in the **Node Latency Matrix**.
3. Define **Network Groups** for WAF routing branches or feed node quality signals into SmartDNS policies.

See [Network Quality Assessment & Probing Design](../design/network-quality-design.md) for collection internals.
