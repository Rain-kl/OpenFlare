# 参考手册与规格

你会学到：哪些信息属于稳定技术参考资料，以及如何快速定位配置、命令行工具、RESTful API 规范、性能基准和架构模型。

本部分收敛 OpenFlare 运行环境、接口定义、工程结构与基准规格层面的稳定参考信息，供部署运维、二次开发与集成排查时系统性查阅。

---

## 核心参考板块索引

| 参考板块 | 文档页面 | 涵盖内容与核心规格 |
| --- | --- | --- |
| **配置与参数** | [配置项清单](./configuration.md) | `config.yaml` 基础配置、环境变量映射覆盖、`w_system_configs` 数据库运行时热更参数、边缘网关安全限流以及 Relay / OpenFlared 守护进程配置 |
| **命令行与脚本** | [命令与脚本参考](./cli.md) | `openflare-server`、`openflare-edge`、`openflare-relay`、`flared` 二进制命令行子命令与参数、Makefile 构建与测试指令以及前端代码检查工具 |
| **接口与协议** | [API 与协议参考](./api.md) | OpenAPI 2.0 / Swagger 规范源文件 (`docs/swagger.json`, `docs/swagger.yaml`)、Swagger UI 交互控制台、Access Token / Session 认证机制、统一响应信封结构以及核心路由分组 |
| **性能基准** | [Pingora 性能实测](./pingora-benchmark.md) | 跨地域公网点对点直连压测数据、Pingora 架构下完整 WAF、仅 IP 过滤与关闭 WAF 的吞吐/延迟损耗对比及多线程横向扩展评估 |

---

## 关联设计与开发规范

除运行时规格与接口参考外，涉及底层实现机制与工程设计决策的内容可进一步查阅设计板块：

- **系统整体架构**：[整体架构设计](../design/architecture.md)（控制面与数据面解耦拓扑）
- **Pingora 边缘引擎**：[Pingora Edge 架构设计](../design/pingora-edge-design.md)与[本地开发指南](../design/pingora-edge-development.md)
- **内网穿透与中继**：[Tunnel & Relay 隧道设计](../design/tunnel-design.md)
- **WAF 与防护引擎**：[WAF 架构设计](../design/waf-design.md)与[规则编排模型](../design/waf-orchestration-design.md)
- **工程目录结构**：[代码仓库结构与工程规范](../design/index.md#仓库结构)
