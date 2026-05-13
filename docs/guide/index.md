# 指南

本部分面向使用者和部署者，帮助你把 OpenFlare 从首次启动推进到第一份可运行的代理配置。

推荐阅读顺序：

1. [快速开始](./quick-start.md)：用 Docker Compose 启动 Server，并完成首次登录。
2. [部署说明](./deployment.md)：查看生产部署、Agent 一键安装、联调与升级。
3. [SSO 登录配置](./sso.md)：配置 GitHub OAuth 或标准 OIDC 登录入口。
4. [启动 Server](./server.md)：了解源码启动、前端构建和 Swagger 入口。
5. [接入 Agent](./agent.md)：选择 `agent_token` 或 `discovery_token`，让节点上线。
6. [发布第一份配置](./first-site.md)：创建网站配置，发布并确认节点应用。
7. [升级与维护](./upgrade.md)：了解升级、卸载、验证和日常维护入口。

如果你要参与开发，先阅读 [设计](../design/) 与 [开发约束](../design/development.md)，再进入代码修改。
