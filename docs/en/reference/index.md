# Reference Manual & Specifications

Authoritative index of system configuration, CLI tooling, RESTful APIs, performance benchmarks, and third-party acknowledgments.

---

## Core Reference Index

| Reference Section | Page | Topics and Key Specifications |
| --- | --- | --- |
| **Configuration** | [Configuration Reference](./configuration.md) | `config.yaml` and environment overrides, `w_system_configs` runtime keys, and parameters/state files for `openflare-edge`, `openflare-dns`, `openflare-relay`, and `openflared` |
| **CLI & Tooling** | [Commands & Scripts](./cli.md) | Subcommands and flags for `openflare-server`, `openflare` CLI, `openflare-edge`, `openflare-dns`, `openflare-relay`, and `openflared`, plus Makefile build and quality targets |
| **API & Protocols** | [API & Protocols](./api.md) | OpenAPI 2.0 / Swagger specs (`api/swagger/swagger.json`, `api/swagger/swagger.yaml`), authentication schemes, response envelopes, and route groups |
| **Benchmarks** | [OpenFlare Edge Performance Benchmarks](./pingora-benchmark.md) | Cross-region benchmarking data, throughput/latency comparisons across WAF modes, and multi-threaded scaling |
| **Acknowledgments** | [Acknowledgments & Third-Party Notices](./credits.md) | Core derived software, IP/ASN intelligence sources, and third-party license notices |

---

## Related Design & Architecture Standards

For internal mechanisms and architectural rationale, refer to the design documentation:

- **System Topology**: [System Architecture](../design/architecture.md)
- **OpenFlare Edge Engine**: [OpenFlare Edge Architecture Design](../design/pingora-edge-design.md)
- **Tunnel & Relay**: [Tunnel Architecture Design](../design/tunnel-design.md)
- **WAF Engine**: [WAF Orchestration Rule Design](../design/waf-orchestration-design.md)
- **Repository Structure**: [Product Boundaries & Repository Structure](../design/index.md)
