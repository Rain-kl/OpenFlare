# Reference Manual & Specifications

You will learn: what information serves as stable technical reference material, and how to quickly navigate configuration, command-line tooling, RESTful API specifications, performance benchmarks, and architecture models.

This section consolidates stable reference material across OpenFlare runtime environments, interface definitions, engineering structure, and performance specifications for deployment operations, development, and system integration.

---

## Core Reference Index

| Reference Section | Page | Topics and Key Specifications |
| --- | --- | --- |
| **Configuration** | [Configuration Reference](./configuration.md) | `config.yaml` base settings, environment variable overrides, `w_system_configs` runtime dynamic options, edge security limits, and Relay / OpenFlared daemon configurations |
| **CLI & Tooling** | [Commands & Scripts](./cli.md) | `openflare-server`, `openflare-edge`, `openflare-relay`, and `flared` binary subcommands and flags, Makefile build/test targets, and frontend linters |
| **API & Protocols** | [API & Protocols](./api.md) | OpenAPI 2.0 / Swagger definition files (`docs/swagger.json`, `docs/swagger.yaml`), Swagger UI interactive console, Access Token / Session authentication, response envelopes, and core endpoints |
| **Benchmarks** | [OpenFlare Edge Performance Benchmarks](./pingora-benchmark.md) | Direct cross-region benchmarking data, throughput/latency comparisons across full WAF, IP-filtering, and disabled WAF, and multi-threaded scaling results |

---

## Related Design & Architecture Standards

For deeper insights into the underlying architecture and engineering design rationale, refer to the design documentation:

- **System Topology**: [System Architecture](../design/architecture.md) (control-plane and data-plane decoupling)
- **OpenFlare Edge Engine**: [OpenFlare Edge Architecture](../design/pingora-edge-design.md) and [Local Edge Development](../design/pingora-edge-development.md)
- **Tunnel & Relay**: [Tunnel and Relay Design](../design/tunnel-design.md)
- **WAF Engine**: [WAF Architecture](../design/waf-design.md) and [Rule Orchestration](../design/waf-orchestration-design.md)
- **Repository Structure**: [Repository Structure and Monorepo Layering](../design/index.md#repository-structure)
