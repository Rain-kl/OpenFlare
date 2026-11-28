# 参考手册与规格

系统参数、命令行工具、RESTful API 规范、性能实测与开源致谢索引，供部署运维、自动化集成与排查检索。

---

## 核心参考板块索引

| 参考板块 | 文档页面 | 涵盖内容与核心规格 |
| --- | --- | --- |
| **配置与参数** | [配置项清单](./configuration.md) | `config.yaml` 与环境变量映射、`w_system_configs` 运行时配置，以及 `openflare-edge`、`openflare-dns`、`openflare-relay`、`openflared` 参数与状态文件 |
| **命令行与脚本** | [命令与脚本参考](./cli.md) | `openflare-server`、`openflare` CLI、`openflare-edge`、`openflare-dns`、`openflare-relay`、`openflared` 命令以及 Makefile 构建与质量门禁 |
| **接口与协议** | [API 与协议参考](./api.md) | OpenAPI 2.0 / Swagger 规范 (`api/swagger/swagger.json`, `api/swagger/swagger.yaml`)、认证凭证体系、响应信封与核心路由分组 |
| **性能基准** | [OpenFlare Edge 性能实测](./pingora-benchmark.md) | 跨地域公网点对点实测数据、不同 WAF 模式吞吐/延迟损耗对比与多线程横向扩展评估 |
| **引用与致谢** | [引用与致谢](./credits.md) | 衍生核心组件、IP/ASN 情报数据源与主要开源依赖许可声明 |

---

## 关联设计与架构规范

底层实现机制与工程架构决策见设计文档：

- **系统整体架构**：[系统架构](../design/architecture.md)
- **OpenFlare Edge 边缘引擎**：[OpenFlare Edge 架构设计](../design/pingora-edge-design.md)
- **内网穿透与中继**：[内网穿透架构设计](../design/tunnel-design.md)
- **WAF 与防护引擎**：[WAF 可编排规则设计](../design/waf-orchestration-design.md)
- **工程目录结构**：[产品边界与仓库结构](../design/index.md)
