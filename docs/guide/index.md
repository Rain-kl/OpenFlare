# 指南

你会学到：OpenFlare 文档如何组织、首次运行应该读哪些页面，以及部署、使用、排查和开发分别从哪里开始。

OpenFlare 是一套自托管的 OpenResty 控制面。它把反向代理网站配置、配置版本发布、Agent 节点同步、TLS 证书和基础观测放到一个管理端中，适合单团队或单组织管理多台代理节点。

## 推荐阅读路径

如果你第一次接触 OpenFlare，按下面顺序阅读：

1. [快速开始](./quick-start.md)：用 Docker Compose 启动 Server，登录管理端，并接入第一个 Agent。
2. [发布第一份配置](./first-site.md)：快速新建一条最基础的 HTTP 反代站点规则，并验证节点生效状态。
3. [新建反代配置](./proxy-config.md)：一步一步了解如何从证书导入与申请开始，配置 HTTPS 加密、上游源站与边缘缓存。
4. [Zone 域名迁移](./zone-domain-migration.md)：从旧托管域名/路由内嵌域名升级到 Zone 模型（goose 自动导入），含备份、验收与回滚说明。
5. [Pages 静态托管使用](./pages-usage.md)：了解静态项目 ZIP 上传限制、SPA Fallback、以及内置 API 反向代理配置。
6. [内网穿透与隧道使用](./tunnel-usage.md)：部署 Relay 与 Client，实现安全、无公网 IP 反向穿透。
7. [WAF 安全防护使用](./waf-usage.md)：配置 WAF 规则组，掌握 IP 黑白名单、自动/订阅 IP 组、地域限制与 PoW CC 防护。
8. [WAF 自动 IP 组语法](./waf-ip-group-expr.md)：编写自动 IP 组 Expr 规则，了解关键字含义和预设规则。
9. [Uptime Kuma 监控同步](./uptime-kuma.md)：配置并使用 Uptime Kuma 自动差分同步和监控范围控制。
10. [SSO 登录配置](./sso.md)：配置 GitHub 或 OIDC 实现第三方单点登录 (SSO) 接入。
11. [故障排查](./troubleshooting.md)：按症状排查登录、数据库、节点同步、OpenResty、边缘缓存命中与前端构建问题。
12. [引用与致谢](./credits.md)：查看系统依赖的优秀开源项目与社区致谢清单。

## 按角色查找

| 你想做什么 | 推荐入口 |
| --- | --- |
| 5 分钟内跑起管理端 | [快速开始](./quick-start.md) |
| 发布第一条反向代理配置 | [发布第一份配置](./first-site.md) |
| 配置域名证书、反代与边缘缓存 | [新建反代配置](./proxy-config.md)（含缓存说明） |
| 静态资源不命中缓存 | [故障排查 · 边缘缓存](./troubleshooting.md#边缘缓存命中率异常) |
| 托管单页应用或静态网站 | [Pages 静态托管使用](./pages-usage.md) |
| 配置内网穿透映射 | [内网穿透与隧道使用](./tunnel-usage.md) |
| 配置防 CC 与 IP 组拦截 | [WAF 安全防护使用](./waf-usage.md) |
| 编写自动 IP 组规则 | [WAF 自动 IP 组语法](./waf-ip-group-expr.md) |
| 自动同步监测站点状态 | [Uptime Kuma 监控同步](./uptime-kuma.md) |
| 接入或重装节点 Agent | [接入 Agent](../deployment/agent.md) |
| 从源码启动 Server | [启动 Server](../deployment/server.md) |
| 配置 GitHub 或 OIDC 登录 | [SSO 登录配置](./sso.md) |
| 升级 Server 或 Agent | [升级与维护](../deployment/upgrade.md) |
| 理解架构和发布模型 | [系统架构](../design/architecture.md) 与 [Agent 与发布模型](../design/agent-design.md) |
| 查看开源引用与致谢 | [引用与致谢](./credits.md) |

## 文档分区

`guide/` 面向使用者和部署者，提供从安装到日常操作的可执行步骤。

`reference/` 收敛稳定事实，例如配置字段、命令、API 响应约定和仓库结构。

`design/` 面向维护者和贡献者，描述产品边界、系统架构、Agent 与发布模型和工程约束。新增能力或改变边界前，应先更新对应设计文档。
