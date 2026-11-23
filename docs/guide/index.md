# 指南概览

OpenFlare 是基于 OpenFlare Edge 边缘架构的分布式反向代理与边缘安全网关。平台提供多节点集群编排、配置快照热同步、边缘 WAF、PoW 防 CC 人机验证、内网穿透（Tunnels）与 Pages 静态托管能力，支持独立运行或与上层 CDN 协同防御。

指南文档按使用流程划分为五个模块，帮助使用者完成从系统初始化、日常流量调度到安全防御与故障定位的全流程操作。

---

## 指南模块索引

### 入门指引 (Getting Started)
适合首次部署或初次接入边缘网关的使用者：
- [快速开始](./quick-start.md)：使用 Docker Compose 部署 Server 控制端、完成首次登录、获取凭证并接入第一个边缘节点。
- [创建第一条反向代理规则](./first-site.md)：以最精简的 HTTP 方式创建首个代理站点并验证节点同步状态。
- [首页地理分布](./dashboard-map.md)：了解控制台全球态势图、2D/3D 地图切换机制、指标统计口径与国家请求分布。

### 流量代理与加速 (Traffic & Routing)
配置核心反向代理路由、TLS 加密与边缘缓存策略：
- [新建反代配置](./proxy-config.md)：配置上游源站、域名路由绑定、通配符域名匹配、IP HTTPS 回源与访问认证。
- [TLS 证书与自动续期](./certificates.md)：手动导入已有证书或通过 ACME 协议与 Cloudflare DNS API 实现通配符证书自动申请与续签。
- [OpenFlare Edge 性能策略](./performance-policies.md)：配置节点工作线程数、回源连接池、HTTP/2 并发流、TinyUFO 内存缓存与磁盘缓存。
- [Zone 域名迁移](./zone-domain-migration.md)：了解从旧版域名模型到 Zone 根域模型的自动迁移机制、备份要求与升级验收步骤。

### 边缘安全防护 (Edge Security)
配置边缘安全防火墙与人机验证挑战：
- [WAF 安全防护使用](./waf-usage.md)：使用可视化 DAG 编排 WAF 规则流，配置 IP 匹配、地域拦截与 OWASP CRS 攻击防护规则组。
- [PoW 防护](./pow-usage.md)：为指定路由配置客户端工作量证明（PoW）防 CC 挑战，管理触发路径与自定义验证界面。
- [WAF 自动 IP 组语法](./waf-ip-group-expr.md)：编写基于 Expr 表达式的动态 IP 聚合与自动封禁规则，支持自定义状态码与错误率分析。

### 应用与集成 (Apps & Integrations)
扩展网关能力，接入静态站点托管、内网穿透与监控：
- [Pages 静态托管使用](./pages-usage.md)：部署预构建静态应用归档，配置 SPA Fallback 与 API 路径反向代理，管理版本发布与回滚。
- [内网穿透与隧道使用](./tunnel-usage.md)：部署 Relay 中继节点与 OpenFlared 客户端，为无公网 IP 的内网服务提供反向穿透与网关安全防护。
- [Uptime Kuma 监控同步](./uptime-kuma.md)：配置 Uptime Kuma 自动化差分同步，自动创建、更新与清理代理站点的可用性监控项。
- [SSO 登录配置](./sso.md)：配置标准 OIDC 认证源，接入第三方单点登录，支持账号自动绑定与权限管理。

### 运维支持 (Operations)
日常运维、故障定位与开源依赖参考：
- [故障排查](./troubleshooting.md)：按症状排查控制面启动、数据库连接、Agent 注册、OpenFlare Edge 配置应用与缓存命中异常。
- [开源致谢](./credits.md)：查看系统核心开源项目、衍生代码来源、第三方组件与许可证声明。

---

## 场景快速跳转

| 操作目标 | 推荐文档 |
| --- | --- |
| 启动控制端并接入首个节点 | [快速开始](./quick-start.md) |
| 发布基础 HTTP 代理测试规则 | [创建第一条反向代理规则](./first-site.md) |
| 为域名配置 HTTPS 证书与源站反代 | [新建反代配置](./proxy-config.md) 与 [TLS 证书与自动续期](./certificates.md) |
| 调整边缘节点线程、连接池与缓存容量 | [OpenFlare Edge 性能策略](./performance-policies.md) |
| 开启 WAF 拦截规则与防 CC 人机验证 | [WAF 安全防护使用](./waf-usage.md) 与 [PoW 防护](./pow-usage.md) |
| 托管单页前端应用并配置 API 转发 | [Pages 静态托管使用](./pages-usage.md) |
| 将内网服务器服务映射到公网 | [内网穿透与隧道使用](./tunnel-usage.md) |
| 同步站点可用性监控任务 | [Uptime Kuma 监控同步](./uptime-kuma.md) |
| 接入企业 OIDC / OAuth2 登录 | [SSO 登录配置](./sso.md) |
| 节点未同步配置或缓存未命中 | [故障排查](./troubleshooting.md) |

---

## 文档架构关系

- [安装部署](../deployment/index.md)：选择部署方式、安装组件与升级节点。
- [技术参考](../reference/index.md)：查询配置项、命令行、接口协议与性能基准。
- [架构设计](../design/index.md)：了解系统架构、数据模型、状态同步与安全机制。
- [更新日志](../changelog/index.md)：查看版本变更与升级注意事项。
