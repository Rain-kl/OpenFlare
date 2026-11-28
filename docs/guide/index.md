# 使用指南

按功能查阅 OpenFlare 控制台、CLI 与边缘节点的操作步骤。

## 入门与代理

| 指南 | 内容 |
| --- | --- |
| [快速开始](./quick-start.md) | 启动 Server，接入首个 `openflare-edge` 节点 |
| [配置反向代理](./proxy-config.md) | 配置源站、域名、证书、站点认证与缓存并验证同步 |
| [TLS 证书与自动续期](./certificates.md) | 导入 PEM 证书或通过 ACME DNS-01 自动签发续期 |
| [OpenFlare Edge 性能策略](./performance-policies.md) | 调整工作线程、回源连接池与缓存存储 |
| [SmartDNS 智能权威解析](./smartdns.md) | 配置 NS 委派、权威实例与调度策略 |

## 安全、集成与运维

| 指南 | 内容 |
| --- | --- |
| [WAF 安全防护](./waf-usage.md) | 编排 WAF 流程图、攻击防护组与全局白名单 |
| [PoW 防护](./pow-usage.md) | 为路由开启 PoW 验证并自定义挑战页 |
| [WAF 自动 IP 组语法](./waf-ip-group-expr.md) | 使用 Expr 表达式自动维护 IP 组 |
| [Pages 静态托管](./pages-usage.md) | 通过上传、Remote URL 或 GitHub Release 部署静态站点 |
| [内网穿透与隧道](./tunnel-usage.md) | 配置中继池、Relay 与 `openflared`，发布 L7 与 L4 服务 |
| [Uptime Kuma 监控同步](./uptime-kuma.md) | 将启用站点同步至 Uptime Kuma |
| [SSO 登录配置](./sso.md) | 接入 OIDC 第三方单点登录 |
| [网络工具与质量情报](./network-tools.md) | 运行多点探测、ASN 与 DNS 诊断及质量模拟 |
| [CLI 与 MCP](./automation.md) | 授权 `openflare` CLI 与 MCP 管理站点和节点 |
| [更换会话密钥](./session-secret-rotation.md) | 迁移加密凭据后轮换 `app.session_secret` |
| [故障排查](./troubleshooting.md) | 按现象排查 Server、登录、节点、HTTPS 与缓存问题 |

部署架构见 [安装部署](../deployment/index.md)，配置与接口见 [技术参考](../reference/index.md)，底层原理见 [架构设计](../design/index.md)。
